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

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
)

// sessionCookieName scopes the login cookie to this product.
const sessionCookieName = "__Host-gophenberg_session"

// Config carries the stores and plugin surfaces the server serves.
type Config struct {
	Users authkit.AdminStore
	// Content persists the content the CMS serves.
	Content content.Store
	// Types persists the content type registry. Nil leaves the registry routes unhandled.
	Types content.TypeStore
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
	// Media is the media library uploads land in. Nil leaves the media routes unhandled.
	Media MediaLibrary
	// MediaStore persists the media library. Nil leaves the media routes unhandled.
	MediaStore media.Store
	// MediaFiles serves the stored uploads publicly. Nil leaves the media prefix reserved but empty.
	MediaFiles fs.FS
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
	s := &server{
		auth: auth, users: cfg.Users, content: cfg.Content, themes: cfg.Themes,
		media: cfg.Media, mediaStore: cfg.MediaStore, version: cfg.Version,
		types: content.NewRegistry(cfg.Types),
	}
	router := chi.NewRouter()
	router.Use(trustForwarded(cfg.TrustedProxies))
	router.With(ratelimit.Middleware(ratelimit.Config{TrustedProxies: cfg.TrustedProxies})).
		Post("/api/auth/login", auth.Login)
	router.Post("/api/auth/logout", auth.Logout)
	router.Group(func(public chi.Router) {
		public.Use(contentHeaders)
		public.Get("/api/content/v1", s.handleContentHandshake())
		public.Get("/api/content/v1/posts", s.handlePublishedList())
		public.Get("/api/content/v1/posts/{type}/{slug}", s.handlePublishedItem())
	})
	router.Group(func(protected chi.Router) {
		protected.Use(auth.RequireSession)
		protected.Get("/api/auth/session", auth.Session)
		protected.Get("/api/users", admin.List)
		protected.Post("/api/users", admin.Create)
		protected.Patch("/api/users/{id}", admin.SetDisabled)
		if cfg.Types != nil {
			protected.Get("/api/types", s.handleTypeList())
			protected.Post("/api/types", s.handleTypeCreate())
			protected.Patch("/api/types/{key}", s.handleTypePatch())
			protected.Delete("/api/types/{key}", s.handleTypeDelete())
		}
		protected.Get("/api/content", s.handleContentList())
		protected.Post("/api/content", s.handleContentCreate())
		protected.Get("/api/content/counts", s.handleContentCounts())
		protected.Get("/api/content/{id}", s.handleContentGet())
		protected.Patch("/api/content/{id}", s.handleContentPatch())
		protected.Delete("/api/content/{id}", s.handleContentDelete())
		protected.Post("/api/content/{id}/restore", s.handleContentRestore())
		protected.Post("/api/content/{id}/autosave", s.handleAutosaveSave())
		protected.Get("/api/content/{id}/autosave", s.handleAutosaveGet())
		protected.Get("/api/content/{id}/revisions", s.handleRevisionList())
		protected.Get("/api/content/{id}/revisions/{revisionID}", s.handleRevisionGet())
		protected.Delete("/api/content/{id}/revisions/{revisionID}", s.handleRevisionDelete())
		protected.Get("/api/version", s.handleVersion())
		if cfg.Media != nil && cfg.MediaStore != nil {
			protected.Get("/api/media", s.handleMediaList())
			protected.Post("/api/media", s.handleMediaUpload())
			protected.Get("/api/media/{id}", s.handleMediaGet())
			protected.Patch("/api/media/{id}", s.handleMediaPatch())
			protected.Delete("/api/media/{id}", s.handleMediaDelete())
		}
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
	if cfg.MediaFiles != nil {
		router.With(identify(cfg.Version)).Handle(mediaPrefix+"/*", mediaAssets(cfg.MediaFiles))
	}
	site := builtInSite(cfg)
	renderer := identify(cfg.Version)(site)
	public := identify(cfg.Version)(themedSite(cfg.Theme, site, cfg.ThemeTimeout))
	router.NotFound(fallbackHandler(adminApp(cfg), renderer, public))
	return router
}

type server struct {
	auth       *authkit.Handlers
	users      authkit.AdminStore
	content    content.Store
	types      *content.Registry
	themes     Themes
	media      MediaLibrary
	mediaStore media.Store
	version    string
}
