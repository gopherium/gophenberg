// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

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
		Key:       row.Key,
		Title:     row.Title,
		Location:  location,
		Position:  int(row.Position),
		Active:    row.Active,
		Origin:    originOf(row.Origin),
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

// locationJSON returns the location rules as the jsonb column holds them.
func locationJSON(location content.Rules) []byte {
	raw, _ := json.Marshal(location.Normalize())
	return raw
}

// fieldsByGroup returns the declared rows as trees of fields, keyed by the group holding them.
func fieldsByGroup(declared []db.CoreContentField) map[int][]content.Field {
	inside := map[int][]content.Field{}
	for _, row := range declared {
		if row.ParentFieldID.Valid {
			at := int(row.ParentFieldID.Int32)
			inside[at] = append(inside[at], toField(row))
		}
	}
	held := map[int][]content.Field{}
	for _, row := range declared {
		if row.ParentFieldID.Valid {
			continue
		}
		held[int(row.GroupID)] = append(held[int(row.GroupID)], grownField(toField(row), inside))
	}
	return held
}

// grownField returns the field carrying the sub fields standing under it in its group, however deep they run.
func grownField(f content.Field, inside map[int][]content.Field) content.Field {
	if len(inside[f.ID]) == 0 {
		return f
	}
	grown := make([]content.Field, 0, len(inside[f.ID]))
	for _, sub := range inside[f.ID] {
		sub.GroupID = f.GroupID
		grown = append(grown, grownField(sub, inside))
	}
	f.Fields = grown
	return f
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
	held := fieldsByGroup(declared)
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
	if ok && found.Origin != "" {
		return content.Group{}, content.OwnedBy(found.Origin)
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
		Key:       typeKey + "-fields",
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
	var created content.Group
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("postgres: lock field groups: %w", err)
		}
		key, err := groupKeyFor(ctx, queries, g)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		row, err := queries.CreateFieldGroup(ctx, db.CreateFieldGroupParams{
			Key: key, Title: g.Title, Location: locationJSON(g.Location),
			Origin: originColumn(g.Origin), CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return groupWriteFailure(err)
		}
		created, err = toGroup(row)
		return err
	})
	return created, err
}

// groupKeyFor returns the key the group asked for, or one minted from its title that no group holds.
func groupKeyFor(ctx context.Context, queries *db.Queries, g content.Group) (string, error) {
	if g.Key != "" {
		return g.Key, nil
	}
	held, err := queries.FieldGroupKeys(ctx)
	if err != nil {
		return "", fmt.Errorf("postgres: list field group keys: %w", err)
	}
	taken := make(map[string]bool, len(held))
	for _, key := range held {
		taken[key] = true
	}
	stem := content.GroupKeyFrom(g.Title)
	key := stem
	for n := 2; taken[key]; n++ {
		key = fmt.Sprintf("%s-%d", stem, n)
	}
	return key, nil
}

// groupWriteFailure returns the error a group write carries, and wraps anything else.
func groupWriteFailure(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolationCode && pgErr.ConstraintName == groupKeyConstraint {
		return content.ErrGroupKeyTaken
	}
	return fmt.Errorf("postgres: create field group: %w", err)
}

