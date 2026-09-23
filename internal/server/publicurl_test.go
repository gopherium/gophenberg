// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/server"
)

// guardedServer returns a server with a session-exempt plugin write, the times it ran and what it logged.
func guardedServer(t *testing.T, public string, trustedProxies ...string) (http.Handler, *int, *bytes.Buffer) {
	t.Helper()
	address, err := server.ParsePublicURL(public)
	if err != nil {
		t.Fatalf("ParsePublicURL(%q) error = %v, want nil", public, err)
	}
	calls := 0
	plugin := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusNoContent)
	})
	logs := new(bytes.Buffer)
	return server.NewServer(server.Config{
		Users:             newFakeUserStore(),
		Plugins:           map[string]http.Handler{"form": plugin},
		PluginPublicPaths: map[string][]string{"form": {"/submit"}},
		TrustedProxies:    trustedProxies,
		PublicURL:         address,
		Logger:            slog.New(slog.NewTextHandler(logs, nil)),
	}), &calls, logs
}

// sentTo builds a write to the plugin form as it reaches the server at the host from a page at the origin.
func sentTo(host, fetchSite, origin string) *http.Request {
	request := browserRequest(http.MethodPost, "/api/plugins/form/submit", fetchSite, origin)
	request.Host = host
	request.RemoteAddr = "203.0.113.5:1234"
	return request
}

// refusedFor asserts the answer refuses the write and the log names the reason.
func refusedFor(t *testing.T, recorder *httptest.ResponseRecorder, calls int, logs *bytes.Buffer, reason string) {
	t.Helper()
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusForbidden)
	}
	if calls != 0 {
		t.Errorf("plugin writes = %d, want none", calls)
	}
	if !strings.Contains(recorder.Body.String(), `"request_cross_origin"`) {
		t.Errorf("body = %s, want the request_cross_origin code", recorder.Body.String())
	}
	line := logs.String()
	if !strings.Contains(line, `msg="write refused"`) || !strings.Contains(line, "reason="+reason) ||
		!strings.Contains(line, "method=POST") || !strings.Contains(line, "path=/api/plugins/form/submit") {
		t.Errorf("log = %q, want a write refused for the reason %s naming the method and path", line, reason)
	}
}

func TestParsePublicURLLeavesAnEmptyAddressUnset(t *testing.T) {
	t.Parallel()

	address, err := server.ParsePublicURL("")

	if err != nil || address != nil {
		t.Errorf("ParsePublicURL(\"\") = %v, %v, want no address and no error", address, err)
	}
}

func TestParsePublicURLKeepsTheSchemeAndTheHost(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]string{
		"https://cms.example":    "https://cms.example",
		"HTTPS://CMS.Example/":   "https://cms.example",
		"http://localhost:8081":  "http://localhost:8081",
		"http://[::1]:8081":      "http://[::1]:8081",
		"https://cms.example:44": "https://cms.example:44",
	} {
		address, err := server.ParsePublicURL(raw)

		if err != nil || address.String() != want {
			t.Errorf("ParsePublicURL(%q) = %v, %v, want %s", raw, address, err, want)
		}
	}
}

func TestParsePublicURLRefusesWhatNamesNoSiteAddress(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"cms.example", "ftp://cms.example", "https://", "https://visitor@cms.example",
		"https://cms.example/blog", "https://cms.example/?page=1", "https://cms.example/#top",
		"https://cms.example:port", "::not an address", "https://:443", "https://:", "http://:8081/",
	} {
		address, err := server.ParsePublicURL(raw)

		if !errors.Is(err, server.ErrPublicURL) || !strings.Contains(err.Error(), raw) || address != nil {
			t.Errorf("ParsePublicURL(%q) = %v, %v, want ErrPublicURL naming the value", raw, address, err)
		}
	}
}

