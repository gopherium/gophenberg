// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// Revisions returns the post's revisions newest first, without their content.
func (s *PostStore) Revisions(ctx context.Context, postID uuid.UUID) ([]post.Revision, error) {
	rows, err := s.queries.ListRevisions(ctx, postID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list revisions: %w", err)
	}
	revisions := make([]post.Revision, len(rows))
	for i, row := range rows {
		revisions[i] = post.Revision{
			ID:        row.ID,
			PostID:    row.PostID,
			Kind:      post.RevisionKind(row.Kind),
			AuthorID:  row.AuthorID,
			Title:     row.Title,
			Excerpt:   row.Excerpt,
			CreatedAt: row.CreatedAt.UTC(),
		}
	}
	return revisions, nil
}

// RevisionByID returns the post's revision, or [post.ErrRevisionNotFound].
func (s *PostStore) RevisionByID(ctx context.Context, postID, revisionID uuid.UUID) (post.Revision, error) {
	row, err := s.queries.GetRevision(ctx, db.GetRevisionParams{PostID: postID, ID: revisionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return post.Revision{}, post.ErrRevisionNotFound
	}
	if err != nil {
		return post.Revision{}, fmt.Errorf("postgres: get revision: %w", err)
	}
	return post.Revision{
		ID:        row.ID,
		PostID:    row.PostID,
		Kind:      post.RevisionKind(row.Kind),
		AuthorID:  row.AuthorID,
		Title:     row.Title,
		Content:   row.Content,
		Excerpt:   row.Excerpt,
		CreatedAt: row.CreatedAt.UTC(),
	}, nil
}

// DeleteRevision removes the post's revision, or reports [post.ErrRevisionNotFound].
func (s *PostStore) DeleteRevision(ctx context.Context, postID, revisionID uuid.UUID) error {
	rows, err := s.queries.DeleteRevision(ctx, db.DeleteRevisionParams{PostID: postID, ID: revisionID})
	if err != nil {
		return fmt.Errorf("postgres: delete revision: %w", err)
	}
	if rows == 0 {
		return post.ErrRevisionNotFound
	}
	return nil
}

// SaveAutosave stores the author's autosave of the post, replacing any earlier one.
func (s *PostStore) SaveAutosave(ctx context.Context, autosave post.Revision) (post.Revision, error) {
	row, err := s.queries.UpsertAutosave(ctx, db.UpsertAutosaveParams{
		ID:        autosave.ID,
		PostID:    autosave.PostID,
		AuthorID:  autosave.AuthorID,
		Title:     autosave.Title,
		Content:   autosave.Content,
		Excerpt:   autosave.Excerpt,
		CreatedAt: autosave.CreatedAt,
	})
	if err != nil {
		return post.Revision{}, fmt.Errorf("postgres: save autosave: %w", err)
	}
	return toRevision(row.ID, row.PostID, row.Kind, row.AuthorID, row.Title, row.Content, row.Excerpt, row.CreatedAt), nil
}

// Autosave returns the author's autosave of the post, or [post.ErrRevisionNotFound].
func (s *PostStore) Autosave(ctx context.Context, postID, authorID uuid.UUID) (post.Revision, error) {
	row, err := s.queries.GetAutosave(ctx, db.GetAutosaveParams{PostID: postID, AuthorID: authorID})
	if errors.Is(err, pgx.ErrNoRows) {
		return post.Revision{}, post.ErrRevisionNotFound
	}
	if err != nil {
		return post.Revision{}, fmt.Errorf("postgres: get autosave: %w", err)
	}
	return toRevision(row.ID, row.PostID, row.Kind, row.AuthorID, row.Title, row.Content, row.Excerpt, row.CreatedAt), nil
}

// toRevision builds a revision from its stored columns.
func toRevision(
	id, postID uuid.UUID, kind string, authorID uuid.UUID, title, content, excerpt string, createdAt time.Time,
) post.Revision {
	return post.Revision{
		ID:        id,
		PostID:    postID,
		Kind:      post.RevisionKind(kind),
		AuthorID:  authorID,
		Title:     title,
		Content:   content,
		Excerpt:   excerpt,
		CreatedAt: createdAt.UTC(),
	}
}

// snapshotRevision stores the snapshot and prunes revisions beyond the cap, sparing autosaves.
func snapshotRevision(ctx context.Context, queries *db.Queries, snapshot post.Revision, revisionCap int) error {
	err := queries.CreateRevision(ctx, db.CreateRevisionParams{
		ID:        snapshot.ID,
		PostID:    snapshot.PostID,
		Kind:      string(snapshot.Kind),
		AuthorID:  snapshot.AuthorID,
		Title:     snapshot.Title,
		Content:   snapshot.Content,
		Excerpt:   snapshot.Excerpt,
		CreatedAt: snapshot.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("postgres: create revision: %w", err)
	}
	if revisionCap <= 0 {
		return nil
	}
	err = queries.PruneRevisions(ctx, db.PruneRevisionsParams{PostID: snapshot.PostID, Keep: int32(revisionCap)})
	if err != nil {
		return fmt.Errorf("postgres: prune revisions: %w", err)
	}
	return nil
}
