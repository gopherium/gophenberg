// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"net/http"

	"github.com/gopherium/gophenberg/internal/content"
)

// ContentChanges hears each saved write of an item, with the item before it and after it, nil after a delete.
type ContentChanges interface {
	Saved(ctx context.Context, before content.Content, after *content.Content)
}

// saved hands the write from before to after to the site's content changes, on a context that outlives the request.
func (s *server) saved(r *http.Request, before content.Content, after *content.Content) {
	if s.changes == nil {
		return
	}
	s.changes.Saved(context.WithoutCancel(r.Context()), before, after)
}
