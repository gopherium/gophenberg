// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"
	"slices"
	"strings"

	"github.com/gopherium/gophenberg/internal/content"
)

// leaving takes away every group the admin agreed to lose, keeping one handing fields on until they have moved.
func (r *run) leaving(ctx context.Context) error {
	for _, c := range r.plan.Changes {
		if c.Subject != SubjectGroup || c.Action != ActionDelete {
			continue
		}
		if !r.allows(c) {
			r.left(c)
			continue
		}
		if r.handsOn(c.Key) {
			if err := r.emptied(ctx, c); err != nil {
				return err
			}
			continue
		}
		if err := r.registry.DeleteGroupSettled(ctx, r.groupKeyed(c.Key).ID, r.settled); err != nil {
			return err
		}
		r.did(c)
	}
	return nil
}

// handsOn reports whether the plan moves a field out of the group with its values.
func (r *run) handsOn(group string) bool {
	for _, c := range r.plan.Changes {
		if c.Reason == ReasonCarried && c.From == group {
			return true
		}
	}
	return false
}

// emptied takes away the fields the group keeps to itself, holding the group until the others have moved.
func (r *run) emptied(ctx context.Context, c Change) error {
	g := r.groupKeyed(c.Key)
	dropped := make([]string, 0, len(g.Fields))
	for _, f := range g.Fields {
		if _, carried := r.landingOf(g.Key, f.Key); !carried {
			dropped = append(dropped, f.Key)
		}
	}
	if err := r.registry.DeleteFieldsOfGroupSettled(ctx, g.ID, dropped, r.settled); err != nil {
		return err
	}
	r.handing = append(r.handing, c)
	return nil
}

// landingOf returns the create another group gains for the field the group hands on, reporting false for none.
func (r *run) landingOf(group, key string) (Change, bool) {
	for _, c := range r.plan.Changes {
		if c.Action == ActionCreate && c.Reason == ReasonCarried && c.From == group && c.Key == key {
			return c, true
		}
	}
	return Change{}, false
}

// vacated takes away every field the file moved elsewhere, carrying the ones that keep their values.
func (r *run) vacated(ctx context.Context) error {
	for _, c := range r.plan.Changes {
		if c.Subject != SubjectField || c.Action != ActionDelete || !movedAway(c) {
			continue
		}
		if !r.allows(c) {
			r.left(c)
			continue
		}
		if err := r.vacate(ctx, c); err != nil {
			return err
		}
		r.did(c)
	}
	return r.handedOver(ctx)
}

// vacate moves the field the change names with its values when they are carried, taking it away otherwise.
func (r *run) vacate(ctx context.Context, c Change) error {
	if c.Reason != ReasonCarried {
		return r.removeField(ctx, c)
	}
	return r.carry(ctx, c.Group, c.Key)
}

// carry moves the field to the group the plan lands it in, answering for its readers and its rules itself.
func (r *run) carry(ctx context.Context, group, key string) error {
	landing, _ := r.landingOf(group, key)
	held := r.fieldAt(group, key)
	answering := content.Settled{held.ID: true}
	for id := range r.settled {
		answering[id] = true
	}
	_, err := r.registry.MoveFieldSettled(ctx, held.ID, r.groupKeyed(landing.Group).ID, 0, answering)
	return err
}

// handedOver moves the fields every held group hands on, then takes the group away.
func (r *run) handedOver(ctx context.Context) error {
	for _, c := range r.handing {
		for _, f := range r.groupKeyed(c.Key).Fields {
			if err := r.carry(ctx, c.Key, f.Key); err != nil {
				return err
			}
		}
		if err := r.registry.DeleteGroupSettled(ctx, r.groupKeyed(c.Key).ID, r.settled); err != nil {
			return err
		}
		r.did(c)
	}
	return nil
}

