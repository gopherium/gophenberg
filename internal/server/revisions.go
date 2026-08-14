// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
)

type revisionResponse struct {
	ID         uuid.UUID `json:"id"`
	ContentID  uuid.UUID `json:"content_id"`
	Kind       string    `json:"kind"`
	AuthorID   uuid.UUID `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Title      string    `json:"title"`
	Excerpt    string    `json:"excerpt"`
	CreatedAt  time.Time `json:"created_at"`
}

type revisionDetailResponse struct {
	revisionResponse
	Content string         `json:"content"`
	Fields  content.Values `json:"fields"`
}

type revisionListResponse struct {
	Items []revisionResponse `json:"items"`
}

// newRevisionResponse builds a revisionResponse, normalizing the timestamp to UTC.
func newRevisionResponse(r content.Revision, authorName string) revisionResponse {
	return revisionResponse{
		ID:         r.ID,
		ContentID:  r.ContentID,
		Kind:       string(r.Kind),
		AuthorID:   r.AuthorID,
		AuthorName: authorName,
		Title:      r.Title,
		Excerpt:    r.Excerpt,
		CreatedAt:  r.CreatedAt.UTC(),
	}
}

// handleRevisionList returns an http.HandlerFunc listing an item's revisions newest first.
func (s *server) handleRevisionList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, "malformed content id")
			return
		}
		if _, err := s.content.ByID(r.Context(), contentID); err != nil {
			respondDomainError(w, err)
			return
		}
		revisions, err := s.content.Revisions(r.Context(), contentID)
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
		contentID, revisionID, ok := parseRevisionIDs(w, r)
		if !ok {
			return
		}
		revision, err := s.content.RevisionByID(r.Context(), contentID, revisionID)
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
			Fields:           heldValues(revision.Fields),
		})
	}
}

// handleRevisionDelete returns an http.HandlerFunc removing one revision.
func (s *server) handleRevisionDelete() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentID, revisionID, ok := parseRevisionIDs(w, r)
		if !ok {
			return
		}
		if err := s.content.DeleteRevision(r.Context(), contentID, revisionID); err != nil {
			respondDomainError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// parseRevisionIDs reads the content and revision ids from the URL, reporting whether both parsed.
func parseRevisionIDs(w http.ResponseWriter, r *http.Request) (uuid.UUID, uuid.UUID, bool) {
	contentID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		authkit.RespondError(w, http.StatusBadRequest, "malformed content id")
		return uuid.Nil, uuid.Nil, false
	}
	revisionID, err := uuid.Parse(chi.URLParam(r, "revisionID"))
	if err != nil {
		authkit.RespondError(w, http.StatusBadRequest, "malformed revision id")
		return uuid.Nil, uuid.Nil, false
	}
	return contentID, revisionID, true
}