// UpdateGroup stores the group's title, location and active flag with the settings of the fields it points anew.
func (s *TypeStore) UpdateGroup(
	ctx context.Context, g content.Group, repointed []content.Field, recheck content.Recheck,
) (content.Group, error) {
	var updated content.Group
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("postgres: lock field groups: %w", err)
		}
		if err := rechecked(ctx, queries, recheck); err != nil {
			return err
		}
		if err := groupStandsAlone(ctx, queries, g); err != nil {
			return err
		}
		now := time.Now().UTC()
		row, err := queries.UpdateFieldGroup(ctx, db.UpdateFieldGroupParams{
			Title: g.Title, Location: locationJSON(g.Location), Active: g.Active,
			UpdatedAt: now, ID: int32(g.ID),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return content.ErrGroupNotFound
		}
		if err != nil {
			return err
		}
		if err := repoint(ctx, queries, g.ID, repointed, now); err != nil {
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

// repoint stores the settings of each field pointed anew, refusing one that changed since it was read.
func repoint(ctx context.Context, queries *db.Queries, groupID int, fields []content.Field, now time.Time) error {
	for _, f := range fields {
		_, err := queries.UpdateContentField(ctx, db.UpdateContentFieldParams{
			Label: f.Label, Required: f.Required, Settings: settingsJSON(f.Settings),
			UpdatedAt: now, ExpectedUpdatedAt: f.UpdatedAt, GroupID: int32(groupID), Key: f.Key,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return content.ErrConflict
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// updateGroupFailure returns the error a group update carries, and wraps anything else.
func updateGroupFailure(err error) error {
	if errors.Is(err, content.ErrGroupNotFound) || errors.Is(err, content.ErrFieldTaken) || fromCheck(err) {
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

// DeleteGroup removes the group, its fields and their stored values in one transaction, once the check passes.
func (s *TypeStore) DeleteGroup(ctx context.Context, id int, recheck content.Recheck) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return deleteGroupRows(ctx, s.queries.WithTx(tx), id, recheck)
	})
	if errors.Is(err, content.ErrGroupNotFound) || fromCheck(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("postgres: delete field group: %w", err)
	}
	return nil
}

// deleteGroupRows removes the group and its own fields, carrying what it stores inside other groups' containers.
func deleteGroupRows(ctx context.Context, queries *db.Queries, id int, recheck content.Recheck) error {
	groups, held, err := lockedGroup(ctx, queries, id, recheck)
	if err != nil {
		return err
	}
	if err := queries.CarryStrayFieldsOfGroup(ctx, int32(id)); err != nil {
		return err
	}
	if err := deleteFieldsOf(ctx, queries, groups, held); err != nil {
		return err
	}
	_, err = queries.DeleteFieldGroup(ctx, int32(id))
	return err
}

// lockedGroup locks the field groups and returns every stored group with the identified one, once the check passes.
func lockedGroup(
	ctx context.Context, queries *db.Queries, id int, recheck content.Recheck,
) ([]content.Group, content.Group, error) {
	if err := queries.LockFieldGroups(ctx); err != nil {
		return nil, content.Group{}, err
	}
	if err := rechecked(ctx, queries, recheck); err != nil {
		return nil, content.Group{}, err
	}
	groups, err := groupsWithFields(ctx, queries)
	if err != nil {
		return nil, content.Group{}, err
	}
	held, found := groupByID(groups, id)
	if !found {
		return nil, content.Group{}, content.ErrGroupNotFound
	}
	return groups, held, nil
}

// recheckError carries what the caller's check raised under the lock, reading as the check wrote it.
type recheckError struct{ err error }

// Error returns the message the check wrote.
func (e recheckError) Error() string { return e.err.Error() }

// Unwrap returns what the check raised.
func (e recheckError) Unwrap() error { return e.err }

// rechecked runs the caller's check on the groups and types stored, once the field groups are locked.
func rechecked(ctx context.Context, queries *db.Queries, recheck content.Recheck) error {
	if recheck == nil {
		return nil
	}
	groups, err := groupsWithFields(ctx, queries)
	if err != nil {
		return err
	}
	types, err := storedTypes(ctx, queries)
	if err != nil {
		return err
	}
	if err := recheck(groups, types); err != nil {
		return recheckError{err}
	}
	return nil
}

// fromCheck reports whether the error is what the caller's check raised under the lock.
func fromCheck(err error) bool {
	var held recheckError
	return errors.As(err, &held)
}

// DeleteFieldsOfGroup removes the group's named top level fields, sweeping their values as a group delete does.
func (s *TypeStore) DeleteFieldsOfGroup(
	ctx context.Context, groupID int, keys []string, recheck content.Recheck,
) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		groups, held, err := lockedGroup(ctx, queries, groupID, recheck)
		if err != nil {
			return err
		}
		held.Fields = slices.DeleteFunc(slices.Clone(held.Fields), func(f content.Field) bool {
			return !slices.Contains(keys, f.Key)
		})
		return deleteFieldsOf(ctx, queries, groups, held)
	})
	if errors.Is(err, content.ErrGroupNotFound) || fromCheck(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("postgres: delete fields of group: %w", err)
	}
	return nil
}

// deleteFieldsOf removes the group's fields and sweeps their values from the types no other group serves them on.
func deleteFieldsOf(ctx context.Context, queries *db.Queries, groups []content.Group, held content.Group) error {
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

// UpdateFieldInGroup stores the field's label, required flag and settings when the expectation and the check hold.
func (s *TypeStore) UpdateFieldInGroup(
	ctx context.Context, groupID int, f content.Field, expectedUpdatedAt time.Time, recheck content.Recheck,
) (content.Field, error) {
	var updated content.Field
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("postgres: lock field groups: %w", err)
		}
		if err := rechecked(ctx, queries, recheck); err != nil {
			return err
		}
		row, err := queries.UpdateContentField(ctx, db.UpdateContentFieldParams{
			Label:             f.Label,
			Required:          f.Required,
			Settings:          settingsJSON(f.Settings),
			UpdatedAt:         f.UpdatedAt,
			ExpectedUpdatedAt: expectedUpdatedAt,
			GroupID:           int32(groupID),
			Key:               f.Key,
		})
		if errors.Is(err, pgx.ErrNoRows) {
			if err := fieldStands(ctx, queries, groupID, f.Key); err != nil {
				return err
			}
			return content.ErrConflict
		}
		if err != nil {
			return fmt.Errorf("postgres: update content field: %w", err)
		}
		updated = toField(row)
		return nil
	})
	return updated, err
}

