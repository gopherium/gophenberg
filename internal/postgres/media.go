// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ media.Store = (*MediaStore)(nil)

// MediaStore persists the media library in the core schema.
type MediaStore struct {
	queries *db.Queries
}

// NewMediaStore returns a [MediaStore] backed by pool.
func NewMediaStore(pool *pgxpool.Pool) *MediaStore {
	return &MediaStore{queries: db.New(pool)}
}

// Create stores a new media item and returns it with its assigned identifier.
func (s *MediaStore) Create(ctx context.Context, m media.Media) (media.Media, error) {
	row, err := s.queries.CreateMedia(ctx, db.CreateMediaParams{
		MediaType:   string(m.Type),
		File:        m.File,
		Title:       m.Title,
		AltText:     m.AltText,
		Caption:     m.Caption,
		Description: m.Description,
		MimeType:    m.MimeType,
		Width:       int32(m.Width),
		Height:      int32(m.Height),
		Filesize:    m.Filesize,
		Sizes:       m.Sizes,
		AuthorID:    m.AuthorID,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	})
	if err != nil {
		return media.Media{}, fmt.Errorf("postgres: create media: %w", err)
	}
	return toMedia(row), nil
}

// ByID returns the media item with the given id, or [media.ErrNotFound].
func (s *MediaStore) ByID(ctx context.Context, id int64) (media.Media, error) {
	row, err := s.queries.GetMedia(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.Media{}, media.ErrNotFound
	}
	if err != nil {
		return media.Media{}, fmt.Errorf("postgres: get media: %w", err)
	}
	return toMedia(row), nil
}

// List returns the media items matching the filter, newest first, and the
// total number matching it.
func (s *MediaStore) List(ctx context.Context, f media.Filter) ([]media.Media, int, error) {
	search := escapeLike(f.Search)
	mimes := escapedPrefixes(f.Mimes)
	total, err := s.queries.CountMedia(ctx, db.CountMediaParams{
		MediaType: string(f.Type),
		Mimes:     mimes,
		Search:    search,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: count media: %w", err)
	}
	rows, err := s.queries.ListMedia(ctx, db.ListMediaParams{
		MediaType: string(f.Type),
		Mimes:     mimes,
		Search:    search,
		RowLimit:  int32(f.PerPage),
		RowOffset: pageOffset(f.Page, f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list media: %w", err)
	}
	items := make([]media.Media, len(rows))
	for i, row := range rows {
		items[i] = toMedia(row)
	}
	return items, int(total), nil
}

// Update stores the media item's descriptions, or reports a missing item or a stale edit.
func (s *MediaStore) Update(ctx context.Context, m media.Media, expectedUpdatedAt time.Time) (media.Media, error) {
	row, err := s.queries.UpdateMedia(ctx, db.UpdateMediaParams{
		ID:                m.ID,
		Title:             m.Title,
		AltText:           m.AltText,
		Caption:           m.Caption,
		Description:       m.Description,
		UpdatedAt:         m.UpdatedAt,
		ExpectedUpdatedAt: expectedUpdatedAt,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := s.ByID(ctx, m.ID); err != nil {
			return media.Media{}, err
		}
		return media.Media{}, media.ErrConflict
	}
	if err != nil {
		return media.Media{}, fmt.Errorf("postgres: update media: %w", err)
	}
	return toMedia(row), nil
}

// Delete removes the media item and returns it, or reports [media.ErrNotFound].
func (s *MediaStore) Delete(ctx context.Context, id int64) (media.Media, error) {
	row, err := s.queries.DeleteMedia(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return media.Media{}, media.ErrNotFound
	}
	if err != nil {
		return media.Media{}, fmt.Errorf("postgres: delete media: %w", err)
	}
	return toMedia(row), nil
}

// escapedPrefixes returns the prefixes with the pattern characters of LIKE escaped.
func escapedPrefixes(prefixes []string) []string {
	escaped := make([]string, len(prefixes))
	for i, prefix := range prefixes {
		escaped[i] = escapeLike(prefix)
	}
	return escaped
}

// toMedia maps a stored row to a domain media item with UTC timestamps.
func toMedia(row db.CoreMedia) media.Media {
	return media.Media{
		ID:          row.ID,
		Type:        media.Type(row.MediaType),
		File:        row.File,
		Title:       row.Title,
		AltText:     row.AltText,
		Caption:     row.Caption,
		Description: row.Description,
		MimeType:    row.MimeType,
		Width:       int(row.Width),
		Height:      int(row.Height),
		Filesize:    row.Filesize,
		Sizes:       row.Sizes,
		AuthorID:    row.AuthorID,
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
}
