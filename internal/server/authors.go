// SPDX-License-Identifier: Apache-2.0

package server

import (
	"bytes"
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"
)

// authorView is one account that may write, as the authors list names it.
type authorView struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// authorsResponse names every account that may write.
type authorsResponse struct {
	Items []authorView `json:"items"`
}

// handleAuthorList returns an http.HandlerFunc naming every account that may write, sorted by name.
func (s *server) handleAuthorList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		users, err := s.users.ListUsers(r.Context())
		if err != nil {
			respondDomainError(w, fmt.Errorf("server: list authors: %w", err))
			return
		}
		items := make([]authorView, len(users))
		for i, u := range users {
			items[i] = authorView{ID: u.ID, Name: u.Name}
		}
		slices.SortFunc(items, func(a, b authorView) int {
			return cmp.Or(strings.Compare(a.Name, b.Name), bytes.Compare(a.ID[:], b.ID[:]))
		})
		authkit.Respond(w, http.StatusOK, authorsResponse{Items: items})
	}
}