// UpdateSubField carries the edit onto the sub field the identity names.
func (s *TypeStore) UpdateSubField(
	ctx context.Context, id int, f content.Field, expectedUpdatedAt time.Time,
) (content.Field, error) {
	row, err := s.queries.UpdateSubContentField(ctx, db.UpdateSubContentFieldParams{
		Label:             f.Label,
		Required:          f.Required,
		Settings:          settingsJSON(f.Settings),
		UpdatedAt:         f.UpdatedAt,
		ExpectedUpdatedAt: expectedUpdatedAt,
		ID:                int32(id),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Field{}, content.ErrConflict
	}
	if err != nil {
		return content.Field{}, fmt.Errorf("postgres: update sub field: %w", err)
	}
	return toField(row), nil
}

// ReorderSubFields stands the sub fields of the container in the order the keys name.
func (s *TypeStore) ReorderSubFields(ctx context.Context, parentID int, keys []string) error {
	if err := s.queries.ReorderSubContentFields(ctx, db.ReorderSubContentFieldsParams{
		ParentFieldID: pgtype.Int4{Int32: int32(parentID), Valid: true},
		Keys:          keys,
	}); err != nil {
		return fmt.Errorf("postgres: reorder sub fields: %w", err)
	}
	return nil
}

// fieldStands reports whether the group still declares the field.
func fieldStands(ctx context.Context, queries *db.Queries, groupID int, key string) error {
	groups, err := groupsWithFields(ctx, queries)
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

// DeleteFieldInGroup removes the field and sweeps its values from the types its group serves the key on.
func (s *TypeStore) DeleteFieldInGroup(ctx context.Context, groupID int, key string, recheck content.Recheck) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := queries.LockFieldGroups(ctx); err != nil {
			return err
		}
		if err := rechecked(ctx, queries, recheck); err != nil {
			return err
		}
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
		return deleteFieldRow(ctx, queries, groupID, key, servedOn(groups, matched, groupID, []string{key}))
	})
	if errors.Is(err, content.ErrGroupNotFound) || fromCheck(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("postgres: delete content field: %w", err)
	}
	return nil
}

// DeleteSubField removes the field standing inside a container, and its values on the types its group serves.
func (s *TypeStore) DeleteSubField(ctx context.Context, id int, recheck content.Recheck) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return deleteSubFieldRows(ctx, s.queries.WithTx(tx), id, recheck)
	})
	if errors.Is(err, content.ErrFieldNotFound) || errors.Is(err, content.ErrGroupNotFound) || fromCheck(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("postgres: delete sub field: %w", err)
	}
	return nil
}

