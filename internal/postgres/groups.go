// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// locationParams holds the built in rule sources the store evaluates locations with.
var locationParams = content.DefaultParamRegistry(nil)

// toGroup maps a stored row to a domain group without its fields.
func toGroup(row db.CoreFieldGroup) (content.Group, error) {
	var location content.Rules
	if err := json.Unmarshal(row.Location, &location); err != nil {
		return content.Group{}, fmt.Errorf("postgres: reading a group location: %w", err)
	}
	return content.Group{
		ID:        int(row.ID),
		Title:     row.Title,
		Location:  location,
		Position:  int(row.Position),
		Active:    row.Active,
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

// locationJSON returns the location rules as the jsonb column holds them.
func locationJSON(location content.Rules) []byte {
	raw, _ := json.Marshal(location.Normalize())
	return raw
}

// groupsWithFields loads every group in position order with its fields attached.
func groupsWithFields(ctx context.Context, queries *db.Queries) ([]content.Group, error) {
	rows, err := queries.ListFieldGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: list field groups: %w", err)
	}
	declared, err := queries.ListContentFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: list content fields: %w", err)
	}
	held := make(map[int][]content.Field, len(rows))
	for _, row := range declared {
		held[int(row.GroupID)] = append(held[int(row.GroupID)], toField(row))
	}
	groups := make([]content.Group, len(rows))
	for i, row := range rows {
		group, err := toGroup(row)
		if err != nil {
			return nil, err
		}
		group.Fields = held[group.ID]
		groups[i] = group
	}
	return groups, nil
}

// screenOf returns the location screen of a content type.
func screenOf(typeKey string) content.Screen {
	return content.Screen{content.ScreenContentType: typeKey}
}

// flattenedFields returns the fields the active matching groups serve on the type.
func flattenedFields(groups []content.Group, typeKey string) []content.Field {
	var fields []content.Field
	for _, g := range groups {
		if !g.Active || !g.Location.Match(screenOf(typeKey), locationParams) {
			continue
		}
		for _, f := range g.Fields {
			f.TypeKey = typeKey
			fields = append(fields, f)
		}
	}
	return fields
}

// typesMatchedBy returns the stored type keys the group's rules name, active or not.
func typesMatchedBy(ctx context.Context, queries *db.Queries, g content.Group) ([]string, error) {
	keys, err := queries.TypeKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: list type keys: %w", err)
	}
	var matched []string
	for _, key := range keys {
		if g.Location.Match(screenOf(key), locationParams) {
			matched = append(matched, key)
		}
	}
	return matched, nil
}

// matchingGroupIDs returns the ids of the active groups matching the type.
func matchingGroupIDs(ctx context.Context, queries *db.Queries, typeKey string) ([]int32, error) {
	rows, err := queries.ListFieldGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: list field groups: %w", err)
	}
	var ids []int32
	for _, row := range rows {
		group, err := toGroup(row)
		if err != nil {
			return nil, err
		}
		if group.Active && group.Location.Match(screenOf(typeKey), locationParams) {
			ids = append(ids, int32(group.ID))
		}
	}
	return ids, nil
}

// literalLocationOf returns the one rule location naming the type.
func literalLocationOf(typeKey string) content.Rules {
	return content.Rules{{{
		Source: content.ScreenContentType, Operator: content.OperatorIs, Value: typeKey,
	}}}
}

// groupForType returns the type's literal group, else its first active matching group.
func groupForType(ctx context.Context, queries *db.Queries, typeKey string) (content.Group, bool, error) {
	row, err := queries.GroupByLocation(ctx, locationJSON(literalLocationOf(typeKey)))
	if err == nil {
		group, mapErr := toGroup(row)
		return group, mapErr == nil, mapErr
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return content.Group{}, false, fmt.Errorf("postgres: find the type's group: %w", err)
	}
	groups, err := groupsWithFields(ctx, queries)
	if err != nil {
		return content.Group{}, false, err
	}
	for _, g := range groups {
		if g.Active && g.Location.Match(screenOf(typeKey), locationParams) {
			return g, true, nil
		}
	}
	return content.Group{}, false, nil
}

// groupForWrite returns the group a per type field write targets, raising the default when none matches.
func groupForWrite(ctx context.Context, queries *db.Queries, typeKey string) (content.Group, error) {
	found, ok, err := groupForType(ctx, queries, typeKey)
	if err != nil {
		return content.Group{}, err
	}
	if ok {
		return found, nil
	}
	named, err := queries.GetContentType(ctx, typeKey)
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Group{}, content.ErrTypeNotFound
	}
	if err != nil {
		return content.Group{}, fmt.Errorf("postgres: get content type: %w", err)
	}
	now := time.Now().UTC()
	row, err := queries.CreateFieldGroup(ctx, db.CreateFieldGroupParams{
		Title:     named.SingularLabel + " fields",
		Location:  locationJSON(literalLocationOf(typeKey)),
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return content.Group{}, fmt.Errorf("postgres: raise the type's group: %w", err)
	}
	return toGroup(row)
}

