// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gopherium/gophenberg/internal/server"
)

// assetServer returns a handler serving the site's own assets from a web directory holding them.
func assetServer(t *testing.T) http.Handler {
	t.Helper()
	return server.NewServer(server.Config{
		Users:   newFakeUserStore(),
		Posts:   newFakePostStore(),
		Version: "1.2.3",
		Web: fstest.MapFS{
			"index.html":             {Data: []byte("<!doctype html><title>Admin</title>")},
			"gophenberg/blocks.css":  {Data: []byte(".wp-block-quote{margin:0}")},
			"gophenberg/presets.css": {Data: []byte(":root{--wp--preset--color--black:#000}")},
			"gophenberg/favicon.svg": {Data: []byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`)},
		},
	})
}

func TestSiteAssetsAreServedUnderTheirOwnPath(t *testing.T) {
	t.Parallel()

	handler := assetServer(t)
	cases := []struct {
		path        string
		want        string
		contentType string
	}{
		{path: "/gophenberg/blocks.css", want: ".wp-block-quote", contentType: "text/css"},
		{path: "/gophenberg/presets.css", want: "--wp--preset--color--black", contentType: "text/css"},
		{path: "/gophenberg/favicon.svg", want: "<svg", contentType: "image/svg+xml"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			recorder := doRequest(t, handler, http.MethodGet, tc.path, "")

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if !strings.Contains(recorder.Body.String(), tc.want) {
				t.Errorf("body = %q, want it to carry %q", recorder.Body.String(), tc.want)
			}
			if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, tc.contentType) {
				t.Errorf("Content-Type = %q, want %q", got, tc.contentType)
			}
		})
	}
}

func TestSiteAssetsAreCacheableAndIdentified(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, assetServer(t), http.MethodGet, "/gophenberg/blocks.css", "")

	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=3600" {
		t.Errorf("Cache-Control = %q, want %q", got, "public, max-age=3600")
	}
	if got := recorder.Header().Get("X-Generator"); got != wantGenerator {
		t.Errorf("X-Generator = %q, want %q", got, wantGenerator)
	}
}

func TestAMissingAssetIsNotServedAPage(t *testing.T) {
	t.Parallel()

	recorder := doRequest(t, assetServer(t), http.MethodGet, "/gophenberg/missing.css", "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if strings.Contains(recorder.Body.String(), "<html") {
		t.Errorf("body = %q, want no HTML page for a missing asset", recorder.Body.String())
	}
}

func TestSiteAssetsStayInsideTheirDirectory(t *testing.T) {
	t.Parallel()

	handler := assetServer(t)

	for _, path := range []string{"/gophenberg/../index.html", "/gophenberg/%2e%2e/index.html"} {
		recorder := doRequest(t, handler, http.MethodGet, path, "")
		if strings.Contains(recorder.Body.String(), "<title>Admin</title>") {
			t.Errorf("GET %s served the admin app, want the asset directory kept closed", path)
		}
	}
}

func TestWithoutAWebDirectoryTheAssetsAre404(t *testing.T) {
	t.Parallel()

	handler := server.NewServer(server.Config{Users: newFakeUserStore(), Version: "1.2.3"})

	recorder := doRequest(t, handler, http.MethodGet, "/gophenberg/blocks.css", "")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d when nothing is configured to serve", recorder.Code, http.StatusNotFound)
	}
}
