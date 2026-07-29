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

type revisionResponse struct {
	ID         uuid.UUID `json:"id"`
	PostID     uuid.UUID `json:"post_id"`
	Kind       string    `json:"kind"`
	AuthorID   uuid.UUID `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Title      string    `json:"title"`
	Excerpt    string    `json:"excerpt"`
	CreatedAt  time.Time `json:"created_at"`
}

type revisionDetailResponse struct {
	revisionResponse
	Content string `json:"content"`
}

type revisionListResponse struct {
	Items []revisionResponse `json:"items"`
}

// newRevisionResponse builds a revisionResponse, normalizing the timestamp to UTC.
func newRevisionResponse(r post.Revision, authorName string) revisionResponse {
	return revisionResponse{
		ID:         r.ID,
		PostID:     r.PostID,
		Kind:       string(r.Kind),
		AuthorID:   r.AuthorID,
		AuthorName: authorName,
		Title:      r.Title,
		Excerpt:    r.Excerpt,
		CreatedAt:  r.CreatedAt.UTC(),
	}
}

// handleRevisionList returns an http.HandlerFunc listing a post's revisions newest first.
func (s *server) handleRevisionList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed post id")
			return
		}
		if _, err := s.posts.ByID(r.Context(), postID); err != nil {
			respondDomainError(w, err)
			return
		}
		revisions, err := s.posts.Revisions(r.Context(), postID)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		names, err := s.authorNames(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		items := make([]revisionResponse, len(revisions))
		for i, revision := range revisions {
			items[i] = newRevisionResponse(revision, names[revision.AuthorID])
		}
		authkit.Respond(w, http.StatusOK, revisionListResponse{Items: items})
	}
}

// handleRevisionGet returns an http.HandlerFunc responding with one revision and its content.
func (s *server) handleRevisionGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, revisionID, ok := parseRevisionIDs(w, r)
		if !ok {
			return
		}
		revision, err := s.posts.RevisionByID(r.Context(), postID, revisionID)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		names, err := s.authorNames(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, revisionDetailResponse{
			revisionResponse: newRevisionResponse(revision, names[revision.AuthorID]),
			Content:          revision.Content,
		})
	}
}

// handleRevisionDelete returns an http.HandlerFunc removing one revision.
func (s *server) handleRevisionDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		postID, revisionID, ok := parseRevisionIDs(w, r)
		if !ok {
			return
		}
		if err := s.posts.DeleteRevision(r.Context(), postID, revisionID); err != nil {
			respondDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// parseRevisionIDs reads the post and revision ids from the URL, reporting whether both parsed.
func parseRevisionIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	postID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		authkit.RespondError(w, http.StatusBadRequest, "malformed post id")
		return uuid.Nil, uuid.Nil, false
	}
	revisionID, err := uuid.Parse(chi.URLParam(r, "revisionID"))
	if err != nil {
		authkit.RespondError(w, http.StatusBadRequest, "malformed revision id")
		return uuid.Nil, uuid.Nil, false
	}
	return postID, revisionID, true
}