// ownerOf returns the matching group declaring the key on the type, or reports the field missing.
func ownerOf(ctx context.Context, queries *db.Queries, typeKey, key string) (content.Group, error) {
	groups, err := groupsWithFields(ctx, queries)
	if err != nil {
		return content.Group{}, err
	}
	for _, g := range groups {
		if !g.Location.Match(screenOf(typeKey), locationParams) {
			continue
		}
		for _, f := range g.Fields {
			if f.Key == key {
				return g, nil
			}
		}
	}
	return content.Group{}, content.ErrFieldNotFound
}

// ListGroups returns every field group in position order with its fields attached.
func (s *TypeStore) ListGroups(ctx context.Context) ([]content.Group, error) {
	return groupsWithFields(ctx, s.queries)
}

// CreateGroup stores a new field group at the end of the order.
func (s *TypeStore) CreateGroup(ctx context.Context, g content.Group) (content.Group, error) {
	now := time.Now().UTC()
	row, err := s.queries.CreateFieldGroup(ctx, db.CreateFieldGroupParams{
		Title: g.Title, Location: locationJSON(g.Location), CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return content.Group{}, fmt.Errorf("postgres: create field group: %w", err)
	}
	return toGroup(row)
}

// UpdateGroup stores the group's title, location and active flag.
func (s *TypeStore) UpdateGroup(ctx context.Context, g content.Group) (content.Group, error) {
	row, err := s.queries.UpdateFieldGroup(ctx, db.UpdateFieldGroupParams{
		Title: g.Title, Location: locationJSON(g.Location), Active: g.Active,
		UpdatedAt: time.Now().UTC(), ID: int32(g.ID),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Group{}, content.ErrGroupNotFound
	}
	if err != nil {
		return content.Group{}, fmt.Errorf("postgres: update field group: %w", err)
	}
	return toGroup(row)
}

// DeleteGroup removes the group, its fields and their stored values in one transaction.
func (s *TypeStore) DeleteGroup(ctx context.Context, id int) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		groups, err := groupsWithFields(ctx, queries)
		if err != nil {
			return err
		}
		held, found := groupByID(groups, id)
		if !found {
			return content.ErrGroupNotFound
		}
		matched, err := typesMatchedBy(ctx, queries, held)
		if err != nil {
			return err
		}
		for _, f := range held.Fields {
			if err := deleteFieldRow(ctx, queries, held.ID, f.Key, matched); err != nil {
				return err
			}
		}
		if _, err := queries.DeleteFieldGroup(ctx, int32(id)); err != nil {
			return err
		}
		return nil
	})
	if errors.Is(err, content.ErrGroupNotFound) {
		return err
	}
	if err != nil {
		return fmt.Errorf("postgres: delete field group: %w", err)
	}
	return nil
}

// groupByID returns the group holding the identifier, and whether one does.
func groupByID(groups []content.Group, id int) (content.Group, bool) {
	for _, g := range groups {
		if g.ID == id {
			return g, true
		}
	}
	return content.Group{}, false
}

// deleteFieldRow removes one field row and sweeps its values from the matched types.
func deleteFieldRow(ctx context.Context, queries *db.Queries, groupID int, key string, matched []string) error {
	if _, err := queries.DeleteContentField(ctx, db.DeleteContentFieldParams{
		GroupID: int32(groupID), Key: key,
	}); err != nil {
		return err
	}
	if err := queries.ClearContentFieldValues(ctx, db.ClearContentFieldValuesParams{
		Key: key, Types: matched,
	}); err != nil {
		return err
	}
	return queries.ClearRevisionFieldValues(ctx, db.ClearRevisionFieldValuesParams{
		Key: key, Types: matched,
	})
}

// ReorderGroups stores the given order on the groups.
func (s *TypeStore) ReorderGroups(ctx context.Context, ids []int) error {
	ordered := make([]int32, len(ids))
	for i, id := range ids {
		ordered[i] = int32(id)
	}
	if err := s.queries.ReorderFieldGroups(ctx, ordered); err != nil {
		return fmt.Errorf("postgres: reorder field groups: %w", err)
	}
	return nil
}

// CreateFieldInGroup declares the field inside the group.
func (s *TypeStore) CreateFieldInGroup(ctx context.Context, groupID int, f content.Field) (content.Field, error) {
	row, err := s.queries.CreateContentField(ctx, db.CreateContentFieldParams{
		GroupID:   int32(groupID),
		Key:       f.Key,
		Label:     f.Label,
		Kind:      string(f.Kind),
		RelatesTo: targetOf(f),
		Many:      f.Many,
		Required:  f.Required,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
	})
	if err != nil {
		return content.Field{}, fieldWriteFailure(err)
	}
	return toField(row), nil
}
