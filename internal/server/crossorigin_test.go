// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/server"
)

// crossOriginServer returns a server with a session-exempt plugin write and the number of times it ran.
func crossOriginServer(trustedProxies ...string) (http.Handler, *int) {
	calls := 0
	plugin := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	return server.NewServer(server.Config{
		Users:             newFakeUserStore(),
		Plugins:           map[string]http.Handler{"form": plugin},
		PluginPublicPaths: map[string][]string{"form": {"/submit"}},
		TrustedProxies:    trustedProxies,
	}), &calls
}

// browserRequest builds a request as received by a site served at cms.example.
func browserRequest(method, path, fetchSite, origin string) *http.Request {
	request := httptest.NewRequest(method, path, nil)
	request.Host = "cms.example"
	if fetchSite != "" {
		request.Header.Set("Sec-Fetch-Site", fetchSite)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	return request
}

func TestCrossOriginBrowserWritesAreRefused(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		fetchSite string
		origin    string
		https     bool
	}{
		{name: "cross site", fetchSite: "cross-site", origin: "https://attacker.example"},
		{name: "sibling subdomain", fetchSite: "same-site", origin: "https://sibling.example"},
		{name: "origin fallback", origin: "https://attacker.example"},
		{name: "insecure origin fallback", origin: "http://cms.example", https: true},
		{name: "opaque origin", origin: "null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, calls := crossOriginServer()
			request := browserRequest(http.MethodPost, "/api/plugins/form/submit", tc.fetchSite, tc.origin)
			if tc.https {
				request.TLS = &tls.ConnectionState{}
			}
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
			}
			if *calls != 0 {
				t.Fatalf("plugin writes = %d, want none", *calls)
			}
			if control := recorder.Header().Get("Cache-Control"); control != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", control)
			}
			var body struct {
				Code string `json:"code"`
			}
			if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
				t.Fatalf("decoding response body: %v", err)
			}
			if body.Code != "request_cross_origin" {
				t.Errorf("error code = %q, want request_cross_origin", body.Code)
			}
		})
	}
}

func TestCrossOriginProtectionKeepsLegitimateTraffic(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		method    string
		fetchSite string
		origin    string
		https     bool
	}{
		{name: "same origin browser write", method: http.MethodPost, fetchSite: "same-origin", origin: "https://cms.example"},
		{name: "same origin fallback", method: http.MethodPost, origin: "https://cms.example", https: true},
		{name: "same origin fallback over plain http", method: http.MethodPost, origin: "http://cms.example"},
		{name: "same origin fallback with TLS ended upstream", method: http.MethodPost, origin: "https://cms.example"},
		{name: "server write", method: http.MethodPost},
		{name: "cross origin read", method: http.MethodGet, fetchSite: "cross-site", origin: "https://reader.example"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, calls := crossOriginServer()
			request := browserRequest(tc.method, "/api/plugins/form/submit", tc.fetchSite, tc.origin)
			if tc.https {
				request.TLS = &tls.ConnectionState{}
			}
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
			}
			if *calls != 1 {
				t.Errorf("plugin calls = %d, want one", *calls)
			}
		})
	}
}

func TestCrossOriginFallbackUsesTrustedForwardedOrigin(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name          string
		origin        string
		forwardedHost string
		wantStatus    int
		wantCalls     int
	}{
		{
			name: "matching host", origin: "https://cms.example", forwardedHost: "cms.example",
			wantStatus: http.StatusNoContent, wantCalls: 1,
		},
		{
			name: "forwarded default port", origin: "https://cms.example", forwardedHost: "cms.example:443",
			wantStatus: http.StatusNoContent, wantCalls: 1,
		},
		{
			name: "origin default port", origin: "https://cms.example:443", forwardedHost: "cms.example",
			wantStatus: http.StatusNoContent, wantCalls: 1,
		},
		{
			name: "different non-default port", origin: "https://cms.example", forwardedHost: "cms.example:444",
			wantStatus: http.StatusForbidden,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, calls := crossOriginServer("192.0.2.0/24")
			request := browserRequest(http.MethodPost, "/api/plugins/form/submit", "", tc.origin)
			request.Host = "gophenberg:8081"
			request.RemoteAddr = "192.0.2.10:1234"
			request.Header.Set("X-Forwarded-Host", tc.forwardedHost)
			request.Header.Set("X-Forwarded-Proto", "https")
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tc.wantStatus)
			}
			if *calls != tc.wantCalls {
				t.Errorf("plugin calls = %d, want %d", *calls, tc.wantCalls)
			}
		})
	}
}

func TestCrossOriginFallbackIgnoresForwardedHeadersFromAnUntrustedPeer(t *testing.T) {
	t.Parallel()

	handler, calls := crossOriginServer()
	request := browserRequest(http.MethodPost, "/api/plugins/form/submit", "", "https://cms.example")
	request.Host = "gophenberg:8081"
	request.RemoteAddr = "203.0.113.5:1234"
	request.Header.Set("X-Forwarded-Host", "cms.example")
	request.Header.Set("X-Forwarded-Proto", "https")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if *calls != 0 {
		t.Errorf("plugin calls = %d, want none", *calls)
	}
}

func TestCrossOriginLogoutCannotEndASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(server.Config{Users: users})
	cookie := loginCookie(t, handler)
	logout := browserRequest(http.MethodPost, "/api/auth/logout", "same-site", "https://sibling.example")
	logout.AddCookie(cookie)
	logoutRecorder := httptest.NewRecorder()

	handler.ServeHTTP(logoutRecorder, logout)

	if logoutRecorder.Code != http.StatusForbidden {
		t.Fatalf("logout status = %d, want %d", logoutRecorder.Code, http.StatusForbidden)
	}
	session := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	session.AddCookie(cookie)
	sessionRecorder := httptest.NewRecorder()
	handler.ServeHTTP(sessionRecorder, session)
	if sessionRecorder.Code != http.StatusOK {
		t.Fatalf("session status after refused logout = %d, want %d", sessionRecorder.Code, http.StatusOK)
	}
	if !strings.Contains(sessionRecorder.Body.String(), "ada@example.com") {
		t.Errorf("session body = %q, want the logged-in user", sessionRecorder.Body.String())
	}
}
