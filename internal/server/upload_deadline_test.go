// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/server"
)

// deadlineRecordingWriter is a response writer that notes the read deadline it is given.
type deadlineRecordingWriter struct {
	http.ResponseWriter
	deadline *time.Time
}

// SetReadDeadline notes the deadline.
func (w deadlineRecordingWriter) SetReadDeadline(at time.Time) error {
	*w.deadline = at
	return nil
}

// uploadDeadlineOf serves the request and returns how far past its start the handler moved the read deadline.
func uploadDeadlineOf(t *testing.T, handler http.Handler, request *http.Request) time.Duration {
	t.Helper()
	var deadline time.Time
	started := time.Now()
	handler.ServeHTTP(deadlineRecordingWriter{ResponseWriter: httptest.NewRecorder(), deadline: &deadline}, request)
	if deadline.IsZero() {
		t.Fatal("the upload never moved its read deadline")
	}
	return deadline.Sub(started)
}

func TestAMediaUploadGetsTheUploadTimeoutTheSiteNames(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := authedServerWithStores(t, server.Config{
		Users: users, Content: newFakePostStore(), Types: newFakeTypeStore(),
		Media: mediahost.New(mediahost.Config{Dir: t.TempDir()}), MediaStore: newFakeMediaStore(),
		UploadTimeout: 90 * time.Second,
	})
	contentType, body := multipartFile(t, "harbor.jpg", smallJPEG(t))
	request := httptest.NewRequest(http.MethodPost, "/api/media", body)
	request.Header.Set("Content-Type", contentType)

	moved := uploadDeadlineOf(t, handler, request)

	if moved < 90*time.Second || moved > 95*time.Second {
		t.Errorf("the read deadline moved %v past the start, want the 90s the site names", moved)
	}
}

func TestAThemeUploadGetsFiveMinutesWhenTheSiteNamesNoUploadTimeout(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, &servingThemes{})
	contentType, body := uploadBody(t, "aurora.zip", []byte("an archive"))
	request := httptest.NewRequest(http.MethodPost, "/api/themes", body)
	request.Header.Set("Content-Type", contentType)

	moved := uploadDeadlineOf(t, handler, request)

	if moved < server.DefaultUploadTimeout || moved > server.DefaultUploadTimeout+5*time.Second {
		t.Errorf("the read deadline moved %v past the start, want the default %v", moved, server.DefaultUploadTimeout)
	}
}
