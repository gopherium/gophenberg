// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/publicsite"
)

// adminPrefix is the URL prefix the admin single-page app is served under.
const adminPrefix = "/admin"

// fallbackHandler routes unhandled paths to the JSON 404, the admin app, the renderer, or the theme.
func fallbackHandler(admin, renderer, public http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if handler := reservedHandler(r.URL, renderer); handler != nil {
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
func reservedHandler(address *url.URL, renderer http.Handler) http.Handler {
	path, escaped := address.Path, address.EscapedPath()
	switch {
	case reserved(path, "/api"), reserved(path, assetPrefix), reserved(path, mediaPrefix):
		return http.HandlerFunc(respondNotFound)
	case reserved(path, themePrefix) && dotSegments(path):
		return http.HandlerFunc(respondNotFound)
	case reserved(path, themePrefix):
		return renderer
	case reserved(themePath(escaped), themePrefix), reserved(themeFilePath(escaped), themePrefix):
		return http.HandlerFunc(respondNotFound)
	}
	return nil
}

// dotSegments reports whether path holds a "." or ".." segment.
func dotSegments(path string) bool {
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "." || segment == ".." {
			return true
		}
	}
	return false
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

// adminApp returns the single-page app keeping hashed files for assetCache, or the JSON 404 when none is configured.
// dialogWrite is the limit the admin page carries for a busy dialog's write.
func adminApp(cfg Config, assetCache string, dialogWrite time.Duration) http.Handler {
	if cfg.Web == nil {
		return http.HandlerFunc(respondNotFound)
	}
	return spaHandler(cfg.Web, assetCache, dialogWrite)
}

// builtInSite returns the renderer serving the public site, or the JSON 404 when no store or no
// registry is configured.
func builtInSite(cfg Config, types *content.Registry) http.Handler {
	if cfg.Content == nil || cfg.Types == nil {
		return http.HandlerFunc(respondNotFound)
	}
	return publicsite.New(publicsite.Config{
		Content: cfg.Content,
		Locale:  cfg.Settings,
		Types:   types,
		Title:   cfg.SiteTitle,
		Version: cfg.Version,
		Head:    cfg.PluginHead,
	})
}

// respondNotFound reports that nothing lives at the address.
func respondNotFound(w http.ResponseWriter, _ *http.Request) {
	authkit.RespondError(w, http.StatusNotFound, authkit.ErrorResponse{
		Message: "not found", Code: "route_not_found",
	})
}

// spaHandler serves the single-page app from webFS with the admin prefix stripped, index.html
// standing in for paths without a matching file, and hashed files kept for assetCache.
// The admin page carries dialogWrite, how long a busy dialog waits on its write.
func spaHandler(webFS fs.FS, assetCache string, dialogWrite time.Duration) http.HandlerFunc {
	fileServer := http.FileServerFS(webFS)
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(strings.TrimPrefix(path.Clean(r.URL.Path), adminPrefix), "/")
		if adminPageName(webFS, name) && serveAdminPage(w, r, webFS, dialogWrite) {
			return
		}
		r = r.Clone(r.Context())
		r.URL.Path = "/" + name
		if name != "" && !servesAFile(webFS, name) {
			r.URL.Path = "/"
		}
		w.Header().Set("Cache-Control", adminCacheFor(r.URL.Path, assetCache))
		fileServer.ServeHTTP(w, r)
	}
}

// adminPageName reports whether name is answered with the admin page rather than a file of its own.
func adminPageName(webFS fs.FS, name string) bool {
	return name == "" || !servesAFile(webFS, name)
}

// serveAdminPage writes the admin page, with the dialog write limit in it, and reports whether it could be read.
func serveAdminPage(w http.ResponseWriter, r *http.Request, webFS fs.FS, dialogWrite time.Duration) bool {
	page, err := fs.ReadFile(webFS, "index.html")
	if err != nil {
		return false
	}
	body := pageWithDialogLimit(page, dialogWrite)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", adminPageCache)
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(body))
	return true
}

// pageWithDialogLimit returns page with the dialog write limit written just after its head, or at its start
// when it has no head.
func pageWithDialogLimit(page []byte, limit time.Duration) []byte {
	meta := fmt.Sprintf(`<meta name="gophenberg-dialog-write-timeout" content="%d">`, limit.Milliseconds())
	const head = "<head>"
	at := bytes.Index(bytes.ToLower(page), []byte(head))
	written := []byte(meta)
	if at < 0 {
		return append(written, page...)
	}
	at += len(head)
	out := make([]byte, 0, len(page)+len(written))
	out = append(out, page[:at]...)
	out = append(out, written...)
	return append(out, page[at:]...)
}
