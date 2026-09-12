// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
)

// ErrTooManyTargets reports that a relation field holding one was given more.
var ErrTooManyTargets = errors.New("content: field holds one target")

// ErrRepeatedTarget reports that a relation field was given the same target twice.
var ErrRepeatedTarget = errors.New("content: repeated target")

// ErrTargetNotFound reports that a relation names an item nothing holds.
var ErrTargetNotFound = errors.New("content: target not found")

// ErrSelfTarget reports that a relation field points an item at itself.
var ErrSelfTarget = errors.New("content: an item cannot point at itself")

// ErrTargetType reports that a relation names an item of the wrong type.
var ErrTargetType = errors.New("content: target is not the type the field points at")

// FieldTargets names one relation field beside the items its values point at.
type FieldTargets struct {
	Field   Field
	Targets []uuid.UUID
}

// Target names one item a relation field points at, as a public reader sees it.
type Target struct {
	ID    uuid.UUID
	Title string
	Path  string
}

// Targets holds the items a content item points at, named and addressed, keyed by field key.
type Targets map[string][]Target

// Pointer names one item pointing at another through a relation field, as a public reader sees it.
type Pointer struct {
	ID    uuid.UUID
	Type  string
	Title string
	Path  string
}

// HeldTargets returns every relation field the fields declare beside the targets the values point at.
func HeldTargets(fields []Field, values Values) ([]FieldTargets, error) {
	var held []FieldTargets
	if err := targetsUnder(fields, values, &held); err != nil {
		return nil, err
	}
	return held, nil
}

// HeldIdentities returns every identity the values name through the fields' relations.
func HeldIdentities(fields []Field, values Values) map[uuid.UUID]bool {
	held, err := HeldTargets(fields, values)
	if err != nil {
		return nil
	}
	identities := make(map[uuid.UUID]bool, len(held))
	for _, ft := range held {
		for _, target := range ft.Targets {
			identities[target] = true
		}
	}
	return identities
}

// targetsUnder gathers the targets each relation field holds, descending into every container.
func targetsUnder(fields []Field, values Values, held *[]FieldTargets) error {
	for _, f := range fields {
		if f.Kind == FieldKindRelation {
			targets, err := targetsOf(f, values[f.Key])
			if err != nil {
				return err
			}
			gather(held, f, targets)
			continue
		}
		if !f.Kind.Holds() {
			continue
		}
		if err := targetsInside(f, values[f.Key], held); err != nil {
			return err
		}
	}
	return nil
}

// targetsInside gathers the targets the rows or the object a container holds point at.
func targetsInside(f Field, value any, held *[]FieldTargets) error {
	if rows, listed := value.([]any); listed {
		for _, row := range rows {
			if err := targetsInside(f, row, held); err != nil {
				return err
			}
		}
		return nil
	}
	inside, standing := value.(map[string]any)
	if !standing {
		inside = Values{}
	}
	return targetsUnder(f.Fields, inside, held)
}

// gather adds the targets to the field's own, keeping each one once in the order they were named.
func gather(held *[]FieldTargets, f Field, targets []uuid.UUID) {
	for i, ft := range *held {
		if ft.Field.ID != f.ID {
			continue
		}
		for _, target := range targets {
			if !slices.Contains((*held)[i].Targets, target) {
				(*held)[i].Targets = append((*held)[i].Targets, target)
			}
		}
		return
	}
	*held = append(*held, FieldTargets{Field: f, Targets: targets})
}

// targetsOf returns the identities a relation value names, or the reason it names none.
func targetsOf(f Field, value any) ([]uuid.UUID, error) {
	if value == nil {
		return []uuid.UUID{}, nil
	}
	listed, ok := value.([]any)
	if !ok {
		return nil, Refuse(ErrFieldShape, "field_shape_list",
			fmt.Sprintf("%s: %s holds a list of targets", ErrFieldShape, f.Key), Details{"field": f.Key})
	}
	if !f.Many && len(listed) > 1 {
		return nil, Refuse(ErrTooManyTargets, "too_many_targets",
			fmt.Sprintf("%s: %s", ErrTooManyTargets, f.Key), Details{"field": f.Key})
	}
	targets := make([]uuid.UUID, 0, len(listed))
	held := make(map[uuid.UUID]bool, len(listed))
	for _, raw := range listed {
		target, err := targetID(f, raw)
		if err != nil {
			return nil, err
		}
		if held[target] {
			return nil, Refuse(ErrRepeatedTarget, "target_repeated",
				fmt.Sprintf("%s: %s", ErrRepeatedTarget, f.Key), Details{"field": f.Key})
		}
		held[target] = true
		targets = append(targets, target)
	}
	return targets, nil
}

// targetID returns the identity a listed target names.
func targetID(f Field, raw any) (uuid.UUID, error) {
	written, ok := raw.(string)
	if !ok {
		return uuid.Nil, Refuse(ErrFieldShape, "field_shape_identity",
			fmt.Sprintf("%s: %s holds identities", ErrFieldShape, f.Key), Details{"field": f.Key})
	}
	target, err := uuid.Parse(written)
	if err != nil {
		return uuid.Nil, Refuse(ErrFieldShape, "field_shape_identity",
			fmt.Sprintf("%s: %s holds identities", ErrFieldShape, f.Key), Details{"field": f.Key})
	}
	return target, nil
}

// SelfTargeted reports whether any relation the item's values name points at the item itself.
func (c Content) SelfTargeted(fields []Field) error {
	held, err := HeldTargets(fields, c.Fields)
	if err != nil {
		return err
	}
	for _, ft := range held {
		if slices.Contains(ft.Targets, c.ID) {
			return Refuse(ErrSelfTarget, "target_is_self",
				fmt.Sprintf("%s: %s", ErrSelfTarget, ft.Field.Key), Details{"field": ft.Field.Key})
		}
	}
	return nil
}

// Filled reports whether every required field the conditions show holds a value.
func Filled(values Values, fields []Field) error {
	hidden := Hidden(fields, values)
	for _, f := range fields {
		if hidden[f.Key] {
			continue
		}
		if f.Required && empty(values[f.Key]) {
			return Refuse(ErrFieldRequired, "field_required",
				fmt.Sprintf("%s: %s", ErrFieldRequired, f.Key), Details{"field": f.Key})
		}
		if !f.Kind.Holds() {
			continue
		}
		if err := filledInside(f, values[f.Key]); err != nil {
			return err
		}
	}
	return nil
}

// filledInside reports whether every required field inside a stored container holds a value.
func filledInside(f Field, value any) error {
	if value == nil {
		return nil
	}
	if rows, listed := value.([]any); listed {
		for _, row := range rows {
			if err := filledInside(f, row); err != nil {
				return err
			}
		}
		return nil
	}
	inside, held := value.(map[string]any)
	if !held {
		return nil
	}
	return Filled(inside, f.Fields)
}
