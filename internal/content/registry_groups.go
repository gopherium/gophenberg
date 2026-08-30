// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Groups returns every field group in position order with its fields attached.
func (r *Registry) Groups(ctx context.Context) ([]Group, error) {
	return r.store.ListGroups(ctx)
}

// CreateGroup stores a new field group, or reports why the registry refuses it.
func (r *Registry) CreateGroup(ctx context.Context, g Group) (Group, error) {
	settled, err := r.settledGroup(ctx, g)
	if err != nil {
		return Group{}, err
	}
	created, err := r.store.CreateGroup(ctx, settled)
	if err != nil {
		return Group{}, err
	}
	r.invalidate()
	return created, nil
}

// UpdateGroup stores the group's title, location and active flag, or reports why it stands as it is.
func (r *Registry) UpdateGroup(ctx context.Context, g Group) (Group, error) {
	settled, err := r.settledGroup(ctx, g)
	if err != nil {
		return Group{}, err
	}
	if err := r.freeOfCollisions(ctx, settled); err != nil {
		return Group{}, err
	}
	updated, err := r.store.UpdateGroup(ctx, settled)
	if err != nil {
		return Group{}, err
	}
	r.invalidate()
	return updated, nil
}

// settledGroup returns the group ready to store, or the reason it is not one.
func (r *Registry) settledGroup(ctx context.Context, g Group) (Group, error) {
	g.Title = strings.TrimSpace(g.Title)
	if g.Title == "" {
		return Group{}, ErrInvalidGroupTitle
	}
	g.Location = g.Location.Normalize()
	if err := g.Location.Validate(r.Params(ctx)); err != nil {
		return Group{}, err
	}
	return g, nil
}

// DeleteGroup removes the group with its fields and their values, or reports it missing.
func (r *Registry) DeleteGroup(ctx context.Context, id int) error {
	if err := r.store.DeleteGroup(ctx, id); err != nil {
		return err
	}
	r.invalidate()
	return nil
}

// ReorderGroups stores the given order, or reports why the order does not stand.
func (r *Registry) ReorderGroups(ctx context.Context, ids []int) ([]Group, error) {
	held, err := r.Groups(ctx)
	if err != nil {
		return nil, err
	}
	if err := groupOrderCovers(held, ids); err != nil {
		return nil, err
	}
	if err := r.store.ReorderGroups(ctx, ids); err != nil {
		return nil, err
	}
	r.invalidate()
	return r.Groups(ctx)
}

// groupOrderCovers reports whether ids name every stored group exactly once.
func groupOrderCovers(held []Group, ids []int) error {
	if len(ids) != len(held) {
		return Refuse(ErrGroupOrder, "group_order_incomplete",
			"content: the order leaves stored groups out", nil)
	}
	seen := make(map[int]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			return Refuse(ErrGroupOrder, "group_order_incomplete",
				"content: the order names a group twice", Details{"group": id})
		}
		if _, found := groupOf(held, id); !found {
			return ErrGroupNotFound
		}
		seen[id] = true
	}
	return nil
}

// groupOf returns the group carrying the identifier, and whether one does.
func groupOf(held []Group, id int) (Group, bool) {
	for _, g := range held {
		if g.ID == id {
			return g, true
		}
	}
	return Group{}, false
}

// CreateFieldInGroup declares the field inside the group, or reports why the registry refuses it.
func (r *Registry) CreateFieldInGroup(ctx context.Context, groupID int, f Field) (Field, error) {
	if err := f.Validate(); err != nil {
		return Field{}, err
	}
	if f.Kind == FieldKindRelation {
		if _, err := r.ByKey(ctx, f.RelatesTo); err != nil {
			return Field{}, ErrTargetUnknown
		}
	}
	if err := r.keyFree(ctx, groupID, f.Key, 0); err != nil {
		return Field{}, err
	}
	created, err := r.store.CreateFieldInGroup(ctx, groupID, f)
	if err != nil {
		return Field{}, err
	}
	r.invalidate()
	return created, nil
}

// UpdateFieldInGroup carries the field's label and required flag when the expectation still holds.
func (r *Registry) UpdateFieldInGroup(
	ctx context.Context, groupID int, f Field, expectedUpdatedAt time.Time,
) (Field, error) {
	held, err := r.heldField(ctx, groupID, f.Key)
	if err != nil {
		return Field{}, err
	}
	held.Label, held.Required, held.Settings = f.Label, f.Required, f.Settings
	held.UpdatedAt = time.Now().UTC()
	if err := held.Validate(); err != nil {
		return Field{}, err
	}
	updated, err := r.store.UpdateFieldInGroup(ctx, groupID, held, expectedUpdatedAt)
	if err != nil {
		return Field{}, err
	}
	r.invalidate()
	return updated, nil
}

