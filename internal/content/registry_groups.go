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
	held, stored, err := r.groupAmong(ctx, settled.ID)
	if err != nil {
		return Group{}, err
	}
	if err := pluginKeepsGroup(ctx, stored, settled); err != nil {
		return Group{}, err
	}
	settled.Origin = stored.Origin
	if err := r.sourcesStand(ctx, held, settled, stored.Fields); err != nil {
		return Group{}, err
	}
	if err := r.freeOfCollisions(ctx, held, stored, settled); err != nil {
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
	g.Key = strings.TrimSpace(g.Key)
	if err := ValidGroupKey(g.Key); err != nil {
		return Group{}, err
	}
	g.Location = g.Location.Normalize()
	if err := g.Location.Validate(r.Params(ctx)); err != nil {
		return Group{}, err
	}
	return g, nil
}

// DeleteGroup removes the group with its fields and their values, or reports it missing.
func (r *Registry) DeleteGroup(ctx context.Context, id int) error {
	return r.DeleteGroupSettled(ctx, id, nil)
}

// DeleteGroupSettled removes the group with its fields and their values, overlooking the readers the caller settles.
func (r *Registry) DeleteGroupSettled(ctx context.Context, id int, settled Settled) error {
	groups, stored, err := r.groupAmong(ctx, id)
	if err != nil {
		return err
	}
	if err := keptFrom(ctx, stored.Origin); err != nil {
		return err
	}
	if err := GroupKept(settled.across(groups), stored); err != nil {
		return err
	}
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
	if err := f.standsAlone(); err != nil {
		return Field{}, err
	}
	held, target, err := r.groupAmong(ctx, groupID)
	if err != nil {
		return Field{}, err
	}
	if err := keptFrom(ctx, target.Origin); err != nil {
		return Field{}, err
	}
	if err := r.pointsSomewhere(ctx, held, target, f); err != nil {
		return Field{}, err
	}
	if err := r.uncollided(ctx, held, target, []string{f.Key}, 0); err != nil {
		return Field{}, err
	}
	if err := Stands(target.Fields, f); err != nil {
		return Field{}, err
	}
	created, err := r.store.CreateFieldInGroup(ctx, groupID, f)
	if err != nil {
		return Field{}, err
	}
	r.invalidate()
	return created, nil
}

// CreateSubField declares the field inside the container the parent names.
func (r *Registry) CreateSubField(ctx context.Context, parentID int, f Field) (Field, error) {
	parent, _, depth, err := r.fieldByID(ctx, parentID)
	if err != nil {
		return Field{}, err
	}
	if err := pluginKeepsField(ctx, parent); err != nil {
		return Field{}, err
	}
	if _, err := fieldAmong(parent.Fields, f.Key); err == nil {
		return Field{}, ErrFieldTaken
	}
	if err := Stands(parent.Fields, f); err != nil {
		return Field{}, err
	}
	if depth+1 > r.FieldDepth() {
		return Field{}, ErrFieldTooDeep
	}
	created, err := r.store.CreateSubField(ctx, parentID, f)
	if err != nil {
		return Field{}, err
	}
	r.invalidate()
	return created, nil
}

// UpdateSubField carries a label, a required flag and settings onto the field standing inside a container.
func (r *Registry) UpdateSubField(
	ctx context.Context, id int, f Field, expectedUpdatedAt time.Time,
) (Field, error) {
	held, beside, _, err := r.fieldByID(ctx, id)
	if err != nil {
		return Field{}, err
	}
	if err := pluginKeepsField(ctx, held); err != nil {
		return Field{}, err
	}
	held.Label, held.Required, held.Settings = f.Label, f.Required, f.Settings
	held.UpdatedAt = time.Now().UTC()
	if err := held.Validate(); err != nil {
		return Field{}, err
	}
	if err := Stands(beside, held); err != nil {
		return Field{}, err
	}
	updated, err := r.store.UpdateSubField(ctx, id, held, expectedUpdatedAt)
	if err != nil {
		return Field{}, err
	}
	r.invalidate()
	return updated, nil
}

// ReorderSubFields stands the fields inside the container in the order the keys name.
func (r *Registry) ReorderSubFields(ctx context.Context, parentID int, keys []string) ([]Field, error) {
	held, _, _, err := r.fieldByID(ctx, parentID)
	if err != nil {
		return nil, err
	}
	if !held.Kind.Holds() {
		return nil, Refuse(ErrFieldShape, "field_parent_holds_none",
			"content: the field holds no sub fields", Details{"field": held.Key})
	}
	if err := orderCovers(held.Fields, keys); err != nil {
		return nil, err
	}
	if err := r.store.ReorderSubFields(ctx, parentID, keys); err != nil {
		return nil, err
	}
	r.invalidate()
	reordered, _, _, err := r.fieldByID(ctx, parentID)
	if err != nil {
		return nil, err
	}
	return reordered.Fields, nil
}

