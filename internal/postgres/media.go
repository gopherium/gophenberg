// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
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

// ByIDs returns the media items stored under the identities, leaving out any that are gone.
func (s *MediaStore) ByIDs(ctx context.Context, ids []int64) ([]media.Media, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.queries.ListMediaByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("postgres: list media by ids: %w", err)
	}
	listed := make([]media.Media, len(rows))
	for i, row := range rows {
		listed[i] = toMedia(row)
	}
	return listed, nil
}

// Saved reports which of the files a stored item holds as its own file.
func (s *MediaStore) Saved(ctx context.Context, files []string) (map[string]bool, error) {
	held, err := s.queries.ListSavedMediaFiles(ctx, files)
	if err != nil {
		return nil, fmt.Errorf("postgres: list saved media files: %w", err)
	}
	saved := make(map[string]bool, len(held))
	for _, file := range held {
		saved[file] = true
	}
	return saved, nil
}

// List returns the media items the filter matches, sorted as it names, beside their authors' names, with their total.
func (s *MediaStore) List(ctx context.Context, f media.Filter) ([]media.Listed, int, error) {
	narrowed := mediaNarrowing(f)
	total, err := s.queries.CountMedia(ctx, narrowed)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: count media: %w", err)
	}
	rows, err := s.queries.ListMedia(ctx, db.ListMediaParams{
		MediaType:       narrowed.MediaType,
		Mimes:           narrowed.Mimes,
		Authors:         narrowed.Authors,
		ExcludedAuthors: narrowed.ExcludedAuthors,
		Before:          narrowed.Before,
		After:           narrowed.After,
		ModifiedBefore:  narrowed.ModifiedBefore,
		ModifiedAfter:   narrowed.ModifiedAfter,
		Search:          narrowed.Search,
		OrderBy:         string(f.OrderBy),
		OrderDir:        string(f.Order),
		RowLimit:        int32(f.PerPage),
		RowOffset:       int32(content.PageOffset(f.Page, f.PerPage)),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list media: %w", err)
	}
	items := make([]media.Listed, len(rows))
	for i, row := range rows {
		items[i] = listedMedia(row)
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

// mediaNarrowing returns the arguments the shared media list and count queries narrow the filter by.
func mediaNarrowing(f media.Filter) db.CountMediaParams {
	return db.CountMediaParams{
		MediaType:       string(f.Type),
		Mimes:           escapedPrefixes(f.Mimes),
		Authors:         append([]uuid.UUID{}, f.Authors...),
		ExcludedAuthors: append([]uuid.UUID{}, f.ExcludeAuthors...),
		Before:          f.Before,
		After:           f.After,
		ModifiedBefore:  f.ModifiedBefore,
		ModifiedAfter:   f.ModifiedAfter,
		Search:          dbkit.EscapeLike(f.Search),
	}
}

// escapedPrefixes returns the prefixes with the pattern characters of LIKE escaped.
func escapedPrefixes(prefixes []string) []string {
	escaped := make([]string, len(prefixes))
	for i, prefix := range prefixes {
		escaped[i] = dbkit.EscapeLike(prefix)
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

// listedMedia maps a listed row to the media item it stands for, beside its author's name, with UTC timestamps.
func listedMedia(row db.ListMediaRow) media.Listed {
	return media.Listed{
		Media: media.Media{
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
		},
		AuthorName: row.AuthorName,
	}
}
