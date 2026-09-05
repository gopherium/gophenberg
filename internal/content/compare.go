// SPDX-License-Identifier: Apache-2.0

package content

import (
	"strconv"
	"strings"
)

// compare reports whether a held value satisfies the operator and the rule value, judged by the value's shape.
func compare(operator string, held any, value string) bool {
	switch operator {
	case OperatorIs:
		return equals(held, value)
	case OperatorIsNot:
		return !equals(held, value)
	case OperatorLess:
		return ordered(held, value, -1)
	case OperatorGreater:
		return ordered(held, value, 1)
	case OperatorContains:
		return contains(held, value)
	case OperatorEmpty:
		return empty(held)
	case OperatorNotEmpty:
		return !empty(held)
	default:
		return false
	}
}

// equals reports whether a held string, number or boolean is the rule value.
func equals(held any, value string) bool {
	switch typed := held.(type) {
	case string:
		return typed == value
	case bool:
		return strconv.FormatBool(typed) == value
	}
	number, numeric := settingNumber(held)
	parsed, err := strconv.ParseFloat(value, 64)
	return numeric && err == nil && parsed == number
}

// ordered reports whether a held string or number sits on the given side of the rule value.
func ordered(held any, value string, side int) bool {
	if typed, worded := held.(string); worded {
		return strings.Compare(typed, value)*side > 0
	}
	number, numeric := settingNumber(held)
	parsed, err := strconv.ParseFloat(value, 64)
	return numeric && err == nil && (number-parsed)*float64(side) > 0
}

// contains reports whether a held string holds the rule value inside it, or a held list holds it as a member.
func contains(held any, value string) bool {
	switch typed := held.(type) {
	case string:
		return strings.Contains(typed, value)
	case []any:
		for _, member := range typed {
			if word, worded := member.(string); worded && word == value {
				return true
			}
		}
	}
	return false
}
