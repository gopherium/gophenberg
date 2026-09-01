// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"

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

// Relations holds the items a content item points at, keyed by field key.
type Relations map[string][]uuid.UUID

// Target names one item a relation field points at, as a public reader sees it.
type Target struct {
	ID    uuid.UUID
	Title string
	Path  string
}

// Targets holds the items a content item points at, named and addressed, keyed by field key.
type Targets map[string][]Target

// SplitValues divides a patch into the scalar values and the targets its type declares.
func SplitValues(patch Values, fields []Field) (Values, Relations, error) {
	declared := make(map[string]Field, len(fields))
	for _, f := range fields {
		declared[f.Key] = f
	}
	scalars := make(Values, len(patch))
	relations := make(Relations)
	for key, value := range patch {
		f, found := declared[key]
		if !found {
			return nil, nil, Refuse(ErrUnknownField, "field_unknown",
				fmt.Sprintf("%s: %s", ErrUnknownField, key), Details{"field": key})
		}
		if f.Kind != FieldKindRelation {
			scalars[key] = value
			continue
		}
		targets, err := targetsOf(f, value)
		if err != nil {
			return nil, nil, err
		}
		relations[key] = targets
	}
	return scalars, relations, nil
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

// SelfTargeted reports whether any relation field of the item points at the item itself.
func (c Content) SelfTargeted() error {
	for key, targets := range c.Relations {
		for _, target := range targets {
			if target == c.ID {
				return Refuse(ErrSelfTarget, "target_is_self",
					fmt.Sprintf("%s: %s", ErrSelfTarget, key), Details{"field": key})
			}
		}
	}
	return nil
}

// Merge returns the stored targets with the patch applied, where a named field replaces its targets.
func (r Relations) Merge(patch Relations) Relations {
	merged := make(Relations, len(r)+len(patch))
	for key, targets := range r {
		merged[key] = targets
	}
	for key, targets := range patch {
		merged[key] = targets
	}
	return merged
}

// Filled reports whether every required field of the type holds a value.
func Filled(values Values, relations Relations, fields []Field) error {
	for _, f := range fields {
		if f.Required && unfilled(values, relations, f) {
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
	return Filled(inside, Relations{}, f.Fields)
}

// unfilled reports whether the field stands empty on the item.
func unfilled(values Values, relations Relations, f Field) bool {
	if f.Kind == FieldKindRelation {
		return len(relations[f.Key]) == 0
	}
	return empty(values[f.Key])
}
