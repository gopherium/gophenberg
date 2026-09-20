// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/gopherium/gouncer/authkit"
)

// crossOriginProtection rejects unsafe browser requests that did not originate from the public site.
func crossOriginProtection(trustedProxies []string) func(http.Handler) http.Handler {
	trustedPrefixes := parsePrefixes(trustedProxies)
	deny := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", uncachedControl)
		authkit.RespondError(w, http.StatusForbidden, authkit.ErrorResponse{
			Message: "cross-origin request refused", Code: "request_cross_origin",
		})
	})
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(deny)
	return func(next http.Handler) http.Handler {
		protected := protection.Handler(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if needsOriginFallback(r) {
				if sameRequestOrigin(r, trustedPrefixes) {
					next.ServeHTTP(w, r)
				} else {
					deny.ServeHTTP(w, r)
				}
				return
			}
			protected.ServeHTTP(w, r)
		})
	}
}

// needsOriginFallback reports whether an unsafe request only carries the older Origin signal.
func needsOriginFallback(r *http.Request) bool {
	if r.Header.Get("Sec-Fetch-Site") != "" || r.Header.Get("Origin") == "" {
		return false
	}
	return r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
}

// sameRequestOrigin compares the browser origin with the effective public scheme and host.
func sameRequestOrigin(r *http.Request, trustedPrefixes []netip.Prefix) bool {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || !validOrigin(origin) {
		return false
	}
	scheme, host := publicRequestOrigin(r, trustedPrefixes)
	if scheme == "" {
		return false
	}
	public := &url.URL{Scheme: scheme, Host: host}
	return strings.EqualFold(origin.Scheme, public.Scheme) &&
		strings.EqualFold(origin.Hostname(), public.Hostname()) &&
		effectiveOriginPort(origin) == effectiveOriginPort(public)
}

// effectiveOriginPort returns the explicit port or the default for the scheme.
func effectiveOriginPort(origin *url.URL) string {
	if port := origin.Port(); port != "" {
		return port
	}
	if strings.EqualFold(origin.Scheme, "https") {
		return "443"
	}
	return "80"
}

// validOrigin reports whether parsed is the scheme-and-authority form browsers send in Origin.
func validOrigin(parsed *url.URL) bool {
	return parsed.Scheme != "" && parsed.Host != "" && parsed.User == nil && parsed.Path == "" &&
		parsed.RawQuery == "" && parsed.Fragment == ""
}

// publicRequestOrigin returns the request scheme and host seen by the browser.
// The scheme is unknown when a trusted proxy omits it.
func publicRequestOrigin(r *http.Request, trustedPrefixes []netip.Prefix) (string, string) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	} else if trustedPeer(r.RemoteAddr, trustedPrefixes) {
		scheme = ""
	}
	if forwarded := firstForwarded(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		scheme = forwarded
	}
	host := r.Host
	if forwarded := firstForwarded(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		host = forwarded
	}
	return scheme, host
}

// firstForwarded returns the first value from a comma-separated forwarded header.
func firstForwarded(header string) string {
	value, _, _ := strings.Cut(header, ",")
	return strings.TrimSpace(value)
}
