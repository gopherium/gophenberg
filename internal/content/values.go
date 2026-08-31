// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
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
		if err := valueStands(f, value, bounded); err != nil {
			return err
		}
	}
	return nil
}

// valueStands reports whether one value matches its field, checking the bounds only when asked.
func valueStands(f Field, value any, bounded bool) error {
	if f.Kind == FieldKindRelation {
		return Refuse(ErrFieldShape, "field_shape_value",
			fmt.Sprintf("%s: %s holds targets rather than a value", ErrFieldShape, f.Key),
			Details{"field": f.Key})
	}
	if value == nil {
		return nil
	}
	if f.Kind.Holds() {
		return heldStands(f, value, bounded)
	}
	if !holdsShape(f, value) {
		return Refuse(ErrFieldShape, "field_shape_kind",
			fmt.Sprintf("%s: %s holds %s", ErrFieldShape, f.Key, f.Kind),
			Details{"field": f.Key, "kind": string(f.Kind)})
	}
	if err := holdsEachOnce(f, value); err != nil {
		return err
	}
	if !bounded {
		return nil
	}
	return withinBounds(f, value)
}

// heldStands reports whether a container's value matches the sub fields it declares.
func heldStands(f Field, value any, bounded bool) error {
	if f.Kind == FieldKindSection {
		return insideStands(f, value, bounded)
	}
	rows, listed := value.([]any)
	if !listed {
		return wrongShape(f)
	}
	for _, row := range rows {
		if err := insideStands(f, row, bounded); err != nil {
			return err
		}
	}
	if !bounded {
		return nil
	}
	return rowsWithinBounds(f, len(rows))
}

// insideStands reports whether one object matches the sub fields the container declares.
func insideStands(f Field, value any, bounded bool) error {
	inside, held := value.(map[string]any)
	if !held {
		return wrongShape(f)
	}
	return Values(inside).validate(f.Fields, bounded)
}

// wrongShape returns the error naming the shape the field holds.
func wrongShape(f Field) error {
	return Refuse(ErrFieldShape, "field_shape_kind",
		fmt.Sprintf("%s: %s holds %s", ErrFieldShape, f.Key, f.Kind),
		Details{"field": f.Key, "kind": string(f.Kind)})
}

// rowsWithinBounds reports whether the row count sits between the min and the max the field names.
func rowsWithinBounds(f Field, held int) error {
	if low, named := settingNumber(f.Settings[SettingMin]); named && float64(held) < low {
		return outOfBounds(f, "field_rows_min", SettingMin, low)
	}
	if high, named := settingNumber(f.Settings[SettingMax]); named && float64(held) > high {
		return outOfBounds(f, "field_rows_max", SettingMax, high)
	}
	return nil
}

// holdsEachOnce reports whether a list field names each of its members once.
func holdsEachOnce(f Field, value any) error {
	members, many := value.([]any)
	if !many {
		return nil
	}
	seen := make(map[any]bool, len(members))
	for _, member := range members {
		named := namedOnce(f, member)
		if seen[named] {
			return Refuse(ErrFieldShape, "field_repeated",
				fmt.Sprintf("%s: %s names the same one twice", ErrFieldShape, f.Key),
				Details{"field": f.Key})
		}
		seen[named] = true
	}
	return nil
}

// namedOnce returns the member as the one thing it names, however a caller wrote it.
func namedOnce(f Field, member any) any {
	if f.Kind != FieldKindMedia {
		return member
	}
	id, _ := MediaIdentity(member)
	return id
}

// withinBounds reports whether the value sits inside the bounds its field's settings name.
func withinBounds(f Field, value any) error {
	if len(f.Settings) == 0 {
		return nil
	}
	if f.Kind == FieldKindChoice {
		return choiceWithinBounds(f, value)
	}
	if held, ok := settingNumber(value); ok && f.Kind == FieldKindNumber {
		return numberWithinBounds(f, held)
	}
	if held, ok := value.(string); ok && f.Kind == FieldKindText {
		return textWithinBounds(f, held)
	}
	return nil
}

