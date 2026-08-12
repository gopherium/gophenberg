// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gopherium/gophenberg/internal/server"
)

// publicServer returns a handler serving the admin app, the API, and the public site over one post.
func publicServer(t *testing.T) http.Handler {
	t.Helper()
	posts := newFakePostStore()
	posts.add(publishedFixture(t, "hello-world", blockMarkup, time.Now().UTC()))
	return server.NewServer(server.Config{
		Users:     newFakeUserStore(),
		Content:   posts,
		SiteTitle: "A Test Site",
		Version:   "1.2.3",
		Web: fstest.MapFS{
			"index.html":    {Data: []byte("<!doctype html><title>Admin</title>")},
			"assets/app.js": {Data: []byte("console.log('app')")},
		},
	})
}

func TestPublicSiteTakesTheRoot(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, publicServer(t), http.MethodGet, "/", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "A Test Site") {
		t.Errorf("body = %q, want the public site", body)
	}
	if strings.Contains(body, "<title>Admin</title>") {
		t.Errorf("body = %q, want the public site rather than the admin app", body)
	}
}

func TestPublicSiteServesAPostAtItsAddress(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, publicServer(t), http.MethodGet, "/post/hello-world", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "A Published Post") {
		t.Errorf("body = %q, want the published post", recorder.Body.String())
	}
}

func TestPublicSiteAnswersAnUnservedAddressWithItsOwnNotFound(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, publicServer(t), http.MethodGet, "/nothing/here", "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if !strings.Contains(recorder.Body.String(), "A Test Site") {
		t.Errorf("body = %q, want the rendered not found page", recorder.Body.String())
	}
}

func TestServerKeepsEachRouteClassToItself(t *testing.T) {
	t.Parallel()

	handler := publicServer(t)
	cases := []struct {
		path   string
		status int
		want   string
		absent string
	}{
		{path: "/", status: http.StatusOK, want: "A Test Site"},
		{path: "/post/hello-world", status: http.StatusOK, want: "A Published Post"},
		{path: "/api/unknown", status: http.StatusNotFound, want: `"error"`, absent: "A Test Site"},
		{path: "/api", status: http.StatusNotFound, want: `"error"`, absent: "A Test Site"},
		{path: "/gophenberg", status: http.StatusNotFound, want: "", absent: "A Test Site"},
		{path: "/api/content/v1", status: http.StatusOK, want: `"api":1`, absent: "A Test Site"},
		{path: "/admin/", status: http.StatusOK, want: "<title>Admin</title>"},
		{path: "/admin/posts", status: http.StatusOK, want: "<title>Admin</title>"},
		{path: "/admin/assets/app.js", status: http.StatusOK, want: "console.log"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			recorder := doRequest(t, handler, http.MethodGet, tc.path, "")

			if recorder.Code != tc.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.status)
			}
			if !strings.Contains(recorder.Body.String(), tc.want) {
				t.Errorf("body = %q, want it to carry %q", recorder.Body.String(), tc.want)
			}
			if tc.absent != "" && strings.Contains(recorder.Body.String(), tc.absent) {
				t.Errorf("body = %q, want %q kept out of it", recorder.Body.String(), tc.absent)
			}
		})
	}
}

func TestAdminStillCanonicalizesToItsTrailingSlash(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, publicServer(t), http.MethodGet, "/admin", "")

	if recorder.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMovedPermanently)
	}
	if got := recorder.Header().Get("Location"); got != "/admin/" {
		t.Errorf("Location = %q, want %q", got, "/admin/")
	}
}

func TestWithoutAStoreThePublicRootIs404(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{Users: newFakeUserStore()})

	recorder := doRequest(t, handler, http.MethodGet, "/", "")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d when no store is configured", recorder.Code, http.StatusNotFound)
	}
}

func TestPublicSiteNeedsNoSession(t *testing.T) {
	t.Parallel()

	handler := publicServer(t)

	for _, path := range []string{"/", "/post/hello-world"} {
		recorder := doRequest(t, handler, http.MethodGet, path, "")
		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d without a session", path, recorder.Code, http.StatusOK)
		}
	}
}
