// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net/http"

	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/gouncer/authkit"
)

// InstalledPlugins answers the plugins the site holds, read fresh on each request.
type InstalledPlugins interface {
	// Installed returns every listed plugin with its state, by name then id.
	Installed(ctx context.Context) ([]pluginkit.InstalledPlugin, error)
}

// pluginListResponse is GET /api/plugin-admin: every plugin the site holds, in an object later fields join.
type pluginListResponse struct {
	Plugins []pluginkit.InstalledPlugin `json:"plugins"`
}

// handlePluginList answers every plugin the site holds with its state, and the step and reason of a pause.
func (s *server) handlePluginList(installed InstalledPlugins) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		plugins, err := installed.Installed(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, pluginListResponse{Plugins: plugins})
	}
}