// choiceWithinBounds reports whether every held value sits among the field's choices.
func choiceWithinBounds(f Field, value any) error {
	if custom, _ := f.Settings[SettingAllowCustom].(bool); custom {
		return nil
	}
	held := choiceValues(f.Settings[SettingChoices])
	if len(held) == 0 {
		return nil
	}
	members, many := value.([]any)
	if !many {
		members = []any{value}
	}
	for _, member := range members {
		if !held[member.(string)] {
			return Refuse(ErrFieldBounds, "field_choice",
				fmt.Sprintf("%s: %s is not among the choices", ErrFieldBounds, f.Key),
				Details{"field": f.Key})
		}
	}
	return nil
}

// outOfBounds returns the error naming the field and the limit it passed.
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

// textWithinBounds reports whether the text is no longer than maxlength and matches its variant.
func textWithinBounds(f Field, held string) error {
	longest, named := settingNumber(f.Settings[SettingMaxLength])
	if named && float64(len([]rune(held))) > longest {
		return outOfBounds(f, "field_length", SettingMaxLength, longest)
	}
	return textFormat(f, held)
}

// emailWord matches something before an at sign, something after, and a dot in the domain.
var emailWord = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// textFormat reports whether the text reads as the variant its field carries.
func textFormat(f Field, held string) error {
	variant, _ := f.Settings[SettingVariant].(string)
	if !readsAsVariant(variant, held) {
		return badFormat(f, variant)
	}
	return nil
}

// readsAsVariant reports whether the text reads as the named variant.
func readsAsVariant(variant, held string) bool {
	if variant == "email" {
		return emailWord.MatchString(held)
	}
	if variant == "url" {
		return webAddress(held)
	}
	return true
}

// badFormat returns the error naming the field and the format it missed.
func badFormat(f Field, variant string) error {
	return Refuse(ErrFieldBounds, "field_format",
		fmt.Sprintf("%s: %s holds no %s", ErrFieldBounds, f.Key, variant),
		Details{"field": f.Key, "variant": variant})
}

// webAddress reports whether the text reads as an http or https address.
func webAddress(held string) bool {
	parsed, err := url.Parse(held)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// holdsShape reports whether the value carries the shape the field stores.
func holdsShape(f Field, value any) bool {
	switch {
	case f.Kind == FieldKindChoice && choiceMultiple(f):
		return holdsEvery(value, isWord)
	case f.Kind == FieldKindChoice:
		return isWord(value)
	case f.Kind == FieldKindMedia && f.Many:
		return holdsEvery(value, isMediaID)
	default:
		return holdsKind(value, f.Kind)
	}
}

// choiceMultiple reports whether the choice field holds several values.
func choiceMultiple(f Field) bool {
	multiple, _ := f.Settings[SettingMultiple].(bool)
	return multiple
}

// isWord reports whether the value is a string.
func isWord(value any) bool {
	_, ok := value.(string)
	return ok
}

// holdsEvery reports whether the value is a list whose members all pass the check.
func holdsEvery(value any, check func(any) bool) bool {
	members, ok := value.([]any)
	if !ok {
		return false
	}
	for _, member := range members {
		if !check(member) {
			return false
		}
	}
	return true
}

// holdsKind reports whether the value carries the shape the kind stores.
func holdsKind(value any, kind FieldKind) bool {
	switch kind {
	case FieldKindText:
		_, ok := value.(string)
		return ok
	case FieldKindNumber:
		return isNumber(value)
	case FieldKindMedia:
		return isMediaID(value)
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

// isMediaID reports whether the value is a whole number the library can store as an identity.
func isMediaID(value any) bool {
	_, ok := MediaIdentity(value)
	return ok
}

// MediaIdentity returns the identity the value names, and whether the library can store it.
func MediaIdentity(value any) (int64, bool) {
	switch held := value.(type) {
	case int:
		return int64(held), held >= 1
	case int32:
		return int64(held), held >= 1
	case int64:
		return held, held >= 1
	case float32:
		return storableIdentity(float64(held))
	case float64:
		return storableIdentity(held)
	default:
		return 0, false
	}
}

// storableIdentity returns the number as an identity, and whether the library can store it.
func storableIdentity(held float64) (int64, bool) {
	if held < 1 || held >= math.MaxInt64 || held != math.Trunc(held) {
		return 0, false
	}
	return int64(held), true
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
	if members, ok := value.([]any); ok {
		return len(members) == 0
	}
	written, ok := value.(string)
	return ok && written == ""
}