// deleteSubFieldRows removes the sub field once the check passes, sweeping its values where no other group serves them.
func deleteSubFieldRows(ctx context.Context, queries *db.Queries, id int, recheck content.Recheck) error {
	if err := queries.LockFieldGroups(ctx); err != nil {
		return err
	}
	if err := rechecked(ctx, queries, recheck); err != nil {
		return err
	}
	groups, err := groupsWithFields(ctx, queries)
	if err != nil {
		return err
	}
	group, dropped, path, found := fieldPathIn(groups, id)
	if !found {
		return content.ErrFieldNotFound
	}
	matched, err := typesMatchedBy(ctx, queries, group)
	if err != nil {
		return err
	}
	if _, err := queries.DeleteFieldByID(ctx, int32(id)); err != nil {
		return err
	}
	return sweepField(ctx, queries, dropped, path, servedOn(groups, matched, group.ID, path))
}

// fieldPathIn returns the group holding the field, the field, and the keys addressing it from the group.
func fieldPathIn(groups []content.Group, id int) (content.Group, content.Field, []string, bool) {
	for _, group := range groups {
		if held, path, found := pathToField(group.Fields, id); found {
			return group, held, path, true
		}
	}
	return content.Group{}, content.Field{}, nil, false
}

// pathToField returns the field and the keys addressing it among the declared ones, however deep it stands.
func pathToField(declared []content.Field, id int) (content.Field, []string, bool) {
	for _, f := range declared {
		if f.ID == id {
			return f, []string{f.Key}, true
		}
		if held, inside, found := pathToField(f.Fields, id); found {
			return held, append([]string{f.Key}, inside...), true
		}
	}
	return content.Field{}, nil, false
}

// sweepField removes what the field held at the path from every item and revision of the matched types.
func sweepField(ctx context.Context, queries *db.Queries, f content.Field, path, matched []string) error {
	if f.Kind == content.FieldKindLayout {
		return sweepLayout(ctx, queries, path, matched)
	}
	return sweepPath(ctx, queries, path, matched)
}

// sweepPath removes whatever stands at the path from every item and revision of the matched types.
func sweepPath(ctx context.Context, queries *db.Queries, path []string, matched []string) error {
	if err := queries.StripContentFieldPath(ctx, db.StripContentFieldPathParams{
		Path: path, Types: matched, Key: path[0],
	}); err != nil {
		return err
	}
	return queries.StripRevisionFieldPath(ctx, db.StripRevisionFieldPathParams{
		Path: path, Types: matched, Key: path[0],
	})
}

// sweepLayout removes the rows a layout held from every item and revision of the matched types.
func sweepLayout(ctx context.Context, queries *db.Queries, path []string, matched []string) error {
	if err := queries.StripContentLayout(ctx, db.StripContentLayoutParams{
		Path: path, Types: matched, Key: path[0],
	}); err != nil {
		return err
	}
	return queries.StripRevisionLayout(ctx, db.StripRevisionLayoutParams{
		Path: path, Types: matched, Key: path[0],
	})
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

// MoveField carries the field to a group's top or into a container within the limit, sweeping its values when it
// enters or leaves one.
func (s *TypeStore) MoveField(
	ctx context.Context, id, toGroup, toParent, limit int, recheck content.Recheck,
) (content.Field, error) {
	var moved content.Field
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("postgres: lock field groups: %w", err)
		}
		if err := rechecked(ctx, queries, recheck); err != nil {
			return err
		}
		move, err := movePlanned(ctx, queries, id, toGroup, toParent, limit)
		if err != nil {
			return err
		}
		if err := move.keyFree(ctx, queries); err != nil {
			return err
		}
		moved, err = move.write(ctx, queries)
		return err
	})
	if err == nil {
		return moved, nil
	}
	return content.Field{}, moveFailure(err)
}

// moveFailure returns the refusal a field move carries, and wraps anything else.
func moveFailure(err error) error {
	if fromCheck(err) {
		return err
	}
	for _, refusal := range []error{
		content.ErrFieldNotFound, content.ErrGroupNotFound, content.ErrFieldTaken,
		content.ErrFieldInsideItself, content.ErrFieldTooDeep,
	} {
		if errors.Is(err, refusal) {
			return err
		}
	}
	return fieldWriteFailure(err)
}

// fieldMove is one field leaving its place for another, as the store carries it.
type fieldMove struct {
	id, toGroup, toParent int
	leaving               content.Field
	source                content.Group
	groups                []content.Group
	path                  []string
	depth                 int
}

