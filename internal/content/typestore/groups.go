// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// locationParams holds the built in rule sources the store evaluates locations with.
var locationParams = content.DefaultParamRegistry(nil)

// GroupFromRow returns the group a stored row holds, naming the engine when its location does not read.
func GroupFromRow(engine dbkit.Engine, row GroupRow) (content.Group, error) {
	var location content.Rules
	if err := json.Unmarshal(row.Location, &location); err != nil {
		return content.Group{}, fmt.Errorf("%s: reading a group location: %w", engine, err)
	}
	return content.Group{
		ID:        row.ID,
		Key:       row.Key,
		Title:     row.Title,
		Location:  location,
		Position:  row.Position,
		Active:    row.Active,
		Origin:    row.Origin,
		CreatedAt: row.CreatedAt.UTC(),
		UpdatedAt: row.UpdatedAt.UTC(),
	}, nil
}

// locationJSON returns the location rules as the jsonb column holds them.
func locationJSON(location content.Rules) []byte {
	raw, _ := json.Marshal(location.Normalize())
	return raw
}

// FieldsByGroup returns the stored field rows as trees, keyed by the group holding them.
func FieldsByGroup(declared []FieldRow) map[int][]content.Field {
	inside := map[int][]content.Field{}
	for _, row := range declared {
		if row.ParentID != 0 {
			at := row.ParentID
			inside[at] = append(inside[at], toField(row))
		}
	}
	held := map[int][]content.Field{}
	for _, row := range declared {
		if row.ParentID != 0 {
			continue
		}
		held[row.GroupID] = append(held[row.GroupID], grownField(toField(row), inside))
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
func (s *Store) groupsWithFields(ctx context.Context, queries Queries) ([]content.Group, error) {
	rows, err := queries.ListFieldGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: list field groups: %w", s.backend.Engine, err)
	}
	declared, err := queries.ListContentFields(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: list content fields: %w", s.backend.Engine, err)
	}
	held := FieldsByGroup(declared)
	groups := make([]content.Group, len(rows))
	for i, row := range rows {
		group, err := GroupFromRow(s.backend.Engine, row)
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

// Serves reports whether the group is active and its location matches the type.
func Serves(g content.Group, typeKey string) bool {
	return g.Active && g.Location.Match(screenOf(typeKey), locationParams)
}

// flattenedFields returns the fields the groups serving the type declare, the first group holding a key winning it.
func flattenedFields(groups []content.Group, typeKey string) []content.Field {
	var fields []content.Field
	served := make(map[string]bool)
	for _, g := range groups {
		if !Serves(g, typeKey) {
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

// storedTypeKeys returns the key of every registered type.
func (s *Store) storedTypeKeys(ctx context.Context, queries Queries) ([]string, error) {
	keys, err := queries.TypeKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: list type keys: %w", s.backend.Engine, err)
	}
	return keys, nil
}

// ListGroups returns every field group in position order with its fields attached.
func (s *Store) ListGroups(ctx context.Context) ([]content.Group, error) {
	return s.groupsWithFields(ctx, s.backend.Pool)
}

// CreateGroup stores a new field group at the end of the order.
func (s *Store) CreateGroup(ctx context.Context, g content.Group) (content.Group, error) {
	var created content.Group
	err := s.backend.InTx(ctx, func(queries Queries) error {
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("%s: lock field groups: %w", s.backend.Engine, err)
		}
		key, err := s.groupKeyFor(ctx, queries, g)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		row, err := queries.CreateFieldGroup(ctx, GroupRow{
			Key: key, Title: g.Title, Location: locationJSON(g.Location),
			Origin: g.Origin, CreatedAt: now, UpdatedAt: now,
		})
		if err != nil {
			return s.groupWriteFailure(err)
		}
		created, err = GroupFromRow(s.backend.Engine, row)
		return err
	})
	return created, err
}

// groupKeyFor returns the key the group asked for, or one minted from its title that no group holds.
func (s *Store) groupKeyFor(ctx context.Context, queries Queries, g content.Group) (string, error) {
	if g.Key != "" {
		return g.Key, nil
	}
	held, err := queries.FieldGroupKeys(ctx)
	if err != nil {
		return "", fmt.Errorf("%s: list field group keys: %w", s.backend.Engine, err)
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
func (s *Store) groupWriteFailure(err error) error {
	if taken := s.errorFor(groupWriteErrors, err); taken != nil {
		return taken
	}
	return fmt.Errorf("%s: create field group: %w", s.backend.Engine, err)
}

// UpdateGroup stores the group's title, location and active flag with the settings of the fields it points anew.
func (s *Store) UpdateGroup(
	ctx context.Context, g content.Group, repointed []content.Field, recheck content.Recheck,
) (content.Group, error) {
	var updated content.Group
	err := s.backend.InTx(ctx, func(queries Queries) error {
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("%s: lock field groups: %w", s.backend.Engine, err)
		}
		if err := s.rechecked(ctx, queries, recheck); err != nil {
			return err
		}
		if err := s.groupStandsAlone(ctx, queries, g); err != nil {
			return err
		}
		now := time.Now().UTC()
		row, err := queries.UpdateFieldGroup(ctx, GroupRow{
			Title: g.Title, Location: locationJSON(g.Location), Active: g.Active,
			UpdatedAt: now, ID: g.ID,
		})
		if errors.Is(err, sql.ErrNoRows) {
			return content.ErrGroupNotFound
		}
		if err != nil {
			return err
		}
		if err := repoint(ctx, queries, g.ID, repointed, now); err != nil {
			return err
		}
		updated, err = GroupFromRow(s.backend.Engine, row)
		return err
	})
	if err != nil {
		return content.Group{}, s.updateGroupFailure(err)
	}
	return updated, nil
}

// repoint stores the settings of each field pointed anew, refusing one that changed since it was read.
func repoint(ctx context.Context, queries Queries, groupID int, fields []content.Field, now time.Time) error {
	for _, f := range fields {
		_, err := queries.UpdateContentField(ctx, FieldRow{
			Label: f.Label, Required: f.Required, Settings: settingsJSON(f.Settings),
			UpdatedAt: now, GroupID: groupID, Key: f.Key,
		}, f.UpdatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return content.ErrConflict
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// updateGroupFailure returns the error a group update carries, and wraps anything else.
func (s *Store) updateGroupFailure(err error) error {
	if errors.Is(err, content.ErrGroupNotFound) || errors.Is(err, content.ErrFieldTaken) || fromCheck(err) {
		return err
	}
	return fmt.Errorf("%s: update field group: %w", s.backend.Engine, err)
}

// groupStandsAlone reports whether the group's stored keys stay free of every rival it would share a type with.
func (s *Store) groupStandsAlone(ctx context.Context, queries Queries, g content.Group) error {
	groups, err := s.groupsWithFields(ctx, queries)
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
	types, err := s.storedTypes(ctx, queries)
	if err != nil {
		return err
	}
	g.Fields = held.Fields
	return content.Uncollided(types, groups, g, keys, 0, locationParams)
}

// DeleteGroup removes the group, its fields and their stored values in one transaction, once the check passes.
func (s *Store) DeleteGroup(ctx context.Context, id int, recheck content.Recheck) error {
	err := s.backend.InTx(ctx, func(queries Queries) error {
		return s.deleteGroupRows(ctx, queries, id, recheck)
	})
	if errors.Is(err, content.ErrGroupNotFound) || fromCheck(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("%s: delete field group: %w", s.backend.Engine, err)
	}
	return nil
}

// deleteGroupRows removes the group and its own fields, carrying what it stores inside other groups' containers.
func (s *Store) deleteGroupRows(ctx context.Context, queries Queries, id int, recheck content.Recheck) error {
	groups, held, err := s.lockedGroup(ctx, queries, id, recheck)
	if err != nil {
		return err
	}
	if err := queries.CarryStrayFieldsOfGroup(ctx, id); err != nil {
		return err
	}
	if err := s.deleteFieldsOf(ctx, queries, groups, held); err != nil {
		return err
	}
	_, err = queries.DeleteFieldGroup(ctx, id)
	return err
}

// lockedGroup locks the field groups and returns every stored group with the identified one, once the check passes.
func (s *Store) lockedGroup(
	ctx context.Context, queries Queries, id int, recheck content.Recheck,
) ([]content.Group, content.Group, error) {
	if err := queries.LockFieldGroups(ctx); err != nil {
		return nil, content.Group{}, err
	}
	if err := s.rechecked(ctx, queries, recheck); err != nil {
		return nil, content.Group{}, err
	}
	groups, err := s.groupsWithFields(ctx, queries)
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
func (s *Store) rechecked(ctx context.Context, queries Queries, recheck content.Recheck) error {
	if recheck == nil {
		return nil
	}
	groups, err := s.groupsWithFields(ctx, queries)
	if err != nil {
		return err
	}
	types, err := s.storedTypes(ctx, queries)
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
func (s *Store) DeleteFieldsOfGroup(
	ctx context.Context, groupID int, keys []string, recheck content.Recheck,
) error {
	err := s.backend.InTx(ctx, func(queries Queries) error {
		return s.deleteNamedFields(ctx, queries, groupID, keys, recheck)
	})
	if refusedFieldDelete(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("%s: delete fields of group: %w", s.backend.Engine, err)
	}
	return nil
}

// deleteNamedFields removes the group's named top level fields once the check passes, sweeping as a group delete does.
func (s *Store) deleteNamedFields(
	ctx context.Context, queries Queries, groupID int, keys []string, recheck content.Recheck,
) error {
	groups, held, err := s.lockedGroup(ctx, queries, groupID, recheck)
	if err != nil {
		return err
	}
	if !holdsEvery(held, keys) {
		return content.ErrFieldNotFound
	}
	held.Fields = slices.DeleteFunc(slices.Clone(held.Fields), func(f content.Field) bool {
		return !slices.Contains(keys, f.Key)
	})
	return s.deleteFieldsOf(ctx, queries, groups, held)
}

// holdsEvery reports whether the group declares a field under each key.
func holdsEvery(g content.Group, keys []string) bool {
	for _, key := range keys {
		if !holdsKey(g, key) {
			return false
		}
	}
	return true
}

// refusedFieldDelete reports whether the error refuses a field delete as the caller reads it.
func refusedFieldDelete(err error) bool {
	return errors.Is(err, content.ErrGroupNotFound) || errors.Is(err, content.ErrFieldNotFound) || fromCheck(err)
}

// deleteFieldsOf removes the group's fields and sweeps their values from the types no other group serves them on.
func (s *Store) deleteFieldsOf(ctx context.Context, queries Queries, groups []content.Group, held content.Group) error {
	types, err := s.storedTypes(ctx, queries)
	if err != nil {
		return err
	}
	for _, f := range held.Fields {
		swept := sweptByDelete(groups, types, held.ID, f.Key)
		if err := deleteFieldRow(ctx, queries, groups, held.ID, f.Key, swept); err != nil {
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
func (s *Store) storedTypes(ctx context.Context, queries Queries) ([]content.Type, error) {
	names, err := s.storedTypeKeys(ctx, queries)
	if err != nil {
		return nil, err
	}
	types := make([]content.Type, len(names))
	for i, key := range names {
		types[i] = content.Type{Key: key}
	}
	return types, nil
}

// keyFreeInGroup reports whether the key stays open on every type the group serves.
func (s *Store) keyFreeInGroup(ctx context.Context, queries Queries, groupID int, key string, leaving int) error {
	groups, err := s.groupsWithFields(ctx, queries)
	if err != nil {
		return err
	}
	target, found := groupByID(groups, groupID)
	if !found {
		return content.ErrGroupNotFound
	}
	types, err := s.storedTypes(ctx, queries)
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

// deleteFieldRow removes one field row and its values, revisions and index rows from the given types.
func deleteFieldRow(
	ctx context.Context, queries Queries, groups []content.Group, groupID int, key string, swept []string,
) error {
	if _, err := queries.DeleteContentField(ctx, groupID, key); err != nil {
		return err
	}
	if err := queries.ClearContentFieldValues(ctx, key, swept); err != nil {
		return err
	}
	if err := queries.ClearRevisionFieldValues(ctx, key, swept); err != nil {
		return err
	}
	return dropIndexAt(ctx, queries, groups, []string{key}, swept)
}

// ReorderGroups stores the given order on the groups.
func (s *Store) ReorderGroups(ctx context.Context, ids []int) error {
	if err := s.backend.Pool.ReorderFieldGroups(ctx, ids); err != nil {
		return fmt.Errorf("%s: reorder field groups: %w", s.backend.Engine, err)
	}
	return nil
}

// UpdateFieldInGroup stores the field's label, required flag and settings when the expectation and the check hold.
func (s *Store) UpdateFieldInGroup(
	ctx context.Context, groupID int, f content.Field, expectedUpdatedAt time.Time, recheck content.Recheck,
) (content.Field, error) {
	var updated content.Field
	err := s.backend.InTx(ctx, func(queries Queries) error {
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("%s: lock field groups: %w", s.backend.Engine, err)
		}
		if err := s.rechecked(ctx, queries, recheck); err != nil {
			return err
		}
		row, err := queries.UpdateContentField(ctx, FieldRow{
			Label:     f.Label,
			Required:  f.Required,
			Settings:  settingsJSON(f.Settings),
			UpdatedAt: f.UpdatedAt,
			GroupID:   groupID,
			Key:       f.Key,
		}, expectedUpdatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			if err := s.fieldStands(ctx, queries, groupID, f.Key); err != nil {
				return err
			}
			return content.ErrConflict
		}
		if err != nil {
			return fmt.Errorf("%s: update content field: %w", s.backend.Engine, err)
		}
		updated = toField(row)
		return nil
	})
	return updated, err
}

// UpdateSubField carries the edit onto the sub field the identity names.
func (s *Store) UpdateSubField(
	ctx context.Context, id int, f content.Field, expectedUpdatedAt time.Time,
) (content.Field, error) {
	row, err := s.backend.Pool.UpdateSubContentField(ctx, FieldRow{
		Label:     f.Label,
		Required:  f.Required,
		Settings:  settingsJSON(f.Settings),
		UpdatedAt: f.UpdatedAt,
		ID:        id,
	}, expectedUpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return content.Field{}, s.subFieldMissed(ctx, id)
	}
	if err != nil {
		return content.Field{}, fmt.Errorf("%s: update sub field: %w", s.backend.Engine, err)
	}
	return toField(row), nil
}

// subFieldMissed returns field not found when no field carries the id, and a conflict when one still stands.
func (s *Store) subFieldMissed(ctx context.Context, id int) error {
	if err := s.fieldCarried(ctx, s.backend.Pool, id); err != nil {
		return err
	}
	return content.ErrConflict
}

// ReorderSubFields stands the sub fields of the container in the order the keys name.
func (s *Store) ReorderSubFields(ctx context.Context, parentID int, keys []string) error {
	moved, err := s.backend.Pool.ReorderSubContentFields(ctx, parentID, keys)
	if err != nil {
		return fmt.Errorf("%s: reorder sub fields: %w", s.backend.Engine, err)
	}
	if moved > 0 {
		return nil
	}
	return s.fieldCarried(ctx, s.backend.Pool, parentID)
}

// fieldStands reports whether the group still declares the field.
func (s *Store) fieldStands(ctx context.Context, queries Queries, groupID int, key string) error {
	groups, err := s.groupsWithFields(ctx, queries)
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

// DeleteFieldInGroup removes the field, sweeping its values as a group delete does.
func (s *Store) DeleteFieldInGroup(ctx context.Context, groupID int, key string, recheck content.Recheck) error {
	err := s.backend.InTx(ctx, func(queries Queries) error {
		return s.deleteNamedFields(ctx, queries, groupID, []string{key}, recheck)
	})
	if refusedFieldDelete(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("%s: delete content field: %w", s.backend.Engine, err)
	}
	return nil
}

// DeleteSubField removes the sub field, and its values where no other active group serves its path.
func (s *Store) DeleteSubField(ctx context.Context, id int, recheck content.Recheck) error {
	err := s.backend.InTx(ctx, func(queries Queries) error {
		return s.deleteSubFieldRows(ctx, queries, id, recheck)
	})
	if errors.Is(err, content.ErrFieldNotFound) || fromCheck(err) {
		return err
	}
	if err != nil {
		return fmt.Errorf("%s: delete sub field: %w", s.backend.Engine, err)
	}
	return nil
}

// deleteSubFieldRows removes the sub field once the check passes, sweeping values no other active group serves.
func (s *Store) deleteSubFieldRows(ctx context.Context, queries Queries, id int, recheck content.Recheck) error {
	if err := queries.LockFieldGroups(ctx); err != nil {
		return err
	}
	if err := s.rechecked(ctx, queries, recheck); err != nil {
		return err
	}
	groups, err := s.groupsWithFields(ctx, queries)
	if err != nil {
		return err
	}
	group, dropped, path, found := fieldPathIn(groups, id)
	if !found {
		return content.ErrFieldNotFound
	}
	typeKeys, err := s.storedTypeKeys(ctx, queries)
	if err != nil {
		return err
	}
	if _, err := queries.DeleteFieldByID(ctx, id); err != nil {
		return err
	}
	return sweepField(ctx, queries, groups, dropped, path, servedOn(groups, typeKeys, group.ID, path))
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

// sweepField removes what the field held at the path from every item, revision and index row of the swept types.
func sweepField(
	ctx context.Context, queries Queries, groups []content.Group, f content.Field, path, swept []string,
) error {
	sweep := sweepPath
	if f.Kind == content.FieldKindLayout {
		sweep = sweepLayout
	}
	if err := sweep(ctx, queries, path, swept); err != nil {
		return err
	}
	return dropIndexAt(ctx, queries, groups, path, swept)
}

// dropIndexAt removes the index rows of every relation standing at the path in any group from the swept types.
func dropIndexAt(ctx context.Context, queries Queries, groups []content.Group, path, swept []string) error {
	var fields []int
	for _, g := range groups {
		if f, found := fieldAt(g.Fields, path); found {
			fields = relationsBelow(f, fields)
		}
	}
	if len(fields) == 0 {
		return nil
	}
	return queries.DeleteRelationsOfFields(ctx, swept, fields)
}

// fieldAt returns the field standing at the path among the fields, and whether one does.
func fieldAt(fields []content.Field, path []string) (content.Field, bool) {
	for _, f := range fields {
		if f.Key != path[0] {
			continue
		}
		if len(path) == 1 {
			return f, true
		}
		return fieldAt(f.Fields, path[1:])
	}
	return content.Field{}, false
}

// sweepPath removes whatever stands at the path from every item and revision of the swept types.
func sweepPath(ctx context.Context, queries Queries, path []string, swept []string) error {
	if err := queries.StripContentFieldPath(ctx, Strip{
		Path: path, Types: swept, Key: path[0],
	}); err != nil {
		return err
	}
	return queries.StripRevisionFieldPath(ctx, Strip{
		Path: path, Types: swept, Key: path[0],
	})
}

// sweepLayout removes the rows a layout held from every item and revision of the swept types.
func sweepLayout(ctx context.Context, queries Queries, path []string, swept []string) error {
	if err := queries.StripContentLayout(ctx, Strip{
		Path: path, Types: swept, Key: path[0],
	}); err != nil {
		return err
	}
	return queries.StripRevisionLayout(ctx, Strip{
		Path: path, Types: swept, Key: path[0],
	})
}

// ReorderFieldsInGroup stores the declaration order of a group's fields.
func (s *Store) ReorderFieldsInGroup(ctx context.Context, groupID int, keys []string) error {
	moved, err := s.backend.Pool.ReorderContentFields(ctx, groupID, keys)
	if err != nil {
		return fmt.Errorf("%s: reorder content fields: %w", s.backend.Engine, err)
	}
	if moved > 0 {
		return nil
	}
	return s.groupStands(ctx, s.backend.Pool, groupID)
}

// groupStands returns group not found when no stored group carries the identifier.
func (s *Store) groupStands(ctx context.Context, queries Queries, id int) error {
	groups, err := s.groupsWithFields(ctx, queries)
	if err != nil {
		return err
	}
	if _, found := groupByID(groups, id); !found {
		return content.ErrGroupNotFound
	}
	return nil
}

// fieldCarried returns field not found when no stored field carries the identifier.
func (s *Store) fieldCarried(ctx context.Context, queries Queries, id int) error {
	groups, err := s.groupsWithFields(ctx, queries)
	if err != nil {
		return err
	}
	if _, _, _, found := fieldPathIn(groups, id); !found {
		return content.ErrFieldNotFound
	}
	return nil
}

// MoveField carries the field to a group's top or into a container within the limit, sweeping what it leaves.
func (s *Store) MoveField(
	ctx context.Context, id, toGroup, toParent, limit int, recheck content.Recheck,
) (content.Field, error) {
	var moved content.Field
	err := s.backend.InTx(ctx, func(queries Queries) error {
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("%s: lock field groups: %w", s.backend.Engine, err)
		}
		if err := s.rechecked(ctx, queries, recheck); err != nil {
			return err
		}
		move, err := s.movePlanned(ctx, queries, id, toGroup, toParent, limit)
		if err != nil {
			return err
		}
		if move.stands() {
			moved = move.leaving
			return nil
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
	return content.Field{}, s.moveFailure(err)
}

// moveFailure returns the error a field move carries, and wraps anything else.
func (s *Store) moveFailure(err error) error {
	if fromCheck(err) {
		return err
	}
	for _, carried := range []error{
		content.ErrFieldNotFound, content.ErrGroupNotFound, content.ErrFieldTaken,
		content.ErrFieldInsideItself, content.ErrFieldTooDeep,
	} {
		if errors.Is(err, carried) {
			return err
		}
	}
	return s.fieldWriteFailure(err)
}

// fieldMove is one field leaving its place for another, as the store carries it.
type fieldMove struct {
	store                 *Store
	id, toGroup, toParent int
	leaving               content.Field
	source                content.Group
	groups                []content.Group
	path                  []string
	depth                 int
}

// movePlanned resolves where the field stands and where it is asked to stand, refusing a landing past the limit.
func (s *Store) movePlanned(ctx context.Context, queries Queries, id, toGroup, toParent, limit int) (fieldMove, error) {
	groups, err := s.groupsWithFields(ctx, queries)
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
		store: s, id: id, toGroup: toGroup, toParent: toParent,
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

// stands reports whether the field already stands where the move asks it to.
func (m fieldMove) stands() bool {
	return m.toGroup == m.source.ID && m.toParent == m.leaving.ParentID
}

// keyFree re-checks the key against the rival groups when the field lands at a group's top.
func (m fieldMove) keyFree(ctx context.Context, queries Queries) error {
	if m.toParent != 0 {
		return nil
	}
	leaving := 0
	if m.leaving.ParentID == 0 {
		leaving = m.source.ID
	}
	return m.store.keyFreeInGroup(ctx, queries, m.toGroup, m.leaving.Key, leaving)
}

// write reparents the field, recounts the depth below it and sweeps what its old path held where nothing serves it.
func (m fieldMove) write(ctx context.Context, queries Queries) (content.Field, error) {
	row, err := queries.ReparentContentField(ctx, m.id, m.toGroup, m.toParent, time.Now().UTC())
	if err != nil {
		return content.Field{}, err
	}
	if err := queries.RecountContentFieldDepth(ctx, m.id, m.toGroup, m.depth); err != nil {
		return content.Field{}, err
	}
	typeKeys, err := m.store.storedTypeKeys(ctx, queries)
	if err != nil {
		return content.Field{}, err
	}
	if m.leaving.ParentID == m.toParent {
		typeKeys = m.behind(typeKeys)
	}
	return toField(row), m.sweep(ctx, queries, typeKeys)
}

// behind returns the types the landing group does not reach.
func (m fieldMove) behind(typeKeys []string) []string {
	landing, _ := groupByID(m.groups, m.toGroup)
	return slices.DeleteFunc(typeKeys, func(key string) bool {
		return landing.Location.Match(screenOf(key), locationParams)
	})
}

// sweep strips the old path where no other active group serves it and drops the field's relation rows on every type.
func (m fieldMove) sweep(ctx context.Context, queries Queries, typeKeys []string) error {
	if len(typeKeys) == 0 {
		return nil
	}
	swept := servedOn(m.groups, typeKeys, m.source.ID, m.path)
	if err := sweepField(ctx, queries, m.groups, m.leaving, m.path, swept); err != nil {
		return err
	}
	fields := relationsBelow(m.leaving, nil)
	if len(fields) == 0 {
		return nil
	}
	return queries.DeleteRelationsOfFields(ctx, typeKeys, fields)
}

// servedOn returns the type keys on which no other active group serves the whole path.
func servedOn(groups []content.Group, typeKeys []string, groupID int, path []string) []string {
	held := make([]string, 0, len(typeKeys))
	for _, typeKey := range typeKeys {
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

// relationsBelow appends the identities of the relation fields the field is or holds, however deep.
func relationsBelow(f content.Field, held []int) []int {
	if f.Kind == content.FieldKindRelation {
		held = append(held, f.ID)
	}
	for _, inside := range f.Fields {
		held = relationsBelow(inside, held)
	}
	return held
}

// settledFieldWrite runs the write once the check passes and the key is held free of every rival group sharing a type.
func (s *Store) settledFieldWrite(
	ctx context.Context, groupID int, key string, recheck content.Recheck, write func(Queries) error,
) error {
	err := s.backend.InTx(ctx, func(queries Queries) error {
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("%s: lock field groups: %w", s.backend.Engine, err)
		}
		if err := s.rechecked(ctx, queries, recheck); err != nil {
			return err
		}
		if err := s.keyFreeInGroup(ctx, queries, groupID, key, 0); err != nil {
			return err
		}
		return write(queries)
	})
	if err == nil {
		return nil
	}
	if errors.Is(err, content.ErrGroupNotFound) || errors.Is(err, content.ErrFieldTaken) || fromCheck(err) {
		return err
	}
	return s.fieldWriteFailure(err)
}

// CreateSubField declares the field inside the container the parent names, within the limit.
func (s *Store) CreateSubField(
	ctx context.Context, parentID int, f content.Field, limit int,
) (content.Field, error) {
	var created content.Field
	err := s.backend.InTx(ctx, func(queries Queries) error {
		if err := queries.LockFieldGroups(ctx); err != nil {
			return fmt.Errorf("%s: lock field groups: %w", s.backend.Engine, err)
		}
		parent, settled, err := standingParent(ctx, queries, parentID, f, limit)
		if err != nil {
			return err
		}
		row, err := queries.CreateSubContentField(ctx, FieldRow{
			GroupID:   parent.GroupID,
			Origin:    settled.Origin,
			ParentID:  parentID,
			Key:       settled.Key,
			Label:     settled.Label,
			Kind:      string(settled.Kind),
			RelatesTo: settled.RelatesTo,
			Many:      settled.Many,
			Required:  settled.Required,
			CreatedAt: settled.CreatedAt,
			UpdatedAt: settled.UpdatedAt,
			Settings:  settingsJSON(settled.Settings),
			Depth:     parent.Depth + 1,
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
	return content.Field{}, s.fieldWriteFailure(err)
}

// standingParent returns the parent row and the field it settled on, or the reason the sub field may not stand there.
func standingParent(
	ctx context.Context, queries Queries, parentID int, f content.Field, limit int,
) (FieldRow, content.Field, error) {
	parent, err := queries.FieldByID(ctx, parentID)
	if errors.Is(err, sql.ErrNoRows) {
		return parent, f, content.ErrFieldNotFound
	}
	if err != nil {
		return parent, f, err
	}
	settled, err := content.NewSubField(f, content.FieldKind(parent.Kind))
	if err != nil {
		return parent, f, err
	}
	if err := content.WithinDepth(settled, parent.Depth+1, limit); err != nil {
		return parent, f, err
	}
	return parent, settled, nil
}

// CreateFieldInGroup declares the field inside the group.
func (s *Store) CreateFieldInGroup(
	ctx context.Context, groupID int, f content.Field, recheck content.Recheck,
) (content.Field, error) {
	var created content.Field
	err := s.settledFieldWrite(ctx, groupID, f.Key, recheck, func(queries Queries) error {
		row, err := queries.CreateContentField(ctx, FieldRow{
			GroupID:   groupID,
			Origin:    f.Origin,
			Key:       f.Key,
			Label:     f.Label,
			Kind:      string(f.Kind),
			RelatesTo: f.RelatesTo,
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
