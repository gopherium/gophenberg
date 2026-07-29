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

// postPatchRequest carries the editable fields, where a nil field is unchanged.
type postPatchRequest struct {
	Title   *string `json:"title"`
	Content *string `json:"content"`
	Excerpt *string `json:"excerpt"`
	Slug    *string `json:"slug"`
	Status  *string `json:"status"`
}

// applyTo edits the post, reporting whether anything changed and whether the snapshotted fields did.
func (req postPatchRequest) applyTo(p *post.Post) (changed, contentChanged bool, err error) {
	for field, value := range map[*string]*string{
		&p.Title:   req.Title,
		&p.Content: req.Content,
		&p.Excerpt: req.Excerpt,
	} {
		if value != nil && *field != *value {
			*field = *value
			changed, contentChanged = true, true
		}
	}
	if req.Slug != nil {
		if slug := post.Slugify(*req.Slug); slug != p.Slug {
			p.Slug = slug
			changed = true
		}
	}
	if req.Status != nil {
		status, parseErr := post.ParseStatus(*req.Status)
		if parseErr != nil {
			return false, false, parseErr
		}
		if status != p.Status {
			if transitionErr := p.Transition(status); transitionErr != nil {
				return false, false, transitionErr
			}
			return true, contentChanged, nil
		}
	}
	if changed {
		p.UpdatedAt = time.Now().UTC()
	}
	return changed, contentChanged, nil
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
		stored, err := s.posts.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		previous := stored
		changed, contentChanged, err := req.applyTo(&stored)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if !changed {
			s.respondPost(w, r, http.StatusOK, stored)
			return
		}
		var snapshot *post.Revision
		revisionCap := 0
		if postType, ok := post.TypeByName(stored.Type); contentChanged && ok && postType.Revisions {
			identity := authkit.IdentityFromContext(r.Context())
			revision, revisionErr := post.NewRevision(previous, post.RevisionKindRevision, identity.ID)
			if revisionErr != nil {
				respondDomainError(w, revisionErr)
				return
			}
			snapshot = &revision
			revisionCap = postType.RevisionCap
		}
		updated, err := s.posts.Update(r.Context(), stored, snapshot, revisionCap)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.respondPost(w, r, http.StatusOK, updated)
	}
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