// fieldByID returns the declared field carrying the identity, the level it stands on and how many containers hold it.
func (r *Registry) fieldByID(ctx context.Context, id int) (Field, []Field, int, error) {
	groups, err := r.Groups(ctx)
	if err != nil {
		return Field{}, nil, 0, err
	}
	for _, g := range groups {
		if held, beside, depth, found := fieldNumbered(g.Fields, id, 0); found {
			return held, beside, depth, nil
		}
	}
	return Field{}, nil, 0, ErrFieldNotFound
}

// fieldNumbered returns the field carrying the identity, the level it stands on and how many containers hold it.
func fieldNumbered(fields []Field, id, depth int) (Field, []Field, int, bool) {
	for _, f := range fields {
		if f.ID == id {
			return f, fields, depth, true
		}
		if held, beside, below, found := fieldNumbered(f.Fields, id, depth+1); found {
			return held, beside, below, true
		}
	}
	return Field{}, nil, 0, false
}

// DeleteSubField removes the field standing inside a container, and the values every item held under it.
func (r *Registry) DeleteSubField(ctx context.Context, id int) error {
	return r.DeleteSubFieldSettled(ctx, id, nil)
}

// DeleteSubFieldSettled removes the field standing inside a container, overlooking the readers the caller settles.
func (r *Registry) DeleteSubFieldSettled(ctx context.Context, id int, settled Settled) error {
	held, beside, _, err := r.fieldByID(ctx, id)
	if err != nil {
		return err
	}
	if err := pluginKeepsField(ctx, held); err != nil {
		return err
	}
	if err := Unreferenced(settled.among(beside), held.Key); err != nil {
		return err
	}
	if err := r.store.DeleteSubField(ctx, id); err != nil {
		return err
	}
	r.invalidate()
	return nil
}

