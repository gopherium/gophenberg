// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/sqlite/columns"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// MediaStore meets the media library contract.
var _ media.Store = (*MediaStore)(nil)

// MediaStore persists the media library in the core tables of the site's SQLite file.
type MediaStore struct {
	queries *db.Queries
}

// NewMediaStore returns a [MediaStore] on the site's one SQLite handle.
func NewMediaStore(handle *sql.DB) *MediaStore {
	return &MediaStore{queries: db.New(handle)}
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
		Width:       int64(m.Width),
		Height:      int64(m.Height),
		Filesize:    m.Filesize,
		Sizes:       columns.Sizes(m.Sizes),
		AuthorID:    m.AuthorID,
		CreatedAt:   dbkit.Time{Time: m.CreatedAt},
		UpdatedAt:   dbkit.Time{Time: m.UpdatedAt},
	})
	if err != nil {
		return media.Media{}, failed("create media", err)
	}
	return toMedia(row), nil
}

// ByID returns the media item with the given id, or [media.ErrNotFound].
func (s *MediaStore) ByID(ctx context.Context, id int64) (media.Media, error) {
	row, err := s.queries.GetMedia(ctx, id)
	return itemOrMissing("get media", row, err)
}

// ByIDs returns the media items stored under the identities, leaving out any that are gone.
func (s *MediaStore) ByIDs(ctx context.Context, ids []int64) ([]media.Media, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.queries.ListMediaByIDs(ctx, jsonArray(ids))
	if err != nil {
		return nil, failed("list media by ids", err)
	}
	listed := make([]media.Media, len(rows))
	for i, row := range rows {
		listed[i] = toMedia(row)
	}
	return listed, nil
}

// Saved reports which of the files a stored item holds as its own file.
func (s *MediaStore) Saved(ctx context.Context, files []string) (map[string]bool, error) {
	held, err := s.queries.ListSavedMediaFiles(ctx, jsonArray(files))
	if err != nil {
		return nil, failed("list saved media files", err)
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
		return nil, 0, failed("count media", err)
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
		RowLimit:        int64(f.PerPage),
		RowOffset:       pageOffset(f.Page, f.PerPage),
	})
	if err != nil {
		return nil, 0, failed("list media", err)
	}
	items := make([]media.Listed, len(rows))
	for i, row := range rows {
		items[i] = media.Listed{Media: toMedia(row.CoreMedia), AuthorName: row.AuthorName}
	}
	return items, int(total), nil
}

// Update stores the media item's descriptions, or reports a missing item or a stale edit.
func (s *MediaStore) Update(ctx context.Context, m media.Media, expectedUpdatedAt time.Time) (media.Media, error) {
	row, err := s.queries.UpdateMedia(ctx, db.UpdateMediaParams{
		Title:             m.Title,
		AltText:           m.AltText,
		Caption:           m.Caption,
		Description:       m.Description,
		UpdatedAt:         dbkit.Time{Time: m.UpdatedAt},
		ID:                m.ID,
		ExpectedUpdatedAt: dbkit.Time{Time: expectedUpdatedAt},
	})
	if errors.Is(err, sql.ErrNoRows) {
		return media.Media{}, s.staleOrMissing(ctx, m.ID)
	}
	if err != nil {
		return media.Media{}, failed("update media", err)
	}
	return toMedia(row), nil
}

// Delete removes the media item and returns it, or reports [media.ErrNotFound].
func (s *MediaStore) Delete(ctx context.Context, id int64) (media.Media, error) {
	row, err := s.queries.DeleteMedia(ctx, id)
	return itemOrMissing("delete media", row, err)
}

// staleOrMissing returns why an edit of the item with id stored nothing, a missing item, a failed read or a conflict.
func (s *MediaStore) staleOrMissing(ctx context.Context, id int64) error {
	if _, err := s.ByID(ctx, id); err != nil {
		return err
	}
	return media.ErrConflict
}

// itemOrMissing returns the item row holds, [media.ErrNotFound] when no row came back, or err under action.
func itemOrMissing(action string, row db.CoreMedia, err error) (media.Media, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return media.Media{}, media.ErrNotFound
	}
	if err != nil {
		return media.Media{}, failed(action, err)
	}
	return toMedia(row), nil
}

// mediaNarrowing returns the arguments the media list and count queries narrow the filter by.
func mediaNarrowing(f media.Filter) db.CountMediaParams {
	return db.CountMediaParams{
		MediaType:       string(f.Type),
		Mimes:           jsonArray(f.Mimes),
		Authors:         jsonArray(f.Authors),
		ExcludedAuthors: jsonArray(f.ExcludeAuthors),
		Before:          f.Before,
		After:           f.After,
		ModifiedBefore:  f.ModifiedBefore,
		ModifiedAfter:   f.ModifiedAfter,
		Search:          f.Search,
	}
}

// jsonArray returns values as the JSON array text the queries read with json_each, an empty array for none.
func jsonArray[T string | int | int64 | uuid.UUID](values []T) string {
	text, _ := json.Marshal(append([]T{}, values...))
	return string(text)
}

// pageOffset returns the row offset of a page, bounded to the largest offset SQLite takes.
func pageOffset(page, perPage int) int64 {
	skipped, size := int64(page)-1, int64(perPage)
	if size > 0 && skipped > math.MaxInt64/size {
		return math.MaxInt64
	}
	return skipped * size
}

// toMedia maps a stored row to a domain media item.
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
		Sizes:       media.Sizes(row.Sizes),
		AuthorID:    row.AuthorID,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}
