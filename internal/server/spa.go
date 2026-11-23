// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/gopherium/gouncer/authkit"
	"golang.org/x/net/html"

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
func adminApp(cfg Config, assetCache string, dialogWrite time.Duration) http.Handler {
	if cfg.Web == nil {
		return http.HandlerFunc(respondNotFound)
	}
	return spaHandler(cfg.Web, assetCache, dialogLimitMeta(dialogWrite))
}

// dialogLimitMeta returns the element carrying the dialog write limit in whole milliseconds.
func dialogLimitMeta(limit time.Duration) []byte {
	return fmt.Appendf(nil, `<meta name="gophenberg-dialog-write-timeout" content="%d">`, limit.Milliseconds())
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

// spaHandler serves the single-page app from webFS with the admin prefix stripped, index.html with head
// written in standing in for paths without a matching file, and hashed files kept for assetCache.
func spaHandler(webFS fs.FS, assetCache string, head []byte) http.HandlerFunc {
	fileServer := http.FileServerFS(webFS)
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(strings.TrimPrefix(path.Clean(r.URL.Path), adminPrefix), "/")
		standsIn := name == "" || !servesAFile(webFS, name)
		if standsIn && serveAdminPage(w, r, webFS, head) {
			return
		}
		r = r.Clone(r.Context())
		r.URL.Path = "/" + name
		if standsIn {
			r.URL.Path = "/"
		}
		w.Header().Set("Cache-Control", adminCacheFor(r.URL.Path, assetCache))
		fileServer.ServeHTTP(w, r)
	}
}

// serveAdminPage writes index.html with head in it and reports whether the page could be read.
func serveAdminPage(w http.ResponseWriter, r *http.Request, webFS fs.FS, head []byte) bool {
	page, err := fs.ReadFile(webFS, "index.html")
	if err != nil {
		return false
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", adminPageCache)
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(withHead(page, head)))
	return true
}

// withHead returns page with head written just after the doctype, html and head tags it opens with.
func withHead(page, head []byte) []byte {
	at := headOffset(page)
	return slices.Concat(page[:at], head, page[at:])
}

// headOffset returns the offset just after the doctype, html and head tags the page opens with.
func headOffset(page []byte) int {
	tokens := html.NewTokenizer(bytes.NewReader(page))
	offset, at := 0, 0
	for kind := tokens.Next(); kind != html.ErrorToken; kind = tokens.Next() {
		offset += len(tokens.Raw())
		switch {
		case opensThePage(tokens, kind):
			at = offset
		case !blank(tokens, kind):
			return at
		}
	}
	return at
}

// opensThePage reports whether the current token is a closed doctype or an html or head start tag.
func opensThePage(tokens *html.Tokenizer, kind html.TokenType) bool {
	switch kind {
	case html.DoctypeToken:
		return bytes.HasSuffix(tokens.Raw(), []byte(">"))
	case html.StartTagToken, html.SelfClosingTagToken:
		name, _ := tokens.TagName()
		return string(name) == "html" || string(name) == "head"
	}
	return false
}

// byteOrderMark is the mark a UTF-8 page may open with.
const byteOrderMark = string(rune(0xFEFF))

// blank reports whether the current token is a comment or text holding only spaces.
func blank(tokens *html.Tokenizer, kind html.TokenType) bool {
	if kind == html.TextToken {
		return len(bytes.Trim(tokens.Raw(), " \t\n\r\f"+byteOrderMark)) == 0
	}
	return kind == html.CommentToken
}
