// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// themePrefix is the URL prefix a theme keeps to itself, never reachable from outside.
const themePrefix = "/_gophenberg"

// themeHeaderTimeout is how long a theme has to begin answering before the renderer takes over.
const themeHeaderTimeout = 10 * time.Second

// originHeaders are the forwarded values a theme builds public addresses from.
var originHeaders = []string{"X-Forwarded-Proto", "X-Forwarded-Host"}

// forwardedByProxy returns the origin headers surviving on a request, which only a trusted peer set.
func forwardedByProxy(request *http.Request) map[string]string {
	declared := make(map[string]string, len(originHeaders))
	for _, name := range originHeaders {
		if value := request.Header.Get(name); value != "" {
			declared[name] = value
		}
	}
	return declared
}

// Theme is the running theme the public site is served through.
type Theme interface {
	// Healthy reports whether the theme is serving.
	Healthy() bool
	// Target returns the address the theme serves on.
	Target() string
}

// themedSite returns the handler serving the public site through a healthy theme, or the renderer.
func themedSite(theme Theme, renderer http.Handler, wait time.Duration) http.Handler {
	if theme == nil {
		return renderer
	}
	if wait == 0 {
		wait = themeHeaderTimeout
	}
	proxy := themeProxy(theme, renderer, wait)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !theme.Healthy() {
			renderer.ServeHTTP(w, r)
			return
		}
		proxy.ServeHTTP(w, r)
	})
}

// themeProxy returns the proxy passing a request to the theme, falling back before the first byte.
func themeProxy(theme Theme, renderer http.Handler, wait time.Duration) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			declared := forwardedByProxy(request.In)
			request.SetXForwarded()
			for name, value := range declared {
				request.Out.Header.Set(name, value)
			}
			if target, err := url.Parse(theme.Target()); err == nil {
				request.SetURL(target)
			}
		},
		Transport: &http.Transport{ResponseHeaderTimeout: wait},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, _ error) {
			renderer.ServeHTTP(w, r)
		},
	}
}
