// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/post"
)

// postPatchRequest carries the version the edit was prepared against and the
// editable fields, where a nil field is unchanged.
type postPatchRequest struct {
	UpdatedAt *time.Time `json:"updated_at"`
	Title     *string    `json:"title"`
	Content   *string    `json:"content"`
	Excerpt   *string    `json:"excerpt"`
	Slug      *string    `json:"slug"`
	Status    *string    `json:"status"`
}

// applyTo edits the post, reporting whether anything changed and whether the snapshotted fields did.
func (req postPatchRequest) applyTo(p *post.Post) (changed, contentChanged bool, err error) {
	contentChanged = req.applySnapshotted(p)
	changed = contentChanged
	if req.Slug != nil {
		if slug := post.Slugify(*req.Slug); slug != p.Slug {
			p.Slug = slug
			changed = true
		}
	}
	transitioned, err := applyPostStatus(p, req.Status)
	if err != nil {
		return false, false, err
	}
	if transitioned {
		return true, contentChanged, nil
	}
	if changed {
		p.UpdatedAt = time.Now().UTC()
	}
	return changed, contentChanged, nil
}

// applySnapshotted edits the revisioned fields, reporting whether any moved.
func (req postPatchRequest) applySnapshotted(p *post.Post) bool {
	changed := false
	for field, value := range map[*string]*string{
		&p.Title:   req.Title,
		&p.Content: req.Content,
		&p.Excerpt: req.Excerpt,
	} {
		if value != nil && *field != *value {
			*field = *value
			changed = true
		}
	}
	return changed
}

// applyPostStatus transitions the post to the requested status, reporting
// whether the status moved.
func applyPostStatus(p *post.Post, raw *string) (bool, error) {
	if raw == nil {
		return false, nil
	}
	status, err := post.ParseStatus(*raw)
	if err != nil {
		return false, err
	}
	if status == p.Status {
		return false, nil
	}
	if err := p.Transition(status); err != nil {
		return false, err
	}
	return true, nil
}

// handlePostCreate returns an http.HandlerFunc storing a draft authored by the requester.
func (s *server) handlePostCreate() http.HandlerFunc {
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
		postType := req.Type
		if postType == "" {
			postType = post.TypePost
		}
		identity := authkit.IdentityFromContext(r.Context())
		p, err := post.New(postType, req.Title, identity.ID)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		created, err := s.posts.Create(r.Context(), p)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondPost(w, r, http.StatusCreated, created)
	}
}

// handlePostPatch returns an http.HandlerFunc applying a partial edit to a post.
func (s *server) handlePostPatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed post id")
			return
		}
		req, err := authkit.Decode[postPatchRequest](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed json")
			return
		}
		if req.UpdatedAt == nil {
			authkit.RespondError(w, http.StatusBadRequest, "missing updated_at")
			return
		}
		updated, err := s.patchPost(r, id, *req.UpdatedAt, req)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondPost(w, r, http.StatusOK, updated)
	}
}

// versionedPost returns the stored post when it still holds version, or [post.ErrConflict].
func (s *server) versionedPost(r *http.Request, id uuid.UUID, version time.Time) (post.Post, error) {
	stored, err := s.posts.ByID(r.Context(), id)
	if err != nil {
		return post.Post{}, err
	}
	if !version.Equal(stored.UpdatedAt) {
		return post.Post{}, post.ErrConflict
	}
	return stored, nil
}

// patchPost applies the edit to the stored post, snapshotting a revision when
// the type keeps them.
func (s *server) patchPost(
	r *http.Request, id uuid.UUID, version time.Time, req postPatchRequest,
) (post.Post, error) {
	stored, err := s.versionedPost(r, id, version)
	if err != nil {
		return post.Post{}, err
	}
	previous := stored
	changed, contentChanged, err := req.applyTo(&stored)
	if err != nil {
		return post.Post{}, err
	}
	if !changed {
		return stored, nil
	}
	snapshot, revisionCap, err := revisionFor(r, previous, contentChanged)
	if err != nil {
		return post.Post{}, err
	}
	return s.posts.Update(r.Context(), stored, previous.UpdatedAt, snapshot, revisionCap)
}

// revisionFor snapshots the previous post when its type keeps revisions and
// the snapshotted fields changed.
func revisionFor(r *http.Request, previous post.Post, contentChanged bool) (*post.Revision, int, error) {
	postType, ok := post.TypeByName(previous.Type)
	if !contentChanged || !ok || !postType.Revisions {
		return nil, 0, nil
	}
	identity := authkit.IdentityFromContext(r.Context())
	revision, err := post.NewRevision(previous, post.RevisionKindRevision, identity.ID)
	if err != nil {
		return nil, 0, err
	}
	return &revision, postType.RevisionCap, nil
}

// handlePostDelete returns an http.HandlerFunc trashing a post, or removing it
// outright when the request forces it.
func (s *server) handlePostDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed post id")
			return
		}
		if r.URL.Query().Get("force") == "true" {
			if err := s.posts.Delete(r.Context(), id); err != nil {
				respondDomainError(w, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		trashed, err := s.posts.Trash(r.Context(), id, time.Now().UTC())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondPost(w, r, http.StatusOK, trashed)
	}
}

// handlePostRestore returns an http.HandlerFunc returning a trashed post to draft.
func (s *server) handlePostRestore() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed post id")
			return
		}
		stored, err := s.posts.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if err := stored.Restore(); err != nil {
			respondDomainError(w, err)
			return
		}
		restored, err := s.posts.Restore(r.Context(), id, stored.UpdatedAt)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondPost(w, r, http.StatusOK, restored)
	}
}
