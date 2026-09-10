// SPDX-License-Identifier: Apache-2.0

// Package contentbridge serves published content to plugins over the sdk seam.
package contentbridge

import (
	"context"
	"errors"
	"fmt"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/publichtml"
	"github.com/gopherium/gophenberg/internal/served"
	"github.com/gopherium/gophenberg/sdk"
)

var _ sdk.ContentReader = reader{}

// Types reads the content type a published item stands under and the groups its fields stand in.
type Types interface {
	ByKey(ctx context.Context, key string) (content.Type, error)
	Groups(ctx context.Context) ([]content.Group, error)
	Params(ctx context.Context) *content.ParamRegistry
}

// reader reads published content for plugins through the content store.
type reader struct {
	store   content.Store
	types   Types
	library served.Library
}

// New returns an [sdk.ContentReader] backed by store, reading its field definitions through types.
func New(store content.Store, types Types, library served.Library) sdk.ContentReader {
	return reader{store: store, types: types, library: library}
}

// stores returns the readers a public answer is shaped through.
func (r reader) stores() served.Stores {
	return served.Stores{Links: r.store, Groups: r.types, Library: r.library}
}

// ListPublished returns the newest published items of the given type, capped at limit.
func (r reader) ListPublished(ctx context.Context, contentType string, limit int) ([]sdk.Item, error) {
	found, _, err := r.store.List(ctx, content.Filter{
		Type:    contentType,
		Status:  content.StatusPublished,
		OrderBy: content.OrderByDate,
		Order:   content.OrderDesc,
		Page:    1,
		PerPage: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("contentbridge: list published content: %w", err)
	}
	t, err := r.types.ByKey(ctx, contentType)
	if err != nil {
		return nil, fmt.Errorf("contentbridge: read the type %s: %w", contentType, err)
	}
	published := make([]sdk.Item, 0, len(found))
	for _, c := range found {
		withContent, serving, err := r.stillPublished(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("contentbridge: read published content %s: %w", c.ID, err)
		}
		if !serving {
			continue
		}
		item, err := r.toSDKItem(ctx, t, withContent)
		if err != nil {
			return nil, fmt.Errorf("contentbridge: shape published content %s: %w", c.ID, err)
		}
		published = append(published, item)
	}
	return published, nil
}

// stillPublished returns the listed item carrying its content, and whether it is still the published one.
func (r reader) stillPublished(ctx context.Context, listed content.Content) (content.Content, bool, error) {
	current, err := r.store.PublishedByPath(ctx, listed.Path)
	if errors.Is(err, content.ErrNotFound) {
		return content.Content{}, false, nil
	}
	if err != nil {
		return content.Content{}, false, err
	}
	return current, current.ID == listed.ID, nil
}

// toSDKItem maps a stored content item to the shape plugins read, which is the shape a theme reads.
func (r reader) toSDKItem(ctx context.Context, t content.Type, c content.Content) (sdk.Item, error) {
	values, _, err := served.Values(ctx, r.stores(), t, c)
	if err != nil {
		return sdk.Item{}, err
	}
	published := c.UpdatedAt
	if c.PublishedAt != nil {
		published = *c.PublishedAt
	}
	return sdk.Item{
		ID:          c.ID,
		Type:        c.Type,
		Path:        c.Path,
		Slug:        c.Slug,
		Title:       c.Title,
		Excerpt:     c.Excerpt,
		Content:     publichtml.Sanitize(c.Content),
		Fields:      map[string]any(values),
		PublishedAt: published,
		UpdatedAt:   c.UpdatedAt,
	}, nil
}