// fields stores the fields the envelope brings and carries what it changed onto the stored ones, backlinks last.
func (r *run) fields(ctx context.Context) error {
	for _, late := range []bool{false, true} {
		for _, declared := range r.envelope.Groups {
			stored := r.groupKeyed(declared.Key)
			if err := r.fieldsUnder(ctx, stored.ID, declared.Key, "", 0, pass(declared.Fields, late)); err != nil {
				return err
			}
		}
	}
	return nil
}

// pass returns the declared fields one pass stores, the backlinks when late and every other kind before them.
func pass(declared []FieldDefinition, late bool) []FieldDefinition {
	held := make([]FieldDefinition, 0, len(declared))
	for _, d := range declared {
		if (d.Kind == string(content.FieldKindBacklinks)) == late {
			held = append(held, d)
		}
	}
	return held
}

// fieldsUnder stores or carries the fields declared at one level, then the ones standing inside them.
func (r *run) fieldsUnder(
	ctx context.Context, groupID int, group, path string, parentID int, declared []FieldDefinition,
) error {
	for _, d := range declared {
		key := path + d.Key
		inside, err := r.oneField(ctx, groupID, group, key, parentID, d)
		if err != nil {
			return err
		}
		if inside == 0 || len(d.Fields) == 0 {
			continue
		}
		if err := r.fieldsUnder(ctx, groupID, group, key+".", inside, d.Fields); err != nil {
			return err
		}
	}
	return nil
}

// oneField stores or carries one field, returning the identity the fields inside it would stand under.
func (r *run) oneField(
	ctx context.Context, groupID int, group, key string, parentID int, d FieldDefinition,
) (int, error) {
	if r.declined[Confirmed{Subject: SubjectField, Key: key, Group: group}] {
		r.leftAlong(group, key)
		return 0, nil
	}
	planned := r.plannedFor(SubjectField, group, key)
	if len(planned) == 0 {
		return r.fieldAt(group, key).ID, nil
	}
	if len(planned) == 2 {
		return r.replaceField(ctx, groupID, parentID, d, planned)
	}
	if planned[0].Action == ActionCreate && r.fieldAt(group, key).ID != 0 {
		return r.carriedField(ctx, groupID, group, key, d, planned[0])
	}
	if planned[0].Action == ActionCreate {
		return r.createField(ctx, groupID, parentID, d, afresh(planned[0]))
	}
	return r.carryField(ctx, groupID, group, key, d, planned[0])
}

// afresh returns the planned create without the origin of a move, for a field the import stands anew.
func afresh(c Change) Change {
	c.Reason, c.From = "", ""
	return c
}

// carriedField carries what the file names onto a field that moved in with its values, when it differs.
func (r *run) carriedField(
	ctx context.Context, groupID int, group, key string, d FieldDefinition, planned Change,
) (int, error) {
	held := r.fieldAt(group, key)
	if sameStoredField(d, held) {
		r.did(planned)
		return held.ID, nil
	}
	return r.carryField(ctx, groupID, group, key, d, planned)
}

// replaceField takes a field away and stands the file's own in its place, when the admin agreed to lose it.
func (r *run) replaceField(
	ctx context.Context, groupID int, parentID int, d FieldDefinition, planned []Change,
) (int, error) {
	if !r.allows(planned[0]) {
		r.leftAlong(planned[0].Group, planned[0].Key)
		return 0, nil
	}
	if err := r.removeField(ctx, planned[0]); err != nil {
		return 0, err
	}
	r.did(planned[0])
	return r.createField(ctx, groupID, parentID, d, planned[1])
}

// leftAlong records every change the plan holds for the field and the fields inside it as left undone.
func (r *run) leftAlong(group, key string) {
	for _, c := range r.plan.Changes {
		if c.Subject == SubjectField && c.Group == group && (c.Key == key || strings.HasPrefix(c.Key, key+".")) {
			r.left(c)
		}
	}
}

