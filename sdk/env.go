// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Env reads the settings under the program prefix.
type Env struct {
	Prefix string
	Getenv func(string) string
}

// Key returns the full name of the setting called name.
func (e Env) Key(name string) string {
	return e.Prefix + name
}

// Value returns the setting's value with surrounding spaces trimmed, empty when it is unset.
func (e Env) Value(name string) string {
	if e.Getenv == nil {
		return ""
	}
	return strings.TrimSpace(e.Getenv(e.Key(name)))
}

// Required returns the setting's value, an error naming it when it is empty.
func (e Env) Required(name string) (string, error) {
	value := e.Value(name)
	if value == "" {
		return "", fmt.Errorf("%s is required", e.Key(name))
	}
	return value, nil
}

// Duration returns the setting as a duration above zero, the fallback when it is empty.
func (e Env) Duration(name string, fallback time.Duration) (time.Duration, error) {
	return parse(e, name, fallback, positiveDuration)
}

// Count returns the setting as a whole number above zero, the fallback when it is empty.
func (e Env) Count(name string, fallback int) (int, error) {
	return parse(e, name, fallback, positiveWhole)
}

// Counts returns the setting as rising whole numbers above zero split by commas, the fallback when it is empty.
func (e Env) Counts(name string, fallback []int) ([]int, error) {
	return parse(e, name, fallback, risingWholes)
}

// Flag returns the setting as true or false, the fallback when it is empty.
func (e Env) Flag(name string, fallback bool) (bool, error) {
	return parse(e, name, fallback, trueOrFalse)
}

// Within returns the settings under the prefix followed by more.
func (e Env) Within(more string) Env {
	return Env{Prefix: e.Prefix + more, Getenv: e.Getenv}
}

// parse returns the setting read by read, the fallback when it is empty, any error naming the setting.
func parse[T any](e Env, name string, fallback T, read func(string) (T, error)) (T, error) {
	value := e.Value(name)
	if value == "" {
		return fallback, nil
	}
	held, err := read(value)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("%s: %w", e.Key(name), err)
	}
	return held, nil
}

// positiveDuration reads value as a duration above zero.
func positiveDuration(value string) (time.Duration, error) {
	read, err := time.ParseDuration(value)
	if err != nil {
		return 0, complaint("must be a duration like 30s", value)
	}
	if read <= 0 {
		return 0, complaint("must stand above zero", value)
	}
	return read, nil
}

// positiveWhole reads value as a whole number above zero.
func positiveWhole(value string) (int, error) {
	read, err := strconv.ParseInt(value, 10, 0)
	overflowed := errors.Is(err, strconv.ErrRange)
	switch {
	case err != nil && !overflowed:
		return 0, complaint("must be a whole number", value)
	case read <= 0:
		return 0, complaint("must stand above zero", value)
	case overflowed:
		return 0, complaint("must stand at or below "+strconv.Itoa(math.MaxInt), value)
	}
	return int(read), nil
}

// risingWholes reads value as whole numbers above zero split by commas, each above the one before.
func risingWholes(value string) ([]int, error) {
	entries := strings.Split(value, ",")
	read := make([]int, 0, len(entries))
	for _, entry := range entries {
		count, err := positiveWhole(strings.TrimSpace(entry))
		if err != nil {
			return nil, fmt.Errorf("%w in %q", err, value)
		}
		if len(read) > 0 && count <= read[len(read)-1] {
			return nil, complaint("must list each number once from the smallest up", value)
		}
		read = append(read, count)
	}
	return read, nil
}

// trueOrFalse reads value as true or false.
func trueOrFalse(value string) (bool, error) {
	read, err := strconv.ParseBool(value)
	if err != nil {
		return false, complaint("must be true or false", value)
	}
	return read, nil
}

// complaint returns the error saying what a setting must be and the value it holds.
func complaint(must, value string) error {
	return fmt.Errorf("%s, got %q", must, value)
}
