// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"
)

// photoUpload returns a multipart body carrying a small photo under the media library's file field.
func photoUpload(t *testing.T) (string, []byte) {
	t.Helper()
	var photo bytes.Buffer
	if err := jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 40, 30)), nil); err != nil {
		t.Fatalf("encoding the photo: %v", err)
	}
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", "harbor.jpg")
	if err != nil {
		t.Fatalf("building the upload: %v", err)
	}
	if _, err := part.Write(photo.Bytes()); err != nil {
		t.Fatalf("writing the upload: %v", err)
	}
	if err := form.Close(); err != nil {
		t.Fatalf("closing the upload: %v", err)
	}
	return form.FormDataContentType(), body.Bytes()
}

// uploadStatus posts the photo, pausing halfway through its body, and returns the status, zero on a broken connection.
func uploadStatus(t *testing.T, client *http.Client, base string, pause time.Duration) int {
	t.Helper()
	contentType, body := photoUpload(t)
	reader, writer := io.Pipe()
	go func() {
		half := len(body) / 2
		_, _ = writer.Write(body[:half])
		time.Sleep(pause)
		_, _ = writer.Write(body[half:])
		_ = writer.Close()
	}()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/api/media", reader)
	if err != nil {
		t.Fatalf("building the upload request: %v", err)
	}
	request.Header.Set("Content-Type", contentType)
	request.ContentLength = int64(len(body))
	response, err := client.Do(request)
	if err != nil {
		return 0
	}
	defer func() { _ = response.Body.Close() }()
	return response.StatusCode
}

func TestRunCutsAnUploadThatStallsPastTheUploadTimeout(t *testing.T) {
	t.Parallel()

	address := freeAddress(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL":   emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":           address,
		"GOPHENBERG_WEB_DIR":        t.TempDir(),
		"GOPHENBERG_MEDIA_DIR":      t.TempDir(),
		"GOPHENBERG_UPLOAD_TIMEOUT": "300ms",
	}
	if err := seedDemoData(t.Context(), testGetenv(env), new(bytes.Buffer)); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, testGetenv(env), io.Discard, noPlugins) }()
	base := "http://" + address
	awaitServer(t, base)
	client := loggedInClient(t, base)

	if status := uploadStatus(t, client, base, 0); status != http.StatusCreated {
		t.Fatalf("an upload sent at once answered %d, want %d", status, http.StatusCreated)
	}
	if status := uploadStatus(t, client, base, 1500*time.Millisecond); status == http.StatusCreated {
		t.Errorf("an upload that paused for 1.5s answered %d, want it cut at the 300ms upload timeout", status)
	}

	cancel()
	<-done
}
