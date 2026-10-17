// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// List returns every registered type in registration order with its flattened fields attached.
func (s *Store) List(ctx context.Context) ([]content.Type, error) {
	rows, err := s.backend.Pool.ListContentTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: list content types: %w", s.backend.Engine, err)
	}
	groups, err := s.groupsWithFields(ctx, s.backend.Pool)
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
func (s *Store) ByKey(ctx context.Context, key string) (content.Type, error) {
	row, err := s.backend.Pool.GetContentType(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return content.Type{}, content.ErrTypeNotFound
	}
	if err != nil {
		return content.Type{}, fmt.Errorf("%s: get content type: %w", s.backend.Engine, err)
	}
	groups, err := s.groupsWithFields(ctx, s.backend.Pool)
	if err != nil {
		return content.Type{}, err
	}
	t := toType(row)
	t.Fields = flattenedFields(groups, key)
	return t, nil
}

// Create stores a new content type, or reports the key or route word already in use.
func (s *Store) Create(ctx context.Context, t content.Type) (content.Type, error) {
	row, err := s.backend.Pool.CreateContentType(ctx, TypeRow{
		Origin:        t.Origin,
		Key:           t.Key,
		SingularLabel: t.SingularLabel,
		PluralLabel:   t.PluralLabel,
		Description:   t.Description,
		RouteWord:     t.RouteWord,
		Hierarchical:  t.Hierarchical,
		Revisions:     t.Revisions,
		RevisionCap:   t.RevisionCap,
		PageKind:      string(t.PageKind),
		IsDefault:     t.Default,
		Active:        t.Active,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	})
	if taken := s.errorFor(typeWriteErrors, err); taken != nil {
		return content.Type{}, taken
	}
	if err != nil {
		return content.Type{}, fmt.Errorf("%s: create content type: %w", s.backend.Engine, err)
	}
	return toType(row), nil
}

// Update stores the type's edits and carries its content to the route word, or reports [content.ErrTypeNotFound].
func (s *Store) Update(ctx context.Context, t content.Type) (content.Type, error) {
	var updated content.Type
	err := s.backend.InTx(ctx, func(queries Queries) error {
		stored, err := s.writeType(ctx, queries, t)
		updated = stored
		return err
	})
	if err != nil {
		return content.Type{}, s.updateTypeFailure(err)
	}
	return updated, nil
}

// Nested returns how many items of the type sit inside another, the ones in the trash included.
func (s *Store) Nested(ctx context.Context, key string) (int, error) {
	held, err := s.backend.Pool.CountNestedContent(ctx, key)
	if err != nil {
		return 0, fmt.Errorf("%s: count nested content: %w", s.backend.Engine, err)
	}
	return int(held), nil
}

// updateParams returns the row the type writes over its stored one.
func updateParams(t content.Type) TypeRow {
	return TypeRow{
		Key:           t.Key,
		SingularLabel: t.SingularLabel,
		PluralLabel:   t.PluralLabel,
		Description:   t.Description,
		RouteWord:     t.RouteWord,
		Hierarchical:  t.Hierarchical,
		Revisions:     t.Revisions,
		RevisionCap:   t.RevisionCap,
		PageKind:      string(t.PageKind),
		IsDefault:     t.Default,
		Active:        t.Active,
		UpdatedAt:     t.UpdatedAt,
	}
}

// carryContent moves every address of the type from the route word it answered under.
func carryContent(ctx context.Context, queries Queries, t content.Type, was string) error {
	if err := queries.DeferAddressCheck(ctx); err != nil {
		return err
	}
	return queries.RetypeContentPaths(ctx, TypeRow{
		RouteWord: t.RouteWord,
		UpdatedAt: t.UpdatedAt,
		Key:       t.Key,
	}, was)
}

// writeType stores the edited type through the queries, handing the root over when the edit asks for it.
func (s *Store) writeType(ctx context.Context, queries Queries, t content.Type) (content.Type, error) {
	was, err := queries.LockContentType(ctx, t.Key)
	if errors.Is(err, sql.ErrNoRows) {
		return content.Type{}, content.ErrTypeNotFound
	}
	if err != nil {
		return content.Type{}, err
	}
	t, err = readied(ctx, queries, t, was)
	if err != nil {
		return content.Type{}, err
	}
	row, err := queries.UpdateContentType(ctx, updateParams(t))
	if err != nil {
		return content.Type{}, err
	}
	if was.RouteWord == t.RouteWord {
		return toType(row), nil
	}
	return toType(row), carryContent(ctx, queries, t, was.RouteWord)
}

