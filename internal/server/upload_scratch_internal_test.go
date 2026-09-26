// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// spilledUpload returns a media upload whose multipart form wrote its file to a scratch file in dir.
func spilledUpload(t *testing.T, dir string) *http.Request {
	t.Helper()
	t.Setenv("TMPDIR", dir)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(mediaUploadField, "manual.pdf")
	if err != nil {
		t.Fatalf("building the upload: %v", err)
	}
	if _, err := part.Write([]byte("%PDF-1.4 a spilled upload")); err != nil {
		t.Fatalf("writing the upload: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing the upload: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/media", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if err := request.ParseMultipartForm(1); err != nil {
		t.Fatalf("parsing the upload: %v", err)
	}
	return request
}

// loggingServer returns a server whose log lands in the returned buffer.
func loggingServer() (*server, *bytes.Buffer) {
	var logged bytes.Buffer
	return &server{logger: slog.New(slog.NewTextHandler(&logged, nil))}, &logged
}

func TestRemoveUploadScratchRemovesTheFilesQuietly(t *testing.T) {
	scratch := t.TempDir()
	request := spilledUpload(t, scratch)
	s, logged := loggingServer()

	s.removeUploadScratch(request)

	if left, _ := os.ReadDir(scratch); len(left) != 0 {
		t.Errorf("the scratch folder holds %d entries, want the upload's file removed", len(left))
	}
	if logged.Len() != 0 {
		t.Errorf("logged %q, want nothing for a removal that worked", logged.String())
	}
}

func TestRemoveUploadScratchLogsTheFilesItCannotRemove(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root removes files from a folder it cannot write to")
	}
	scratch := t.TempDir()
	request := spilledUpload(t, scratch)
	if err := os.Chmod(scratch, 0o500); err != nil {
		t.Fatalf("taking write permission away: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(scratch, 0o700) })
	s, logged := loggingServer()

	s.removeUploadScratch(request)

	if got := logged.String(); !strings.Contains(got, "upload scratch files left behind") ||
		!strings.Contains(got, "path=/api/media") {
		t.Errorf("logged %q, want a warning naming the upload's path", got)
	}
}

func TestLoggerOfHandsOverTheConfiguredLogger(t *testing.T) {
	t.Parallel()

	configured := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	if got := loggerOf(Config{Logger: configured}); got != configured {
		t.Errorf("loggerOf() = %p, want the configured logger %p", got, configured)
	}
}

func TestLoggerOfDiscardsWhenNoneIsConfigured(t *testing.T) {
	t.Parallel()

	got := loggerOf(Config{})

	if got == nil || got.Enabled(context.Background(), slog.LevelError) {
		t.Errorf("loggerOf() = %v, want a logger that discards every level", got)
	}
}

func TestRemoveUploadScratchLeavesARequestWithoutAForm(t *testing.T) {
	s, logged := loggingServer()

	s.removeUploadScratch(httptest.NewRequest(http.MethodPost, "/api/media", nil))

	if logged.Len() != 0 {
		t.Errorf("logged %q, want nothing for a request that parsed no form", logged.String())
	}
}
