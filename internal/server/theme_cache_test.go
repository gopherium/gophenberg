// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// downTheme returns a holder holding a theme that is not serving.
func downTheme() *themehost.Holder {
	holder := themehost.NewHolder()
	holder.Swap(themehost.NewSupervisor(themehost.SupervisorConfig{}))
	return holder
}

// assertCacheControl fails the test unless the answer carries the given Cache-Control.
func assertCacheControl(t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int, want string) {
	t.Helper()

	result := recorder.Result()
	if result.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", result.StatusCode, wantStatus)
	}
	if got := result.Header.Get("Cache-Control"); got != want {
		t.Errorf("Cache-Control = %q, want %q", got, want)
	}
}

func TestTheRendererStaysCacheableWithNoThemeHeld(t *testing.T) {
	t.Parallel()

	handler := themedServer(t, themehost.NewHolder(), nil)

	assertCacheControl(t, doRequest(t, handler, http.MethodGet, "/hello-world", ""), http.StatusOK, "")
}

func TestTheRendererStandingInForADownThemeIsNotStored(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		path   string
		status int
	}{
		{name: "a post", path: "/hello-world", status: http.StatusOK},
		{name: "a redirect to the one address", path: "/hello-world/page/2", status: http.StatusMovedPermanently},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler := themedServer(t, downTheme(), nil)

			assertCacheControl(t, doRequest(t, handler, http.MethodGet, tc.path, ""), tc.status, "no-store")
		})
	}
}

func TestTheRendererCatchingAFailedProxyIsNotStored(t *testing.T) {
	t.Parallel()

	dead := httptest.NewServer(http.NotFoundHandler())
	target := dead.URL
	dead.Close()
	handler := themedServer(t, stubTheme{target: target, healthy: true}, nil)

	assertCacheControl(t, doRequest(t, handler, http.MethodGet, "/hello-world", ""), http.StatusOK, "no-store")
	assertCacheControl(t, doRequest(t, handler, http.MethodGet, "/hello-world/page/2", ""), http.StatusMovedPermanently,
		"no-store")
}

// unheldTheme is a healthy theme that reports no hold, as after a deactivation.
type unheldTheme struct{ target string }

// Healthy reports that the unheld theme is serving.
func (unheldTheme) Healthy() bool { return true }

// Held reports that no theme is chosen to serve.
func (unheldTheme) Held() bool { return false }

// Target returns the address the unheld theme serves on.
func (u unheldTheme) Target() string { return u.target }

func TestTheRendererCatchingAFailedProxyWithNoThemeHeldStaysCacheable(t *testing.T) {
	t.Parallel()

	dead := httptest.NewServer(http.NotFoundHandler())
	target := dead.URL
	dead.Close()
	handler := themedServer(t, unheldTheme{target: target}, nil)

	assertCacheControl(t, doRequest(t, handler, http.MethodGet, "/hello-world", ""), http.StatusOK, "")
}

func TestAHealthyThemeKeepsItsOwnCacheControl(t *testing.T) {
	t.Parallel()

	upstream := themeUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		_, _ = w.Write([]byte("themed"))
	})
	handler := themedServer(t, stubTheme{target: upstream.URL, healthy: true}, nil)

	assertCacheControl(t, doRequest(t, handler, http.MethodGet, "/hello-world", ""), http.StatusOK,
		"public, max-age=60")
}
