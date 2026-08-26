// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gopherium/gophenberg/internal/server"
)

// spaServer returns a server serving a small in-memory web bundle.
func spaServer(t *testing.T) http.Handler {
	t.Helper()
	return server.NewServer(server.Config{
		Users: newFakeUserStore(),
		Web: fstest.MapFS{
			"index.html":    {Data: []byte("<!doctype html><title>Gophenberg</title>")},
			"assets/app.js": {Data: []byte("console.log('app')")},
		},
	})
}

func TestAdminWithoutSlashRedirectsToTheSPA(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, spaServer(t), http.MethodGet, "/admin", "")

	if recorder.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMovedPermanently)
	}
	if got := recorder.Header().Get("Location"); got != "/admin/" {
		t.Errorf("Location = %q, want %q", got, "/admin/")
	}
}

func TestServesTheSPAUnderAdmin(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, spaServer(t), http.MethodGet, "/admin/", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "Gophenberg") {
		t.Errorf("body = %q, want the SPA index.html", recorder.Body.String())
	}
}

func TestServesSPAAssetsUnderAdmin(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, spaServer(t), http.MethodGet, "/admin/assets/app.js", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "console.log") {
		t.Errorf("body = %q, want the asset contents", recorder.Body.String())
	}
}

func TestFallsBackToIndexForAdminClientRoutes(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, spaServer(t), http.MethodGet, "/admin/posts", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "Gophenberg") {
		t.Errorf("client route body = %q, want the SPA index.html fallback", recorder.Body.String())
	}
}

func TestADirectoryUnderTheSPAFallsBackToTheApp(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, spaServer(t), http.MethodGet, "/admin/assets/", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d rather than a redirect", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "Gophenberg") {
		t.Errorf("body = %q, want the app rather than a directory listing", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "app.js</a>") {
		t.Errorf("body = %q, want no directory listing", recorder.Body.String())
	}
}

func TestUnknownAPIPathIsNotServedTheSPA(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, spaServer(t), http.MethodGet, "/api/nope", "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if strings.Contains(recorder.Body.String(), "Gophenberg") {
		t.Error("an unknown API path was served the SPA, want a JSON 404")
	}
}

func TestWithoutWebFSUnknownPathsAre404(t *testing.T) {
	t.Parallel()

	srv := server.NewServer(server.Config{Users: newFakeUserStore()})

	recorder := doRequest(t, srv, http.MethodGet, "/", "")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d when no SPA is configured", recorder.Code, http.StatusNotFound)
	}
}
