// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"io/fs"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

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
	body := recorder.Body.String()
	if !strings.Contains(body, "Gophenberg") {
		t.Errorf("body = %q, want the SPA index.html", body)
	}
	if !strings.HasPrefix(body, `<meta name="gophenberg-dialog-write-timeout" content="60000">`) {
		t.Errorf("body = %q, want the default dialog write limit written into the page", body)
	}
}

func TestServesSPAAssetsUnderAdmin(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, spaServer(t), http.MethodGet, "/admin/assets/app.js", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "console.log") {
		t.Errorf("body = %q, want the asset contents", body)
	}
	if strings.Contains(body, "gophenberg-dialog-write-timeout") {
		t.Errorf("body = %q, want the asset left without the dialog write limit", body)
	}
}

func TestFallsBackToIndexForAdminClientRoutes(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, spaServer(t), http.MethodGet, "/admin/posts", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "Gophenberg") {
		t.Errorf("client route body = %q, want the SPA index.html fallback", body)
	}
	if !strings.HasPrefix(body, `<meta name="gophenberg-dialog-write-timeout" content="60000">`) {
		t.Errorf("client route body = %q, want the dialog write limit written into the page", body)
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

func TestAdminPageIsCheckedOnEveryLoad(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{
		Users: newFakeUserStore(),
		Web: fstest.MapFS{
			"index.html":               {Data: []byte("<!doctype html><title>Gophenberg</title>")},
			"favicon.svg":              {Data: []byte("<svg></svg>")},
			"assets/app.js":            {Data: []byte("console.log('unhashed')")},
			"assets/index-DWLUpakq.js": {Data: []byte("console.log('hashed')")},
		},
	})

	unhashed := []string{"/admin/", "/admin/posts", "/admin/favicon.svg", "/admin/assets/", "/admin/assets/app.js"}
	for _, path := range unhashed {
		recorder := doRequest(t, handler, http.MethodGet, path, "")

		if held := recorder.Header().Get("Cache-Control"); recorder.Code != http.StatusOK || held != "no-cache" {
			t.Errorf("GET %s = %d with Cache-Control %q, want 200 and no-cache, since it names no content hash", path,
				recorder.Code, held)
		}
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

// dialogWriteLimitPage is an admin page with a head the limit can be written into.
const dialogWriteLimitPage = "<!doctype html><head><title>Gophenberg</title></head><p>admin</p>"

// dialogLimitMeta returns the element the admin page carries for limit.
func dialogLimitMeta(limit string) string {
	return `<meta name="gophenberg-dialog-write-timeout" content="` + limit + `">`
}

func TestAdminPageCarriesTheDialogWriteLimitItWasGiven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		lists server.ListSettings
		path  string
		limit string
	}{
		{name: "the default on the page", path: "/admin/", limit: "60000"},
		{
			name: "a configured limit", lists: server.ListSettings{DialogWriteTimeout: 45 * time.Second},
			path: "/admin/", limit: "45000",
		},
		{
			name: "the shortest limit on a client route", lists: server.ListSettings{DialogWriteTimeout: time.Millisecond},
			path: "/admin/posts", limit: "1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := server.NewServer(server.Config{
				Users: newFakeUserStore(),
				Web:   fstest.MapFS{"index.html": {Data: []byte(dialogWriteLimitPage)}},
				Lists: tc.lists,
			})
			recorder := doRequest(t, handler, http.MethodGet, tc.path, "")

			meta := dialogLimitMeta(tc.limit)
			body := recorder.Body.String()
			if recorder.Code != http.StatusOK || !strings.Contains(body, "<head>"+meta) {
				t.Errorf("GET %s = %d %q, want the page with %s written into its head", tc.path, recorder.Code, body, meta)
			}
			if strings.Replace(body, meta, "", 1) != dialogWriteLimitPage {
				t.Errorf("GET %s = %q, want the page otherwise unchanged", tc.path, body)
			}
		})
	}
}

