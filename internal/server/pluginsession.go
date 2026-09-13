// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/sdk"
)

// guardPlugin returns the middleware requiring and filing a session on a plugin route, kept out of every cache.
func guardPlugin(auth *authkit.Handlers) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return readerOnly(auth.RequireSession(fileSession(next)))
	}
}

// fileSession copies the authenticated identity onto the sdk seam, capabilities included.
func fileSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity := authkit.IdentityFromContext(r.Context())
		filed := sdk.NewSession(
			identity.ID, identity.Email, identity.Name, identity.Role,
			role.CapabilitiesOf(identity.Role),
		)
		next.ServeHTTP(w, r.WithContext(sdk.WithSession(r.Context(), filed)))
	})
}
