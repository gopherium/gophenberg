// SPDX-License-Identifier: Apache-2.0

package content

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ErrFieldFilterInvalid reports a field filter term a list cannot read.
var ErrFieldFilterInvalid = errors.New("content: field filter invalid")

// The characters wrapping the key of a field filter parameter.
const (
	fieldTermPrefix = "field["
	fieldTermSuffix = "]"
)

// termCoercers holds the readers of a filter value, one per kind a filter reads.
var termCoercers = map[FieldKind]func(Field, string) (any, error){
	FieldKindText:    coerceText,
	FieldKindNumber:  coerceNumber,
	FieldKindBoolean: coerceBoolean,
	FieldKindDate:    coerceDate,
	FieldKindChoice:  coerceChoice,
}

// ParseFieldFilter returns the containment object the field[<key>] parameters name, each coerced by its field's kind.
func ParseFieldFilter(query url.Values, fields []Field) (map[string]any, error) {
	var terms map[string]any
	for name, raws := range query {
		key, isTerm := termKey(name)
		if !isTerm {
			continue
		}
		if len(raws) > 1 {
			return nil, refuseTerm(key, "named more than once")
		}
		f, err := fieldAmong(fields, key)
		if err != nil {
			return nil, refuseTerm(key, "not a field of the type")
		}
		value, err := coerceTerm(f, raws[0])
		if err != nil {
			return nil, err
		}
		if terms == nil {
			terms = map[string]any{}
		}
		terms[key] = value
	}
	return terms, nil
}

// NamesFieldFilter reports whether the query names any field filter term.
func NamesFieldFilter(query url.Values) bool {
	for name := range query {
		if _, isTerm := termKey(name); isTerm {
			return true
		}
	}
	return false
}

// termKey returns the key a field[<key>] parameter names and whether the name has that shape.
func termKey(name string) (string, bool) {
	rest, prefixed := strings.CutPrefix(name, fieldTermPrefix)
	if !prefixed {
		return "", false
	}
	return strings.CutSuffix(rest, fieldTermSuffix)
}

// coerceTerm returns the raw value in the shape the field stores, refusing a kind no filter reads.
func coerceTerm(f Field, raw string) (any, error) {
	coerce, filterable := termCoercers[f.Kind]
	if !filterable {
		return nil, refuseTerm(f.Key, "not a kind a filter reads")
	}
	return coerce(f, raw)
}

// coerceText returns the raw value as written.
func coerceText(_ Field, raw string) (any, error) {
	return raw, nil
}

// coerceNumber returns the number the raw value writes, refusing words, NaN and the infinities.
func coerceNumber(f Field, raw string) (any, error) {
	number, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return nil, refuseTerm(f.Key, "not a number")
	}
	return number, nil
}

// coerceBoolean returns the boolean the words true and false name, refusing any other word.
func coerceBoolean(f Field, raw string) (any, error) {
	if raw != "true" && raw != "false" {
		return nil, refuseTerm(f.Key, "not true or false")
	}
	return raw == "true", nil
}

// coerceDate returns the raw value once it reads as a calendar date.
func coerceDate(f Field, raw string) (any, error) {
	if _, err := time.Parse(dateLayout, raw); err != nil {
		return nil, refuseTerm(f.Key, "not a date")
	}
	return raw, nil
}

// coerceChoice returns the raw value, inside a one member list when the field holds several.
func coerceChoice(f Field, raw string) (any, error) {
	if choiceMultiple(f) {
		return []any{raw}, nil
	}
	return raw, nil
}

// refuseTerm returns the error naming what a field filter term could not read.
func refuseTerm(key, reason string) error {
	return fmt.Errorf("%w: field[%s] %s", ErrFieldFilterInvalid, key, reason)
}

// ListedValues returns the shown values under the fields opted into the admin list.
func ListedValues(fields []Field, values Values) Values {
	shown := Shown(fields, values)
	listed := Values{}
	for _, f := range fields {
		if value, held := shown[f.Key]; held && Listed(f) {
			listed[f.Key] = value
		}
	}
	return listed
}
