// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/big"
	"slices"
)

// fieldsContain answers fields_contain(fields, filter) as jsonb's @> answers it, NULL for a NULL argument.
func fieldsContain(args []driver.Value) (driver.Value, error) {
	if args[0] == nil || args[1] == nil {
		return nil, nil
	}
	fields, err := decodeDocument(args[0])
	if err != nil {
		return nil, fmt.Errorf("fields_contain: the fields: %w", err)
	}
	filter, err := decodeDocument(args[1])
	if err != nil {
		return nil, fmt.Errorf("fields_contain: the filter: %w", err)
	}
	return containsAtTop(fields, filter), nil
}

// containsAtTop reports whether stored holds filter at the top of a document, where a list also holds a lone member.
func containsAtTop(stored, filter any) bool {
	if _, list := stored.([]any); list && plain(filter) {
		return containsList(stored, []any{filter})
	}
	return contains(stored, filter)
}

// plain reports whether value is a JSON value that is neither an object nor a list.
func plain(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return false
	}
	return true
}

// contains reports whether stored holds filter below the top of a document, each JSON kind of filter by its own rule.
func contains(stored, filter any) bool {
	switch want := filter.(type) {
	case map[string]any:
		return containsObject(stored, want)
	case []any:
		return containsList(stored, want)
	case json.Number:
		return containsNumber(stored, want)
	case string:
		return containsSame(stored, want)
	case bool:
		return containsSame(stored, want)
	}
	return stored == nil
}

// containsObject reports whether stored is an object holding each key of filter with a value that holds the filter's.
func containsObject(stored any, filter map[string]any) bool {
	object, ok := stored.(map[string]any)
	if !ok {
		return false
	}
	for key, want := range filter {
		if have, found := object[key]; !found || !contains(have, want) {
			return false
		}
	}
	return true
}

// containsList reports whether stored is a list where some member holds each member of filter.
func containsList(stored any, filter []any) bool {
	list, ok := stored.([]any)
	if !ok {
		return false
	}
	for _, want := range filter {
		if !slices.ContainsFunc(list, func(have any) bool { return contains(have, want) }) {
			return false
		}
	}
	return true
}

// containsNumber reports whether stored is a number of the same value as filter, every digit of both kept.
func containsNumber(stored any, filter json.Number) bool {
	number, ok := stored.(json.Number)
	if !ok {
		return false
	}
	have, readHave := new(big.Rat).SetString(number.String())
	want, readWant := new(big.Rat).SetString(filter.String())
	return readHave && readWant && have.Cmp(want) == 0
}

// containsSame reports whether stored is a value of the same Go type as filter and equal to it.
func containsSame[T string | bool](stored any, filter T) bool {
	value, ok := stored.(T)
	return ok && value == filter
}
