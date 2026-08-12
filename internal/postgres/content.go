// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ content.Store = (*ContentStore)(nil)

// slugAttempts bounds the suffixes tried when a slug is taken.
const slugAttempts = 20

// slugConstraint names the unique constraint over a content type and slug.
const slugConstraint = "content_type_slug_unique"

// trashSuffixAlphabet holds the characters a trashed slug suffix draws from.
const trashSuffixAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// trashSuffixLength is the number of random characters in a trashed slug suffix.
const trashSuffixLength = 8

// ContentStore persists content items in the core schema.
type ContentStore struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewContentStore returns a [ContentStore] backed by pool.
func NewContentStore(pool *pgxpool.Pool) *ContentStore {
	return &ContentStore{pool: pool, queries: db.New(pool)}
}

// Create stores a new content item, suffixing its slug until the type accepts it.
func (s *ContentStore) Create(ctx context.Context, c content.Content) (content.Content, error) {
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		created, err := s.create(ctx, c, numberedSlug(c.Slug, attempt))
		if isSlugTaken(err) {
			continue
		}
		return created, err
	}
	created, err := s.create(ctx, c, identifiedSlug(c.Slug, c.ID))
	if isSlugTaken(err) {
		return content.Content{}, content.ErrSlugTaken
	}
	return created, err
}

// create stores the content item under slug.
func (s *ContentStore) create(ctx context.Context, c content.Content, slug string) (content.Content, error) {
	row, err := s.queries.CreateContent(ctx, db.CreateContentParams{
		ID:          c.ID,
		Type:        c.Type,
		Status:      string(c.Status),
		Slug:        slug,
		Title:       c.Title,
		Content:     c.Content,
		Excerpt:     c.Excerpt,
		AuthorID:    c.AuthorID,
		PublishedAt: c.PublishedAt,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	})
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: create content: %w", err)
	}
	return toContent(row), nil
}

// ByID returns the content item with the given id, or [content.ErrNotFound].
func (s *ContentStore) ByID(ctx context.Context, id uuid.UUID) (content.Content, error) {
	return byID(ctx, s.queries, id)
}

// byID returns the content item with the given id through queries, or [content.ErrNotFound].
func byID(ctx context.Context, queries *db.Queries, id uuid.UUID) (content.Content, error) {
	row, err := queries.GetContent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: get content: %w", err)
	}
	return toContent(row), nil
}

// PublishedBySlug returns the published item of the given type and slug, or [content.ErrNotFound].
func (s *ContentStore) PublishedBySlug(ctx context.Context, contentType, slug string) (content.Content, error) {
	row, err := s.queries.GetPublishedContent(ctx, db.GetPublishedContentParams{Type: contentType, Slug: slug})
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: get published content: %w", err)
	}
	return toContent(row), nil
}

