// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ content.TypeStore = (*TypeStore)(nil)

// routeWordConstraint names the unique index over the route words in use.
const routeWordConstraint = "content_types_route_word_idx"

// uniqueViolationCode is what Postgres reports when a row breaks a unique index.
const uniqueViolationCode = "23505"

// restrictViolationCode is what Postgres reports when a restricted row is still referenced.
const restrictViolationCode = "23001"

// TypeStore persists the content type registry in the core schema.
type TypeStore struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

// NewTypeStore returns a [TypeStore] backed by pool.
func NewTypeStore(pool *pgxpool.Pool) *TypeStore {
	return &TypeStore{queries: db.New(pool), pool: pool}
}

// List returns every registered type in registration order.
func (s *TypeStore) List(ctx context.Context) ([]content.Type, error) {
	rows, err := s.queries.ListContentTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: list content types: %w", err)
	}
	types := make([]content.Type, len(rows))
	for i, row := range rows {
		types[i] = toType(row)
	}
	return types, nil
}

// ByKey returns the type carrying the key, or [content.ErrTypeNotFound].
func (s *TypeStore) ByKey(ctx context.Context, key string) (content.Type, error) {
	row, err := s.queries.GetContentType(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Type{}, content.ErrTypeNotFound
	}
	if err != nil {
		return content.Type{}, fmt.Errorf("postgres: get content type: %w", err)
	}
	return toType(row), nil
}

// Create stores a new content type, or reports the key or route word already in use.
func (s *TypeStore) Create(ctx context.Context, t content.Type) (content.Type, error) {
	row, err := s.queries.CreateContentType(ctx, db.CreateContentTypeParams{
		Key:           t.Key,
		SingularLabel: t.SingularLabel,
		PluralLabel:   t.PluralLabel,
		RouteWord:     t.RouteWord,
		Hierarchical:  t.Hierarchical,
		Revisions:     t.Revisions,
		RevisionCap:   int32(t.RevisionCap),
		PageKind:      string(t.PageKind),
		IsDefault:     t.Default,
		Active:        t.Active,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	})
	if taken := takenBy(err); taken != nil {
		return content.Type{}, taken
	}
	if err != nil {
		return content.Type{}, fmt.Errorf("postgres: create content type: %w", err)
	}
	return toType(row), nil
}

// Update stores the type's editable fields and carries its content to the route
// word, or reports [content.ErrTypeNotFound].
func (s *TypeStore) Update(ctx context.Context, t content.Type) (content.Type, error) {
	var updated content.Type
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		was, err := queries.LockContentType(ctx, t.Key)
		if errors.Is(err, pgx.ErrNoRows) {
			return content.ErrTypeNotFound
		}
		if err != nil {
			return err
		}
		row, err := queries.UpdateContentType(ctx, updateParams(t))
		if err != nil {
			return err
		}
		updated = toType(row)
		if was.RouteWord == t.RouteWord {
			return nil
		}
		return carryContent(ctx, tx, queries, t, was.RouteWord)
	})
	if err != nil {
		if errors.Is(err, content.ErrTypeNotFound) {
			return content.Type{}, err
		}
		if taken := takenBy(err); taken != nil {
			return content.Type{}, taken
		}
		if isSlugTaken(err) {
			return content.Type{}, content.ErrSlugTaken
		}
		return content.Type{}, fmt.Errorf("postgres: update content type: %w", err)
	}
	return updated, nil
}

// updateParams returns the row the type writes over its stored one.
func updateParams(t content.Type) db.UpdateContentTypeParams {
	return db.UpdateContentTypeParams{
		Key:           t.Key,
		SingularLabel: t.SingularLabel,
		PluralLabel:   t.PluralLabel,
		RouteWord:     t.RouteWord,
		Hierarchical:  t.Hierarchical,
		Revisions:     t.Revisions,
		RevisionCap:   int32(t.RevisionCap),
		PageKind:      string(t.PageKind),
		IsDefault:     t.Default,
		Active:        t.Active,
		UpdatedAt:     t.UpdatedAt,
	}
}

// carryContent moves every address of the type from the route word it answered under.
func carryContent(ctx context.Context, tx pgx.Tx, queries *db.Queries, t content.Type, was string) error {
	if _, err := tx.Exec(ctx, deferAddressCheck); err != nil {
		return err
	}
	return queries.RetypeContentPaths(ctx, db.RetypeContentPathsParams{
		RouteWord: t.RouteWord,
		Was:       was,
		UpdatedAt: t.UpdatedAt,
		Key:       t.Key,
	})
}

// Delete removes the type, or reports it missing or still in use.
func (s *TypeStore) Delete(ctx context.Context, key string) error {
	rows, err := s.queries.DeleteContentType(ctx, key)
	if isTypeInUse(err) {
		return content.ErrTypeInUse
	}
	if err != nil {
		return fmt.Errorf("postgres: delete content type: %w", err)
	}
	if rows == 0 {
		return content.ErrTypeNotFound
	}
	return nil
}

// takenBy returns the domain error for a unique violation over a key or a route word.
func takenBy(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != uniqueViolationCode {
		return nil
	}
	if pgErr.ConstraintName == routeWordConstraint {
		return content.ErrRouteWordTaken
	}
	return content.ErrTypeTaken
}

// isTypeInUse reports whether err is content still referencing the type.
func isTypeInUse(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == restrictViolationCode
}

// toType maps a stored row to a domain content type with UTC timestamps.
func toType(row db.CoreContentType) content.Type {
	return content.Type{
		Key:           row.Key,
		SingularLabel: row.SingularLabel,
		PluralLabel:   row.PluralLabel,
		RouteWord:     row.RouteWord,
		Hierarchical:  row.Hierarchical,
		Revisions:     row.Revisions,
		RevisionCap:   int(row.RevisionCap),
		PageKind:      content.PageKind(row.PageKind),
		Default:       row.IsDefault,
		Active:        row.Active,
		CreatedAt:     row.CreatedAt.UTC(),
		UpdatedAt:     row.UpdatedAt.UTC(),
	}
}
