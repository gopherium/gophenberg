// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ content.TypeStore = (*TypeStore)(nil)

// routeWordConstraint names the unique index over the route words in use.
const routeWordConstraint = "content_types_route_word_idx"

// typeKeyConstraint names the primary key over the type keys in use.
const typeKeyConstraint = "content_types_pkey"

// uniqueViolationCode is what Postgres reports when a row breaks a unique index.
const uniqueViolationCode = "23505"

// restrictViolationCode is what Postgres reports when a restricted row is still referenced.
const restrictViolationCode = "23001"

// foreignKeyViolationCode is what Postgres reports when a reference points at no row.
const foreignKeyViolationCode = "23503"

// fieldKeyConstraint names the unique index over one group's field keys.
const fieldKeyConstraint = "content_fields_scope_key_unique"

// groupKeyConstraint names the unique constraint over field group keys.
const groupKeyConstraint = "field_groups_key_unique"

// fieldGroupConstraint names the reference a field keeps to its group.
const fieldGroupConstraint = "content_fields_group_fkey"

// fieldTargetConstraint names the reference a relation field keeps to its target type.
const fieldTargetConstraint = "content_fields_relates_to_fkey"

// TypeStore persists the content type registry in the core schema.
type TypeStore struct {
	queries *db.Queries
	pool    *pgxpool.Pool
}

// NewTypeStore returns a [TypeStore] backed by pool.
func NewTypeStore(pool *pgxpool.Pool) *TypeStore {
	return &TypeStore{queries: db.New(pool), pool: pool}
}

// List returns every registered type in registration order with its flattened fields attached.
func (s *TypeStore) List(ctx context.Context) ([]content.Type, error) {
	rows, err := s.queries.ListContentTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: list content types: %w", err)
	}
	groups, err := groupsWithFields(ctx, s.queries)
	if err != nil {
		return nil, err
	}
	types := make([]content.Type, len(rows))
	for i, row := range rows {
		types[i] = toType(row)
		types[i].Fields = flattenedFields(groups, row.Key)
	}
	return types, nil
}

// ByKey returns the type carrying the key with its flattened fields, or [content.ErrTypeNotFound].
func (s *TypeStore) ByKey(ctx context.Context, key string) (content.Type, error) {
	row, err := s.queries.GetContentType(ctx, key)
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Type{}, content.ErrTypeNotFound
	}
	if err != nil {
		return content.Type{}, fmt.Errorf("postgres: get content type: %w", err)
	}
	groups, err := groupsWithFields(ctx, s.queries)
	if err != nil {
		return content.Type{}, err
	}
	t := toType(row)
	t.Fields = flattenedFields(groups, key)
	return t, nil
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
		stored, err := s.writeType(ctx, tx, t)
		updated = stored
		return err
	})
	if err != nil {
		return content.Type{}, updateTypeFailure(err)
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

// writeType stores the edited type inside tx, handing the root over when the edit asks for it.
func (s *TypeStore) writeType(ctx context.Context, tx pgx.Tx, t content.Type) (content.Type, error) {
	queries := s.queries.WithTx(tx)
	was, err := queries.LockContentType(ctx, t.Key)
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Type{}, content.ErrTypeNotFound
	}
	if err != nil {
		return content.Type{}, err
	}
	if t.Default && !was.IsDefault {
		if err := handRootOver(ctx, tx, queries, t.UpdatedAt); err != nil {
			return content.Type{}, err
		}
		t.RouteWord = ""
	}
	row, err := queries.UpdateContentType(ctx, updateParams(t))
	if err != nil {
		return content.Type{}, err
	}
	if was.RouteWord == t.RouteWord {
		return toType(row), nil
	}
	return toType(row), carryContent(ctx, tx, queries, t, was.RouteWord)
}

// updateTypeFailure returns the error the type update carries, and wraps anything else.
func updateTypeFailure(err error) error {
	if errors.Is(err, content.ErrTypeNotFound) ||
		errors.Is(err, content.ErrRouteWordReserved) ||
		errors.Is(err, content.ErrInvalidRouteWord) {
		return err
	}
	if taken := takenBy(err); taken != nil {
		return taken
	}
	if isSlugTaken(err) {
		return content.ErrSlugTaken
	}
	return fmt.Errorf("postgres: update content type: %w", err)
}

