// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"time"
)

// ErrUnknownField reports that the type declares no field under the key.
var ErrUnknownField = errors.New("content: unknown field")

// ErrFieldShape reports that a value is not the kind its field holds.
var ErrFieldShape = errors.New("content: wrong kind of value")

// ErrFieldRequired reports that a field must hold a value before publishing.
var ErrFieldRequired = errors.New("content: field is required")

// ErrRelationValuesWait reports that relation values wait for the relation table.
var ErrRelationValuesWait = errors.New("content: relation values wait for the relation table")

// dateLayout is how a date field writes a day.
const dateLayout = "2006-01-02"

// Values holds a content item's scalar field values keyed by field key.
type Values map[string]any

// Validate reports whether every value matches the kind its field declares.
func (v Values) Validate(fields []Field) error {
	declared := make(map[string]Field, len(fields))
	for _, f := range fields {
		declared[f.Key] = f
	}
	for key, value := range v {
		f, found := declared[key]
		if !found {
			return fmt.Errorf("%w: %s", ErrUnknownField, key)
		}
		if f.Kind == FieldKindRelation {
			return fmt.Errorf("%w: %s", ErrRelationValuesWait, key)
		}
		if value == nil {
			continue
		}
		if !holdsKind(value, f.Kind) {
			return fmt.Errorf("%w: %s holds %s", ErrFieldShape, key, f.Kind)
		}
	}
	return nil
}

// holdsKind reports whether the value carries the shape the kind stores.
func holdsKind(value any, kind FieldKind) bool {
	switch kind {
	case FieldKindText:
		_, ok := value.(string)
		return ok
	case FieldKindNumber, FieldKindMedia:
		return isNumber(value)
	case FieldKindBoolean:
		_, ok := value.(bool)
		return ok
	case FieldKindDate:
		return isDay(value)
	default:
		return false
	}
}

// isNumber reports whether the value is a number, however it was decoded.
func isNumber(value any) bool {
	switch value.(type) {
	case float64, float32, int, int32, int64:
		return true
	default:
		return false
	}
}

// isDay reports whether the value is a day written as a date.
func isDay(value any) bool {
	written, ok := value.(string)
	if !ok {
		return false
	}
	_, err := time.Parse(dateLayout, written)
	return err == nil
}

// Merge returns the stored values with the patch applied, where a nil value clears its key.
func (v Values) Merge(patch Values) Values {
	merged := make(Values, len(v)+len(patch))
	for key, value := range v {
		merged[key] = value
	}
	for key, value := range patch {
		if value == nil {
			delete(merged, key)
			continue
		}
		merged[key] = value
	}
	return merged
}

// Filled reports whether every required field holds a value.
func (v Values) Filled(fields []Field) error {
	for _, f := range fields {
		if !f.Required {
			continue
		}
		if empty(v[f.Key]) {
			return fmt.Errorf("%w: %s", ErrFieldRequired, f.Key)
		}
	}
	return nil
}

// empty reports whether a value stands for a field nobody filled in.
func empty(value any) bool {
	if value == nil {
		return true
	}
	written, ok := value.(string)
	return ok && written == ""
}
