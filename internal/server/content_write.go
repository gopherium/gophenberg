// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// contentPatchRequest carries the version the edit was prepared against and the
// editable fields, where a nil field is unchanged.
type contentPatchRequest struct {
	UpdatedAt *time.Time `json:"updated_at"`
	Title     *string    `json:"title"`
	Content   *string    `json:"content"`
	Excerpt   *string    `json:"excerpt"`
	Slug      *string    `json:"slug"`
	Status    *string    `json:"status"`
}

// applyTo edits the item, reporting whether anything changed and whether the snapshotted fields did.
func (req contentPatchRequest) applyTo(c *content.Content) (changed, contentChanged bool, err error) {
	contentChanged = req.applySnapshotted(c)
	changed = contentChanged
	if req.Slug != nil {
		if slug := content.Slugify(*req.Slug); slug != c.Slug {
			c.Slug = slug
			changed = true
		}
	}
	transitioned, err := applyContentStatus(c, req.Status)
	if err != nil {
		return false, false, err
	}
	if transitioned {
		return true, contentChanged, nil
	}
	if changed {
		c.UpdatedAt = time.Now().UTC()
	}
	return changed, contentChanged, nil
}

// applySnapshotted edits the revisioned fields, reporting whether any moved.
func (req contentPatchRequest) applySnapshotted(c *content.Content) bool {
	changed := false
	for field, value := range map[*string]*string{
		&c.Title:   req.Title,
		&c.Content: req.Content,
		&c.Excerpt: req.Excerpt,
	} {
		if value != nil && *field != *value {
			*field = *value
			changed = true
		}
	}
	return changed
}

// applyContentStatus transitions the item to the requested status, reporting
// whether the status moved.
func applyContentStatus(c *content.Content, raw *string) (bool, error) {
	if raw == nil {
		return false, nil
	}
	status, err := content.ParseStatus(*raw)
	if err != nil {
		return false, err
	}
	if status == c.Status {
		return false, nil
	}
	if err := c.Transition(status); err != nil {
		return false, err
	}
	return true, nil
}

// handleContentCreate returns an http.HandlerFunc storing a draft authored by the requester.
func (s *server) handleContentCreate() http.HandlerFunc {
	type request struct {
		Type  string `json:"type"`
		Title string `json:"title"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := authkit.Decode[request](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed json")
			return
		}
		contentType, err := s.typeAsked(r, req.Type)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		identity := authkit.IdentityFromContext(r.Context())
		c, err := content.New(contentType, req.Title, identity.ID)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		created, err := s.content.Create(r.Context(), c)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondContent(w, r, http.StatusCreated, created)
	}
}

// handleContentPatch returns an http.HandlerFunc applying a partial edit to an item.
func (s *server) handleContentPatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed content id")
			return
		}
		req, err := authkit.Decode[contentPatchRequest](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed json")
			return
		}
		if req.UpdatedAt == nil {
			authkit.RespondError(w, http.StatusBadRequest, "missing updated_at")
			return
		}
		updated, err := s.patchContent(r, id, *req.UpdatedAt, req)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondContent(w, r, http.StatusOK, updated)
	}
}

// versionedContent returns the stored item when it still holds version, or [content.ErrConflict].
func (s *server) versionedContent(r *http.Request, id uuid.UUID, version time.Time) (content.Content, error) {
	stored, err := s.content.ByID(r.Context(), id)
	if err != nil {
		return content.Content{}, err
	}
	if !version.Equal(stored.UpdatedAt) {
		return content.Content{}, content.ErrConflict
	}
	return stored, nil
}

// patchContent applies the edit to the stored item, snapshotting a revision when
// the type keeps them.
func (s *server) patchContent(
	r *http.Request, id uuid.UUID, version time.Time, req contentPatchRequest,
) (content.Content, error) {
	stored, err := s.versionedContent(r, id, version)
	if err != nil {
		return content.Content{}, err
	}
	previous := stored
	changed, contentChanged, err := req.applyTo(&stored)
	if err != nil {
		return content.Content{}, err
	}
	if !changed {
		return stored, nil
	}
	snapshot, revisionCap, err := s.revisionFor(r, previous, contentChanged)
	if err != nil {
		return content.Content{}, err
	}
	return s.content.Update(r.Context(), stored, previous.UpdatedAt, snapshot, revisionCap)
}

// revisionFor snapshots the previous item when its type keeps revisions and
// the snapshotted fields changed.
func (s *server) revisionFor(
	r *http.Request, previous content.Content, contentChanged bool,
) (*content.Revision, int, error) {
	contentType, err := s.types.Active(r.Context(), previous.Type)
	if err != nil {
		return nil, 0, err
	}
	if !contentChanged || !contentType.Revisions {
		return nil, 0, nil
	}
	identity := authkit.IdentityFromContext(r.Context())
	revision, err := content.NewRevision(previous, content.RevisionKindRevision, identity.ID)
	if err != nil {
		return nil, 0, err
	}
	return &revision, contentType.RevisionCap, nil
}

// typeAsked returns the active type the request names, or the registry's default when it names none.
func (s *server) typeAsked(r *http.Request, key string) (content.Type, error) {
	if key == "" {
		return s.types.Default(r.Context())
	}
	asked, err := s.types.Active(r.Context(), key)
	if errors.Is(err, content.ErrTypeNotFound) {
		return content.Type{}, content.ErrInvalidType
	}
	return asked, err
}

// handleContentDelete returns an http.HandlerFunc trashing an item, or removing it
// outright when the request forces it.
func (s *server) handleContentDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed content id")
			return
		}
		if r.URL.Query().Get("force") == "true" {
			if err := s.content.Delete(r.Context(), id); err != nil {
				respondDomainError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		trashed, err := s.content.Trash(r.Context(), id, time.Now().UTC())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondContent(w, r, http.StatusOK, trashed)
	}
}

// handleContentRestore returns an http.HandlerFunc returning a trashed item to draft.
func (s *server) handleContentRestore() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed content id")
			return
		}
		stored, err := s.content.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if err := stored.Restore(); err != nil {
			respondDomainError(w, err)
			return
		}
		restored, err := s.content.Restore(r.Context(), id, stored.UpdatedAt)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondContent(w, r, http.StatusOK, restored)
	}
}
