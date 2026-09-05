// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gopherium/gophenberg/internal/server"
)

// cachingServer returns a handler serving assets, media and the content API under the given windows.
func cachingServer(t *testing.T, cache server.CachePolicy) http.Handler {
	t.Helper()
	posts := newFakePostStore()
	return server.NewServer(server.Config{
		Users:      newFakeUserStore(),
		Content:    posts,
		Types:      newFakeTypeStore(),
		Version:    "1.2.3",
		Cache:      cache,
		Web:        fstest.MapFS{"gophenberg/site.css": {Data: []byte("body{}")}},
		MediaFiles: fstest.MapFS{"picture.jpg": {Data: []byte("not really a picture")}},
	})
}

// cacheControlAt returns the Cache-Control header the handler answers at the path.
func cacheControlAt(t *testing.T, handler http.Handler, path string) string {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder.Header().Get("Cache-Control")
}

func TestCacheWindowsFallBackToTheirDefaults(t *testing.T) {
	t.Parallel()

	handler := cachingServer(t, server.CachePolicy{})

	for name, asked := range map[string]struct {
		path string
		want string
	}{
		"site assets": {"/gophenberg/site.css", "public, max-age=3600"},
		"media":       {"/media/picture.jpg", "public, max-age=3600"},
		"the content API": {
			"/api/content/v1/items", "public, s-maxage=60, stale-while-revalidate=300",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if held := cacheControlAt(t, handler, asked.path); held != asked.want {
				t.Errorf("Cache-Control = %q, want %q", held, asked.want)
			}
		})
	}
}

func TestCacheWindowsCarryWhatTheDeploymentNamed(t *testing.T) {
	t.Parallel()

	handler := cachingServer(t, server.CachePolicy{
		AssetMaxAge:                 2 * time.Hour,
		MediaMaxAge:                 90 * time.Second,
		ContentSharedMaxAge:         30 * time.Second,
		ContentStaleWhileRevalidate: 10 * time.Minute,
	})

	for name, asked := range map[string]struct {
		path string
		want string
	}{
		"site assets": {"/gophenberg/site.css", "public, max-age=7200"},
		"media":       {"/media/picture.jpg", "public, max-age=90"},
		"the content API": {
			"/api/content/v1/items", "public, s-maxage=30, stale-while-revalidate=600",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if held := cacheControlAt(t, handler, asked.path); held != asked.want {
				t.Errorf("Cache-Control = %q, want %q", held, asked.want)
			}
		})
	}
}

func TestTheLocaleAnswerStaysOutOfSharedCaches(t *testing.T) {
	t.Parallel()

	handler := cachingServer(t, server.CachePolicy{ContentSharedMaxAge: 30 * time.Second})

	held := cacheControlAt(t, handler, "/api/locale")

	if held != "private, no-store" {
		t.Errorf("Cache-Control = %q, want the locale answer kept out of shared caches", held)
	}
}
