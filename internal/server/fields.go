// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// fieldResponse is a field definition as the admin API answers it.
type fieldResponse struct {
	Key       string    `json:"key"`
	Label     string    `json:"label"`
	Kind      string    `json:"kind"`
	RelatesTo string    `json:"relates_to,omitempty"`
	Many      bool      `json:"many"`
	Required  bool      `json:"required"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// fieldListResponse is one type's definition list as the admin API answers it.
type fieldListResponse struct {
	Items []fieldResponse `json:"items"`
}

// newFieldResponse builds a fieldResponse, normalizing timestamps to UTC.
func newFieldResponse(f content.Field) fieldResponse {
	return fieldResponse{
		Key:       f.Key,
		Label:     f.Label,
		Kind:      string(f.Kind),
		RelatesTo: f.RelatesTo,
		Many:      f.Many,
		Required:  f.Required,
		CreatedAt: f.CreatedAt.UTC(),
		UpdatedAt: f.UpdatedAt.UTC(),
	}
}

// handleFieldList returns an http.HandlerFunc listing a type's field definitions.
func (s *server) handleFieldList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := s.types.ByKey(r.Context(), chi.URLParam(r, "key"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]fieldResponse, len(t.Fields))
		for i, f := range t.Fields {
			items[i] = newFieldResponse(f)
		}
		authkit.Respond(w, http.StatusOK, fieldListResponse{Items: items})
	}
}

// handleFieldCreate returns an http.HandlerFunc declaring a field on a type.
func (s *server) handleFieldCreate() http.HandlerFunc {
	type request struct {
		Key       string `json:"key"`
		Label     string `json:"label"`
		Kind      string `json:"kind"`
		RelatesTo string `json:"relates_to"`
		Many      bool   `json:"many"`
		Required  bool   `json:"required"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := authkit.Decode[request](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed json", Code: "body_malformed",
			})
			return
		}
		asked, err := content.NewField(content.Field{
			TypeKey:   chi.URLParam(r, "key"),
			Key:       req.Key,
			Label:     req.Label,
			Kind:      content.FieldKind(req.Kind),
			RelatesTo: req.RelatesTo,
			Many:      req.Many,
			Required:  req.Required,
		})
		if err != nil {
			respondDomainError(w, err)
			return
		}
		created, err := s.types.CreateField(r.Context(), asked)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusCreated, newFieldResponse(created))
	}
}

// handleFieldPatch returns an http.HandlerFunc carrying a field's label and required flag.
func (s *server) handleFieldPatch() http.HandlerFunc {
	type request struct {
		Label    *string `json:"label"`
		Required *bool   `json:"required"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		t, err := s.types.ByKey(r.Context(), chi.URLParam(r, "key"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		stored, err := s.declaredField(t, chi.URLParam(r, "fieldKey"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req, err := authkit.Decode[request](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed json", Code: "body_malformed",
			})
			return
		}
		if req.Label != nil {
			stored.Label = *req.Label
		}
		if req.Required != nil {
			stored.Required = *req.Required
		}
		updated, err := s.types.UpdateField(r.Context(), stored)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newFieldResponse(updated))
	}
}

// handleFieldOrder returns an http.HandlerFunc storing a type's field declaration order.
func (s *server) handleFieldOrder() http.HandlerFunc {
	type request struct {
		Order []string `json:"order"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := authkit.Decode[request](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed json", Code: "body_malformed",
			})
			return
		}
		reordered, err := s.types.ReorderFields(r.Context(), chi.URLParam(r, "key"), req.Order)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]fieldResponse, len(reordered))
		for i, f := range reordered {
			items[i] = newFieldResponse(f)
		}
		authkit.Respond(w, http.StatusOK, fieldListResponse{Items: items})
	}
}

// handleFieldDelete returns an http.HandlerFunc removing a field and its values.
func (s *server) handleFieldDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := s.types.DeleteField(r.Context(), chi.URLParam(r, "key"), chi.URLParam(r, "fieldKey"))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// declaredField returns the type's field carrying the key, or [content.ErrFieldNotFound].
func (s *server) declaredField(t content.Type, key string) (content.Field, error) {
	for _, f := range t.Fields {
		if f.Key == key {
			return f, nil
		}
	}
	return content.Field{}, content.ErrFieldNotFound
}
