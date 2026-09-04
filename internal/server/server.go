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
	"github.com/gopherium/gophenberg/internal/role"
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
	// Registry is the content registry the server shares with the host, built over Types when nil.
	Registry *content.Registry
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
	// Cache is how long each kind of public answer may be kept. A zero window applies its default.
	Cache CachePolicy
	// ThemeTimeout is how long a theme has to begin answering. Zero applies the default.
	ThemeTimeout time.Duration
	// Version is the application version reported at /api/version.
	Version string
	// Settings persists the values the site chooses for itself. Nil leaves the site default unread.
	Settings SiteSettings
	// Readers persists the values one reader chooses. Nil leaves the locale preference unhandled.
	Readers ReaderSettings
	// DefinitionsImportCap is the largest definitions file an import takes. Zero applies its default.
	DefinitionsImportCap int64
}

// registryOf returns the registry the configuration hands over, or one built over its type store.
func registryOf(cfg Config) *content.Registry {
	if cfg.Registry != nil {
		return cfg.Registry
	}
	return content.NewRegistry(cfg.Types)
}

// NewServer returns the HTTP handler serving the CMS API. Every route
// requires a login session except login, logout, and each plugin's
// declared public paths.
func NewServer(cfg Config) http.Handler {
	auth := authkit.New(authkit.Config{
		Store: cfg.Users, CookieName: sessionCookieName, Privileged: role.Privileged(),
	})
	admin := authkit.NewAdmin(authkit.AdminConfig{Store: cfg.Users, Privileged: role.Privileged()})
	s := &server{
		auth: auth, users: cfg.Users, content: cfg.Content, themes: cfg.Themes,
		media: cfg.Media, mediaStore: cfg.MediaStore, version: cfg.Version,
		settings: cfg.Settings, readers: cfg.Readers,
		types:          registryOf(cfg),
		definitionsCap: definitionsCapOf(cfg),
	}
	s.addresses = content.NewResolver(cfg.Content, s.types)
	headers := headersFor(cfg.Cache)
	router := chi.NewRouter()
	router.Use(trustForwarded(cfg.TrustedProxies))
	router.With(ratelimit.Middleware(ratelimit.Config{TrustedProxies: cfg.TrustedProxies})).
		Post("/api/auth/login", auth.Login)
	router.Post("/api/auth/logout", auth.Logout)
	router.Group(func(public chi.Router) {
		public.Use(contentHeaders(headers.content))
		public.Get("/api/locale", s.handleLocaleGet())
		public.Get("/api/content/v1", s.handleContentHandshake())
		public.Get("/api/content/v1/items", s.handlePublishedList())
		public.Get("/api/content/v1/resolve", s.handleContentResolve())
	})
	router.Group(func(protected chi.Router) {
		protected.Use(auth.RequireSession)
		protected.Get("/api/auth/session", auth.Session)
		s.mountOpen(protected, cfg)
	})
	router.Group(func(privileged chi.Router) {
		privileged.Use(auth.RequireSession, auth.RequirePrivilege)
		s.mountAdmin(privileged, admin, cfg)
	})
	for id, handler := range cfg.Plugins {
		prefix := "/api/plugins/" + id
		guarded := pluginkit.Protect(handler, cfg.PluginPublicPaths[id], guardPlugin(auth))
		router.Mount(prefix, http.StripPrefix(prefix, guarded))
	}
	router.With(identify(cfg.Version)).Handle(assetPrefix+"/*", siteAssets(cfg.Web, headers.asset))
	if cfg.MediaFiles != nil {
		router.With(identify(cfg.Version)).Handle(mediaPrefix+"/*", mediaAssets(cfg.MediaFiles, headers.media))
	}
	site := builtInSite(cfg, s.types)
	renderer := identify(cfg.Version)(site)
	public := identify(cfg.Version)(themedSite(cfg.Theme, site, cfg.ThemeTimeout))
	router.NotFound(fallbackHandler(adminApp(cfg), renderer, public))
	return router
}