// toContent maps a stored row to a domain content item with UTC timestamps.
func toContent(row db.CoreContent) content.Content {
	return content.Content{
		ID:          row.ID,
		Type:        row.Type,
		Status:      content.Status(row.Status),
		Slug:        row.Slug,
		Title:       row.Title,
		Content:     row.Content,
		Excerpt:     row.Excerpt,
		AuthorID:    row.AuthorID,
		PublishedAt: utcOrNil(row.PublishedAt),
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
}

// List returns the items matching the filter without their content, and the
// total number matching it.
func (s *ContentStore) List(ctx context.Context, f content.Filter) ([]content.Content, int, error) {
	search := escapeLike(f.Search)
	total, err := s.queries.CountContent(ctx, db.CountContentParams{
		Type:   f.Type,
		Status: string(f.Status),
		Search: search,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: count content: %w", err)
	}
	rows, err := s.queries.ListContent(ctx, db.ListContentParams{
		Type:      f.Type,
		Status:    string(f.Status),
		Search:    search,
		OrderBy:   string(f.OrderBy),
		OrderDir:  string(f.Order),
		RowLimit:  int32(f.PerPage),
		RowOffset: pageOffset(f.Page, f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list content: %w", err)
	}
	items := make([]content.Content, len(rows))
	for i, row := range rows {
		items[i] = content.Content{
			ID:          row.ID,
			Type:        row.Type,
			Status:      content.Status(row.Status),
			Slug:        row.Slug,
			Title:       row.Title,
			Excerpt:     row.Excerpt,
			AuthorID:    row.AuthorID,
			PublishedAt: utcOrNil(row.PublishedAt),
			CreatedAt:   row.CreatedAt.UTC(),
			UpdatedAt:   row.UpdatedAt.UTC(),
		}
	}
	return items, int(total), nil
}

// Update stores the item's editable fields and any snapshot, suffixing its slug until the type accepts it.
func (s *ContentStore) Update(
	ctx context.Context, c content.Content, expectedUpdatedAt time.Time, snapshot *content.Revision,
	revisionCap int,
) (content.Content, error) {
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		updated, err := s.update(ctx, c, numberedSlug(c.Slug, attempt), expectedUpdatedAt, snapshot, revisionCap)
		if isSlugTaken(err) {
			continue
		}
		if err != nil {
			return content.Content{}, err
		}
		return updated, nil
	}
	return content.Content{}, content.ErrSlugTaken
}

// update writes the content item and any snapshot under slug.
func (s *ContentStore) update(
	ctx context.Context, c content.Content, slug string, expectedUpdatedAt time.Time,
	snapshot *content.Revision, revisionCap int,
) (content.Content, error) {
	var updated content.Content
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		row, err := queries.UpdateContent(ctx, db.UpdateContentParams{
			ID:                c.ID,
			Status:            string(c.Status),
			Slug:              slug,
			Title:             c.Title,
			Content:           c.Content,
			Excerpt:           c.Excerpt,
			PublishedAt:       c.PublishedAt,
			UpdatedAt:         c.UpdatedAt,
			ExpectedUpdatedAt: expectedUpdatedAt,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err := byID(ctx, queries, c.ID); err != nil {
				return err
			}
			return content.ErrConflict
		}
		if err != nil {
			return err
		}
		updated = toContent(row)
		if snapshot == nil {
			return nil
		}
		return snapshotRevision(ctx, queries, *snapshot, revisionCap)
	})
	if err != nil {
		if errors.Is(err, content.ErrNotFound) || errors.Is(err, content.ErrConflict) {
			return content.Content{}, err
		}
		return content.Content{}, fmt.Errorf("postgres: update content: %w", err)
	}
	return updated, nil
}

// Trash marks the content item trashed and frees its slug for reuse.
func (s *ContentStore) Trash(ctx context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	row, err := s.queries.TrashContent(ctx, db.TrashContentParams{
		ID:        id,
		Suffix:    trashSuffix(),
		UpdatedAt: updatedAt,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: trash content: %w", err)
	}
	return toContent(row), nil
}

// Restore returns a trashed content item to draft. It recovers the original
// slug, or leaves the trashed one in place when the original is taken.
func (s *ContentStore) Restore(ctx context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	row, err := s.queries.RestoreContent(ctx, db.RestoreContentParams{ID: id, UpdatedAt: updatedAt})
	if isSlugTaken(err) {
		row, err = s.queries.RestoreContentKeepingSlug(
			ctx, db.RestoreContentKeepingSlugParams{ID: id, UpdatedAt: updatedAt},
		)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: restore content: %w", err)
	}
	return toContent(row), nil
}

// Counts returns the number of items of the type in each status.
func (s *ContentStore) Counts(ctx context.Context, contentType string) (map[content.Status]int, error) {
	rows, err := s.queries.CountContentByStatus(ctx, contentType)
	if err != nil {
		return nil, fmt.Errorf("postgres: count content by status: %w", err)
	}
	counts := make(map[content.Status]int, len(rows))
	for _, row := range rows {
		counts[content.Status(row.Status)] = int(row.Total)
	}
	return counts, nil
}

// Delete removes the content item, or reports [content.ErrNotFound].
func (s *ContentStore) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := s.queries.DeleteContent(ctx, id)
	if err != nil {
		return fmt.Errorf("postgres: delete content: %w", err)
	}
	if rows == 0 {
		return content.ErrNotFound
	}
	return nil
}

// pageOffset returns the row offset of a page, bounded to what the query accepts.
func pageOffset(page, perPage int) int32 {
	if perPage > 0 && page-1 > math.MaxInt32/perPage {
		return math.MaxInt32
	}
	return int32((page - 1) * perPage)
}

// utcOrNil returns the instant in UTC, or nil when it is unset.
func utcOrNil(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

// numberedSlug returns slug for the first attempt, and slug with the attempt
// number appended after that.
func numberedSlug(slug string, attempt int) string {
	if attempt == 1 {
		return slug
	}
	return slug + "-" + strconv.Itoa(attempt)
}

// identifiedSlug returns slug carrying the id of the content item holding it.
func identifiedSlug(slug string, id uuid.UUID) string {
	return slug + "-" + strings.ReplaceAll(id.String(), "-", "")
}

// trashSuffix returns the marker appended to a trashed content item's slug.
func trashSuffix() string {
	var b strings.Builder
	b.WriteString("-trashed-")
	for range trashSuffixLength {
		b.WriteByte(trashSuffixAlphabet[rand.IntN(len(trashSuffixAlphabet))])
	}
	return b.String()
}

// escapeLike returns search with the pattern characters of LIKE escaped.
func escapeLike(search string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(search)
}

// isSlugTaken reports whether err is the unique violation over a type and slug.
func isSlugTaken(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == slugConstraint
}
