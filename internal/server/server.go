// SPDX-License-Identifier: Apache-2.0

// Package server exposes the CMS core over a JSON HTTP API.
package server

import (
	"io/fs"
	"net/http"
	"time"

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
	// Theme serves the public site while it is healthy. Nil leaves the built-in renderer serving.
	Theme Theme
	// Themes is the managed themes directory. Nil leaves the theme routes unhandled.
	Themes Themes
	// ThemeTimeout is how long a theme has to begin answering. Zero applies the default.
	ThemeTimeout time.Duration
	// Version is the application version reported at /api/version.
	Version string
}

// NewServer returns the HTTP handler serving the CMS API. Every route
// requires a login session except login, logout, and each plugin's
// declared public paths.
func NewServer(cfg Config) http.Handler {
	auth := authkit.New(authkit.Config{Store: cfg.Users, CookieName: sessionCookieName})
	admin := authkit.NewAdmin(cfg.Users)
	s := &server{auth: auth, users: cfg.Users, posts: cfg.Posts, themes: cfg.Themes, version: cfg.Version}
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
		if cfg.Themes != nil {
			protected.Get("/api/themes", s.handleThemeList())
			protected.Post("/api/themes", s.handleThemeUpload())
			protected.Post("/api/themes/active", s.handleThemeActivate())
			protected.Delete("/api/themes/active", s.handleThemeDeactivate())
			protected.Post("/api/themes/rollback", s.handleThemeRollback())
		}
	})
	for id, handler := range cfg.Plugins {
		prefix := "/api/plugins/" + id
		guarded := pluginkit.Protect(handler, cfg.PluginPublicPaths[id], auth.RequireSession)
		router.Mount(prefix, http.StripPrefix(prefix, guarded))
	}
	router.With(identify(cfg.Version)).Handle(assetPrefix+"/*", siteAssets(cfg.Web))
	site := builtInSite(cfg)
	renderer := identify(cfg.Version)(site)
	public := identify(cfg.Version)(themedSite(cfg.Theme, site, cfg.ThemeTimeout))
	router.NotFound(fallbackHandler(adminApp(cfg), renderer, public))
	return router
}

type server struct {
	auth    *authkit.Handlers
	users   authkit.AdminStore
	posts   post.Store
	themes  Themes
	version string
}
