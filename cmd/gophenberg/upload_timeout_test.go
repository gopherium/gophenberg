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

// uploadOutcome is how an upload ended, and how long after its request went out.
type uploadOutcome struct {
	status int
	took   time.Duration
	err    error
}

// sendUpload posts the photo, pausing halfway through its body, and returns how the upload ended.
func sendUpload(t *testing.T, client *http.Client, base string, pause time.Duration) uploadOutcome {
	t.Helper()
	contentType, body := photoUpload(t)
	reader, writer := io.Pipe()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/api/media", reader)
	if err != nil {
		t.Fatalf("building the upload request: %v", err)
	}
	request.Header.Set("Content-Type", contentType)
	request.ContentLength = int64(len(body))
	go func() {
		half := len(body) / 2
		_, _ = writer.Write(body[:half])
		time.Sleep(pause)
		_, _ = writer.Write(body[half:])
		_ = writer.Close()
	}()
	sent := time.Now()
	response, err := client.Do(request)
	if err != nil {
		return uploadOutcome{took: time.Since(sent), err: err}
	}
	defer func() { _ = response.Body.Close() }()
	return uploadOutcome{status: response.StatusCode, took: time.Since(sent)}
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

	if whole := sendUpload(t, client, base, 0); whole.err != nil || whole.status != http.StatusCreated {
		t.Fatalf("an upload sent at once answered %d, %v, want %d", whole.status, whole.err, http.StatusCreated)
	}
	cut := sendUpload(t, client, base, 1500*time.Millisecond)
	if cut.err != nil || cut.status != http.StatusBadRequest {
		t.Errorf("an upload that paused for 1.5s answered %d, %v, want the %d the upload timeout gives",
			cut.status, cut.err, http.StatusBadRequest)
	}
	if cut.took < 300*time.Millisecond || cut.took > time.Second {
		t.Errorf("the paused upload ended %v after it went out, want soon after the 300ms upload timeout", cut.took)
	}

	cancel()
	<-done
}
