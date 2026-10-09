// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// keptRows returns how many of n rows a short store keeps, by the rule's name.
var keptRows = map[string]func(n int) int{
	"none":      func(int) int { return 0 },
	"one":       func(n int) int { return min(n, 1) },
	"one-short": func(n int) int { return max(n-1, 0) },
}

// shorten returns the rows the keep rule leaves.
func shorten[T any](rows []T, keep func(n int) int) []T {
	return rows[:keep(len(rows))]
}

// shortContent is a content store whose listings keep only some of their rows.
type shortContent struct {
	content.Store
	keep func(n int) int
}

// List returns the stored listing cut short.
func (s shortContent) List(ctx context.Context, f content.Filter) ([]content.ListedItem, int, error) {
	rows, total, err := s.Store.List(ctx, f)
	return shorten(rows, s.keep), total, err
}

// RelatedTo returns the items pointing at the target cut short.
func (s shortContent) RelatedTo(
	ctx context.Context, target uuid.UUID, page, perPage int,
) ([]content.Content, int, error) {
	rows, total, err := s.Store.RelatedTo(ctx, target, page, perPage)
	return shorten(rows, s.keep), total, err
}

// TargetsByIDs returns the named targets cut short.
func (s shortContent) TargetsByIDs(ctx context.Context, ids []uuid.UUID) ([]content.Target, error) {
	rows, err := s.Store.TargetsByIDs(ctx, ids)
	return shorten(rows, s.keep), err
}

// PointingAt returns the pointers at the target cut short.
func (s shortContent) PointingAt(
	ctx context.Context, target uuid.UUID, field, page, perPage int,
) ([]content.Pointer, int, error) {
	rows, total, err := s.Store.PointingAt(ctx, target, field, page, perPage)
	return shorten(rows, s.keep), total, err
}

// Revisions returns the item's revisions cut short.
func (s shortContent) Revisions(ctx context.Context, contentID uuid.UUID) ([]content.Revision, error) {
	rows, err := s.Store.Revisions(ctx, contentID)
	return shorten(rows, s.keep), err
}

// shortTypes is a type store whose type and group listings keep only some of their rows.
type shortTypes struct {
	content.TypeStore
	keep func(n int) int
}

// List returns the stored types cut short.
func (s shortTypes) List(ctx context.Context) ([]content.Type, error) {
	rows, err := s.TypeStore.List(ctx)
	return shorten(rows, s.keep), err
}

// ListGroups returns the stored groups cut short.
func (s shortTypes) ListGroups(ctx context.Context) ([]content.Group, error) {
	rows, err := s.TypeStore.ListGroups(ctx)
	return shorten(rows, s.keep), err
}
