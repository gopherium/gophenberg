// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gopherium/gophenberg/internal/mediahost"
)

// largeUpload is a payload past the memory a multipart form holds before it writes a scratch file.
const largeUpload = 33 << 20

// largeUploadCap is a media cap that admits a large upload.
const largeUploadCap = 64 << 20

// watchedScratch points the process's scratch files at a folder the test watches and returns it.
func watchedScratch(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	return dir
}

// assertNoScratchLeft fails the test when the watched folder still holds anything.
func assertNoScratchLeft(t *testing.T, dir string) {
	t.Helper()
	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the watched folder: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("the watched folder holds %d entries such as %q, want every scratch file removed",
			len(left), left[0].Name())
	}
}

// paddedPDF returns a PDF document padded to size bytes.
func paddedPDF(size int) []byte {
	document := []byte("%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n")
	return append(document, bytes.Repeat([]byte("\n"), size-len(document))...)
}

// sendMediaUploadUnder posts data to the media route under the named multipart field.
func sendMediaUploadUnder(
	t *testing.T, handler http.Handler, field, filename string, data []byte,
) *httptest.ResponseRecorder {
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
	request := httptest.NewRequest(http.MethodPost, "/api/media", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestUploadingLargeMediaRemovesItsScratchFile(t *testing.T) {
	library := mediahost.New(mediahost.Config{Dir: t.TempDir(), MaxSize: largeUploadCap})
	handler := mediaServer(t, library, newFakeMediaStore())
	scratch := watchedScratch(t)

	recorder := sendMediaUpload(t, handler, "manual.pdf", paddedPDF(largeUpload))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d (%s), want %d", recorder.Code, recorder.Body.String(), http.StatusCreated)
	}
	assertNoScratchLeft(t, scratch)
}

func TestUploadingLargeMediaUnderAnotherFieldRemovesItsScratchFile(t *testing.T) {
	library := mediahost.New(mediahost.Config{Dir: t.TempDir(), MaxSize: largeUploadCap})
	handler := mediaServer(t, library, newFakeMediaStore())
	scratch := watchedScratch(t)

	recorder := sendMediaUploadUnder(t, handler, "attachment", "manual.pdf", paddedPDF(largeUpload))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d (%s), want %d", recorder.Code, recorder.Body.String(), http.StatusBadRequest)
	}
	assertNoScratchLeft(t, scratch)
}

func TestUploadingALargeThemeRemovesItsScratchFile(t *testing.T) {
	handler := themeServer(t, errThemes{err: errors.New("the archive could not be read")})
	scratch := watchedScratch(t)
	contentType, body := uploadBody(t, "aurora.zip", bytes.Repeat([]byte("x"), largeUpload))

	recorder := sendUpload(t, handler, contentType, body)

	if recorder.Code < http.StatusBadRequest {
		t.Fatalf("status = %d, want the archive refused", recorder.Code)
	}
	assertNoScratchLeft(t, scratch)
}
