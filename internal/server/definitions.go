// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"net/http"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/definitions"
)

// definitionsFilename names the file the definitions download arrives as.
const definitionsFilename = "definitions.json"

// DefaultDefinitionsImportCap is how large a definitions file an import takes when nothing else is named.
const DefaultDefinitionsImportCap int64 = 256 << 10

// MaxDefinitionsImportCap is the largest definitions cap the request body reader can carry.
const MaxDefinitionsImportCap int64 = authkit.MaxRequestBodyBytes

// definitionsCapOf returns the definitions import cap the configuration names, or the default when it names none.
func definitionsCapOf(cfg Config) int64 {
	if cfg.DefinitionsImportCap == 0 {
		return DefaultDefinitionsImportCap
	}
	return cfg.DefinitionsImportCap
}

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

// handleDefinitionsPlan returns an http.HandlerFunc reporting what a definitions file would change, changing nothing.
func (s *server) handleDefinitionsPlan() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		envelope, err := decodeKnownCapped[definitions.Envelope](w, r, s.definitionsCap)
		if err != nil {
			s.respondImportBodyError(w, err)
			return
		}
		plan, err := definitions.Compare(r.Context(), s.types, envelope)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, plan)
	}
}

// respondImportBodyError writes a refused definitions body, naming the cap when the file ran past it.
func (s *server) respondImportBodyError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		authkit.RespondError(w, http.StatusRequestEntityTooLarge, authkit.ErrorResponse{
			Message: "the definitions file is too large", Code: "definitions_too_large",
			Meta: map[string]any{"max": s.definitionsCap},
		})
		return
	}
	respondBodyError(w, err)
}