// mountOpen registers the routes every signed in role reaches.
func (s *server) mountOpen(r chi.Router, cfg Config) {
	if cfg.Types != nil {
		r.Get("/api/types", s.handleTypeList())
		r.Get("/api/groups", s.handleGroupList())
		r.Get("/api/groups/params", s.handleGroupParams())
	}
	r.Get("/api/content", s.handleContentList())
	r.Post("/api/content", s.handleContentCreate())
	r.Get("/api/content/counts", s.handleContentCounts())
	r.Get("/api/content/{id}", s.handleContentGet())
	r.Patch("/api/content/{id}", s.handleContentPatch())
	r.Delete("/api/content/{id}", s.handleContentDelete())
	r.Post("/api/content/{id}/restore", s.handleContentRestore())
	r.Post("/api/content/{id}/autosave", s.handleAutosaveSave())
	r.Get("/api/content/{id}/autosave", s.handleAutosaveGet())
	r.Get("/api/content/{id}/revisions", s.handleRevisionList())
	r.Get("/api/content/{id}/revisions/{revisionID}", s.handleRevisionGet())
	r.Delete("/api/content/{id}/revisions/{revisionID}", s.handleRevisionDelete())
	r.Get("/api/version", s.handleVersion())
	if cfg.Readers != nil {
		r.Patch("/api/locale", s.handleLocalePatch())
	}
	if cfg.Settings != nil {
		r.Get("/api/settings", s.handleSettingsGet())
	}
	if cfg.Media != nil && cfg.MediaStore != nil {
		r.Get("/api/media", s.handleMediaList())
		r.Post("/api/media", s.handleMediaUpload())
		r.Get("/api/media/{id}", s.handleMediaGet())
		r.Patch("/api/media/{id}", s.handleMediaPatch())
		r.Delete("/api/media/{id}", s.handleMediaDelete())
	}
}

// mountAdmin registers the routes only a privileged role reaches.
func (s *server) mountAdmin(r chi.Router, admin *authkit.AdminHandlers, cfg Config) {
	r.Get("/api/users", admin.List)
	r.Post("/api/users", admin.Create)
	r.Patch("/api/users/{id}", admin.SetDisabled)
	r.Put("/api/users/{id}/role", admin.SetRole)
	if cfg.Types != nil {
		r.Post("/api/types", s.handleTypeCreate())
		r.Patch("/api/types/{key}", s.handleTypePatch())
		r.Delete("/api/types/{key}", s.handleTypeDelete())
		r.Post("/api/groups", s.handleGroupCreate())
		r.Put("/api/groups/order", s.handleGroupOrder())
		r.Patch("/api/groups/{id}", s.handleGroupPatch())
		r.Delete("/api/groups/{id}", s.handleGroupDelete())
		r.Post("/api/groups/{id}/fields", s.handleGroupFieldCreate())
		r.Put("/api/groups/{id}/fields/order", s.handleGroupFieldOrder())
		r.Patch("/api/groups/{id}/fields/{fieldKey}", s.handleGroupFieldPatch())
		r.Delete("/api/groups/{id}/fields/{fieldKey}", s.handleGroupFieldDelete())
		r.Post("/api/groups/{id}/fields/{fieldKey}/move", s.handleGroupFieldMove())
		r.Post("/api/groups/{id}/fields/{fieldPath}", s.handleSubFieldCreate())
		r.Delete("/api/groups/{id}/inside/{fieldPath}", s.handleSubFieldDelete())
		r.Put("/api/groups/{id}/inside/{fieldPath}/order", s.handleSubFieldOrder())
		r.Get("/api/definitions", s.handleDefinitionsExport())
		r.Post("/api/definitions/plan", s.handleDefinitionsPlan())
	}
	if cfg.Settings != nil {
		r.Patch("/api/settings", s.handleSettingsPatch())
	}
	if cfg.Themes != nil {
		r.Get("/api/themes", s.handleThemeList())
		r.Post("/api/themes", s.handleThemeUpload())
		r.Post("/api/themes/active", s.handleThemeActivate())
		r.Delete("/api/themes/active", s.handleThemeDeactivate())
		r.Post("/api/themes/rollback", s.handleThemeRollback())
	}
}

type server struct {
	auth       *authkit.Handlers
	users      authkit.AdminStore
	content    content.Store
	types      *content.Registry
	addresses  *content.Resolver
	themes     Themes
	media      MediaLibrary
	mediaStore media.Store
	settings   SiteSettings
	readers    ReaderSettings
	version    string

	definitionsCap int64
}
