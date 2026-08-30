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

// flattenedFields returns the fields the active matching groups serve on the type,
// the first group holding a key winning it.
func flattenedFields(groups []content.Group, typeKey string) []content.Field {
	var fields []content.Field
	served := make(map[string]bool)
	for _, g := range groups {
		if !g.Active || !g.Location.Match(screenOf(typeKey), locationParams) {
			continue
		}
		for _, f := range g.Fields {
			if served[f.Key] {
				continue
			}
			served[f.Key] = true
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
	var updated content.Group
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("postgres: lock field groups: %w", err)
		}
		if err := groupStandsAlone(ctx, queries, g); err != nil {
			return err
		}
		row, err := queries.UpdateFieldGroup(ctx, db.UpdateFieldGroupParams{
			Title: g.Title, Location: locationJSON(g.Location), Active: g.Active,
			UpdatedAt: time.Now().UTC(), ID: int32(g.ID),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return content.ErrGroupNotFound
		}
		if err != nil {
			return err
		}
		updated, err = toGroup(row)
		return err
	})
	if err != nil {
		return content.Group{}, updateGroupFailure(err)
	}
	return updated, nil
}

// updateGroupFailure returns the error a group update carries, and wraps anything else.
func updateGroupFailure(err error) error {
	if errors.Is(err, content.ErrGroupNotFound) || errors.Is(err, content.ErrFieldTaken) {
		return err
	}
	return fmt.Errorf("postgres: update field group: %w", err)
}