// readied returns the edit ready to write over the locked row, refusing what the site's content keeps.
func readied(
	ctx context.Context, queries Queries, t content.Type, was TypeRow,
) (content.Type, error) {
	if was.Hierarchical && !t.Hierarchical {
		if err := flattenable(ctx, queries, t.Key); err != nil {
			return content.Type{}, err
		}
	}
	if t.Default && !was.IsDefault {
		if err := handRootOver(ctx, queries, t.UpdatedAt); err != nil {
			return content.Type{}, err
		}
		t.RouteWord = ""
	}
	return t, nil
}

// flattenable returns the reason the locked type may not stop nesting, if there is one.
func flattenable(ctx context.Context, queries Queries, key string) error {
	held, err := queries.CountNestedContent(ctx, key)
	if err != nil {
		return err
	}
	if held > 0 {
		return content.NestingInUse(key, int(held))
	}
	return nil
}

// updateTypeFailure returns the error the type update carries, and wraps anything else.
func (s *Store) updateTypeFailure(err error) error {
	if errors.Is(err, content.ErrTypeNotFound) ||
		errors.Is(err, content.ErrRouteWordReserved) ||
		errors.Is(err, content.ErrInvalidRouteWord) ||
		errors.Is(err, content.ErrNestingInUse) {
		return err
	}
	if taken := s.errorFor(typeUpdateErrors, err); taken != nil {
		return taken
	}
	return fmt.Errorf("%s: update content type: %w", s.backend.Engine, err)
}

// handRootOver moves the type holding the root under a word of its own so another may take it.
func handRootOver(ctx context.Context, queries Queries, at time.Time) error {
	row, err := queries.LockDefaultContentType(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	demoted := toType(row)
	demoted.RouteWord = content.RouteWordForType(demoted.Key, demoted.PluralLabel)
	demoted.Default, demoted.UpdatedAt = false, at
	if err := demoted.Validate(); err != nil {
		return err
	}
	if _, err := queries.UpdateContentType(ctx, updateParams(demoted)); err != nil {
		return err
	}
	return carryContent(ctx, queries, demoted, row.RouteWord)
}

// Delete removes the type, or reports it missing or still in use.
func (s *Store) Delete(ctx context.Context, key string) error {
	rows, err := s.backend.Pool.DeleteContentType(ctx, key)
	if held := s.errorFor(typeDeleteErrors, err); held != nil {
		return held
	}
	if err != nil {
		return fmt.Errorf("%s: delete content type: %w", s.backend.Engine, err)
	}
	if rows == 0 {
		return content.ErrTypeNotFound
	}
	return nil
}

// fieldWriteFailure returns the error a field write carries, and wraps anything else.
func (s *Store) fieldWriteFailure(err error) error {
	if written := s.errorFor(fieldWriteErrors, err); written != nil {
		return written
	}
	return fmt.Errorf("%s: create content field: %w", s.backend.Engine, err)
}

// toField maps a stored row to a domain field definition with UTC timestamps.
func toField(row FieldRow) content.Field {
	return content.Field{
		ID:        row.ID,
		GroupID:   row.GroupID,
		ParentID:  row.ParentID,
		Key:       row.Key,
		Label:     row.Label,
		Kind:      content.FieldKind(row.Kind),
		RelatesTo: row.RelatesTo,
		Many:      row.Many,
		Required:  row.Required,
		Origin:    row.Origin,
		Settings:  settingsOf(row.Settings),
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}
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
func toType(row TypeRow) content.Type {
	return content.Type{
		Key:           row.Key,
		SingularLabel: row.SingularLabel,
		PluralLabel:   row.PluralLabel,
		Description:   row.Description,
		RouteWord:     row.RouteWord,
		Hierarchical:  row.Hierarchical,
		Revisions:     row.Revisions,
		RevisionCap:   row.RevisionCap,
		PageKind:      content.PageKind(row.PageKind),
		Default:       row.IsDefault,
		Active:        row.Active,
		Origin:        row.Origin,
		CreatedAt:     row.CreatedAt.UTC(),
		UpdatedAt:     row.UpdatedAt.UTC(),
	}
}
