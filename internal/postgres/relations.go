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
	declared, err := queries.ListRelationFieldsOfType(ctx, c.Type)
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
	f db.ListRelationFieldsOfTypeRow, targets []uuid.UUID,
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
	ctx context.Context, queries *db.Queries, f db.ListRelationFieldsOfTypeRow, targets []uuid.UUID,
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
