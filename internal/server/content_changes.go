// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// ContentChanges hears each saved write of an item, held from its read until it is told, readied before it runs.
type ContentChanges interface {
	Hold(id uuid.UUID) (release func())
	Before(ctx context.Context, before content.Content) func(ctx context.Context, after *content.Content)
}

// hold holds the item id against the other writes the site's content changes hear, until release.
func (s *server) hold(id uuid.UUID) (release func()) {
	if s.changes == nil {
		return func() {}
	}
	return s.changes.Hold(id)
}

// readied readies the write from before and returns what hands over the item after it, nil after a delete.
func (s *server) readied(r *http.Request, before content.Content) func(after *content.Content) {
	if s.changes == nil {
		return func(*content.Content) {}
	}
	ctx := context.WithoutCancel(r.Context())
	tell := s.changes.Before(ctx, before)
	return func(after *content.Content) { tell(ctx, after) }
}
