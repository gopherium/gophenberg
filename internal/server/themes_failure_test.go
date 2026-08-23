// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/themehost"
)

// errThemes is a themes directory that fails whatever it is asked.
type errThemes struct{ err error }

// List reports the failure.
func (e errThemes) List(context.Context) ([]themehost.Installed, error) { return nil, e.err }

// Install reports the failure.
func (e errThemes) Install(context.Context, string, io.ReaderAt, int64) error { return e.err }

// Activate reports the failure.
func (e errThemes) Activate(context.Context, string) error { return e.err }

// Deactivate reports the failure.
func (e errThemes) Deactivate(context.Context) error { return e.err }

// Rollback reports the failure.
func (e errThemes) Rollback(context.Context) (string, error) { return "", e.err }

// Previous reports the failure.
func (e errThemes) Previous(context.Context) (string, bool, error) { return "", false, e.err }

// listOnlyFailsThemes lists themes but fails to say what a rollback would return to.
type listOnlyFailsThemes struct{ errThemes }

// List returns nothing installed.
func (listOnlyFailsThemes) List(context.Context) ([]themehost.Installed, error) { return nil, nil }

// themeServer returns a signed in handler over the given themes directory.
func themeServer(t *testing.T, themes server.Themes) http.Handler {
	t.Helper()

	users := newFakeUserStore()
	addAda(t, users)
	return authedServerWithStores(t, server.Config{
		Users:   users,
		Content: newFakePostStore(),
		Themes:  themes,
	})
}

// uploadBody returns a multipart body carrying an archive under the given filename.
func uploadBody(t *testing.T, filename string, archive []byte) (string, io.Reader) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("theme", filename)
	if err != nil {
		t.Fatalf("building the upload: %v", err)
	}
	if _, err := part.Write(archive); err != nil {
		t.Fatalf("writing the upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the upload: %v", err)
	}
	return writer.FormDataContentType(), &body
}

// endlessFiller is a stream of one repeated byte that never ends.
type endlessFiller struct{}

// Read fills p with the repeated byte.
func (endlessFiller) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}

// pastTheReadCapBody returns a multipart body whose archive runs past the read cap.
func pastTheReadCapBody(t *testing.T) (string, io.Reader) {
	t.Helper()

	var framing bytes.Buffer
	writer := multipart.NewWriter(&framing)
	if _, err := writer.CreateFormFile("theme", "aurora.zip"); err != nil {
		t.Fatalf("building the upload: %v", err)
	}
	archive := io.LimitReader(endlessFiller{}, themehost.MaxSize+(1<<20))
	return writer.FormDataContentType(), io.MultiReader(&framing, archive)
}

// countingReader is a stream recording how many bytes were taken from it.
type countingReader struct {
	source io.Reader
	read   int64
}

// Read records the bytes handed over.
func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.source.Read(p)
	c.read += int64(n)
	return n, err
}

// sendUpload posts a body to the theme upload route and returns what it answered.
func sendUpload(t *testing.T, handler http.Handler, contentType string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPost, "/api/themes", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestTheThemeRoutesAreAbsentWithoutAThemesDirectory(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := authedServerWithStores(t, server.Config{Users: users, Content: newFakePostStore()})

	recorder := doRequest(t, handler, http.MethodGet, "/api/themes", "")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want the theme routes unhandled", recorder.Code)
	}
}

func TestListingThemesMasksAFailingThemesDirectory(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, errThemes{err: errors.New("the disk is gone")})

	recorder := doRequest(t, handler, http.MethodGet, "/api/themes", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), "the disk is gone") {
		t.Errorf("body = %q, want the internal error masked", recorder.Body.String())
	}
}

func TestListingThemesMasksAFailingRollbackLookup(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, listOnlyFailsThemes{errThemes{err: errors.New("the disk is gone")}})

	recorder := doRequest(t, handler, http.MethodGet, "/api/themes", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestUploadingWithoutAnArchiveIsRefused(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, errThemes{})

	recorder := sendUpload(t, handler, "application/json", strings.NewReader("{}"))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
	if !strings.Contains(recorder.Body.String(), "no theme archive") {
		t.Errorf("body = %q, want it to say no archive arrived", recorder.Body.String())
	}
}

func TestUploadingOverTheCapIsRefused(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, errThemes{})
	contentType, body := uploadBody(t, "aurora.zip", bytes.Repeat([]byte("a"), int(themehost.MaxSize)+1))

	recorder := sendUpload(t, handler, contentType, body)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
	if !strings.Contains(recorder.Body.String(), "too large") {
		t.Errorf("body = %q, want it to say the theme is too large", recorder.Body.String())
	}
}

func TestAnUploadPastTheReadCapIsRefusedBeforeItArrives(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, errThemes{})
	contentType, body := pastTheReadCapBody(t)
	counted := &countingReader{source: body}

	recorder := sendUpload(t, handler, contentType, counted)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
	if !strings.Contains(recorder.Body.String(), "theme_too_large") {
		t.Errorf("body = %q, want it to say the theme is too large", recorder.Body.String())
	}
	if counted.read >= themehost.MaxSize+(1<<20) {
		t.Errorf("bytes read = %d, want the upload refused before the whole archive arrived", counted.read)
	}
}

func TestAnArchiveAtTheCapIsNotRefusedForTheFramingAroundIt(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, errThemes{})
	contentType, body := uploadBody(t, "aurora.zip", bytes.Repeat([]byte("a"), int(themehost.MaxSize)))

	recorder := sendUpload(t, handler, contentType, body)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want an archive at the cap accepted, got %s", recorder.Code, recorder.Body)
	}
}

func TestUploadingMasksAFailingInstall(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, errThemes{err: errors.New("the disk is gone")})
	contentType, body := uploadBody(t, "aurora.zip", []byte("an archive"))

	recorder := sendUpload(t, handler, contentType, body)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	if strings.Contains(recorder.Body.String(), "the disk is gone") {
		t.Errorf("body = %q, want the internal error masked", recorder.Body.String())
	}
}

func TestAnUploadedNameDropsTheDirectoriesAndTheSuffix(t *testing.T) {
	t.Parallel()

	var installed string
	handler := themeServer(t, namingThemes{name: &installed})
	contentType, body := uploadBody(t, `../../etc/aurora.zip`, []byte("an archive"))

	sendUpload(t, handler, contentType, body)

	if installed != "aurora" {
		t.Errorf("installed = %q, want the bare theme name", installed)
	}
}

// namingThemes records the name an upload asked to install under.
type namingThemes struct {
	errThemes
	name *string
}

// Install records the name and accepts the archive.
func (n namingThemes) Install(_ context.Context, name string, _ io.ReaderAt, _ int64) error {
	*n.name = name
	return nil
}

func TestDeactivatingMasksAFailingThemesDirectory(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, errThemes{err: errors.New("the disk is gone")})

	recorder := doRequest(t, handler, http.MethodDelete, "/api/themes/active", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestRollingBackMasksAFailingThemesDirectory(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, errThemes{err: errors.New("the disk is gone")})

	recorder := doRequest(t, handler, http.MethodPost, "/api/themes/rollback", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestActivatingRefusesABodyThatIsNotJSON(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, errThemes{})

	recorder := doRequest(t, handler, http.MethodPost, "/api/themes/active", "not json")

	if recorder.Code < 400 {
		t.Errorf("status = %d, want a body that is not JSON refused", recorder.Code)
	}
}
