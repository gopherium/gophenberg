// SPDX-License-Identifier: Apache-2.0

package server

import (
	"log/slog"
	"net/http"
	"net/netip"
	"net/url"
	"strings"

	"github.com/gopherium/gouncer/authkit"
)

// The reasons a write is refused, as the server logs them.
const (
	refusedFetchSite = "fetch-site"
	refusedOrigin    = "origin"
	refusedScheme    = "scheme"
	refusedHost      = "host"
)

// originGuard judges unsafe requests against the site they were sent to and logs every one it refuses.
type originGuard struct {
	trusted []netip.Prefix
	public  *url.URL
	logger  *slog.Logger
}

// crossOriginProtection rejects unsafe browser requests that did not originate from the public site.
func crossOriginProtection(
	trustedProxies []string, public *url.URL, logger *slog.Logger,
) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	guard := originGuard{trusted: parsePrefixes(trustedProxies), public: public, logger: logger}
	protection := http.NewCrossOriginProtection()
	protection.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guard.refuse(w, r, refusedFetchSite)
	}))
	return func(next http.Handler) http.Handler {
		protected := protection.Handler(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reason, settled := guard.judge(r)
			switch {
			case reason != "":
				guard.refuse(w, r, reason)
			case settled:
				next.ServeHTTP(w, r)
			default:
				protected.ServeHTTP(w, r)
			}
		})
	}
}

// judge returns why the guard refuses the request, or whether it settles the request as the site's own.
func (g originGuard) judge(r *http.Request) (string, bool) {
	if !unsafeMethod(r.Method) {
		return "", false
	}
	if g.public != nil {
		return g.againstPublic(r)
	}
	if needsOriginFallback(r) {
		return g.againstRequest(r), true
	}
	return "", false
}

// againstPublic returns why a write is refused against the public address, or whether its origin settles it.
func (g originGuard) againstPublic(r *http.Request) (string, bool) {
	if !sameOrigin(&url.URL{Scheme: g.public.Scheme, Host: requestHost(r)}, g.public) {
		return refusedHost, true
	}
	raw := r.Header.Get("Origin")
	if raw == "" {
		return "", false
	}
	origin, err := url.Parse(raw)
	if err != nil || !validOrigin(origin) || !sameOrigin(origin, g.public) {
		return refusedOrigin, true
	}
	return "", r.Header.Get("Sec-Fetch-Site") == ""
}

// againstRequest returns why an older browser's write is refused against the origin the request names, or nothing.
func (g originGuard) againstRequest(r *http.Request) string {
	origin, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || !validOrigin(origin) {
		return refusedOrigin
	}
	scheme, host := publicRequestOrigin(r, g.trusted)
	if scheme == "" {
		return refusedScheme
	}
	if !sameOrigin(origin, &url.URL{Scheme: scheme, Host: host}) {
		return refusedOrigin
	}
	return ""
}

// refuse answers the request as a refused write and logs why.
func (g originGuard) refuse(w http.ResponseWriter, r *http.Request, reason string) {
	g.logger.WarnContext(r.Context(), "write refused", "reason", reason, "method", r.Method, "path", r.URL.Path,
		"host", requestHost(r), "origin", r.Header.Get("Origin"), "fetch_site", r.Header.Get("Sec-Fetch-Site"))
	w.Header().Set("Cache-Control", uncachedControl)
	authkit.RespondError(w, http.StatusForbidden, authkit.ErrorResponse{
		Message: "cross-origin request refused", Code: "request_cross_origin",
	})
}

// unsafeMethod reports whether the method may change what the server holds.
func unsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

// needsOriginFallback reports whether a request only carries the older Origin signal.
func needsOriginFallback(r *http.Request) bool {
	return r.Header.Get("Sec-Fetch-Site") == "" && r.Header.Get("Origin") != ""
}

// sameOrigin reports whether both addresses name one scheme, host and port.
func sameOrigin(one, other *url.URL) bool {
	return strings.EqualFold(one.Scheme, other.Scheme) &&
		strings.EqualFold(one.Hostname(), other.Hostname()) &&
		effectiveOriginPort(one) == effectiveOriginPort(other)
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
	return scheme, requestHost(r)
}

// requestHost returns the host the browser sent the request to, as a trusted proxy names it or the request carries it.
func requestHost(r *http.Request) string {
	if forwarded := firstForwarded(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		return forwarded
	}
	return r.Host
}

// firstForwarded returns the first value from a comma-separated forwarded header.
func firstForwarded(header string) string {
	value, _, _ := strings.Cut(header, ",")
	return strings.TrimSpace(value)
}