// createField stores one field the file brings, inside its container when it stands in one.
func (r *run) createField(
	ctx context.Context, groupID, parentID int, d FieldDefinition, planned Change,
) (int, error) {
	wanted := fieldFrom(d)
	wanted.Settings = withheldConditions(wanted.Settings)
	if parentID == 0 {
		created, err := r.registry.CreateFieldInGroup(ctx, groupID, wanted)
		if err != nil {
			return 0, err
		}
		r.did(planned)
		return created.ID, nil
	}
	created, err := r.registry.CreateSubField(ctx, parentID, wanted)
	if err != nil {
		return 0, err
	}
	r.did(planned)
	return created.ID, nil
}

// carryField writes the label, the required flag and the settings the file names onto a stored field.
func (r *run) carryField(
	ctx context.Context, groupID int, group, key string, d FieldDefinition, planned Change,
) (int, error) {
	held := r.fieldAt(group, key)
	wanted := fieldFrom(d)
	wanted.Settings = withheldConditions(wanted.Settings)
	if !strings.Contains(key, ".") {
		if _, err := r.registry.UpdateFieldInGroup(ctx, groupID, wanted, held.UpdatedAt); err != nil {
			return 0, err
		}
		r.did(planned)
		return held.ID, nil
	}
	if _, err := r.registry.UpdateSubField(ctx, held.ID, wanted, held.UpdatedAt); err != nil {
		return 0, err
	}
	r.did(planned)
	return held.ID, nil
}

// conditions writes the rules each field is shown under, now every sibling the file names stands.
func (r *run) conditions(ctx context.Context) error {
	for _, declared := range r.envelope.Groups {
		stored := r.groupKeyed(declared.Key)
		if err := r.conditionsUnder(ctx, stored.ID, declared.Key, "", declared.Fields); err != nil {
			return err
		}
	}
	return nil
}

// conditionsUnder writes the rules one level of fields is shown under, then the levels they hold inside.
func (r *run) conditionsUnder(ctx context.Context, groupID int, group, path string, declared []FieldDefinition) error {
	for _, d := range declared {
		key := path + d.Key
		if err := r.conditionsOn(ctx, groupID, group, key, d); err != nil {
			return err
		}
		if err := r.conditionsUnder(ctx, groupID, group, key+".", d.Fields); err != nil {
			return err
		}
	}
	return nil
}

// conditionsOn writes the settings the file names onto a stored field when the site holds other ones.
func (r *run) conditionsOn(ctx context.Context, groupID int, group, key string, d FieldDefinition) error {
	held := r.fieldAt(group, key)
	if held.ID == 0 || sameSettings(held.Settings, d.Settings) {
		return nil
	}
	wanted := fieldFrom(d)
	if !strings.Contains(key, ".") {
		_, err := r.registry.UpdateFieldInGroup(ctx, groupID, wanted, held.UpdatedAt)
		return err
	}
	_, err := r.registry.UpdateSubField(ctx, held.ID, wanted, held.UpdatedAt)
	return err
}

// removeField takes away the field the change names, however deep it stands.
func (r *run) removeField(ctx context.Context, c Change) error {
	if !strings.Contains(c.Key, ".") {
		return r.registry.DeleteFieldInGroupSettled(ctx, r.groupKeyed(c.Group).ID, c.Key, r.settled)
	}
	return r.registry.DeleteSubFieldSettled(ctx, r.fieldAt(c.Group, c.Key).ID, r.settled)
}

// fieldAt returns the stored field the dotted path names inside the group, zero when the site holds none.
func (r *run) fieldAt(group, path string) content.Field {
	held := content.Field{Fields: r.groupKeyed(group).Fields}
	for _, step := range strings.Split(path, ".") {
		held, _ = fieldByKey(held.Fields, step)
	}
	return held
}

// orders stands every group and every field in the order the envelope lists them.
func (r *run) orders(ctx context.Context) error {
	if err := r.orderGroups(ctx); err != nil {
		return err
	}
	for _, declared := range r.envelope.Groups {
		stored := r.groupKeyed(declared.Key)
		if err := r.orderFields(ctx, stored.ID, stored.Fields, declared.Fields); err != nil {
			return err
		}
		if err := r.orderInside(ctx, stored.Fields, declared.Fields); err != nil {
			return err
		}
	}
	return nil
}

