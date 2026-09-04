// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"context"
	"slices"
	"strings"

	"github.com/gopherium/gophenberg/internal/content"
)

// vacated takes away every field the file moved elsewhere.
func (r *run) vacated(ctx context.Context) error {
	for _, c := range r.plan.Changes {
		if c.Subject != SubjectField || c.Action != ActionDelete || c.Reason != ReasonMoved {
			continue
		}
		if !r.allows(c) {
			r.declineArrivals(c)
			r.left(c)
			continue
		}
		if err := r.removeField(ctx, c); err != nil {
			return err
		}
		r.did(c)
	}
	return nil
}

// declineArrivals holds back the field every group would have gained from a move nobody confirmed.
func (r *run) declineArrivals(lost Change) {
	for _, held := range arrivals(r.plan.Changes, lost) {
		r.declined[Confirmed{Subject: SubjectField, Key: held.Key, Group: held.Group}] = true
	}
}

// fields stores the fields the envelope brings and carries what it changed onto the stored ones.
func (r *run) fields(ctx context.Context) error {
	for _, declared := range r.envelope.Groups {
		stored := r.groupKeyed(declared.Key)
		if err := r.fieldsUnder(ctx, stored.ID, declared.Key, "", 0, declared.Fields); err != nil {
			return err
		}
	}
	return nil
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
		return 0, nil
	}
	planned := r.plannedFor(SubjectField, group, key)
	if len(planned) == 0 {
		return r.fieldAt(group, key).ID, nil
	}
	if len(planned) == 2 {
		return r.replaceField(ctx, groupID, parentID, d, planned)
	}
	if planned[0].Action == ActionCreate {
		return r.createField(ctx, groupID, parentID, d, planned[0])
	}
	return r.carryField(ctx, groupID, group, key, d, planned[0])
}

// replaceField takes a field away and stands the file's own in its place, when the admin agreed to lose it.
func (r *run) replaceField(
	ctx context.Context, groupID int, parentID int, d FieldDefinition, planned []Change,
) (int, error) {
	if !r.allows(planned[0]) {
		r.left(planned[0])
		r.left(planned[1])
		return 0, nil
	}
	if err := r.removeField(ctx, planned[0]); err != nil {
		return 0, err
	}
	r.did(planned[0])
	return r.createField(ctx, groupID, parentID, d, planned[1])
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
		return r.registry.DeleteFieldInGroup(ctx, r.groupKeyed(c.Group).ID, c.Key)
	}
	return r.registry.DeleteSubField(ctx, r.fieldAt(c.Group, c.Key).ID)
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
		wanted = append(wanted, r.groupKeyed(declared.Key).ID)
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

// removals takes away what the file dropped and the admin agreed to lose.
func (r *run) removals(ctx context.Context) error {
	for _, subject := range []string{SubjectField, SubjectGroup, SubjectType} {
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

// remove takes away the one definition the change names.
func (r *run) remove(ctx context.Context, c Change) error {
	if c.Subject == SubjectField {
		return r.removeField(ctx, c)
	}
	if c.Subject == SubjectType {
		return r.registry.Delete(ctx, c.Key)
	}
	return r.registry.DeleteGroup(ctx, r.groupKeyed(c.Key).ID)
}
