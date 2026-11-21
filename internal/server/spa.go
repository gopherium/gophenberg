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

// pageWithDialogLimit returns page with the dialog write limit written just after its opening head tag.
// A page with no head gets the limit just after a leading doctype, or at its start when it has neither.
func pageWithDialogLimit(page []byte, limit time.Duration) []byte {
	meta := []byte(fmt.Sprintf(`<meta name="gophenberg-dialog-write-timeout" content="%d">`, limit.Milliseconds()))
	if at := headTagEnd(page); at >= 0 {
		return insertInto(page, meta, at)
	}
	if at := leadingDoctypeEnd(page); at >= 0 {
		return insertInto(page, meta, at)
	}
	return insertInto(page, meta, 0)
}

// headTagEnd returns the index just after the first opening head tag, or -1 when the page has none.
// The tag may carry attributes. HTML comments are skipped so a comment that holds "<head>" is not the tag.
// The search folds ASCII letters only, so the index stays in the original page.
func headTagEnd(page []byte) int {
	const open = "<head"
	for i := 0; i+len(open) <= len(page); {
		if after, ok := skipHTMLComment(page, i); ok {
			i = after
			continue
		}
		if !asciiHasPrefix(page[i:], open) {
			i++
			continue
		}
		next := i + len(open)
		if next < len(page) && (page[next] == '>' || isHTMLSpace(page[next])) {
			if end := htmlTagEnd(page, next); end >= 0 {
				return end
			}
		}
		i++
	}
	return -1
}

// leadingDoctypeEnd returns the index just after a doctype that opens the page, or -1 when it has none.
// Leading HTML whitespace and comments are skipped so the doctype is still found; the returned index is
// in the original page, so that whitespace stays put when the meta is written after the doctype.
func leadingDoctypeEnd(page []byte) int {
	const marker = "<!doctype"
	i := 0
	for i < len(page) {
		if isHTMLSpace(page[i]) {
			i++
			continue
		}
		if after, ok := skipHTMLComment(page, i); ok {
			i = after
			continue
		}
		break
	}
	if !asciiHasPrefix(page[i:], marker) {
		return -1
	}
	return htmlTagEnd(page, i+len(marker))
}

// skipHTMLComment reports whether page[from:] opens an HTML comment and returns the index just after it.
// An unclosed comment ends at the end of the page.
func skipHTMLComment(page []byte, from int) (int, bool) {
	if from+3 >= len(page) || page[from] != '<' || page[from+1] != '!' || page[from+2] != '-' || page[from+3] != '-' {
		return from, false
	}
	for i := from + 4; i+2 < len(page); i++ {
		if page[i] == '-' && page[i+1] == '-' && page[i+2] == '>' {
			return i + 3, true
		}
	}
	return len(page), true
}

// htmlTagEnd returns the index just after the tag's closing >, starting at from, or -1 when it never closes.
// A > inside a quoted attribute is not the end of the tag.
func htmlTagEnd(page []byte, from int) int {
	var quote byte
	for i := from; i < len(page); i++ {
		c := page[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return i + 1
		}
	}
	return -1
}

// asciiHasPrefix reports whether page begins with prefix, matching ASCII letters without regard to case.
func asciiHasPrefix(page []byte, prefix string) bool {
	if len(page) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		if asciiFold(page[i]) != prefix[i] {
			return false
		}
	}
	return true
}

// asciiFold returns the ASCII lowercase of b, leaving every other byte as it is.
func asciiFold(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// isHTMLSpace reports whether b is an ASCII space an HTML tag may hold after its name.
func isHTMLSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\r', '\f':
		return true
	default:
		return false
	}
}

// insertInto returns page with extra written at at.
func insertInto(page, extra []byte, at int) []byte {
	out := make([]byte, 0, len(page)+len(extra))
	out = append(out, page[:at]...)
	out = append(out, extra...)
	return append(out, page[at:]...)
}