// handRootOver moves the type holding the root under a word of its own so another may take it.
func handRootOver(ctx context.Context, tx pgx.Tx, queries *db.Queries, at time.Time) error {
	row, err := queries.LockDefaultContentType(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	demoted := toType(row)
	demoted.RouteWord = content.Slugify(demoted.PluralLabel)
	demoted.Default, demoted.UpdatedAt = false, at
	if err := demoted.Validate(); err != nil {
		return err
	}
	if _, err := queries.UpdateContentType(ctx, updateParams(demoted)); err != nil {
		return err
	}
	return carryContent(ctx, tx, queries, demoted, row.RouteWord)
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
	if pgErr.ConstraintName == typeKeyConstraint {
		return content.ErrTypeTaken
	}
	return nil
}

// isTypeInUse reports whether err is content still referencing the type.
func isTypeInUse(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == restrictViolationCode
}

// CreateField declares the field on its type's group, raising the group when none matches.
func (s *TypeStore) CreateField(ctx context.Context, f content.Field) (content.Field, error) {
	var created content.Field
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		target, err := groupForWrite(ctx, queries, f.TypeKey)
		if err != nil {
			return err
		}
		row, err := queries.CreateContentField(ctx, db.CreateContentFieldParams{
			GroupID:   int32(target.ID),
			Key:       f.Key,
			Label:     f.Label,
			Kind:      string(f.Kind),
			RelatesTo: targetOf(f),
			Many:      f.Many,
			Required:  f.Required,
			CreatedAt: f.CreatedAt,
			UpdatedAt: f.UpdatedAt,
			Settings:  settingsJSON(f.Settings),
		})
		if err != nil {
			return err
		}
		created = toField(row)
		created.TypeKey = f.TypeKey
		return nil
	})
	if err != nil {
		return content.Field{}, fieldWriteFailure(err)
	}
	return created, nil
}

// fieldWriteFailure returns the error a field write carries, and wraps anything else.
func fieldWriteFailure(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == fieldKeyConstraint {
			return content.ErrFieldTaken
		}
		if pgErr.Code == foreignKeyViolationCode && pgErr.ConstraintName == fieldTargetConstraint {
			return content.ErrTargetUnknown
		}
		if pgErr.Code == foreignKeyViolationCode && pgErr.ConstraintName == fieldGroupConstraint {
			return content.ErrGroupNotFound
		}
	}
	if errors.Is(err, content.ErrTypeNotFound) || errors.Is(err, content.ErrGroupNotFound) {
		return err
	}
	return fmt.Errorf("postgres: create content field: %w", err)
}

// targetOf returns the relation target as the nullable column holds it.
func targetOf(f content.Field) *string {
	if f.RelatesTo == "" {
		return nil
	}
	return &f.RelatesTo
}

// originOf returns the origin a nullable column holds, empty for a definition the site made.
func originOf(origin *string) string {
	if origin == nil {
		return ""
	}
	return *origin
}

// originColumn returns the origin as the nullable column holds it.
func originColumn(origin string) *string {
	if origin == "" {
		return nil
	}
	return &origin
}

// toField maps a stored row to a domain field definition with UTC timestamps.
func toField(row db.CoreContentField) content.Field {
	f := content.Field{
		ID:        int(row.ID),
		GroupID:   int(row.GroupID),
		Key:       row.Key,
		Label:     row.Label,
		Kind:      content.FieldKind(row.Kind),
		Many:      row.Many,
		Required:  row.Required,
		Origin:    originOf(row.Origin),
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}
	if row.RelatesTo != nil {
		f.RelatesTo = *row.RelatesTo
	}
	if row.ParentFieldID.Valid {
		f.ParentID = int(row.ParentFieldID.Int32)
	}
	f.Settings = settingsOf(row.Settings)
	return f
}

// settingsJSON returns the settings as the jsonb column holds them, an empty object for none.
func settingsJSON(settings map[string]any) []byte {
	if len(settings) == 0 {
		return []byte("{}")
	}
	raw, _ := json.Marshal(settings)
	return raw
}

// settingsOf returns the stored settings as the domain reads them, nothing for an empty object.
func settingsOf(raw []byte) map[string]any {
	var settings map[string]any
	if err := json.Unmarshal(raw, &settings); err != nil || len(settings) == 0 {
		return nil
	}
	return settings
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
		Origin:        originOf(row.Origin),
		CreatedAt:     row.CreatedAt.UTC(),
		UpdatedAt:     row.UpdatedAt.UTC(),
	}
}
