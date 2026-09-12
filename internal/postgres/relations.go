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

// resolveTargets returns the targets the index may hold, cleaning the values of a target that is gone.
func resolveTargets(
	ctx context.Context, queries *db.Queries, matching []int32, c content.Content, before content.Values,
) ([]content.FieldTargets, error) {
	declared, err := fieldsOfGroups(ctx, queries, matching)
	if err != nil {
		return nil, err
	}
	held, err := content.HeldTargets(declared, c.Fields)
	if err != nil {
		return nil, err
	}
	kept := content.HeldIdentities(declared, before)
	resolved := make([]content.FieldTargets, 0, len(held))
	for _, ft := range held {
		targets, err := targetsAllowed(ctx, queries, ft, kept)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, content.FieldTargets{Field: ft.Field, Targets: targets})
	}
	return resolved, nil
}

// writeRelations indexes the targets the item's values point at and refreshes what a term page reads.
func writeRelations(
	ctx context.Context, queries *db.Queries, c content.Content, resolved []content.FieldTargets,
) error {
	for _, ft := range resolved {
		if err := carryTargets(ctx, queries, c, ft); err != nil {
			return err
		}
	}
	return queries.RefreshRelationVisibility(ctx, c.ID)
}

// fieldsOfGroups returns the fields the matching groups declare, however deep they stand.
func fieldsOfGroups(ctx context.Context, queries *db.Queries, matching []int32) ([]content.Field, error) {
	rows, err := queries.ListContentFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: list content fields: %w", err)
	}
	held := fieldsByGroup(rows)
	var declared []content.Field
	for _, id := range matching {
		declared = append(declared, held[int(id)]...)
	}
	return declared, nil
}

// carryTargets replaces the rows indexing what one relation field points at.
func carryTargets(
	ctx context.Context, queries *db.Queries, c content.Content, ft content.FieldTargets,
) error {
	fieldID := int32(ft.Field.ID)
	err := queries.ClearRelationsOfField(ctx, db.ClearRelationsOfFieldParams{FromID: c.ID, FieldID: fieldID})
	if err != nil {
		return err
	}
	for i, target := range ft.Targets {
		err := queries.AddRelation(ctx, db.AddRelationParams{
			FromID:   c.ID,
			FieldID:  fieldID,
			ToID:     target,
			Position: int32(i + 1),
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// targetsAllowed returns the targets the index may hold, refusing one the item never held that nothing stores.
func targetsAllowed(
	ctx context.Context, queries *db.Queries, ft content.FieldTargets, kept map[uuid.UUID]bool,
) ([]uuid.UUID, error) {
	if len(ft.Targets) == 0 {
		return nil, nil
	}
	rows, err := queries.TypesOfContent(ctx, ft.Targets)
	if err != nil {
		return nil, err
	}
	held := make(map[uuid.UUID]string, len(rows))
	for _, row := range rows {
		held[row.ID] = row.Type
	}
	indexed := make([]uuid.UUID, 0, len(ft.Targets))
	for _, target := range ft.Targets {
		stored, found := held[target]
		if !found {
			if kept[target] {
				continue
			}
			return nil, fmt.Errorf("%w: %s", content.ErrTargetNotFound, target)
		}
		if stored != ft.Field.RelatesTo {
			return nil, fmt.Errorf("%w: %s holds %s", content.ErrTargetType, ft.Field.Key, stored)
		}
		indexed = append(indexed, target)
	}
	return indexed, nil
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

// PointingAt returns the published items pointing at the target through the field, and how many there are.
func (s *ContentStore) PointingAt(
	ctx context.Context, target uuid.UUID, field, page, perPage int,
) ([]content.Pointer, int, error) {
	total, err := s.queries.CountPointingAt(ctx, db.CountPointingAtParams{
		Target: target, Field: int32(field),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: count pointing at: %w", err)
	}
	rows, err := s.queries.PointingAt(ctx, db.PointingAtParams{
		Target:    target,
		Field:     int32(field),
		RowLimit:  int32(perPage),
		RowOffset: pageOffset(page, perPage),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list pointing at: %w", err)
	}
	held := make([]content.Pointer, len(rows))
	for i, row := range rows {
		held[i] = content.Pointer{ID: row.ID, Type: row.Type, Title: row.Title, Path: row.Path}
	}
	return held, int(total), nil
}

// TargetsByIDs returns the published items of active types the identities name.
func (s *ContentStore) TargetsByIDs(ctx context.Context, ids []uuid.UUID) ([]content.Target, error) {
	rows, err := s.queries.SummariesOfTargets(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("postgres: list target summaries: %w", err)
	}
	held := make([]content.Target, len(rows))
	for i, row := range rows {
		held[i] = content.Target{ID: row.ID, Title: row.Title, Path: row.Path}
	}
	return held, nil
}

// isTargetGone reports whether err is a relation pointing at an item that was removed.
func isTargetGone(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != foreignKeyViolationCode {
		return false
	}
	return strings.HasPrefix(pgErr.ConstraintName, "content_relations_")
}
