// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/server"
)

// deadlineRefusingRecorder is a recorder whose read deadline cannot be moved.
type deadlineRefusingRecorder struct{ *httptest.ResponseRecorder }

// SetReadDeadline reports the failure.
func (deadlineRefusingRecorder) SetReadDeadline(time.Time) error {
	return errors.New("the connection is gone")
}

// removingLibrary is a media library that removes whatever it is given.
type removingLibrary struct{}

// Cap returns the bytes one upload may carry.
func (removingLibrary) Cap() int64 { return 1 << 20 }

// Ingest stores nothing and answers an empty item.
func (removingLibrary) Ingest(context.Context, string, []byte, uuid.UUID) (media.Media, error) {
	return media.Media{}, nil
}

// Remove reports the stored files as gone.
func (removingLibrary) Remove(media.Media) error { return nil }

// undeletableMediaStore answers one stored item and fails whatever else it is asked.
type undeletableMediaStore struct {
	item media.Media
	err  error
}

// Create reports the failure.
func (s undeletableMediaStore) Create(context.Context, media.Media) (media.Media, error) {
	return media.Media{}, s.err
}

// ByID answers the stored item.
func (s undeletableMediaStore) ByID(context.Context, int64) (media.Media, error) {
	return s.item, nil
}

// ByIDs reports the failure.
func (s undeletableMediaStore) ByIDs(context.Context, []int64) ([]media.Media, error) {
	return nil, s.err
}

// List answers nothing stored.
func (s undeletableMediaStore) List(context.Context, media.Filter) ([]media.Media, int, error) {
	return nil, 0, nil
}

// Update reports the failure.
func (s undeletableMediaStore) Update(context.Context, media.Media, time.Time) (media.Media, error) {
	return media.Media{}, s.err
}

// Delete reports the failure.
func (s undeletableMediaStore) Delete(context.Context, int64) (media.Media, error) {
	return media.Media{}, s.err
}

// boundaryOf returns the multipart boundary a content type names.
func boundaryOf(t *testing.T, contentType string) string {
	t.Helper()

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("reading the content type: %v", err)
	}
	boundary, found := params["boundary"]
	if !found {
		t.Fatalf("content type %q names no boundary", contentType)
	}
	return boundary
}

// unreadableUploadForm returns a parsed upload whose stored file opens but cannot be read.
func unreadableUploadForm(t *testing.T, contentType string, body io.Reader) *multipart.Form {
	t.Helper()

	form, err := multipart.NewReader(body, boundaryOf(t, contentType)).ReadForm(0)
	if err != nil {
		t.Fatalf("parsing the upload: %v", err)
	}
	t.Cleanup(func() { _ = form.RemoveAll() })
	headers := form.File["file"]
	if len(headers) != 1 {
		t.Fatalf("parsed file parts = %d, want the upload to carry one", len(headers))
	}
	stored, err := headers[0].Open()
	if err != nil {
		t.Fatalf("opening the stored file: %v", err)
	}
	onDisk, isFile := stored.(*os.File)
	if !isFile {
		t.Fatalf("stored file = %T, want the part held on disk", stored)
	}
	path := onDisk.Name()
	_ = stored.Close()
	replaceWithDirectory(t, path)
	assertOpensButNeverReads(t, headers[0])
	return form
}

// replaceWithDirectory puts a directory where a file stood.
func replaceWithDirectory(t *testing.T, path string) {
	t.Helper()

	if err := os.Remove(path); err != nil {
		t.Fatalf("dropping the stored file: %v", err)
	}
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("putting a directory in its place: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
}

// assertOpensButNeverReads fails unless the part opens and then refuses to be read.
func assertOpensButNeverReads(t *testing.T, header *multipart.FileHeader) {
	t.Helper()

	probe, err := header.Open()
	if err != nil {
		t.Fatalf("Open() = %v, want the stored file to still open", err)
	}
	defer func() { _ = probe.Close() }()
	if _, err := io.ReadAll(probe); err == nil {
		t.Fatalf("ReadAll() = nil, want the stored file to refuse to be read")
	}
}

func TestUploadingMediaMasksARefusedReadDeadline(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)
	contentType, body := multipartFile(t, "file", "harbor.jpg", smallJPEG(t))
	request := httptest.NewRequest(http.MethodPost, "/api/media", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(deadlineRefusingRecorder{ResponseRecorder: recorder}, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("upload status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(recorder.Body.String(), "internal error") {
		t.Errorf("body = %q, want the refused deadline masked", recorder.Body.String())
	}
	if store.count() != 0 {
		t.Errorf("stored items = %d, want the upload dropped before it was read", store.count())
	}
}

func TestUploadingMediaRefusesAStoredFileThatCannotBeRead(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)
	contentType, body := multipartFile(t, "file", "harbor.jpg", smallJPEG(t))
	request := httptest.NewRequest(http.MethodPost, "/api/media", nil)
	request.Header.Set("Content-Type", contentType)
	request.MultipartForm = unreadableUploadForm(t, contentType, body)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("upload status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "upload_file_required") {
		t.Errorf("body = %q, want the unread file reported as no file", recorder.Body.String())
	}
	if store.count() != 0 {
		t.Errorf("stored items = %d, want nothing written for a file that never read", store.count())
	}
}

func TestDeletingMediaReportsAFailingRowDelete(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	stored := media.Media{ID: 1, Type: media.TypeImage, File: "2030/01/harbor.jpg", AuthorID: uuid.Nil}
	handler := authedServerWithStores(t, server.Config{
		Users:      users,
		Content:    newFakePostStore(),
		Media:      removingLibrary{},
		MediaStore: undeletableMediaStore{item: stored, err: media.ErrConflict},
	})

	recorder := doRequest(t, handler, http.MethodDelete, "/api/media/1", "")

	if recorder.Code != http.StatusConflict {
		t.Fatalf("delete status = %d, want %d", recorder.Code, http.StatusConflict)
	}
	if !strings.Contains(recorder.Body.String(), "media_stale_update") {
		t.Errorf("body = %q, want the row delete reported as a stale update", recorder.Body.String())
	}
}