// orderGroups stands the stored groups in the order the envelope lists them, the ones it leaves out last.
func (r *run) orderGroups(ctx context.Context) error {
	wanted := make([]int, 0, len(r.stored))
	for _, declared := range r.envelope.Groups {
		if held, found := groupAmongStored(r.stored, declared.Key); found {
			wanted = append(wanted, held.ID)
		}
	}
	held := make([]int, 0, len(r.stored))
	for _, g := range r.stored {
		held = append(held, g.ID)
		if !groupDeclared(r.envelope.Groups, g.Key) {
			wanted = append(wanted, g.ID)
		}
	}
	if slices.Equal(held, wanted) {
		return nil
	}
	_, err := r.registry.ReorderGroups(ctx, wanted)
	return err
}

// orderFields stands one level of stored fields in the order the envelope lists them.
func (r *run) orderFields(ctx context.Context, groupID int, stored []content.Field, declared []FieldDefinition) error {
	wanted := wantedOrder(stored, declared)
	if slices.Equal(storedOrder(stored), wanted) {
		return nil
	}
	_, err := r.registry.ReorderFieldsInGroup(ctx, groupID, wanted)
	return err
}

// orderInside stands the fields within every stored container in the order the envelope lists them.
func (r *run) orderInside(ctx context.Context, stored []content.Field, declared []FieldDefinition) error {
	for _, held := range stored {
		d, found := declaredByKey(declared, held.Key)
		if !found || len(held.Fields) == 0 {
			continue
		}
		if err := r.orderOne(ctx, held, d); err != nil {
			return err
		}
	}
	return nil
}

// orderOne stands the fields inside one container in the envelope's order, then the ones inside those.
func (r *run) orderOne(ctx context.Context, held content.Field, d FieldDefinition) error {
	wanted := wantedOrder(held.Fields, d.Fields)
	if !slices.Equal(storedOrder(held.Fields), wanted) {
		if _, err := r.registry.ReorderSubFields(ctx, held.ID, wanted); err != nil {
			return err
		}
	}
	return r.orderInside(ctx, held.Fields, d.Fields)
}

// wantedOrder returns every stored key, the ones the envelope names first and in its order.
func wantedOrder(stored []content.Field, declared []FieldDefinition) []string {
	wanted := make([]string, 0, len(stored))
	for _, d := range declared {
		if _, found := fieldByKey(stored, d.Key); found {
			wanted = append(wanted, d.Key)
		}
	}
	for _, held := range stored {
		if !fieldDeclared(declared, held.Key) {
			wanted = append(wanted, held.Key)
		}
	}
	return wanted
}

// storedOrder returns the keys the stored fields stand in.
func storedOrder(stored []content.Field) []string {
	held := make([]string, len(stored))
	for i, f := range stored {
		held[i] = f.Key
	}
	return held
}

// declaredByKey returns the envelope's field carrying the key, reporting false when it holds none.
func declaredByKey(declared []FieldDefinition, key string) (FieldDefinition, bool) {
	for _, d := range declared {
		if d.Key == key {
			return d, true
		}
	}
	return FieldDefinition{}, false
}

// removals takes away the fields and the types the file dropped and the admin agreed to lose.
func (r *run) removals(ctx context.Context) error {
	for _, subject := range []string{SubjectField, SubjectType} {
		for _, c := range r.plan.Changes {
			if c.Subject != subject || c.Action != ActionDelete || c.Reason != ReasonRemoved {
				continue
			}
			if !r.allows(c) {
				r.left(c)
				continue
			}
			if err := r.remove(ctx, c); err != nil {
				return err
			}
			r.did(c)
		}
	}
	return nil
}

// remove takes away the one field or type the change names.
func (r *run) remove(ctx context.Context, c Change) error {
	if c.Subject == SubjectField {
		return r.removeField(ctx, c)
	}
	return r.registry.Delete(ctx, c.Key)
}
