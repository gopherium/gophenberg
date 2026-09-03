// SPDX-License-Identifier: Apache-2.0

package server

import (
	"io/fs"
	"net/http"
)

// assetPrefix is the URL prefix the site's own stylesheets and icon are served under.
const assetPrefix = "/gophenberg"

// assetDir is the directory of the web root holding those files.
const assetDir = "gophenberg"

// siteAssets returns the handler serving the stylesheets and icon a public page loads.
func siteAssets(webFS fs.FS, header string) http.Handler {
	assets, err := assetFS(webFS)
	if err != nil {
		return http.HandlerFunc(respondNotFound)
	}
	return http.StripPrefix(assetPrefix+"/", cacheAssets(http.FileServerFS(assets), header))
}

// assetFS returns the subdirectory of the web root holding the site's assets.
func assetFS(webFS fs.FS) (fs.FS, error) {
	if webFS == nil {
		return nil, fs.ErrNotExist
	}
	return fs.Sub(webFS, assetDir)
}

// cacheAssets returns next with its responses marked cacheable.
func cacheAssets(next http.Handler, header string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", header)
		next.ServeHTTP(w, r)
	})
}
