// SPDX-License-Identifier: Apache-2.0

package contentchanges_test

import (
	"context"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/sdk"
)

// listeners keeps every change it hears, hearing none when deaf.
type listeners struct {
	deaf  bool
	heard []sdk.ContentChange
}

// Listens reports whether the fake hears changes.
func (l *listeners) Listens() bool {
	return !l.deaf
}

// ContentChanged keeps the change.
func (l *listeners) ContentChanged(_ context.Context, change sdk.ContentChange) {
	l.heard = append(l.heard, change)
}

// shaper shapes an item into its title and path, or answers its failure, keeping the titles it shaped.
type shaper struct {
	err      error
	shaped   []string
	deadline bool
}

// Item returns the item holding the stored item's title and path, or the shaper's failure.
func (s *shaper) Item(ctx context.Context, c content.Content) (sdk.Item, error) {
	s.shaped = append(s.shaped, c.Title)
	_, s.deadline = ctx.Deadline()
	return sdk.Item{Title: c.Title, Path: c.Path}, s.err
}
