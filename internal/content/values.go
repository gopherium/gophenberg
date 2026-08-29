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

// dateLayout is how a date field writes a day.
const dateLayout = "2006-01-02"

// Values holds a content item's scalar field values keyed by field key.
type Values map[string]any

// Validate reports whether every value matches the kind its field declares and the bounds it carries.
func (v Values) Validate(fields []Field) error {
	return v.validate(fields, true)
}

// ValidateShape reports whether every value matches its field's kind, leaving the bounds alone.
func (v Values) ValidateShape(fields []Field) error {
	return v.validate(fields, false)
}

// validate reports whether every value stands, checking the bounds only when asked.
func (v Values) validate(fields []Field, bounded bool) error {
	declared := make(map[string]Field, len(fields))
	for _, f := range fields {
		declared[f.Key] = f
	}
	for key, value := range v {
		f, found := declared[key]
		if !found {
			return Refuse(ErrUnknownField, "field_unknown",
				fmt.Sprintf("%s: %s", ErrUnknownField, key), Details{"field": key})
		}
		if f.Kind == FieldKindRelation {
			return Refuse(ErrFieldShape, "field_shape_value",
				fmt.Sprintf("%s: %s holds targets rather than a value", ErrFieldShape, key), Details{"field": key})
		}
		if value == nil {
			continue
		}
		if !holdsKind(value, f.Kind) {
			return Refuse(ErrFieldShape, "field_shape_kind",
				fmt.Sprintf("%s: %s holds %s", ErrFieldShape, key, f.Kind), Details{"field": key, "kind": string(f.Kind)})
		}
		if !bounded {
			continue
		}
		if err := withinBounds(f, value); err != nil {
			return err
		}
	}
	return nil
}

// withinBounds reports whether the value sits inside the bounds its field's settings name.
func withinBounds(f Field, value any) error {
	if len(f.Settings) == 0 {
		return nil
	}
	if held, ok := settingNumber(value); ok && f.Kind == FieldKindNumber {
		return numberWithinBounds(f, held)
	}
	if held, ok := value.(string); ok && f.Kind == FieldKindText {
		return textWithinBounds(f, held)
	}
	return nil
}

// outOfBounds returns the refusal naming the field and the limit it passed.
func outOfBounds(f Field, code, setting string, limit float64) error {
	return Refuse(ErrFieldBounds, code,
		fmt.Sprintf("%s: %s %s %v", ErrFieldBounds, f.Key, setting, limit),
		Details{"field": f.Key, "limit": limit})
}

// numberWithinBounds reports whether the number sits between the min and the max.
func numberWithinBounds(f Field, held float64) error {
	if low, named := settingNumber(f.Settings[SettingMin]); named && held < low {
		return outOfBounds(f, "field_min", SettingMin, low)
	}
	if high, named := settingNumber(f.Settings[SettingMax]); named && held > high {
		return outOfBounds(f, "field_max", SettingMax, high)
	}
	return nil
}

// textWithinBounds reports whether the text is no longer than maxlength.
func textWithinBounds(f Field, held string) error {
	longest, named := settingNumber(f.Settings[SettingMaxLength])
	if named && float64(len([]rune(held))) > longest {
		return outOfBounds(f, "field_length", SettingMaxLength, longest)
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
	_, held := settingNumber(value)
	return held
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

// empty reports whether a value stands for a field nobody filled in.
func empty(value any) bool {
	if value == nil {
		return true
	}
	written, ok := value.(string)
	return ok && written == ""
}
