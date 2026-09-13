// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gopherium/gouncer/authkit/testkit"

	"github.com/gopherium/gophenberg/internal/server"
)

// cachingServer returns a handler serving assets, media and the content API under the given windows.
func cachingServer(t *testing.T, cache server.CachePolicy) http.Handler {
	t.Helper()
	return cachingServerWith(t, newFakeUserStore(), cache)
}

// cachingServerWith returns a handler serving the given accounts beside the public routes under the windows.
func cachingServerWith(t *testing.T, users *testkit.Store, cache server.CachePolicy) http.Handler {
	t.Helper()
	posts := newFakePostStore()
	return server.NewServer(server.Config{
		Users:      users,
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

func TestAFailedPublicAnswerIsNeverCached(t *testing.T) {
	t.Parallel()

	handler := cachingServer(t, server.CachePolicy{})

	for name, path := range map[string]string{
		"an address nothing holds": "/api/content/v1/resolve?path=/nowhere/at/all",
		"a page size out of range": "/api/content/v1/items?per_page=0",
		"a missing site asset":     "/gophenberg/missing.css",
		"a missing upload":         "/media/missing.jpg",
		"a content path no route":  "/api/content/v1/nowhere",
		"a language path no route": "/api/locale/nowhere",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			if recorder.Code < http.StatusBadRequest {
				t.Fatalf("status = %d, want a failed answer to test against", recorder.Code)
			}
			if held := recorder.Header().Get("Cache-Control"); held != "no-store" {
				t.Errorf("Cache-Control = %q, want no cache to keep a failed answer", held)
			}
		})
	}
}

func TestAPublicAPIPathNoRouteHoldsAnswersTheReservedNotFound(t *testing.T) {
	t.Parallel()

	handler := cachingServer(t, server.CachePolicy{})

	for _, path := range []string{"/api/content/v1/nowhere", "/api/locale/nowhere"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

			if recorder.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want the path answered as not found", recorder.Code)
			}
			if code := errorCode(t, recorder); code != "route_not_found" {
				t.Errorf("code = %q, want the reserved answer every unknown API path gets", code)
			}
		})
	}
}

func TestAMethodThePublicRoutesRefuseIsNeverCached(t *testing.T) {
	t.Parallel()

	handler := cachingServer(t, server.CachePolicy{})

	for _, asked := range []struct{ method, path string }{
		{http.MethodPost, "/api/content/v1/items"},
		{http.MethodHead, "/api/content/v1/items"},
		{http.MethodHead, "/api/content/v1"},
		{http.MethodHead, "/api/locale"},
	} {
		t.Run(asked.method+" "+asked.path, func(t *testing.T) {
			t.Parallel()

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(asked.method, asked.path, nil))

			if recorder.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want the method refused", recorder.Code)
			}
			if held := recorder.Header().Get("Cache-Control"); held != "no-store" {
				t.Errorf("Cache-Control = %q, want no cache to keep a refused method", held)
			}
			if allowed := recorder.Header().Get("Allow"); allowed != http.MethodGet {
				t.Errorf("Allow = %q, want the refusal to name the method the route takes", allowed)
			}
		})
	}
}

// signedInAda logs the stock administrator in and returns the session to send.
func signedInAda(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	recorder := doLogin(t, handler, `{"email":"ada@example.com","password":"correct horse battery"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recorder.Code, http.StatusOK)
	}
	return sessionCookie(t, recorder)
}

func TestAnAuthenticatedAnswerStaysOutOfSharedCaches(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := cachingServerWith(t, users, server.CachePolicy{ContentSharedMaxAge: 30 * time.Second})
	cookie := signedInAda(t, handler)

	for name, path := range map[string]string{
		"the session":      "/api/auth/session",
		"the content list": "/api/content",
		"the type list":    "/api/types",
		"the user list":    "/api/users",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.AddCookie(cookie)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if held := recorder.Header().Get("Cache-Control"); held != "private, no-store" {
				t.Errorf("Cache-Control = %q, want the answer kept out of every cache it does not belong to", held)
			}
		})
	}
}

func TestARefusedSessionIsNeverCached(t *testing.T) {
	t.Parallel()

	handler := cachingServer(t, server.CachePolicy{})

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/content", nil))

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
	if held := recorder.Header().Get("Cache-Control"); held != "no-store" {
		t.Errorf("Cache-Control = %q, want no cache to keep a refused session", held)
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
