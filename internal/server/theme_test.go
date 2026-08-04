// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gopherium/gophenberg/internal/server"
)

// stubTheme stands in for a supervised theme the public site is served through.
type stubTheme struct {
	target  string
	healthy bool
}

// Healthy reports whether the stub theme is serving.
func (s stubTheme) Healthy() bool { return s.healthy }

// Target returns the address the stub theme serves on.
func (s stubTheme) Target() string { return s.target }

// themedServer returns a server serving the public position through the given theme.
func themedServer(t *testing.T, theme server.Theme, trusted []string) http.Handler {
	t.Helper()

	posts := newFakePostStore()
	posts.add(publishedFixture(t, "hello-world", blockMarkup, time.Now().UTC()))
	return server.NewServer(server.Config{
		Users:          newFakeUserStore(),
		Posts:          posts,
		SiteTitle:      "A Test Site",
		Version:        "1.2.3",
		Theme:          theme,
		ThemeTimeout:   150 * time.Millisecond,
		TrustedProxies: trusted,
		Web: fstest.MapFS{
			"index.html": {Data: []byte("<!doctype html><title>Admin</title>")},
		},
	})
}

// themeUpstream returns a stub theme server recording what it was asked.
func themeUpstream(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()

	upstream := httptest.NewServer(handler)
	t.Cleanup(upstream.Close)
	return upstream
}

func TestThemeServesThePublicPositionWhenItIsHealthy(t *testing.T) {
	t.Parallel()

	upstream := themeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("themed answer for " + r.URL.Path))
	})
	handler := themedServer(t, stubTheme{target: upstream.URL, healthy: true}, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/post/hello-world", "")

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want the upstream status %d", recorder.Code, http.StatusCreated)
	}
	if !strings.Contains(recorder.Body.String(), "themed answer for /post/hello-world") {
		t.Errorf("body = %q, want the theme's answer", recorder.Body.String())
	}
	if strings.Contains(recorder.Body.String(), "A Test Site") {
		t.Errorf("body = %q, want the theme rather than the built-in renderer", recorder.Body.String())
	}
}

func TestThemedResponsesKeepTheirOwnLinkHeaders(t *testing.T) {
	t.Parallel()

	upstream := themeUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Add("Link", `</style.css>; rel="preload"`)
		_, _ = w.Write([]byte("themed"))
	})
	handler := themedServer(t, stubTheme{target: upstream.URL, healthy: true}, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/", "")

	links := recorder.Header().Values("Link")
	if len(links) != 2 {
		t.Fatalf("Link headers = %v, want the theme's and the product's", links)
	}
	if got := recorder.Header().Get("X-Generator"); !strings.Contains(got, "Gophenberg") {
		t.Errorf("X-Generator = %q, want the product to name itself", got)
	}
}

func TestThemeNeverSeesForwardedHeadersFromAnUntrustedPeer(t *testing.T) {
	t.Parallel()

	seen := make(chan string, 1)
	upstream := themeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("X-Forwarded-Proto")
		_, _ = w.Write([]byte("themed"))
	})

	handler := themedServer(t, stubTheme{target: upstream.URL, healthy: true}, nil)
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.RemoteAddr = "203.0.113.9:5000"
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if got := <-seen; got != "http" {
		t.Errorf("X-Forwarded-Proto = %q, want the value this server stamped rather than the spoof", got)
	}
}

func TestThemeSeesForwardedHeadersFromATrustedProxy(t *testing.T) {
	t.Parallel()

	seen := make(chan string, 1)
	upstream := themeUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		seen <- r.Header.Get("X-Forwarded-Proto")
		_, _ = w.Write([]byte("themed"))
	})

	handler := themedServer(t, stubTheme{target: upstream.URL, healthy: true}, []string{"10.0.0.0/8"})
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.RemoteAddr = "10.1.2.3:5000"
	handler.ServeHTTP(httptest.NewRecorder(), request)

	if got := <-seen; got != "https" {
		t.Errorf("X-Forwarded-Proto = %q, want the trusted proxy's value", got)
	}
}

func TestTheReservedAddressesNeverReachTheTheme(t *testing.T) {
	t.Parallel()

	upstream := themeUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("themed answer"))
	})
	handler := themedServer(t, stubTheme{target: upstream.URL, healthy: true}, nil)

	cases := []struct {
		path   string
		status int
		want   string
	}{
		{path: "/api/unknown", status: http.StatusNotFound, want: `"error"`},
		{path: "/gophenberg/blocks.css", status: http.StatusNotFound, want: ""},
		{path: "/_gophenberg/health", status: http.StatusNotFound, want: "A Test Site"},
		{path: "/_gophenberg", status: http.StatusNotFound, want: "A Test Site"},
		{path: "/admin/", status: http.StatusOK, want: "<title>Admin</title>"},
	}
	for _, testCase := range cases {
		t.Run(testCase.path, func(t *testing.T) {
			t.Parallel()

			recorder := doRequest(t, handler, http.MethodGet, testCase.path, "")

			if recorder.Code != testCase.status {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.status)
			}
			if strings.Contains(recorder.Body.String(), "themed answer") {
				t.Errorf("body = %q, want it never served by the theme", recorder.Body.String())
			}
			if testCase.want != "" && !strings.Contains(recorder.Body.String(), testCase.want) {
				t.Errorf("body = %q, want it to carry %q", recorder.Body.String(), testCase.want)
			}
		})
	}
}

func TestTheRendererServesWhileTheThemeIsDown(t *testing.T) {
	t.Parallel()

	handler := themedServer(t, stubTheme{target: "", healthy: false}, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/post/hello-world", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the renderer to serve", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "A Published Post") {
		t.Errorf("body = %q, want the built-in renderer", recorder.Body.String())
	}
}

func TestTheRendererServesWhenTheThemeRefusesTheConnection(t *testing.T) {
	t.Parallel()

	dead := httptest.NewServer(http.NotFoundHandler())
	target := dead.URL
	dead.Close()
	handler := themedServer(t, stubTheme{target: target, healthy: true}, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/post/hello-world", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the renderer to serve when the theme is unreachable", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "A Published Post") {
		t.Errorf("body = %q, want the built-in renderer", recorder.Body.String())
	}
}

func TestTheRendererServesWhenTheThemeWithholdsItsHeaders(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	upstream := themeUpstream(t, func(w http.ResponseWriter, _ *http.Request) {
		<-release
		_, _ = w.Write([]byte("too late"))
	})
	handler := themedServer(t, stubTheme{target: upstream.URL, healthy: true}, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/post/hello-world", "")
	close(release)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the renderer to serve when the theme stalls", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "A Published Post") {
		t.Errorf("body = %q, want the built-in renderer", recorder.Body.String())
	}
}