// DeleteFieldInGroup removes the field and its values from the types its group matches.
func (r *Registry) DeleteFieldInGroup(ctx context.Context, groupID int, key string) error {
	if _, err := r.heldField(ctx, groupID, key); err != nil {
		return err
	}
	if err := r.store.DeleteFieldInGroup(ctx, groupID, key); err != nil {
		return err
	}
	r.invalidate()
	return nil
}

// ReorderFieldsInGroup stores the declaration order of a group's fields.
func (r *Registry) ReorderFieldsInGroup(ctx context.Context, groupID int, keys []string) ([]Field, error) {
	held, err := r.heldGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if err := orderCovers(held.Fields, keys); err != nil {
		return nil, err
	}
	if err := r.store.ReorderFieldsInGroup(ctx, groupID, keys); err != nil {
		return nil, err
	}
	r.invalidate()
	reordered, err := r.heldGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return reordered.Fields, nil
}

// heldGroup returns the stored group carrying the identifier.
func (r *Registry) heldGroup(ctx context.Context, groupID int) (Group, error) {
	held, err := r.Groups(ctx)
	if err != nil {
		return Group{}, err
	}
	target, found := groupOf(held, groupID)
	if !found {
		return Group{}, ErrGroupNotFound
	}
	return target, nil
}

// heldField returns the stored field a group declares under the key.
func (r *Registry) heldField(ctx context.Context, groupID int, key string) (Field, error) {
	target, err := r.heldGroup(ctx, groupID)
	if err != nil {
		return Field{}, err
	}
	return fieldAmong(target.Fields, key)
}

// MoveField carries the field into another group, keeping the values it holds.
func (r *Registry) MoveField(ctx context.Context, groupID int, key string, toGroup int) (Field, error) {
	if err := r.keyFree(ctx, toGroup, key, groupID); err != nil {
		return Field{}, err
	}
	moved, err := r.store.MoveField(ctx, groupID, key, toGroup)
	if err != nil {
		return Field{}, err
	}
	r.invalidate()
	return moved, nil
}

// keyFree reports whether the key is open on every type the group serves, ignoring the group it leaves.
func (r *Registry) keyFree(ctx context.Context, groupID int, key string, leaving int) error {
	held, err := r.Groups(ctx)
	if err != nil {
		return err
	}
	target, found := groupOf(held, groupID)
	if !found {
		return ErrGroupNotFound
	}
	return r.uncollided(ctx, held, target, []string{key}, leaving)
}

// freeOfCollisions reports whether the group's own keys stay open once it holds the given location.
func (r *Registry) freeOfCollisions(ctx context.Context, asked Group) error {
	held, err := r.Groups(ctx)
	if err != nil {
		return err
	}
	stored, found := groupOf(held, asked.ID)
	if !found {
		return nil
	}
	keys := make([]string, 0, len(stored.Fields))
	for _, f := range stored.Fields {
		keys = append(keys, f.Key)
	}
	asked.Fields = stored.Fields
	return r.uncollided(ctx, held, asked, keys, 0)
}

// uncollided reports whether the keys stay free of every other group sharing a type with this one.
func (r *Registry) uncollided(
	ctx context.Context, held []Group, target Group, keys []string, leaving int,
) error {
	types, err := r.All(ctx)
	if err != nil {
		return err
	}
	return Uncollided(types, held, target, keys, leaving, r.Params(ctx))
}

// Uncollided reports whether the keys stay free of every other group sharing a type with this one.
func Uncollided(
	types []Type, held []Group, target Group, keys []string, leaving int, params *ParamRegistry,
) error {
	if !target.Active || len(keys) == 0 {
		return nil
	}
	wanted := make(map[string]bool, len(keys))
	for _, key := range keys {
		wanted[key] = true
	}
	for _, rival := range held {
		if !rivalOf(target, rival, leaving) || !sharesAType(types, target, rival, params) {
			continue
		}
		if err := rivalFree(rival, wanted); err != nil {
			return err
		}
	}
	return nil
}

// rivalOf reports whether the other group could collide with the target at all.
func rivalOf(target, other Group, leaving int) bool {
	return other.ID != target.ID && other.ID != leaving && other.Active
}

// rivalFree reports whether the rival group holds none of the wanted keys.
func rivalFree(rival Group, wanted map[string]bool) error {
	for _, f := range rival.Fields {
		if wanted[f.Key] {
			return Refuse(ErrFieldTaken, "field_taken",
				fmt.Sprintf("%s: %s in %s", ErrFieldTaken, f.Key, rival.Title),
				Details{"field": f.Key, "group": rival.Title})
		}
	}
	return nil
}

// sharesAType reports whether both groups serve at least one registered type.
func sharesAType(types []Type, one, other Group, params *ParamRegistry) bool {
	for _, t := range types {
		screen := Screen{ScreenContentType: t.Key}
		if one.Location.Match(screen, params) && other.Location.Match(screen, params) {
			return true
		}
	}
	return false
}
