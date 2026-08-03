// SPDX-License-Identifier: Apache-2.0

// Package postbridge serves published posts to plugins over the sdk seam.
package postbridge

import (
	"context"
	"errors"
	"fmt"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/publichtml"
	"github.com/gopherium/gophenberg/sdk"
)

var _ sdk.PostReader = reader{}

// reader reads published posts for plugins through the post store.
type reader struct {
	posts post.Store
}

// New returns an [sdk.PostReader] backed by posts.
func New(posts post.Store) sdk.PostReader {
	return reader{posts: posts}
}

// ListPublished returns the newest published posts of the given type, capped at limit.
func (r reader) ListPublished(ctx context.Context, postType string, limit int) ([]sdk.Post, error) {
	found, _, err := r.posts.List(ctx, post.Filter{
		Type:    postType,
		Status:  post.StatusPublished,
		OrderBy: post.OrderByDate,
		Order:   post.OrderDesc,
		Page:    1,
		PerPage: limit,
	})
	if err != nil {
		return nil, fmt.Errorf("postbridge: list published posts: %w", err)
	}
	published := make([]sdk.Post, 0, len(found))
	for _, p := range found {
		withContent, serving, err := r.stillPublished(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("postbridge: read published post %s: %w", p.ID, err)
		}
		if serving {
			published = append(published, toSDKPost(withContent))
		}
	}
	return published, nil
}

// stillPublished returns the listed post carrying its content, and whether it is still the published one.
func (r reader) stillPublished(ctx context.Context, listed post.Post) (post.Post, bool, error) {
	current, err := r.posts.PublishedBySlug(ctx, listed.Type, listed.Slug)
	if errors.Is(err, post.ErrNotFound) {
		return post.Post{}, false, nil
	}
	if err != nil {
		return post.Post{}, false, err
	}
	return current, current.ID == listed.ID, nil
}

// toSDKPost maps a stored post to the shape plugins read.
func toSDKPost(p post.Post) sdk.Post {
	published := p.UpdatedAt
	if p.PublishedAt != nil {
		published = *p.PublishedAt
	}
	return sdk.Post{
		ID:          p.ID,
		Type:        p.Type,
		Slug:        p.Slug,
		Title:       p.Title,
		Excerpt:     p.Excerpt,
		Content:     publichtml.Sanitize(p.Content),
		PublishedAt: published,
		UpdatedAt:   p.UpdatedAt,
	}
}