// movePlanned resolves where the field stands and where it is asked to stand, refusing a landing past the limit.
func movePlanned(ctx context.Context, queries *db.Queries, id, toGroup, toParent, limit int) (fieldMove, error) {
	groups, err := groupsWithFields(ctx, queries)
	if err != nil {
		return fieldMove{}, err
	}
	source, leaving, path, found := fieldPathIn(groups, id)
	if !found {
		return fieldMove{}, content.ErrFieldNotFound
	}
	landing, found := groupByID(groups, toGroup)
	if !found {
		return fieldMove{}, content.ErrGroupNotFound
	}
	move := fieldMove{
		id: id, toGroup: toGroup, toParent: toParent,
		leaving: leaving, source: source, path: path, groups: groups,
	}
	if toParent != 0 {
		_, above, found := pathToField(landing.Fields, toParent)
		if !found {
			return fieldMove{}, content.ErrFieldNotFound
		}
		if content.Inside(leaving, toParent) {
			return fieldMove{}, content.MovesInsideItself(leaving.Key)
		}
		move.depth = len(above)
	}
	if err := content.WithinDepth(leaving, move.depth, limit); err != nil {
		return fieldMove{}, err
	}
	return move, nil
}

// keyFree re-checks the key against the rival groups when the field lands at a group's top.
func (m fieldMove) keyFree(ctx context.Context, queries *db.Queries) error {
	if m.toParent != 0 {
		return nil
	}
	leaving := 0
	if m.leaving.ParentID == 0 {
		leaving = m.source.ID
	}
	return keyFreeInGroup(ctx, queries, m.toGroup, m.leaving.Key, leaving)
}

// write reparents the field, recounts the depth below it and sweeps what its old path held where nothing serves it.
func (m fieldMove) write(ctx context.Context, queries *db.Queries) (content.Field, error) {
	row, err := queries.ReparentContentField(ctx, db.ReparentContentFieldParams{
		ID: int32(m.id), ToGroup: int32(m.toGroup), ToParent: parentColumn(m.toParent),
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		return content.Field{}, err
	}
	if err := queries.RecountContentFieldDepth(ctx, db.RecountContentFieldDepthParams{
		ID: int32(m.id), Depth: int32(m.depth), ToGroup: int32(m.toGroup),
	}); err != nil {
		return content.Field{}, err
	}
	matched, err := typesMatchedBy(ctx, queries, m.source)
	if err != nil {
		return content.Field{}, err
	}
	if m.leaving.ParentID == m.toParent {
		matched = m.behind(matched)
	}
	return toField(row), m.sweep(ctx, queries, matched)
}

// behind returns the matched types the landing group does not reach.
func (m fieldMove) behind(matched []string) []string {
	landing, _ := groupByID(m.groups, m.toGroup)
	return slices.DeleteFunc(matched, func(key string) bool {
		return landing.Location.Match(screenOf(key), locationParams)
	})
}

// sweep takes what the field held at its old path out of the matched types, their relation rows included.
func (m fieldMove) sweep(ctx context.Context, queries *db.Queries, matched []string) error {
	if len(matched) == 0 {
		return nil
	}
	swept := servedOn(m.groups, matched, m.source.ID, m.path)
	if err := sweepField(ctx, queries, m.leaving, m.path, swept); err != nil {
		return err
	}
	fields := relationsBelow(m.leaving, nil)
	if len(fields) == 0 {
		return nil
	}
	return queries.DeleteRelationsOfFields(ctx, db.DeleteRelationsOfFieldsParams{Types: matched, Fields: fields})
}

// servedOn returns the matched type keys on which no other active group serves the whole path.
func servedOn(groups []content.Group, matched []string, groupID int, path []string) []string {
	held := make([]string, 0, len(matched))
	for _, typeKey := range matched {
		by, found := servingGroup(groups, typeKey, path[0])
		if !found || by.ID == groupID || !declares(by.Fields, path) {
			held = append(held, typeKey)
		}
	}
	return held
}

// servingGroup returns the active group serving the top key on the type, if any does.
func servingGroup(groups []content.Group, typeKey, key string) (content.Group, bool) {
	for _, g := range groups {
		if g.Active && holdsKey(g, key) && g.Location.Match(screenOf(typeKey), locationParams) {
			return g, true
		}
	}
	return content.Group{}, false
}

// declares reports whether a field stands at the path among the fields.
func declares(fields []content.Field, path []string) bool {
	if len(path) == 0 {
		return true
	}
	for _, f := range fields {
		if f.Key == path[0] {
			return declares(f.Fields, path[1:])
		}
	}
	return false
}

// parentColumn returns the parent as the nullable column holds it, null for a group's top.
func parentColumn(parentID int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(parentID), Valid: parentID != 0}
}