func TestAdminPageHeadIsFoundWhateverItsCase(t *testing.T) {
	t.Parallel()

	page := "<!doctype html><HEAD><title>Gophenberg</title></HEAD>"
	handler := server.NewServer(server.Config{
		Users: newFakeUserStore(),
		Web:   fstest.MapFS{"index.html": {Data: []byte(page)}},
		Lists: server.ListSettings{DialogWriteTimeout: time.Second},
	})

	recorder := doRequest(t, handler, http.MethodGet, "/admin/", "")

	meta := dialogLimitMeta("1000")
	body := recorder.Body.String()
	unchanged := strings.Replace(body, meta, "", 1) == page
	if recorder.Code != http.StatusOK || !unchanged || !strings.Contains(body, "<HEAD>"+meta) {
		t.Errorf("body = %q, want %s written after the head", body, meta)
	}
}

func TestAdminPageWithoutAHeadCarriesTheDialogWriteLimitFirst(t *testing.T) {
	t.Parallel()

	page := "<header>Not the head</header><title>Gophenberg</title>"
	handler := server.NewServer(server.Config{
		Users: newFakeUserStore(),
		Web:   fstest.MapFS{"index.html": {Data: []byte(page)}},
		Lists: server.ListSettings{DialogWriteTimeout: 45 * time.Second},
	})

	recorder := doRequest(t, handler, http.MethodGet, "/admin/", "")

	meta := dialogLimitMeta("45000")
	body := recorder.Body.String()
	if recorder.Code != http.StatusOK || !strings.HasPrefix(body, meta) || strings.TrimPrefix(body, meta) != page {
		t.Errorf("body = %q, want %s written at the start and the page otherwise unchanged", body, meta)
	}
}

func TestAdminPageHeadCarriesNoBody(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{
		Users: newFakeUserStore(),
		Web:   fstest.MapFS{"index.html": {Data: []byte(dialogWriteLimitPage)}},
	})

	got := doRequest(t, handler, http.MethodGet, "/admin/", "")
	head := doRequest(t, handler, http.MethodHead, "/admin/", "")

	if head.Code != http.StatusOK || head.Body.Len() != 0 {
		t.Errorf("HEAD = %d with body %q, want 200 and no body", head.Code, head.Body.String())
	}
	if head.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html", head.Header().Get("Content-Type"))
	}
	if head.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", head.Header().Get("Cache-Control"))
	}
	if head.Header().Get("Content-Length") != strconv.Itoa(got.Body.Len()) {
		t.Errorf("Content-Length = %q, want %d", head.Header().Get("Content-Length"), got.Body.Len())
	}
	if !strings.Contains(got.Body.String(), dialogLimitMeta("60000")) {
		t.Errorf("GET = %q, want the dialog write limit", got.Body.String())
	}
}

func TestAdminIndexRedirectsToTheDirectory(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, spaServer(t), http.MethodGet, "/admin/index.html", "")

	if recorder.Code != http.StatusMovedPermanently {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMovedPermanently)
	}
	if got := recorder.Header().Get("Location"); got != "./" {
		t.Errorf("Location = %q, want %q", got, "./")
	}
}

// unreadPage is an admin folder whose files cannot be read.
type unreadPage struct{}

// Open refuses every file.
func (unreadPage) Open(string) (fs.File, error) {
	return nil, fs.ErrPermission
}

func TestAdminPageThatCannotBeReadIsRefused(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{Users: newFakeUserStore(), Web: unreadPage{}})

	for _, path := range []string{"/admin/", "/admin/posts"} {
		recorder := doRequest(t, handler, http.MethodGet, path, "")

		if recorder.Code != http.StatusForbidden {
			t.Errorf("GET %s = %d, want %d when the admin page cannot be read", path, recorder.Code, http.StatusForbidden)
		}
		if strings.Contains(recorder.Body.String(), "gophenberg-dialog-write-timeout") {
			t.Errorf("GET %s = %q, want no dialog write limit on a page that could not be read", path, recorder.Body.String())
		}
	}
}
