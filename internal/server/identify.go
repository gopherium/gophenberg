// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"

	"github.com/gopherium/gophenberg/internal/version"
)

// apiRelation is the link relation naming the content API of this product.
const apiRelation = "https://gophenberg.org/api"

// identify returns middleware naming what served a public response.
func identify(v string) func(http.Handler) http.Handler {
	generator := version.Generator(v)
	link := "</api/content/v1>; rel=\"" + apiRelation + "\""
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Generator", generator)
			w.Header().Add("Link", link)
			next.ServeHTTP(w, r)
		})
	}
}