// relationsBelow appends the identities of the relation fields the field is or holds, however deep.
func relationsBelow(f content.Field, held []int32) []int32 {
	if f.Kind == content.FieldKindRelation {
		held = append(held, int32(f.ID))
	}
	for _, inside := range f.Fields {
		held = relationsBelow(inside, held)
	}
	return held
}

// settledFieldWrite runs the write once the check passes and the key is held free of every rival group sharing a type.
func (s *TypeStore) settledFieldWrite(
	ctx context.Context, groupID int, key string, leaving int, recheck content.Recheck, write func(*db.Queries) error,
) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("postgres: lock field groups: %w", err)
		}
		if err := rechecked(ctx, queries, recheck); err != nil {
			return err
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
		errors.Is(err, content.ErrFieldTaken) || fromCheck(err) {
		return err
	}
	return fieldWriteFailure(err)
}

// CreateSubField declares the field inside the container the parent names, within the limit.
func (s *TypeStore) CreateSubField(
	ctx context.Context, parentID int, f content.Field, limit int,
) (content.Field, error) {
	var created content.Field
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("postgres: lock field groups: %w", err)
		}
		parent, settled, err := standingParent(ctx, queries, parentID, f, limit)
		if err != nil {
			return err
		}
		row, err := queries.CreateSubContentField(ctx, db.CreateSubContentFieldParams{
			GroupID:       parent.GroupID,
			Origin:        originColumn(settled.Origin),
			ParentFieldID: pgtype.Int4{Int32: int32(parentID), Valid: true},
			Key:           settled.Key,
			Label:         settled.Label,
			Kind:          string(settled.Kind),
			RelatesTo:     targetOf(settled),
			Many:          settled.Many,
			Required:      settled.Required,
			CreatedAt:     settled.CreatedAt,
			UpdatedAt:     settled.UpdatedAt,
			Settings:      settingsJSON(settled.Settings),
			Depth:         parent.Depth + 1,
		})
		if err != nil {
			return err
		}
		created = toField(row)
		return nil
	})
	if err == nil {
		return created, nil
	}
	if errors.Is(err, content.ErrFieldNotFound) || errors.Is(err, content.ErrFieldShape) ||
		errors.Is(err, content.ErrFieldTooDeep) {
		return content.Field{}, err
	}
	return content.Field{}, fieldWriteFailure(err)
}

// standingParent returns the row the sub field may stand under and the field it settled on,
// or the reason it may not stand there.
func standingParent(
	ctx context.Context, queries *db.Queries, parentID int, f content.Field, limit int,
) (db.CoreContentField, content.Field, error) {
	parent, err := queries.FieldByID(ctx, int32(parentID))
	if errors.Is(err, pgx.ErrNoRows) {
		return parent, f, content.ErrFieldNotFound
	}
	if err != nil {
		return parent, f, err
	}
	settled, err := content.NewSubField(f, content.FieldKind(parent.Kind))
	if err != nil {
		return parent, f, err
	}
	if err := content.WithinDepth(settled, int(parent.Depth)+1, limit); err != nil {
		return parent, f, err
	}
	return parent, settled, nil
}

// CreateFieldInGroup declares the field inside the group.
func (s *TypeStore) CreateFieldInGroup(
	ctx context.Context, groupID int, f content.Field, recheck content.Recheck,
) (content.Field, error) {
	var created content.Field
	err := s.settledFieldWrite(ctx, groupID, f.Key, 0, recheck, func(queries *db.Queries) error {
		row, err := queries.CreateContentField(ctx, db.CreateContentFieldParams{
			GroupID:   int32(groupID),
			Origin:    originColumn(f.Origin),
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
