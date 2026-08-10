// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/server"
)

// fakeMediaStore holds media items in memory and can be told to fail.
type fakeMediaStore struct {
	mu        sync.Mutex
	next      int64
	items     map[int64]media.Media
	createErr error
}

// newFakeMediaStore returns an empty in-memory media store double.
func newFakeMediaStore() *fakeMediaStore {
	return &fakeMediaStore{items: make(map[int64]media.Media)}
}

// Create stores a new media item, or fails as told.
func (s *fakeMediaStore) Create(_ context.Context, m media.Media) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.createErr != nil {
		return media.Media{}, s.createErr
	}
	s.next++
	m.ID = s.next
	s.items[m.ID] = m
	return m, nil
}

// ByID returns the media item with the given id, or [media.ErrNotFound].
func (s *fakeMediaStore) ByID(_ context.Context, id int64) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, found := s.items[id]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	return m, nil
}

// List returns every stored item and their count.
func (s *fakeMediaStore) List(context.Context, media.Filter) ([]media.Media, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]media.Media, 0, len(s.items))
	for _, m := range s.items {
		items = append(items, m)
	}
	return items, len(items), nil
}

// Update reports the item missing.
func (s *fakeMediaStore) Update(context.Context, media.Media, time.Time) (media.Media, error) {
	return media.Media{}, media.ErrNotFound
}

// Delete reports the item missing.
func (s *fakeMediaStore) Delete(context.Context, int64) (media.Media, error) {
	return media.Media{}, media.ErrNotFound
}

// count returns how many items the store holds.
func (s *fakeMediaStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}

// mediaServer returns a signed in handler over a real library and the given store.
func mediaServer(t *testing.T, library *mediahost.Library, store media.Store) http.Handler {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	return authedServerWithStores(t, server.Config{
		Users:      users,
		Posts:      newFakePostStore(),
		Media:      library,
		MediaStore: store,
	})
}

// smallJPEG returns a small JPEG photograph.
func smallJPEG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for x := 0; x < 40; x++ {
		canvas.Set(x, x%30, color.RGBA{R: uint8(x * 6), A: 255})
	}
	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatalf("encoding the photo: %v", err)
	}
	return buffer.Bytes()
}

// multipartFile returns a multipart body carrying data under the given field.
func multipartFile(t *testing.T, field, filename string, data []byte) (string, io.Reader) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("building the upload: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("writing the upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the upload: %v", err)
	}
	return writer.FormDataContentType(), &body
}

// sendMediaUpload posts a multipart file to the media route.
func sendMediaUpload(t *testing.T, handler http.Handler, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	contentType, body := multipartFile(t, "file", filename, data)
	request := httptest.NewRequest(http.MethodPost, "/api/media", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// mediaBody is the media item shape the admin API answers.
type mediaBody struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	File     string `json:"file"`
	Title    string `json:"title"`
	MimeType string `json:"mime_type"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	AuthorID string `json:"author_id"`
	Sizes    map[string]struct {
		File  string `json:"file"`
		Width int    `json:"width"`
	} `json:"sizes"`
}

func TestMediaRoutesAreAbsentWithoutALibrary(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := authedServerWithStores(t, server.Config{Users: users, Posts: newFakePostStore()})

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want the media routes unhandled", recorder.Code)
	}
}

func TestUploadingMediaStoresItAndAnswersTheItem(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s), want %d", recorder.Code, recorder.Body.String(), http.StatusCreated)
	}
	body := decodeBody[mediaBody](t, recorder)
	if body.ID != 1 || body.Type != "image" || body.Title != "harbor" {
		t.Errorf("body = %+v, want the stored image", body)
	}
	if body.MimeType != "image/jpeg" || body.Width != 40 || body.Height != 30 {
		t.Errorf("body = %+v, want the measured photo", body)
	}
	if _, offered := body.Sizes["full"]; !offered {
		t.Errorf("Sizes = %v, want a full rendition", body.Sizes)
	}
	if body.AuthorID == "" {
		t.Error("AuthorID is empty, want the session identity")
	}
	if store.count() != 1 {
		t.Errorf("the store holds %d items, want the upload stored", store.count())
	}
}

func TestUploadingMediaRefusesWhatTheLibraryRefuses(t *testing.T) {
	t.Parallel()

	store := newFakeMediaStore()
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

	recorder := sendMediaUpload(t, handler, "notes.txt", []byte("meeting notes"))

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	if !strings.Contains(recorder.Body.String(), "the file type is not allowed") {
		t.Errorf("body = %q, want the refusal reason", recorder.Body.String())
	}
	if store.count() != 0 {
		t.Errorf("the store holds %d items, want the refusal to store nothing", store.count())
	}
}

func TestUploadingMediaRefusesAnOversizedFile(t *testing.T) {
	t.Parallel()

	handler := mediaServer(t,
		mediahost.New(mediahost.Config{Dir: t.TempDir(), MaxSize: 1 << 10}), newFakeMediaStore())

	overCap := sendMediaUpload(t, handler, "big.jpg", bytes.Repeat([]byte{0}, 2<<10))
	if overCap.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d for a file over the cap", overCap.Code, http.StatusRequestEntityTooLarge)
	}

	overEnvelope := sendMediaUpload(t, handler, "big.jpg", bytes.Repeat([]byte{0}, 80<<10))
	if overEnvelope.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d past the read cap", overEnvelope.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestUploadingMediaWithoutAFileIsRefused(t *testing.T) {
	t.Parallel()

	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), newFakeMediaStore())

	recorder := doRequest(t, handler, http.MethodPost, "/api/media", `{"not":"a file"}`)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "the upload carries no file") {
		t.Errorf("body = %q, want the missing file named", recorder.Body.String())
	}
}

func TestUploadingMediaCleansUpWhenTheStoreFails(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	store := newFakeMediaStore()
	store.createErr = errors.New("the database is gone")
	handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: dir}), store)

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), "database") {
		t.Errorf("body = %q, want the failure masked", recorder.Body.String())
	}
	left := 0
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			left++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the media directory: %v", err)
	}
	if left != 0 {
		t.Errorf("the media directory holds %d files, want the failed upload cleaned up", left)
	}
}

func TestUploadingMediaNeedsASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(server.Config{
		Users:      users,
		Posts:      newFakePostStore(),
		Media:      mediahost.New(mediahost.Config{Dir: t.TempDir()}),
		MediaStore: newFakeMediaStore(),
	})

	recorder := sendMediaUpload(t, handler, "harbor.jpg", smallJPEG(t))

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
