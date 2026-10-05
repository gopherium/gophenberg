// SPDX-License-Identifier: Apache-2.0

package server

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/role"
)

// emptiedTrash counts what emptying a type's trash deleted and what it left for other accounts.
type emptiedTrash struct {
	Deleted int `json:"deleted"`
	Kept    int `json:"kept"`
}

// handleTrashEmpty returns an http.HandlerFunc deleting for good every trashed item of a type the session may change.
func (s *server) handleTrashEmpty() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("type")
		if key == "" {
			respondDomainError(w, content.ErrInvalidType)
			return
		}
		contentType, err := s.typeAsked(r, key)
		if err != nil {
			respondDomainError(w, err)
			return
		}
		deleted, kept, err := s.content.EmptyTrash(r.Context(), contentType.Key, trashOwner(r))
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, emptiedTrash{Deleted: deleted, Kept: kept})
	}
}

// trashOwner returns the account whose trash the session may empty, none when it may empty every account's.
func trashOwner(r *http.Request) *uuid.UUID {
	identity := authkit.IdentityFromContext(r.Context())
	if role.Can(identity.Role, role.ChangeOthersWork) {
		return nil
	}
	return &identity.ID
}
