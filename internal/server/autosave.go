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

// The places an autosave request writes the editor's buffer.
const (
	autosaveTargetPost     = "post"
	autosaveTargetAutosave = "autosave"
)

type autosaveResponse struct {
	Target  string    `json:"target"`
	PostID  uuid.UUID `json:"post_id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Excerpt string    `json:"excerpt"`
	SavedAt time.Time `json:"saved_at"`
}

// handleAutosaveSave returns an http.HandlerFunc storing the editor's buffer.
func (s *server) handleAutosaveSave() http.HandlerFunc {
	type request struct {
		Title   string `json:"title"`
		Content string `json:"content"`
		Excerpt string `json:"excerpt"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed post id")
			return
		}
		req, err := authkit.Decode[request](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed json")
			return
		}
		stored, err := s.posts.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		identity := authkit.IdentityFromContext(r.Context())
		if stored.Title == req.Title && stored.Content == req.Content && stored.Excerpt == req.Excerpt {
			if err := s.posts.DeleteAutosave(r.Context(), stored.ID, identity.ID); err != nil {
				respondDomainError(w, err)
				return
			}
			authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetPost, stored.ID,
				stored.Title, stored.Content, stored.Excerpt, stored.UpdatedAt))
			return
		}
		buffer := stored
		buffer.Title, buffer.Content, buffer.Excerpt = req.Title, req.Content, req.Excerpt
		if stored.Status == post.StatusDraft && stored.AuthorID == identity.ID {
			s.saveBufferToPost(w, r, buffer, stored.UpdatedAt)
			return
		}
		s.parkBufferAsAutosave(w, r, buffer, identity.ID)
	}
}

// saveBufferToPost writes the buffer to the post itself.
func (s *server) saveBufferToPost(w http.ResponseWriter, r *http.Request, buffer post.Post, expected time.Time) {
	buffer.UpdatedAt = time.Now().UTC()
	updated, err := s.posts.Update(r.Context(), buffer, expected, nil, 0)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetPost, updated.ID,
		updated.Title, updated.Content, updated.Excerpt, updated.UpdatedAt))
}

// parkBufferAsAutosave stores the buffer as the author's autosave.
func (s *server) parkBufferAsAutosave(w http.ResponseWriter, r *http.Request, buffer post.Post, authorID uuid.UUID) {
	autosave, err := post.NewRevision(buffer, post.RevisionKindAutosave, authorID)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	saved, err := s.posts.SaveAutosave(r.Context(), autosave)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetAutosave, saved.PostID,
		saved.Title, saved.Content, saved.Excerpt, saved.CreatedAt))
}

// handleAutosaveGet returns an http.HandlerFunc responding with the requester's autosave.
func (s *server) handleAutosaveGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed post id")
			return
		}
		if _, err := s.posts.ByID(r.Context(), id); err != nil {
			respondDomainError(w, err)
			return
		}
		identity := authkit.IdentityFromContext(r.Context())
		autosave, err := s.posts.Autosave(r.Context(), id, identity.ID)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetAutosave, autosave.PostID,
			autosave.Title, autosave.Content, autosave.Excerpt, autosave.CreatedAt))
	}
}

// newAutosaveResponse builds an autosaveResponse, normalizing the timestamp to UTC.
func newAutosaveResponse(
	target string, postID uuid.UUID, title, content, excerpt string, savedAt time.Time,
) autosaveResponse {
	return autosaveResponse{
		Target:  target,
		PostID:  postID,
		Title:   title,
		Content: content,
		Excerpt: excerpt,
		SavedAt: savedAt.UTC(),
	}
}
