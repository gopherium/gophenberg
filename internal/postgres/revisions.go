// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// contentConstraint is the revision foreign key onto its content item.
const contentConstraint = "content_revisions_content_id_fkey"

// isContentGone reports whether err is a violation of the revision's content foreign key.
func isContentGone(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == contentConstraint
}

// Revisions returns the item's revisions newest first, without their content.
func (s *ContentStore) Revisions(ctx context.Context, contentID uuid.UUID) ([]content.Revision, error) {
	rows, err := s.queries.ListRevisions(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("postgres: list revisions: %w", err)
	}
	revisions := make([]content.Revision, len(rows))
	for i, row := range rows {
		revisions[i] = content.Revision{
			ID:        row.ID,
			ContentID: row.ContentID,
			Kind:      content.RevisionKind(row.Kind),
			AuthorID:  row.AuthorID,
			Title:     row.Title,
			Excerpt:   row.Excerpt,
			CreatedAt: row.CreatedAt.UTC(),
		}
	}
	return revisions, nil
}

// RevisionByID returns the item's revision, or [content.ErrRevisionNotFound].
func (s *ContentStore) RevisionByID(ctx context.Context, contentID, revisionID uuid.UUID) (content.Revision, error) {
	row, err := s.queries.GetRevision(ctx, db.GetRevisionParams{ContentID: contentID, ID: revisionID})
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Revision{}, content.ErrRevisionNotFound
	}
	if err != nil {
		return content.Revision{}, fmt.Errorf("postgres: get revision: %w", err)
	}
	return content.Revision{
		ID:        row.ID,
		ContentID: row.ContentID,
		Kind:      content.RevisionKind(row.Kind),
		AuthorID:  row.AuthorID,
		Title:     row.Title,
		Content:   row.Content,
		Excerpt:   row.Excerpt,
		Fields:    row.Fields,
		CreatedAt: row.CreatedAt.UTC(),
	}, nil
}

// DeleteRevision removes the item's revision, or reports [content.ErrRevisionNotFound].
func (s *ContentStore) DeleteRevision(ctx context.Context, contentID, revisionID uuid.UUID) error {
	rows, err := s.queries.DeleteRevision(ctx, db.DeleteRevisionParams{ContentID: contentID, ID: revisionID})
	if err != nil {
		return fmt.Errorf("postgres: delete revision: %w", err)
	}
	if rows == 0 {
		return content.ErrRevisionNotFound
	}
	return nil
}

// SaveAutosave stores the author's autosave of the item, replacing any earlier one.
func (s *ContentStore) SaveAutosave(ctx context.Context, autosave content.Revision) (content.Revision, error) {
	row, err := s.queries.UpsertAutosave(ctx, db.UpsertAutosaveParams{
		ID:        autosave.ID,
		ContentID: autosave.ContentID,
		AuthorID:  autosave.AuthorID,
		Title:     autosave.Title,
		Content:   autosave.Content,
		Excerpt:   autosave.Excerpt,
		Fields:    storedValues(autosave.Fields),
		CreatedAt: autosave.CreatedAt,
	})
	if isContentGone(err) {
		return content.Revision{}, content.ErrNotFound
	}
	if err != nil {
		return content.Revision{}, fmt.Errorf("postgres: save autosave: %w", err)
	}
	return toRevision(
		row.ID, row.ContentID, row.Kind, row.AuthorID, row.Title, row.Content, row.Excerpt, row.Fields, row.CreatedAt,
	), nil
}

// Autosave returns the author's autosave of the item, or [content.ErrRevisionNotFound].
func (s *ContentStore) Autosave(ctx context.Context, contentID, authorID uuid.UUID) (content.Revision, error) {
	row, err := s.queries.GetAutosave(ctx, db.GetAutosaveParams{ContentID: contentID, AuthorID: authorID})
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Revision{}, content.ErrRevisionNotFound
	}
	if err != nil {
		return content.Revision{}, fmt.Errorf("postgres: get autosave: %w", err)
	}
	return toRevision(
		row.ID, row.ContentID, row.Kind, row.AuthorID, row.Title, row.Content, row.Excerpt, row.Fields, row.CreatedAt,
	), nil
}

// DeleteAutosave removes the author's autosave of the item.
func (s *ContentStore) DeleteAutosave(ctx context.Context, contentID, authorID uuid.UUID) error {
	err := s.queries.DeleteAutosave(ctx, db.DeleteAutosaveParams{ContentID: contentID, AuthorID: authorID})
	if err != nil {
		return fmt.Errorf("postgres: delete autosave: %w", err)
	}
	return nil
}

// toRevision builds a revision from its stored columns.
func toRevision(
	id, contentID uuid.UUID, kind string, authorID uuid.UUID, title, body, excerpt string,
	fields content.Values, createdAt time.Time,
) content.Revision {
	return content.Revision{
		ID:        id,
		ContentID: contentID,
		Kind:      content.RevisionKind(kind),
		AuthorID:  authorID,
		Title:     title,
		Content:   body,
		Excerpt:   excerpt,
		Fields:    fields,
		CreatedAt: createdAt.UTC(),
	}
}

// snapshotRevision stores the snapshot and prunes revisions beyond the cap, sparing autosaves.
func snapshotRevision(ctx context.Context, queries *db.Queries, snapshot content.Revision, revisionCap int) error {
	err := queries.CreateRevision(ctx, db.CreateRevisionParams{
		ID:        snapshot.ID,
		ContentID: snapshot.ContentID,
		Kind:      string(snapshot.Kind),
		AuthorID:  snapshot.AuthorID,
		Title:     snapshot.Title,
		Content:   snapshot.Content,
		Excerpt:   snapshot.Excerpt,
		Fields:    storedValues(snapshot.Fields),
		CreatedAt: snapshot.CreatedAt,
	})
	if err != nil {
		return fmt.Errorf("postgres: create revision: %w", err)
	}
	if revisionCap <= 0 {
		return nil
	}
	err = queries.PruneRevisions(ctx, db.PruneRevisionsParams{ContentID: snapshot.ContentID, Keep: int32(revisionCap)})
	if err != nil {
		return fmt.Errorf("postgres: prune revisions: %w", err)
	}
	return nil
}
