// SPDX-License-Identifier: Apache-2.0

// Package server exposes the CMS core over a JSON HTTP API.
package server

import (
	"io/fs"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/gopherium/gouncer/authkit"
	"github.com/gopherium/gouncer/authkit/ratelimit"
	"github.com/gopherium/pluginkit"
)

// sessionCookieName scopes the login cookie to this product.
const sessionCookieName = "__Host-gophenberg_session"

// Config carries the stores and plugin surfaces the server serves.
type Config struct {
	Users authkit.AdminStore
	// Plugins maps a plugin id to its HTTP handler.
	Plugins map[string]http.Handler
	// PluginPublicPaths maps a plugin id to its session-exempt paths.
	PluginPublicPaths map[string][]string
	// Web serves the single-page app. Nil leaves non-API paths unhandled.
	Web fs.FS
	// TrustedProxies lists the CIDR ranges trusted to set X-Forwarded-For.
	TrustedProxies []string
	// Version is the application version reported at /api/version.
	Version string
}

// NewServer returns the HTTP handler serving the CMS API. Every route
// requires a login session except login, logout, and each plugin's
// declared public paths.
func NewServer(cfg Config) http.Handler {
	auth := authkit.New(authkit.Config{Store: cfg.Users, CookieName: sessionCookieName})
	admin := authkit.NewAdmin(cfg.Users)
	s := &server{auth: auth, version: cfg.Version}
	router := chi.NewRouter()
	router.With(ratelimit.Middleware(ratelimit.Config{TrustedProxies: cfg.TrustedProxies})).
		Post("/api/auth/login", auth.Login)
	router.Post("/api/auth/logout", auth.Logout)
	router.Group(func(protected chi.Router) {
		protected.Use(auth.RequireSession)
		protected.Get("/api/auth/session", auth.Session)
		protected.Get("/api/users", admin.List)
		protected.Post("/api/users", admin.Create)
		protected.Patch("/api/users/{id}", admin.SetDisabled)
		protected.Get("/api/version", s.handleVersion())
	})
	for id, handler := range cfg.Plugins {
		prefix := "/api/plugins/" + id
		guarded := pluginkit.Protect(handler, cfg.PluginPublicPaths[id], auth.RequireSession)
		router.Mount(prefix, http.StripPrefix(prefix, guarded))
	}
	if cfg.Web != nil {
		router.NotFound(spaHandler(cfg.Web))
	}
	return router
}

type server struct {
	auth    *authkit.Handlers
	version string
}
