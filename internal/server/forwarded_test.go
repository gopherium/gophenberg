// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gopherium/gophenberg/internal/server"
)

// headerEchoHandler writes the forwarded headers the request carried.
func headerEchoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Header.Get("X-Forwarded-Proto") + "|" +
			r.Header.Get("X-Forwarded-For") + "|" +
			r.Header.Get("X-Forwarded-Host") + "|" +
			r.Header.Get("Forwarded")))
	})
}

// forwardedServer returns a server echoing forwarded headers at a public plugin path.
func forwardedServer(t *testing.T, trusted []string) http.Handler {
	t.Helper()
	return server.NewServer(server.Config{
		Users:             newFakeUserStore(),
		TrustedProxies:    trusted,
		Plugins:           map[string]http.Handler{"echo": headerEchoHandler()},
		PluginPublicPaths: map[string][]string{"echo": {"/headers"}},
	})
}

// forwardedEcho returns what the handler saw for a request from peer carrying forwarded headers.
func forwardedEcho(t *testing.T, handler http.Handler, peer string) string {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/plugins/echo/headers", nil)
	request.RemoteAddr = peer
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	request.Header.Set("X-Forwarded-Host", "evil.example.com")
	request.Header.Set("Forwarded", "proto=https")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	return recorder.Body.String()
}

func TestForwardedHeadersFromAnUntrustedPeerAreDropped(t *testing.T) {
	t.Parallel()

	handler := forwardedServer(t, nil)

	got := forwardedEcho(t, handler, "203.0.113.5:9999")

	if got != "|||" {
		t.Errorf("forwarded headers = %q, want every one dropped", got)
	}
}

func TestForwardedHeadersFromATrustedProxyAreKept(t *testing.T) {
	t.Parallel()

	handler := forwardedServer(t, []string{"10.0.0.0/8"})

	got := forwardedEcho(t, handler, "10.1.2.3:9999")

	if got != "https|203.0.113.9|evil.example.com|proto=https" {
		t.Errorf("forwarded headers = %q, want them kept for a trusted proxy", got)
	}
}

func TestForwardedHeadersFromAPeerOutsideTheTrustedRangeAreDropped(t *testing.T) {
	t.Parallel()

	handler := forwardedServer(t, []string{"10.0.0.0/8"})

	got := forwardedEcho(t, handler, "203.0.113.5:9999")

	if got != "|||" {
		t.Errorf("forwarded headers = %q, want every one dropped", got)
	}
}

func TestForwardedHeadersFromAnUnreadablePeerAreDropped(t *testing.T) {
	t.Parallel()

	handler := forwardedServer(t, []string{"10.0.0.0/8"})

	got := forwardedEcho(t, handler, "not-an-address")

	if got != "|||" {
		t.Errorf("forwarded headers = %q, want every one dropped", got)
	}
}

func TestForwardedHeadersSurviveForATrustedIPv6Proxy(t *testing.T) {
	t.Parallel()

	handler := forwardedServer(t, []string{"2001:db8::/32"})

	got := forwardedEcho(t, handler, "[2001:db8::1]:9999")

	if got != "https|203.0.113.9|evil.example.com|proto=https" {
		t.Errorf("forwarded headers = %q, want them kept for a trusted proxy", got)
	}
}
