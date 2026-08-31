// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestStrippedTakesAKeyFromTheTop(t *testing.T) {
	t.Parallel()

	held := content.Values{"colour": "red", "doors": "five"}

	swept := held.Stripped([]string{"doors"})

	if !reflect.DeepEqual(swept, content.Values{"colour": "red"}) {
		t.Errorf("Stripped() = %v, want the doors gone", swept)
	}
	if len(held) != 2 {
		t.Errorf("the values held = %v, want the original left alone", held)
	}
}

func TestStrippedTakesAKeyFromInsideASection(t *testing.T) {
	t.Parallel()

	held := content.Values{"specs": map[string]any{"colour": "red", "doors": "five"}}

	swept := held.Stripped([]string{"specs", "doors"})

	want := content.Values{"specs": map[string]any{"colour": "red"}}
	if !reflect.DeepEqual(swept, want) {
		t.Errorf("Stripped() = %v, want the doors gone from inside", swept)
	}
}

func TestStrippedTakesAKeyFromEveryRow(t *testing.T) {
	t.Parallel()

	held := content.Values{"team": []any{
		map[string]any{"name": "Maria Perez", "role": "lead"},
		map[string]any{"name": "Kip", "role": "smith"},
	}}

	swept := held.Stripped([]string{"team", "role"})

	want := content.Values{"team": []any{
		map[string]any{"name": "Maria Perez"},
		map[string]any{"name": "Kip"},
	}}
	if !reflect.DeepEqual(swept, want) {
		t.Errorf("Stripped() = %v, want the role gone from every row", swept)
	}
}

func TestStrippedReachesThroughRowsIntoASection(t *testing.T) {
	t.Parallel()

	held := content.Values{"team": []any{
		map[string]any{"contact": map[string]any{"phone": "184467235", "email": "maria@example.com"}},
	}}

	swept := held.Stripped([]string{"team", "contact", "email"})

	want := content.Values{"team": []any{
		map[string]any{"contact": map[string]any{"phone": "184467235"}},
	}}
	if !reflect.DeepEqual(swept, want) {
		t.Errorf("Stripped() = %v, want the email gone from inside every row's section", swept)
	}
}

func TestStrippedLeavesWhatThePathDoesNotReach(t *testing.T) {
	t.Parallel()

	held := content.Values{"colour": "red", "specs": map[string]any{"doors": "five"}}

	for name, path := range map[string][]string{
		"no path at all":            {},
		"a key nothing holds":       {"absent"},
		"a key inside one absent":   {"absent", "doors"},
		"a key the section lacks":   {"specs", "absent"},
		"a path through a scalar":   {"colour", "deeper"},
		"a path past what is there": {"specs", "doors", "deeper"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			swept := held.Stripped(path)

			if !reflect.DeepEqual(swept, held) {
				t.Errorf("Stripped(%v) = %v, want the values untouched", path, swept)
			}
		})
	}
}