// groupStandsAlone reports whether the group's stored keys stay free of every rival it would share a type with.
func groupStandsAlone(ctx context.Context, queries *db.Queries, g content.Group) error {
	groups, err := groupsWithFields(ctx, queries)
	if err != nil {
		return err
	}
	held, found := groupByID(groups, g.ID)
	if !found {
		return nil
	}
	keys := make([]string, 0, len(held.Fields))
	for _, f := range held.Fields {
		keys = append(keys, f.Key)
	}
	types, err := storedTypes(ctx, queries)
	if err != nil {
		return err
	}
	g.Fields = held.Fields
	return content.Uncollided(types, groups, g, keys, 0, locationParams)
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
		types, err := storedTypes(ctx, queries)
		if err != nil {
			return err
		}
		for _, f := range held.Fields {
			swept := sweptByDelete(groups, types, held.ID, f.Key)
			if err := deleteFieldRow(ctx, queries, held.ID, f.Key, swept); err != nil {
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

// storedTypes returns every registered type carrying nothing but its key.
func storedTypes(ctx context.Context, queries *db.Queries) ([]content.Type, error) {
	names, err := queries.TypeKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: list type keys: %w", err)
	}
	types := make([]content.Type, len(names))
	for i, key := range names {
		types[i] = content.Type{Key: key}
	}
	return types, nil
}

// keyFreeInGroup reports whether the key stays open on every type the group serves.
func keyFreeInGroup(ctx context.Context, queries *db.Queries, groupID int, key string, leaving int) error {
	groups, err := groupsWithFields(ctx, queries)
	if err != nil {
		return err
	}
	target, found := groupByID(groups, groupID)
	if !found {
		return content.ErrGroupNotFound
	}
	types, err := storedTypes(ctx, queries)
	if err != nil {
		return err
	}
	return content.Uncollided(types, groups, target, []string{key}, leaving, locationParams)
}

// holdsKey reports whether the group declares a field under the key.
func holdsKey(g content.Group, key string) bool {
	for _, f := range g.Fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

// servedElsewhere returns the types another active group still serves the key on.
func servedElsewhere(groups []content.Group, types []content.Type, leaving int, key string) map[string]bool {
	spared := make(map[string]bool)
	for _, g := range groups {
		if g.ID == leaving || !g.Active || !holdsKey(g, key) {
			continue
		}
		for _, t := range types {
			if g.Location.Match(screenOf(t.Key), locationParams) {
				spared[t.Key] = true
			}
		}
	}
	return spared
}

// sweptByDelete returns the types a deleted key clears from, sparing those another group still serves it on.
func sweptByDelete(groups []content.Group, types []content.Type, leaving int, key string) []string {
	spared := servedElsewhere(groups, types, leaving, key)
	swept := make([]string, 0, len(types))
	for _, t := range types {
		if !spared[t.Key] {
			swept = append(swept, t.Key)
		}
	}
	return swept
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

// UpdateFieldInGroup stores the field's label, required flag and settings when the expectation still holds.
func (s *TypeStore) UpdateFieldInGroup(
	ctx context.Context, groupID int, f content.Field, expectedUpdatedAt time.Time,
) (content.Field, error) {
	row, err := s.queries.UpdateContentField(ctx, db.UpdateContentFieldParams{
		Label:             f.Label,
		Required:          f.Required,
		Settings:          settingsJSON(f.Settings),
		UpdatedAt:         f.UpdatedAt,
		ExpectedUpdatedAt: expectedUpdatedAt,
		GroupID:           int32(groupID),
		Key:               f.Key,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		if err := s.fieldStands(ctx, groupID, f.Key); err != nil {
			return content.Field{}, err
		}
		return content.Field{}, content.ErrConflict
	}
	if err != nil {
		return content.Field{}, fmt.Errorf("postgres: update content field: %w", err)
	}
	return toField(row), nil
}

// fieldStands reports whether the group still declares the field.
func (s *TypeStore) fieldStands(ctx context.Context, groupID int, key string) error {
	groups, err := groupsWithFields(ctx, s.queries)
	if err != nil {
		return err
	}
	held, found := groupByID(groups, groupID)
	if !found {
		return content.ErrGroupNotFound
	}
	for _, f := range held.Fields {
		if f.Key == key {
			return nil
		}
	}
	return content.ErrFieldNotFound
}

// DeleteFieldInGroup removes the field and sweeps its values from the types its group matches.
func (s *TypeStore) DeleteFieldInGroup(ctx context.Context, groupID int, key string) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		groups, err := groupsWithFields(ctx, queries)
		if err != nil {
			return err
		}
		held, found := groupByID(groups, groupID)
		if !found {
			return content.ErrGroupNotFound
		}
		matched, err := typesMatchedBy(ctx, queries, held)
		if err != nil {
			return err
		}
		return deleteFieldRow(ctx, queries, groupID, key, matched)
	})
	if errors.Is(err, content.ErrGroupNotFound) {
		return err
	}
	if err != nil {
		return fmt.Errorf("postgres: delete content field: %w", err)
	}
	return nil
}

// ReorderFieldsInGroup stores the declaration order of a group's fields.
func (s *TypeStore) ReorderFieldsInGroup(ctx context.Context, groupID int, keys []string) error {
	err := s.queries.ReorderContentFields(ctx, db.ReorderContentFieldsParams{
		Keys:    keys,
		GroupID: int32(groupID),
	})
	if err != nil {
		return fmt.Errorf("postgres: reorder content fields: %w", err)
	}
	return nil
}

// MoveField carries the field into another group, keeping the values it holds.
func (s *TypeStore) MoveField(
	ctx context.Context, groupID int, key string, toGroup int,
) (content.Field, error) {
	var moved content.Field
	err := s.settledFieldWrite(ctx, toGroup, key, groupID, func(queries *db.Queries) error {
		row, err := queries.MoveContentField(ctx, db.MoveContentFieldParams{
			ToGroup: int32(toGroup), UpdatedAt: time.Now().UTC(), GroupID: int32(groupID), Key: key,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return content.ErrFieldNotFound
		}
		if err != nil {
			return err
		}
		moved = toField(row)
		return nil
	})
	if err != nil {
		return content.Field{}, err
	}
	return moved, nil
}

// settledFieldWrite runs the write once the key is held free of every rival group sharing a type.
func (s *TypeStore) settledFieldWrite(
	ctx context.Context, groupID int, key string, leaving int, write func(*db.Queries) error,
) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("postgres: lock field groups: %w", err)
		}
		if err := keyFreeInGroup(ctx, queries, groupID, key, leaving); err != nil {
			return err
		}
		return write(queries)
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, content.ErrFieldNotFound) || errors.Is(err, content.ErrGroupNotFound) ||
		errors.Is(err, content.ErrFieldTaken) {
		return err
	}
	return fieldWriteFailure(err)
}

// CreateFieldInGroup declares the field inside the group.
func (s *TypeStore) CreateFieldInGroup(ctx context.Context, groupID int, f content.Field) (content.Field, error) {
	var created content.Field
	err := s.settledFieldWrite(ctx, groupID, f.Key, 0, func(queries *db.Queries) error {
		row, err := queries.CreateContentField(ctx, db.CreateContentFieldParams{
			GroupID:   int32(groupID),
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
		return nil
	})
	if err != nil {
		return content.Field{}, err
	}
	return created, nil
}
