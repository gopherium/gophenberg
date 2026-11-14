// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// editableUnderLock locks the item, reporting [content.ErrNotFound] or [content.ErrTrashed] when it may not be written.
func editableUnderLock(ctx context.Context, queries Queries, id uuid.UUID) error {
	row, err := queries.LockContent(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return content.ErrNotFound
	}
	if err != nil {
		return err
	}
	return toContent(row).Editable()
}

// Revisions returns the item's revisions newest first, without their content.
func (s *Store) Revisions(ctx context.Context, contentID uuid.UUID) ([]content.Revision, error) {
	rows, err := s.backend.Pool.ListRevisions(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("%s: list revisions: %w", s.backend.Engine, s.classify(err))
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
func (s *Store) RevisionByID(ctx context.Context, contentID, revisionID uuid.UUID) (content.Revision, error) {
	row, err := s.backend.Pool.GetRevision(ctx, contentID, revisionID)
	if errors.Is(err, sql.ErrNoRows) {
		return content.Revision{}, content.ErrRevisionNotFound
	}
	if err != nil {
		return content.Revision{}, fmt.Errorf("%s: get revision: %w", s.backend.Engine, s.classify(err))
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

// DeleteRevision removes the item's revision, reporting [content.ErrRevisionNotFound] or [content.ErrTrashed].
func (s *Store) DeleteRevision(ctx context.Context, contentID, revisionID uuid.UUID) error {
	err := s.backend.InTx(ctx, func(queries Tx) error {
		if err := editableUnderLock(ctx, queries, contentID); err != nil {
			return err
		}
		rows, err := queries.DeleteRevision(ctx, contentID, revisionID)
		if err != nil {
			return err
		}
		if rows == 0 {
			return content.ErrRevisionNotFound
		}
		return nil
	})
	if errors.Is(err, content.ErrNotFound) || errors.Is(err, content.ErrTrashed) ||
		errors.Is(err, content.ErrRevisionNotFound) {
		return err
	}
	if err != nil {
		return fmt.Errorf("%s: delete revision: %w", s.backend.Engine, s.classify(err))
	}
	return nil
}

// SaveAutosave stores the author's autosave of the item, replacing any earlier one, unless the item is in the trash.
func (s *Store) SaveAutosave(ctx context.Context, autosave content.Revision) (content.Revision, error) {
	var row RevisionRow
	err := s.backend.InTx(ctx, func(queries Tx) error {
		if err := editableUnderLock(ctx, queries, autosave.ContentID); err != nil {
			return err
		}
		var err error
		row, err = queries.UpsertAutosave(ctx, RevisionRow{
			ID:        autosave.ID,
			ContentID: autosave.ContentID,
			AuthorID:  autosave.AuthorID,
			Title:     autosave.Title,
			Content:   autosave.Content,
			Excerpt:   autosave.Excerpt,
			Fields:    storedValues(autosave.Fields),
			CreatedAt: autosave.CreatedAt,
		})
		return err
	})
	if errors.Is(err, content.ErrNotFound) || errors.Is(err, content.ErrTrashed) {
		return content.Revision{}, err
	}
	if err != nil {
		return content.Revision{}, fmt.Errorf("%s: save autosave: %w", s.backend.Engine, s.classify(err))
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

// Autosave returns the author's autosave of the item, or [content.ErrRevisionNotFound].
func (s *Store) Autosave(ctx context.Context, contentID, authorID uuid.UUID) (content.Revision, error) {
	row, err := s.backend.Pool.GetAutosave(ctx, contentID, authorID)
	if errors.Is(err, sql.ErrNoRows) {
		return content.Revision{}, content.ErrRevisionNotFound
	}
	if err != nil {
		return content.Revision{}, fmt.Errorf("%s: get autosave: %w", s.backend.Engine, s.classify(err))
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

// DeleteAutosave removes the author's autosave of the item.
func (s *Store) DeleteAutosave(ctx context.Context, contentID, authorID uuid.UUID) error {
	err := s.backend.Pool.DeleteAutosave(ctx, contentID, authorID)
	if err != nil {
		return fmt.Errorf("%s: delete autosave: %w", s.backend.Engine, s.classify(err))
	}
	return nil
}

// snapshotRevision stores the snapshot and prunes revisions beyond the cap, sparing autosaves.
func (s *Store) snapshotRevision(
	ctx context.Context, queries Queries, snapshot content.Revision, revisionCap int,
) error {
	err := queries.CreateRevision(ctx, RevisionRow{
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
		return fmt.Errorf("%s: create revision: %w", s.backend.Engine, s.classify(err))
	}
	if revisionCap <= 0 {
		return nil
	}
	err = queries.PruneRevisions(ctx, snapshot.ContentID, revisionCap)
	if err != nil {
		return fmt.Errorf("%s: prune revisions: %w", s.backend.Engine, s.classify(err))
	}
	return nil
}
