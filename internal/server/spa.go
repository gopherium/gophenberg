// SPDX-License-Identifier: Apache-2.0

package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gopherium/gouncer/authkit"
)

// adminPrefix is the URL prefix the admin single-page app is served under.
const adminPrefix = "/admin"

// fallbackHandler routes unhandled paths to the JSON 404, the admin SPA, or the /admin/ redirect.
func fallbackHandler(webFS fs.FS) http.HandlerFunc {
	serveSPA := spaHandler(webFS)
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/"):
			authkit.RespondError(w, http.StatusNotFound, "not found")
		case r.URL.Path == adminPrefix:
			http.Redirect(w, r, adminPrefix+"/", http.StatusMovedPermanently)
		case strings.HasPrefix(r.URL.Path, adminPrefix+"/"):
			serveSPA(w, r)
		default:
			http.Redirect(w, r, adminPrefix+"/", http.StatusFound)
		}
	}
}

// spaHandler serves the single-page app from webFS with the admin prefix stripped, index.html
// standing in for paths without a matching file.
func spaHandler(webFS fs.FS) http.HandlerFunc {
	fileServer := http.FileServerFS(webFS)
	return func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(strings.TrimPrefix(path.Clean(r.URL.Path), adminPrefix), "/")
		r = r.Clone(r.Context())
		r.URL.Path = "/" + name
		if name != "" {
			if _, err := fs.Stat(webFS, name); err != nil {
				r.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(w, r)
	}
}
