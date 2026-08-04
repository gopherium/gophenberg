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

	"github.com/gopherium/gophenberg/internal/post"
)

// sessionCookieName scopes the login cookie to this product.
const sessionCookieName = "__Host-gophenberg_session"

// Config carries the stores and plugin surfaces the server serves.
type Config struct {
	Users authkit.AdminStore
	// Posts persists the content the CMS serves.
	Posts post.Store
	// Plugins maps a plugin id to its HTTP handler.
	Plugins map[string]http.Handler
	// PluginPublicPaths maps a plugin id to its session-exempt paths.
	PluginPublicPaths map[string][]string
	// Web serves the single-page app under the admin base path. Nil leaves the admin paths unhandled.
	Web fs.FS
	// SiteTitle names the public site in its chrome.
	SiteTitle string
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
	s := &server{auth: auth, users: cfg.Users, posts: cfg.Posts, version: cfg.Version}
	router := chi.NewRouter()
	router.Use(trustForwarded(cfg.TrustedProxies))
	router.With(ratelimit.Middleware(ratelimit.Config{TrustedProxies: cfg.TrustedProxies})).
		Post("/api/auth/login", auth.Login)
	router.Post("/api/auth/logout", auth.Logout)
	router.Group(func(content chi.Router) {
		content.Use(contentHeaders)
		content.Get("/api/content/v1", s.handleContentHandshake())
		content.Get("/api/content/v1/posts", s.handleContentList())
		content.Get("/api/content/v1/posts/{type}/{slug}", s.handleContentPost())
	})
	router.Group(func(protected chi.Router) {
		protected.Use(auth.RequireSession)
		protected.Get("/api/auth/session", auth.Session)
		protected.Get("/api/users", admin.List)
		protected.Post("/api/users", admin.Create)
		protected.Patch("/api/users/{id}", admin.SetDisabled)
		protected.Get("/api/posts", s.handlePostList())
		protected.Post("/api/posts", s.handlePostCreate())
		protected.Get("/api/posts/counts", s.handlePostCounts())
		protected.Get("/api/posts/{id}", s.handlePostGet())
		protected.Patch("/api/posts/{id}", s.handlePostPatch())
		protected.Delete("/api/posts/{id}", s.handlePostDelete())
		protected.Post("/api/posts/{id}/restore", s.handlePostRestore())
		protected.Post("/api/posts/{id}/autosave", s.handleAutosaveSave())
		protected.Get("/api/posts/{id}/autosave", s.handleAutosaveGet())
		protected.Get("/api/posts/{id}/revisions", s.handleRevisionList())
		protected.Get("/api/posts/{id}/revisions/{revisionID}", s.handleRevisionGet())
		protected.Delete("/api/posts/{id}/revisions/{revisionID}", s.handleRevisionDelete())
		protected.Get("/api/version", s.handleVersion())
	})
	for id, handler := range cfg.Plugins {
		prefix := "/api/plugins/" + id
		guarded := pluginkit.Protect(handler, cfg.PluginPublicPaths[id], auth.RequireSession)
		router.Mount(prefix, http.StripPrefix(prefix, guarded))
	}
	router.NotFound(fallbackHandler(adminApp(cfg), publicSite(cfg)))
	return router
}

type server struct {
	auth    *authkit.Handlers
	users   authkit.AdminStore
	posts   post.Store
	version string
}
