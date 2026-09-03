// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/definitions"
)

// definitionsFilename names the file the definitions download arrives as.
const definitionsFilename = "definitions.json"

// handleDefinitionsExport returns an http.HandlerFunc serving the site's content definitions as a download.
func (s *server) handleDefinitionsExport() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envelope, err := definitions.Export(r.Context(), s.types)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		w.Header().Set("Content-Disposition", `attachment; filename="`+definitionsFilename+`"`)
		authkit.Respond(w, http.StatusOK, envelope)
	}
}
