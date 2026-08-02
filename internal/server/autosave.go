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

// autosaveRequest carries the version the buffer was prepared against and the words it holds.
type autosaveRequest struct {
	UpdatedAt *time.Time `json:"updated_at"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	Excerpt   string     `json:"excerpt"`
}

// decodeAutosaveRequest reads an autosave request, reporting whether it may be used.
func decodeAutosaveRequest(w http.ResponseWriter, r *http.Request) (autosaveRequest, bool) {
	req, err := authkit.Decode[autosaveRequest](w, r)
	if err != nil {
		authkit.RespondError(w, http.StatusBadRequest, "malformed json")
		return autosaveRequest{}, false
	}
	if req.UpdatedAt == nil {
		authkit.RespondError(w, http.StatusBadRequest, "missing updated_at")
		return autosaveRequest{}, false
	}
	return req, true
}

// holdsBuffer reports whether the post already holds the words the buffer carries.
func holdsBuffer(p post.Post, req autosaveRequest) bool {
	return p.Title == req.Title && p.Content == req.Content && p.Excerpt == req.Excerpt
}

// writesInPlace reports whether the buffer may be written straight to the post.
func writesInPlace(p post.Post, authorID uuid.UUID, version time.Time) bool {
	return p.Status == post.StatusDraft && p.AuthorID == authorID && version.Equal(p.UpdatedAt)
}

// handleAutosaveSave returns an http.HandlerFunc storing the editor's buffer.
func (s *server) handleAutosaveSave() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed post id")
			return
		}
		req, ok := decodeAutosaveRequest(w, r)
		if !ok {
			return
		}
		stored, err := s.posts.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		s.storeBuffer(w, r, stored, req, authkit.IdentityFromContext(r.Context()).ID)
	}
}

// storeBuffer writes the buffer to the post, parks it, or leaves words the post already holds.
func (s *server) storeBuffer(
	w http.ResponseWriter, r *http.Request, stored post.Post, req autosaveRequest, authorID uuid.UUID,
) {
	if holdsBuffer(stored, req) {
		if req.UpdatedAt.Equal(stored.UpdatedAt) {
			s.clearParkedBuffer(w, r, stored, authorID)
			return
		}
		authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetAutosave, stored.ID,
			req.Title, req.Content, req.Excerpt, stored.UpdatedAt))
		return
	}
	buffer := stored
	buffer.Title, buffer.Content, buffer.Excerpt = req.Title, req.Content, req.Excerpt
	if writesInPlace(stored, authorID, *req.UpdatedAt) {
		s.saveBufferToPost(w, r, buffer, stored.UpdatedAt)
		return
	}
	s.parkBufferAsAutosave(w, r, buffer, authorID)
}

// clearParkedBuffer removes the author's parked buffer and reports the post as it stands.
func (s *server) clearParkedBuffer(
	w http.ResponseWriter, r *http.Request, p post.Post, authorID uuid.UUID,
) {
	if err := s.posts.DeleteAutosave(r.Context(), p.ID, authorID); err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetPost, p.ID,
		p.Title, p.Content, p.Excerpt, p.UpdatedAt))
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