func TestPublicAddressRefusesAWriteSentElsewhere(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		host      string
		fetchSite string
		origin    string
		reason    string
	}{
		{name: "another domain pointed at the server", host: "rebound.example",
			fetchSite: "same-origin", origin: "https://rebound.example", reason: "host"},
		{name: "the server's own address", host: "10.0.0.5:8081", reason: "host"},
		{name: "the public host on another port", host: "cms.example:8443", reason: "host"},
		{name: "a page on another site", host: "cms.example", origin: "https://elsewhere.example", reason: "origin"},
		{name: "a page on the public host over plain HTTP", host: "cms.example",
			origin: "http://cms.example", reason: "origin"},
		{name: "a page on the public host on another port", host: "cms.example",
			origin: "https://cms.example:8443", reason: "origin"},
		{name: "an opaque origin", host: "cms.example", origin: "null", reason: "origin"},
		{name: "a browser naming another site", host: "cms.example", fetchSite: "cross-site", reason: "fetch-site"},
		{name: "a public origin from a page the browser calls another site", host: "cms.example",
			fetchSite: "cross-site", origin: "https://cms.example", reason: "fetch-site"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, calls, logs := guardedServer(t, "https://cms.example")
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, sentTo(tc.host, tc.fetchSite, tc.origin))

			refusedFor(t, recorder, *calls, logs, tc.reason)
		})
	}
}

func TestPublicAddressKeepsTheSitesOwnWrites(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		host      string
		fetchSite string
		origin    string
	}{
		{name: "a browser on the public page", host: "cms.example", fetchSite: "same-origin",
			origin: "https://cms.example"},
		{name: "an older browser on the public page", host: "cms.example", origin: "https://cms.example"},
		{name: "a server naming no page", host: "cms.example"},
		{name: "the public host with its default port", host: "cms.example:443", origin: "https://cms.example"},
		{name: "the public host in capitals", host: "CMS.Example", origin: "HTTPS://CMS.EXAMPLE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, calls, logs := guardedServer(t, "https://cms.example")
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, sentTo(tc.host, tc.fetchSite, tc.origin))

			if recorder.Code != http.StatusNoContent || *calls != 1 {
				t.Errorf("status = %d, writes = %d, want the write taken once", recorder.Code, *calls)
			}
			if logs.Len() != 0 {
				t.Errorf("log = %q, want nothing logged for a kept write", logs.String())
			}
		})
	}
}

func TestPublicAddressTakesAnOlderBrowserBehindAProxyThatHidesTheScheme(t *testing.T) {
	t.Parallel()

	handler, calls, _ := guardedServer(t, "https://cms.example", "10.0.0.0/8")
	request := sentTo("gophenberg:8081", "", "https://cms.example")
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Set("X-Forwarded-Host", "cms.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent || *calls != 1 {
		t.Errorf("status = %d, writes = %d, want the public page's write taken", recorder.Code, *calls)
	}
}

func TestPublicAddressReadsNoForwardedHostFromAnUntrustedPeer(t *testing.T) {
	t.Parallel()

	handler, calls, logs := guardedServer(t, "https://cms.example")
	request := sentTo("gophenberg:8081", "same-origin", "https://cms.example")
	request.Header.Set("X-Forwarded-Host", "cms.example")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	refusedFor(t, recorder, *calls, logs, "host")
}

func TestPublicAddressComparesAnAddressNamingItsPort(t *testing.T) {
	t.Parallel()

	for host, want := range map[string]int{
		"localhost:8081": http.StatusNoContent,
		"localhost":      http.StatusForbidden,
		"[::1]:8081":     http.StatusForbidden,
	} {
		handler, _, _ := guardedServer(t, "http://localhost:8081")
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, sentTo(host, "", ""))

		if recorder.Code != want {
			t.Errorf("a write sent to %s = %d, want %d", host, recorder.Code, want)
		}
	}
}

func TestPublicAddressLeavesReadsOpen(t *testing.T) {
	t.Parallel()

	handler, calls, logs := guardedServer(t, "https://cms.example")
	request := sentTo("rebound.example", "same-origin", "https://rebound.example")
	request.Method = http.MethodGet
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent || *calls != 1 {
		t.Errorf("status = %d, reads = %d, want the read served", recorder.Code, *calls)
	}
	if logs.Len() != 0 {
		t.Errorf("log = %q, want nothing logged for a read", logs.String())
	}
}

func TestCrossOriginRefusalsAreLoggedWithTheirReason(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		trusted   []string
		fetchSite string
		origin    string
		reason    string
	}{
		{name: "a browser naming another site", fetchSite: "cross-site",
			origin: "https://elsewhere.example", reason: "fetch-site"},
		{name: "an older browser on another site", origin: "https://elsewhere.example", reason: "origin"},
		{name: "a proxy hiding the scheme", trusted: []string{"203.0.113.0/24"},
			origin: "https://cms.example", reason: "scheme"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handler, calls, logs := guardedServer(t, "", tc.trusted...)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, sentTo("cms.example", tc.fetchSite, tc.origin))

			refusedFor(t, recorder, *calls, logs, tc.reason)
		})
	}
}
