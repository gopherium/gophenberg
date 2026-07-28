// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ post.Store = (*PostStore)(nil)

// slugAttempts bounds the suffixes tried when a slug is taken.
const slugAttempts = 20

// slugConstraint names the unique constraint over a post type and slug.
const slugConstraint = "posts_type_slug_unique"

// trashSuffixAlphabet holds the characters a trashed slug suffix draws from.
const trashSuffixAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// trashSuffixLength is the number of random characters in a trashed slug suffix.
const trashSuffixLength = 8

// PostStore persists posts in the core schema.
type PostStore struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewPostStore returns a [PostStore] backed by pool.
func NewPostStore(pool *pgxpool.Pool) *PostStore {
	return &PostStore{pool: pool, queries: db.New(pool)}
}

// Create stores a new post, suffixing its slug until the type accepts it.
func (s *PostStore) Create(ctx context.Context, p post.Post) (post.Post, error) {
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		err := s.queries.CreatePost(ctx, db.CreatePostParams{
			ID:          p.ID,
			Type:        p.Type,
			Status:      string(p.Status),
			Slug:        numberedSlug(p.Slug, attempt),
			Title:       p.Title,
			Content:     p.Content,
			Excerpt:     p.Excerpt,
			AuthorID:    p.AuthorID,
			PublishedAt: p.PublishedAt,
			CreatedAt:   p.CreatedAt,
			UpdatedAt:   p.UpdatedAt,
		})
		if isSlugTaken(err) {
			continue
		}
		if err != nil {
			return post.Post{}, fmt.Errorf("postgres: create post: %w", err)
		}
		return s.ByID(ctx, p.ID)
	}
	return post.Post{}, post.ErrSlugTaken
}

// ByID returns the post with the given id, or [post.ErrNotFound].
func (s *PostStore) ByID(ctx context.Context, id uuid.UUID) (post.Post, error) {
	row, err := s.queries.GetPost(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return post.Post{}, post.ErrNotFound
	}
	if err != nil {
		return post.Post{}, fmt.Errorf("postgres: get post: %w", err)
	}
	return post.Post{
		ID:          row.ID,
		Type:        row.Type,
		Status:      post.Status(row.Status),
		Slug:        row.Slug,
		Title:       row.Title,
		Content:     row.Content,
		Excerpt:     row.Excerpt,
		AuthorID:    row.AuthorID,
		PublishedAt: utcOrNil(row.PublishedAt),
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}, nil
}

// List returns the posts matching the filter without their content, and the
// total number matching it.
func (s *PostStore) List(ctx context.Context, f post.Filter) ([]post.Post, int, error) {
	search := escapeLike(f.Search)
	total, err := s.queries.CountPosts(ctx, db.CountPostsParams{
		Type:   f.Type,
		Status: string(f.Status),
		Search: search,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: count posts: %w", err)
	}
	rows, err := s.queries.ListPosts(ctx, db.ListPostsParams{
		Type:      f.Type,
		Status:    string(f.Status),
		Search:    search,
		RowLimit:  int32(f.PerPage),
		RowOffset: int32((f.Page - 1) * f.PerPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list posts: %w", err)
	}
	posts := make([]post.Post, len(rows))
	for i, row := range rows {
		posts[i] = post.Post{
			ID:          row.ID,
			Type:        row.Type,
			Status:      post.Status(row.Status),
			Slug:        row.Slug,
			Title:       row.Title,
			Excerpt:     row.Excerpt,
			AuthorID:    row.AuthorID,
			PublishedAt: utcOrNil(row.PublishedAt),
			CreatedAt:   row.CreatedAt.UTC(),
			UpdatedAt:   row.UpdatedAt.UTC(),
		}
	}
	return posts, int(total), nil
}

// Update stores the post's editable fields, suffixing its slug until the type
// accepts it.
func (s *PostStore) Update(
	ctx context.Context, p post.Post, _ *post.Revision, _ int,
) (post.Post, error) {
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		rows, err := s.queries.UpdatePost(ctx, db.UpdatePostParams{
			ID:          p.ID,
			Status:      string(p.Status),
			Slug:        numberedSlug(p.Slug, attempt),
			Title:       p.Title,
			Content:     p.Content,
			Excerpt:     p.Excerpt,
			PublishedAt: p.PublishedAt,
			UpdatedAt:   p.UpdatedAt,
		})
		if isSlugTaken(err) {
			continue
		}
		if err != nil {
			return post.Post{}, fmt.Errorf("postgres: update post: %w", err)
		}
		if rows == 0 {
			return post.Post{}, post.ErrNotFound
		}
		return s.ByID(ctx, p.ID)
	}
	return post.Post{}, post.ErrSlugTaken
}

// Trash marks the post trashed and frees its slug for reuse.
func (s *PostStore) Trash(ctx context.Context, id uuid.UUID, updatedAt time.Time) (post.Post, error) {
	rows, err := s.queries.TrashPost(ctx, db.TrashPostParams{
		ID:        id,
		Suffix:    trashSuffix(),
		UpdatedAt: updatedAt,
	})
	if err != nil {
		return post.Post{}, fmt.Errorf("postgres: trash post: %w", err)
	}
	if rows == 0 {
		return post.Post{}, post.ErrNotFound
	}
	return s.ByID(ctx, id)
}

// Restore returns a trashed post to draft. It recovers the original slug, or
// leaves the trashed one in place when the original is taken.
func (s *PostStore) Restore(ctx context.Context, id uuid.UUID, updatedAt time.Time) (post.Post, error) {
	rows, err := s.queries.RestorePost(ctx, db.RestorePostParams{ID: id, UpdatedAt: updatedAt})
	if isSlugTaken(err) {
		rows, err = s.queries.RestorePostKeepingSlug(
			ctx, db.RestorePostKeepingSlugParams{ID: id, UpdatedAt: updatedAt},
		)
	}
	if err != nil {
		return post.Post{}, fmt.Errorf("postgres: restore post: %w", err)
	}
	if rows == 0 {
		return post.Post{}, post.ErrNotFound
	}
	return s.ByID(ctx, id)
}

// Delete removes the post, or reports [post.ErrNotFound].
func (s *PostStore) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := s.queries.DeletePost(ctx, id)
	if err != nil {
		return fmt.Errorf("postgres: delete post: %w", err)
	}
	if rows == 0 {
		return post.ErrNotFound
	}
	return nil
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

// trashSuffix returns the marker appended to a trashed post's slug.
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
