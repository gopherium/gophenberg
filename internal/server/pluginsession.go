// SPDX-License-Identifier: Apache-2.0

package server

import (
	"log/slog"
	"net/http"

	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/sdk"
)

// routePanicLine is the error the server logs for each panic of a plugin's route.
const routePanicLine = "plugin route panicked"

// pluginRouteSettings returns what every plugin's routes are protected under, logging each panic to logger.
func pluginRouteSettings(auth *authkit.Handlers, logger *slog.Logger) pluginkit.RouteSettings {
	return pluginkit.RouteSettings{
		Wrap:   guardPlugin(auth),
		Answer: http.HandlerFunc(respondInternal),
		Report: func(failure pluginkit.RouteFailure) {
			logger.Error(routePanicLine, "plugin", failure.ID, "method", failure.Method,
				"path", sdk.PluginRoutesPrefix+failure.ID+failure.Path, "answered", failure.Answered,
				"error", failure.Err, "stack", failure.Stack)
		},
	}
}

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
			sdk.ID(identity.ID), identity.Email, identity.Name, identity.Role, identity.Capabilities,
		)
		next.ServeHTTP(w, r.WithContext(sdk.WithSession(r.Context(), filed)))
	})
}
