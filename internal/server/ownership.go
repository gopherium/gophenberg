// SPDX-License-Identifier: Apache-2.0

package server

import (
	"errors"
	"net/http"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/role"
)

// errNotAuthor refuses a change to work the session does not own.
var errNotAuthor = errors.New("not the author of this item")

// refuseUnlessAuthor refuses unless the session may change work the author owns.
func refuseUnlessAuthor(r *http.Request, author uuid.UUID) error {
	identity := authkit.IdentityFromContext(r.Context())
	if role.MayChange(identity.Rank, identity.ID, author) {
		return nil
	}
	return errNotAuthor
}

// ownedContent returns the stored item once the session may change it.
func (s *server) ownedContent(r *http.Request, id uuid.UUID) (content.Content, error) {
	stored, err := s.content.ByID(r.Context(), id)
	if err != nil {
		return content.Content{}, err
	}
	if err := refuseUnlessAuthor(r, stored.AuthorID); err != nil {
		return content.Content{}, err
	}
	return stored, nil
}
