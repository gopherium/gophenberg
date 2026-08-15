// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// typeResponse is a content type as the admin API answers it.
type typeResponse struct {
	Key           string          `json:"key"`
	SingularLabel string          `json:"singular_label"`
	PluralLabel   string          `json:"plural_label"`
	RouteWord     string          `json:"route_word"`
	Hierarchical  bool            `json:"hierarchical"`
	Revisions     bool            `json:"revisions"`
	RevisionCap   int             `json:"revision_cap"`
	PageKind      string          `json:"page_kind"`
	Default       bool            `json:"default"`
	Active        bool            `json:"active"`
	Fields        []fieldResponse `json:"fields"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// typeListResponse is the registry as the admin API answers it.
type typeListResponse struct {
	Items []typeResponse `json:"items"`
}

// typePatchRequest carries the editable fields, where a nil field is unchanged.
type typePatchRequest struct {
	SingularLabel *string `json:"singular_label"`
	PluralLabel   *string `json:"plural_label"`
	RouteWord     *string `json:"route_word"`
	Hierarchical  *bool   `json:"hierarchical"`
	Revisions     *bool   `json:"revisions"`
	RevisionCap   *int    `json:"revision_cap"`
	PageKind      *string `json:"page_kind"`
	Default       *bool   `json:"default"`
	Active        *bool   `json:"active"`
}

// newTypeResponse builds a typeResponse, normalizing timestamps to UTC.
func newTypeResponse(t content.Type) typeResponse {
	return typeResponse{
		Key:           t.Key,
		SingularLabel: t.SingularLabel,
		PluralLabel:   t.PluralLabel,
		RouteWord:     t.RouteWord,
		Hierarchical:  t.Hierarchical,
		Revisions:     t.Revisions,
		RevisionCap:   t.RevisionCap,
		PageKind:      string(t.PageKind),
		Default:       t.Default,
		Active:        t.Active,
		Fields:        declaredFields(t),
		CreatedAt:     t.CreatedAt.UTC(),
		UpdatedAt:     t.UpdatedAt.UTC(),
	}
}

// declaredFields returns the type's field definitions as the admin API answers them.
func declaredFields(t content.Type) []fieldResponse {
	fields := make([]fieldResponse, len(t.Fields))
	for i, f := range t.Fields {
		fields[i] = newFieldResponse(f)
	}
	return fields
}

// applyTo edits the type, reporting whether anything changed.
func (req typePatchRequest) applyTo(t *content.Type) bool {
	changed := req.applyText(t)
	for field, value := range map[*bool]*bool{
		&t.Hierarchical: req.Hierarchical,
		&t.Revisions:    req.Revisions,
		&t.Default:      req.Default,
		&t.Active:       req.Active,
	} {
		if value != nil && *field != *value {
			*field = *value
			changed = true
		}
	}
	if req.RevisionCap != nil && t.RevisionCap != *req.RevisionCap {
		t.RevisionCap = *req.RevisionCap
		changed = true
	}
	if changed {
		t.UpdatedAt = time.Now().UTC()
	}
	return changed
}

// applyText edits the worded fields, reporting whether any moved.
func (req typePatchRequest) applyText(t *content.Type) bool {
	changed := false
	kind := (*string)(&t.PageKind)
	for field, value := range map[*string]*string{
		&t.SingularLabel: req.SingularLabel,
		&t.PluralLabel:   req.PluralLabel,
		&t.RouteWord:     req.RouteWord,
		kind:             req.PageKind,
	} {
		if value != nil && *field != *value {
			*field = *value
			changed = true
		}
	}
	return changed
}

// handleTypeList returns an http.HandlerFunc listing the registered content types.
func (s *server) handleTypeList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		types, err := s.types.All(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]typeResponse, len(types))
		for i, t := range types {
			items[i] = newTypeResponse(t)
		}
		authkit.Respond(w, http.StatusOK, typeListResponse{Items: items})
	}
}

// handleTypeCreate returns an http.HandlerFunc registering a content type.
func (s *server) handleTypeCreate() http.HandlerFunc {
	type request struct {
		Key           string `json:"key"`
		SingularLabel string `json:"singular_label"`
		PluralLabel   string `json:"plural_label"`
		RouteWord     string `json:"route_word"`
		Hierarchical  bool   `json:"hierarchical"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := authkit.Decode[request](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed json")
			return
		}
		asked, err := content.NewType(req.Key, req.SingularLabel, req.PluralLabel, req.RouteWord)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		asked.Hierarchical = req.Hierarchical
		created, err := s.types.Create(r.Context(), asked)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusCreated, newTypeResponse(created))
	}
}

// handleTypePatch returns an http.HandlerFunc applying a partial edit to a content type.
func (s *server) handleTypePatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stored, err := s.types.ByKey(r.Context(), chi.URLParam(r, "key"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req, err := authkit.Decode[typePatchRequest](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed json")
			return
		}
		if !req.applyTo(&stored) {
			authkit.Respond(w, http.StatusOK, newTypeResponse(stored))
			return
		}
		updated, err := s.types.Update(r.Context(), stored)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newTypeResponse(updated))
	}
}

// handleTypeDelete returns an http.HandlerFunc removing a content type that holds nothing.
func (s *server) handleTypeDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.types.Delete(r.Context(), chi.URLParam(r, "key")); err != nil {
			respondDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
