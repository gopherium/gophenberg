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

// The places an autosave request writes the editor's buffer.
const (
	autosaveTargetPost     = "post"
	autosaveTargetAutosave = "autosave"
)

type autosaveResponse struct {
	Target    string         `json:"target"`
	ContentID uuid.UUID      `json:"content_id"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Excerpt   string         `json:"excerpt"`
	Fields    content.Values `json:"fields"`
	SavedAt   time.Time      `json:"saved_at"`
}

// autosaveRequest carries the version the buffer was prepared against and the words it holds.
type autosaveRequest struct {
	UpdatedAt *time.Time     `json:"updated_at"`
	Title     string         `json:"title"`
	Content   string         `json:"content"`
	Excerpt   string         `json:"excerpt"`
	Fields    content.Values `json:"fields"`
}

// decodeAutosaveRequest reads an autosave request, reporting whether it may be used.
func decodeAutosaveRequest(w http.ResponseWriter, r *http.Request) (autosaveRequest, bool) {
	req, err := authkit.Decode[autosaveRequest](w, r)
	if err != nil {
		authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
			Message: "malformed json", Code: "body_malformed",
		})
		return autosaveRequest{}, false
	}
	if req.UpdatedAt == nil {
		authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
			Message: "missing updated_at", Code: "body_field_required", Meta: map[string]any{"field": "updated_at"},
		})
		return autosaveRequest{}, false
	}
	return req, true
}

// holdsBuffer reports whether the item already holds the words the buffer carries.
func holdsBuffer(c content.Content, req autosaveRequest) bool {
	return c.Title == req.Title && c.Content == req.Content && c.Excerpt == req.Excerpt &&
		sameValues(c.Fields, bufferValues(c, req))
}

// bufferValues returns the values the buffer holds, keeping the stored ones when it names none.
func bufferValues(c content.Content, req autosaveRequest) content.Values {
	if req.Fields == nil {
		return c.Fields
	}
	return req.Fields
}

// writesInPlace reports whether the buffer may be written straight to the item.
func writesInPlace(c content.Content, authorID uuid.UUID, version time.Time) bool {
	return c.Status == content.StatusDraft && c.AuthorID == authorID && version.Equal(c.UpdatedAt)
}

// handleAutosaveSave returns an http.HandlerFunc storing the editor's buffer.
func (s *server) handleAutosaveSave() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed content id", Code: "content_id_malformed",
			})
			return
		}
		req, ok := decodeAutosaveRequest(w, r)
		if !ok {
			return
		}
		stored, err := s.content.ByID(r.Context(), id)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		scalars, bounded, err := s.bufferedValues(r, stored, req)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		req.Fields = scalars
		s.storeBuffer(w, r, stored, req, authkit.IdentityFromContext(r.Context()).ID, bounded)
	}
}

// storeBuffer writes the buffer to the item, parks it, or leaves words the item already holds.
func (s *server) storeBuffer(
	w http.ResponseWriter, r *http.Request, stored content.Content, req autosaveRequest,
	authorID uuid.UUID, bounded bool,
) {
	if holdsBuffer(stored, req) {
		if req.UpdatedAt.Equal(stored.UpdatedAt) {
			s.clearParkedBuffer(w, r, stored, authorID)
			return
		}
		authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetAutosave, stored.ID,
			req.Title, req.Content, req.Excerpt, bufferValues(stored, req), stored.UpdatedAt))
		return
	}
	buffer := stored
	buffer.Title, buffer.Content, buffer.Excerpt = req.Title, req.Content, req.Excerpt
	buffer.Fields = bufferValues(stored, req)
	if bounded && writesInPlace(stored, authorID, *req.UpdatedAt) {
		s.saveBufferToContent(w, r, buffer, stored.UpdatedAt)
		return
	}
	s.parkBufferAsAutosave(w, r, buffer, authorID)
}

// clearParkedBuffer removes the author's parked buffer and reports the item as it stands.
func (s *server) clearParkedBuffer(
	w http.ResponseWriter, r *http.Request, c content.Content, authorID uuid.UUID,
) {
	if err := s.content.DeleteAutosave(r.Context(), c.ID, authorID); err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetPost, c.ID,
		c.Title, c.Content, c.Excerpt, c.Fields, c.UpdatedAt))
}

// saveBufferToContent writes the buffer to the item itself.
func (s *server) saveBufferToContent(
	w http.ResponseWriter, r *http.Request, buffer content.Content, expected time.Time,
) {
	buffer.UpdatedAt = time.Now().UTC()
	updated, err := s.content.Update(r.Context(), buffer, expected, nil, 0)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetPost, updated.ID,
		updated.Title, updated.Content, updated.Excerpt, updated.Fields, updated.UpdatedAt))
}

// parkBufferAsAutosave stores the buffer as the author's autosave.
func (s *server) parkBufferAsAutosave(
	w http.ResponseWriter, r *http.Request, buffer content.Content, authorID uuid.UUID,
) {
	autosave, err := content.NewRevision(buffer, content.RevisionKindAutosave, authorID)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	saved, err := s.content.SaveAutosave(r.Context(), autosave)
	if err != nil {
		respondDomainError(w, err)
		return
	}
	authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetAutosave, saved.ContentID,
		saved.Title, saved.Content, saved.Excerpt, saved.Fields, saved.CreatedAt))
}

// handleAutosaveGet returns an http.HandlerFunc responding with the requester's autosave.
func (s *server) handleAutosaveGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed content id", Code: "content_id_malformed",
			})
			return
		}
		if _, err := s.content.ByID(r.Context(), id); err != nil {
			respondDomainError(w, err)
			return
		}
		identity := authkit.IdentityFromContext(r.Context())
		autosave, err := s.content.Autosave(r.Context(), id, identity.ID)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, newAutosaveResponse(autosaveTargetAutosave, autosave.ContentID,
			autosave.Title, autosave.Content, autosave.Excerpt, autosave.Fields, autosave.CreatedAt))
	}
}

// newAutosaveResponse builds an autosaveResponse, normalizing the timestamp to UTC.
func newAutosaveResponse(
	target string, contentID uuid.UUID, title, body, excerpt string,
	fields content.Values, savedAt time.Time,
) autosaveResponse {
	return autosaveResponse{
		Target:    target,
		ContentID: contentID,
		Title:     title,
		Content:   body,
		Excerpt:   excerpt,
		Fields:    heldValues(fields),
		SavedAt:   savedAt.UTC(),
	}
}

// bufferedValues returns the scalar values the buffer holds and whether they sit inside their bounds.
func (s *server) bufferedValues(
	r *http.Request, c content.Content, req autosaveRequest,
) (content.Values, bool, error) {
	if req.Fields == nil {
		return nil, true, nil
	}
	t, err := s.types.Active(r.Context(), c.Type)
	if err != nil {
		return nil, false, err
	}
	scalars, _, err := content.SplitValues(req.Fields, t.Fields)
	if err != nil {
		return nil, false, err
	}
	if err := scalars.ValidateShape(t.Fields); err != nil {
		return nil, false, err
	}
	if err := content.Concealed(t.Fields, req.Fields, req.Fields); err != nil {
		return nil, false, err
	}
	frozen := frozenValues(c.Fields, scalars, t.Fields)
	return frozen, frozen.Validate(t.Fields) == nil, nil
}

// frozenValues returns the buffer holding the stored value again under every hidden key it left out.
func frozenValues(stored, buffer content.Values, fields []content.Field) content.Values {
	frozen := make(content.Values, len(buffer))
	for key, value := range buffer {
		frozen[key] = value
	}
	for key := range content.Hidden(fields, buffer) {
		if value, held := stored[key]; held {
			frozen[key] = value
		}
	}
	return frozen
}
