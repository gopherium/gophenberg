// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// resolveTargets returns the targets the index may hold, cleaning the values of a target that is gone.
func (s *Store) resolveTargets(
	ctx context.Context, queries Queries, matching []int, c content.Content, before content.Values,
) ([]content.FieldTargets, error) {
	declared, err := s.fieldsOfGroups(ctx, queries, matching)
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
		targets, err := targetsAllowed(ctx, queries, ft, kept[ft.Field.ID])
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, content.FieldTargets{Field: ft.Field, Targets: targets})
	}
	return resolved, nil
}

// writeRelations indexes the targets the item's values point at and refreshes what a term page reads.
func writeRelations(
	ctx context.Context, queries Queries, c content.Content, resolved []content.FieldTargets,
) error {
	for _, ft := range resolved {
		if err := carryTargets(ctx, queries, c, ft); err != nil {
			return err
		}
	}
	return queries.RefreshRelationVisibility(ctx, c.ID)
}

// fieldsOfGroups returns the fields the matching groups declare, however deep they stand.
func (s *Store) fieldsOfGroups(ctx context.Context, queries Queries, matching []int) ([]content.Field, error) {
	rows, err := queries.ListContentFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: list content fields: %w", s.backend.Engine, err)
	}
	held := typestore.FieldsByGroup(rows)
	var declared []content.Field
	for _, id := range matching {
		declared = append(declared, held[id]...)
	}
	return declared, nil
}

// matchingGroupIDs returns the ids of the active groups matching the type.
func (s *Store) matchingGroupIDs(ctx context.Context, queries Queries, typeKey string) ([]int, error) {
	rows, err := queries.ListFieldGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: list field groups: %w", s.backend.Engine, err)
	}
	var ids []int
	for _, row := range rows {
		group, err := typestore.GroupFromRow(s.backend.Engine, row)
		if err != nil {
			return nil, err
		}
		if typestore.Serves(group, typeKey) {
			ids = append(ids, group.ID)
		}
	}
	return ids, nil
}

// carryTargets replaces the rows indexing what one relation field points at.
func carryTargets(
	ctx context.Context, queries Queries, c content.Content, ft content.FieldTargets,
) error {
	fieldID := ft.Field.ID
	err := queries.ClearRelationsOfField(ctx, c.ID, fieldID)
	if err != nil {
		return err
	}
	for i, target := range ft.Targets {
		err := queries.AddRelation(ctx, RelationRow{
			FromID:   c.ID,
			FieldID:  fieldID,
			ToID:     target,
			Position: i + 1,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// targetsAllowed returns the targets the index may hold, refusing one the field never held that nothing stores.
func targetsAllowed(
	ctx context.Context, queries Queries, ft content.FieldTargets, kept map[uuid.UUID]bool,
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
func (s *Store) RelatedTo(
	ctx context.Context, target uuid.UUID, page, perPage int,
) ([]content.Content, int, error) {
	total, err := s.backend.Pool.CountRelatedContent(ctx, target)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: count related content: %w", s.backend.Engine, err)
	}
	rows, err := s.backend.Pool.ListRelatedContent(ctx, target, perPage, content.PageOffset(page, perPage))
	if err != nil {
		return nil, 0, fmt.Errorf("%s: list related content: %w", s.backend.Engine, err)
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
func (s *Store) PointingAt(
	ctx context.Context, target uuid.UUID, field, page, perPage int,
) ([]content.Pointer, int, error) {
	total, err := s.backend.Pool.CountPointingAt(ctx, target, field)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: count pointing at: %w", s.backend.Engine, err)
	}
	rows, err := s.backend.Pool.PointingAt(ctx, target, field, perPage, content.PageOffset(page, perPage))
	if err != nil {
		return nil, 0, fmt.Errorf("%s: list pointing at: %w", s.backend.Engine, err)
	}
	held := make([]content.Pointer, len(rows))
	copy(held, rows)
	return held, int(total), nil
}

// TargetsByIDs returns the published items of active types the identities name.
func (s *Store) TargetsByIDs(ctx context.Context, ids []uuid.UUID) ([]content.Target, error) {
	rows, err := s.backend.Pool.SummariesOfTargets(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("%s: list target summaries: %w", s.backend.Engine, err)
	}
	held := make([]content.Target, len(rows))
	copy(held, rows)
	return held, nil
}
