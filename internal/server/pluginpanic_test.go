// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/server"
)

// panickingRoute returns a plugin handler that panics with text, first sending part of a 200 answer when began.
func panickingRoute(text string, began bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		if began {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "partial")
		}
		panic(text)
	})
}

// panickingPluginServer returns a server whose plugin "notes" panics on its public /boom, and the log it writes.
func panickingPluginServer(t *testing.T, began bool) (http.Handler, *bytes.Buffer) {
	t.Helper()
	logged := &bytes.Buffer{}
	handler := server.NewServer(server.Config{
		Users:             newFakeUserStore(),
		Logger:            slog.New(slog.NewTextHandler(logged, nil)),
		Plugins:           map[string]http.Handler{"notes": panickingRoute("boom", began)},
		PluginPublicPaths: map[string][]string{"notes": {"/boom"}},
	})
	return handler, logged
}

// servePanicking serves r through handler and returns the recorder and what the server got as a panic.
func servePanicking(handler http.Handler, r *http.Request) (recorder *httptest.ResponseRecorder, escaped any) {
	recorder = httptest.NewRecorder()
	defer func() { escaped = recover() }()
	handler.ServeHTTP(recorder, r)
	return recorder, nil
}

// routePanicLine returns the start of the line the server logs for a panic of the route path of notes.
func routePanicLine(path string, answered bool) string {
	return fmt.Sprintf(`msg="plugin route panicked" plugin=notes method=GET path=/api/plugins/notes%s answered=%t `+
		`error="pluginkit: notes route panicked: boom" stack="goroutine `, path, answered)
}

func TestAPanicInAPluginRouteAnswersTheInternalError(t *testing.T) {
	t.Parallel()

	handler, logged := panickingPluginServer(t, false)

	recorder, escaped := servePanicking(handler, httptest.NewRequest(http.MethodGet, "/api/plugins/notes/boom", nil))

	if escaped != nil || recorder.Code != http.StatusInternalServerError {
		t.Fatalf("answered %d with the panic %v, want 500", recorder.Code, escaped)
	}
	var body map[string]string
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil ||
		!maps.Equal(body, map[string]string{"error": "internal error", "code": "internal"}) {
		t.Errorf("the body reads %s (%v), want the site's internal error", recorder.Body.String(), err)
	}
	if control, encoding := recorder.Header().Get("Cache-Control"), recorder.Header().Get("Content-Encoding"); control !=
		"no-store" || encoding != "" {
		t.Errorf("the answer carries Cache-Control %q and Content-Encoding %q, want no-store alone", control, encoding)
	}
	if line := routePanicLine("/boom", true); !strings.Contains(logged.String(), line) {
		t.Errorf("the server logged %q, want %q among it", logged.String(), line)
	}
}

func TestAPanicAfterAPluginRouteBeganItsAnswerCutsIt(t *testing.T) {
	t.Parallel()

	handler, logged := panickingPluginServer(t, true)

	_, escaped := servePanicking(handler, httptest.NewRequest(http.MethodGet, "/api/plugins/notes/boom", nil))

	if escaped != http.ErrAbortHandler {
		t.Errorf("the server got the panic %v, want http.ErrAbortHandler", escaped)
	}
	if line := routePanicLine("/boom", false); !strings.Contains(logged.String(), line) {
		t.Errorf("the server logged %q, want %q among it", logged.String(), line)
	}
}
