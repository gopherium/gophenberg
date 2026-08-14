// SPDX-License-Identifier: Apache-2.0

package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/publicsite"
)

// adminPrefix is the URL prefix the admin single-page app is served under.
const adminPrefix = "/admin"

// fallbackHandler routes unhandled paths to the JSON 404, the admin app, the renderer, or the theme.
func fallbackHandler(admin, renderer, public http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if handler := reservedHandler(r.URL.Path, renderer); handler != nil {
			handler.ServeHTTP(w, r)
			return
		}
		switch {
		case r.URL.Path == adminPrefix:
			http.Redirect(w, r, adminPrefix+"/", http.StatusMovedPermanently)
		case strings.HasPrefix(r.URL.Path, adminPrefix+"/"):
			admin.ServeHTTP(w, r)
		default:
			public.ServeHTTP(w, r)
		}
	}
}

// reservedHandler returns what answers a path a theme may never serve, or nothing when it may.
func reservedHandler(path string, renderer http.Handler) http.Handler {
	switch {
	case reserved(path, "/api"), reserved(path, assetPrefix), reserved(path, mediaPrefix):
		return http.HandlerFunc(respondNotFound)
	case reserved(path, themePrefix):
		return renderer
	}
	return nil
}

// servesAFile reports whether name is a file the web root holds.
func servesAFile(webFS fs.FS, name string) bool {
	info, err := fs.Stat(webFS, name)
	return err == nil && !info.IsDir()
}

// reserved reports whether path is prefix itself or sits under it.
func reserved(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// adminApp returns the single-page app, or the JSON 404 when none is configured.
func adminApp(cfg Config) http.Handler {
	if cfg.Web == nil {
		return http.HandlerFunc(respondNotFound)
	}
	return spaHandler(cfg.Web)
}

// builtInSite returns the renderer serving the public site, or the JSON 404 when no store or no
// registry is configured.
func builtInSite(cfg Config, types *content.Registry) http.Handler {
	if cfg.Content == nil || cfg.Types == nil {
		return http.HandlerFunc(respondNotFound)
	}
	return publicsite.New(publicsite.Config{
		Content: cfg.Content,
		Types:   types,
		Title:   cfg.SiteTitle,
		Version: cfg.Version,
	})
}

// respondNotFound reports that nothing lives at the address.
func respondNotFound(w http.ResponseWriter, _ *http.Request) {
	authkit.RespondError(w, http.StatusNotFound, "not found")
}

// spaHandler serves the single-page app from webFS with the admin prefix stripped, index.html
// standing in for paths without a matching file.
func spaHandler(webFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServerFS(webFS)
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(strings.TrimPrefix(path.Clean(r.URL.Path), adminPrefix), "/")
		r = r.Clone(r.Context())
		r.URL.Path = "/" + name
		if name != "" && !servesAFile(webFS, name) {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	}
}