// UpdateFieldInGroup carries the field's label, required flag and settings when the expectation still holds.
func (r *Registry) UpdateFieldInGroup(
	ctx context.Context, groupID int, f Field, expectedUpdatedAt time.Time,
) (Field, error) {
	groups, target, err := r.groupAmong(ctx, groupID)
	if err != nil {
		return Field{}, err
	}
	held, err := fieldAmong(target.Fields, f.Key)
	if err != nil {
		return Field{}, err
	}
	if err := pluginKeepsField(ctx, held); err != nil {
		return Field{}, err
	}
	held.Label, held.Required, held.Settings = f.Label, f.Required, f.Settings
	held.UpdatedAt = time.Now().UTC()
	if err := held.Validate(); err != nil {
		return Field{}, err
	}
	if err := r.sourceStands(ctx, groups, target, held); err != nil {
		return Field{}, err
	}
	if err := Stands(target.Fields, held); err != nil {
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
	return r.DeleteFieldInGroupSettled(ctx, groupID, key, nil)
}

// DeleteFieldInGroupSettled removes the field and its values, overlooking the readers the caller settles.
func (r *Registry) DeleteFieldInGroupSettled(ctx context.Context, groupID int, key string, settled Settled) error {
	groups, target, err := r.groupAmong(ctx, groupID)
	if err != nil {
		return err
	}
	held, err := fieldAmong(target.Fields, key)
	if err != nil {
		return err
	}
	if err := pluginKeepsField(ctx, held); err != nil {
		return err
	}
	target.Fields = settled.among(target.Fields)
	if err := freeOfReaders(settled.across(groups), target, key); err != nil {
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
	_, target, err := r.groupAmong(ctx, groupID)
	return target, err
}

// MoveField carries the field to the top of the group, or inside the container the parent names, leaving its values.
func (r *Registry) MoveField(ctx context.Context, id, toGroup, toParent int) (Field, error) {
	move, err := r.moveOf(ctx, id, toGroup, toParent)
	if err != nil {
		return Field{}, err
	}
	if move.settled() {
		return move.from.field, nil
	}
	if err := r.moveAllowed(ctx, move); err != nil {
		return Field{}, err
	}
	moved, err := r.store.MoveField(ctx, id, toGroup, toParent)
	if err != nil {
		return Field{}, err
	}
	r.invalidate()
	return moved, nil
}

// fieldMove is one field leaving its place for another, with everything the refusals read.
type fieldMove struct {
	held    []Group
	source  Group
	from    placement
	landing Group
	parent  Field
	depth   int
}

// moveOf resolves where the field stands and where it is asked to stand.
func (r *Registry) moveOf(ctx context.Context, id, toGroup, toParent int) (fieldMove, error) {
	held, err := r.Groups(ctx)
	if err != nil {
		return fieldMove{}, err
	}
	source, from, found := placedInGroups(held, id)
	if !found {
		return fieldMove{}, ErrFieldNotFound
	}
	landing, found := groupOf(held, toGroup)
	if !found {
		return fieldMove{}, ErrGroupNotFound
	}
	move := fieldMove{held: held, source: source, from: from, landing: landing}
	if toParent == 0 {
		return move, nil
	}
	parent, _, depth, found := fieldNumbered(landing.Fields, toParent, 0)
	if !found {
		return fieldMove{}, ErrFieldNotFound
	}
	move.parent, move.depth = parent, depth+1
	return move, nil
}

// settled reports whether the field already stands where it is asked to.
func (m fieldMove) settled() bool {
	return m.landing.ID == m.source.ID && m.parent.ID == m.from.parentID
}

// siblings returns the fields the moved one would stand beside.
func (m fieldMove) siblings() []Field {
	if m.parent.ID != 0 {
		return m.parent.Fields
	}
	return m.landing.Fields
}

// moveAllowed returns the first reason the field may not leave its place for the other, or nothing.
func (r *Registry) moveAllowed(ctx context.Context, m fieldMove) error {
	if err := m.owned(ctx); err != nil {
		return err
	}
	if err := m.placeable(r.FieldDepth()); err != nil {
		return err
	}
	if err := r.keyFree(ctx, m); err != nil {
		return err
	}
	if err := m.unread(); err != nil {
		return err
	}
	if err := r.sourceStands(ctx, m.held, m.landing, m.from.field); err != nil {
		return err
	}
	return Stands(m.siblings(), m.from.field)
}

// owned returns the reason a plugin keeps the field or its destination, or nothing when the site may move it.
func (m fieldMove) owned(ctx context.Context) error {
	if err := pluginKeepsField(ctx, m.from.field); err != nil {
		return err
	}
	if err := keptFrom(ctx, m.landing.Origin); err != nil {
		return err
	}
	return pluginKeepsField(ctx, m.parent)
}

// placeable returns the reason the field cannot stand at the destination, or nothing when it can.
func (m fieldMove) placeable(limit int) error {
	if err := m.outsideItself(); err != nil {
		return err
	}
	if err := m.standing(); err != nil {
		return err
	}
	if m.depth+height(m.from.field) > limit {
		return ErrFieldTooDeep
	}
	return nil
}

// outsideItself returns the refusal to stand the field inside its own tree, or nothing when it lands elsewhere.
func (m fieldMove) outsideItself() error {
	if m.parent.ID == 0 {
		return nil
	}
	_, inside := placedAmong(m.from.field.Fields, m.parent.ID, m.from.field.ID)
	if m.parent.ID != m.from.field.ID && !inside {
		return nil
	}
	return Refuse(ErrFieldInsideItself, "field_moves_inside_itself",
		fmt.Sprintf("%s: %s", ErrFieldInsideItself, m.from.field.Key), Details{"field": m.from.field.Key})
}

// standing returns the reason the field's kind cannot stand at the destination, or nothing when it can.
func (m fieldMove) standing() error {
	if m.parent.ID == 0 {
		return m.from.field.standsAlone()
	}
	return m.from.field.standsInside(m.parent.Kind)
}

// keyFree returns the reason the destination already answers to the field's key, or nothing when it is free.
func (r *Registry) keyFree(ctx context.Context, m fieldMove) error {
	if _, err := fieldAmong(without(m.siblings(), m.from.field.ID), m.from.field.Key); err == nil {
		return ErrFieldTaken
	}
	if m.parent.ID != 0 {
		return nil
	}
	leaving := 0
	if m.from.parentID == 0 {
		leaving = m.source.ID
	}
	return r.uncollided(ctx, m.held, m.landing, []string{m.from.field.Key}, leaving)
}

// unread returns the reason a sibling left behind or a backlinks still reads the field, or nothing.
func (m fieldMove) unread() error {
	if err := Unreferenced(m.from.beside, m.from.field.Key); err != nil {
		return err
	}
	return SourceKeptAlong(m.held, m.source.Key, m.from.path)
}

// pointsSomewhere reports whether a field naming another type or field names one the registry holds.
func (r *Registry) pointsSomewhere(ctx context.Context, held []Group, target Group, f Field) error {
	if f.Kind == FieldKindRelation {
		if _, err := r.ByKey(ctx, f.RelatesTo); err != nil {
			return ErrTargetUnknown
		}
	}
	return r.sourceStands(ctx, held, target, f)
}

// sourcesStand reports whether every backlinks field among them still reads its source from the group.
func (r *Registry) sourcesStand(ctx context.Context, held []Group, target Group, fields []Field) error {
	for _, f := range fields {
		if err := r.sourceStands(ctx, held, target, f); err != nil {
			return err
		}
	}
	return nil
}

// sourceStands reports whether a backlinks field names a relation this registry can read.
func (r *Registry) sourceStands(ctx context.Context, held []Group, target Group, f Field) error {
	if f.Kind != FieldKindBacklinks {
		return nil
	}
	types, err := r.All(ctx)
	if err != nil {
		return err
	}
	_, err = BacklinksSource(held, types, target, f, r.Params(ctx))
	return err
}

// groupAmong returns every stored group and the one carrying the identifier.
func (r *Registry) groupAmong(ctx context.Context, groupID int) ([]Group, Group, error) {
	held, err := r.Groups(ctx)
	if err != nil {
		return nil, Group{}, err
	}
	target, found := groupOf(held, groupID)
	if !found {
		return nil, Group{}, ErrGroupNotFound
	}
	return held, target, nil
}

// freeOfCollisions reports whether the group's own keys stay open once it holds the given location.
func (r *Registry) freeOfCollisions(ctx context.Context, held []Group, stored, asked Group) error {
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
