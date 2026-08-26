// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// writeRelations stores the targets the item points at and refreshes what a term page reads.
func writeRelations(ctx context.Context, queries *db.Queries, c content.Content) error {
	ids, err := matchingGroupIDs(ctx, queries, c.Type)
	if err != nil {
		return err
	}
	declared, err := queries.ListRelationFieldsOfGroups(ctx, ids)
	if err != nil {
		return err
	}
	for _, f := range declared {
		targets, named := c.Relations[f.Key]
		if !named {
			continue
		}
		if err := carryTargets(ctx, queries, c, f, targets); err != nil {
			return err
		}
	}
	return queries.RefreshRelationVisibility(ctx, c.ID)
}

// carryTargets replaces the targets one relation field holds.
func carryTargets(
	ctx context.Context, queries *db.Queries, c content.Content,
	f db.ListRelationFieldsOfGroupsRow, targets []uuid.UUID,
) error {
	if err := targetsAllowed(ctx, queries, f, targets); err != nil {
		return err
	}
	err := queries.ClearRelationsOfField(ctx, db.ClearRelationsOfFieldParams{FromID: c.ID, FieldID: f.ID})
	if err != nil {
		return err
	}
	for i, target := range targets {
		err := queries.AddRelation(ctx, db.AddRelationParams{
			FromID:   c.ID,
			FieldID:  f.ID,
			ToID:     target,
			Position: int32(i + 1),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// targetsAllowed reports whether every target exists and is the type the field points at.
func targetsAllowed(
	ctx context.Context, queries *db.Queries, f db.ListRelationFieldsOfGroupsRow, targets []uuid.UUID,
) error {
	if len(targets) == 0 {
		return nil
	}
	rows, err := queries.TypesOfContent(ctx, targets)
	if err != nil {
		return err
	}
	held := make(map[uuid.UUID]string, len(rows))
	for _, row := range rows {
		held[row.ID] = row.Type
	}
	for _, target := range targets {
		stored, found := held[target]
		if !found {
			return fmt.Errorf("%w: %s", content.ErrTargetNotFound, target)
		}
		if f.RelatesTo == nil || stored != *f.RelatesTo {
			return fmt.Errorf("%w: %s holds %s", content.ErrTargetType, f.Key, stored)
		}
	}
	return nil
}

// RelatedTo returns the published content of active types pointing at the target, newest first.
func (s *ContentStore) RelatedTo(
	ctx context.Context, target uuid.UUID, page, perPage int,
) ([]content.Content, int, error) {
	total, err := s.queries.CountRelatedContent(ctx, target)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: count related content: %w", err)
	}
	rows, err := s.queries.ListRelatedContent(ctx, db.ListRelatedContentParams{
		Target:    target,
		RowLimit:  int32(perPage),
		RowOffset: pageOffset(page, perPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list related content: %w", err)
	}
	items := make([]content.Content, len(rows))
	for i, row := range rows {
		items[i] = content.Content{
			ID:          row.ID,
			Type:        row.Type,
			ParentID:    row.ParentID,
			Path:        row.Path,
			Status:      content.Status(row.Status),
			Slug:        row.Slug,
			Title:       row.Title,
			Excerpt:     row.Excerpt,
			AuthorID:    row.AuthorID,
			Fields:      row.Fields,
			PublishedAt: utcOrNil(row.PublishedAt),
			CreatedAt:   row.CreatedAt.UTC(),
			UpdatedAt:   row.UpdatedAt.UTC(),
		}
	}
	return items, int(total), nil
}

// TargetsOf returns the published targets of active types the item points at, keyed by field key.
func (s *ContentStore) TargetsOf(ctx context.Context, from uuid.UUID) (content.Targets, error) {
	rows, err := s.queries.ListRelationSummaries(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("postgres: list relation summaries: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	targets := make(content.Targets)
	for _, row := range rows {
		targets[row.Key] = append(targets[row.Key], content.Target{
			ID: row.ID, Title: row.Title, Path: row.Path,
		})
	}
	return targets, nil
}

// isTargetGone reports whether err is a relation pointing at an item that was removed.
func isTargetGone(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != foreignKeyViolationCode {
		return false
	}
	return strings.HasPrefix(pgErr.ConstraintName, "content_relations_")
}

// readRelations returns the targets the item points at, keyed by field key.
func readRelations(ctx context.Context, queries *db.Queries, id uuid.UUID) (content.Relations, error) {
	rows, err := queries.ListRelationTargets(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("postgres: list relation targets: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	relations := make(content.Relations)
	for _, row := range rows {
		relations[row.Key] = append(relations[row.Key], row.ToID)
	}
	return relations, nil
}
