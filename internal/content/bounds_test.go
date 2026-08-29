// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// bounded returns a field of the kind carrying the settings.
func bounded(t *testing.T, key string, kind content.FieldKind, settings map[string]any) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: content.TypePost, Key: key, Label: "A Field", Kind: kind, Settings: settings,
	})
	if err != nil {
		t.Fatalf("NewField(%s) error = %v, want nil", key, err)
	}
	return built
}

func TestValuesRefuseWhatTheBoundsForbid(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		field content.Field
		value any
		code  string
	}{
		"a number below min": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"min": float64(10)}),
			float64(5), "field_min",
		},
		"a number above max": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"max": float64(10)}),
			float64(50), "field_max",
		},
		"a text longer than maxlength": {
			bounded(t, "subtitle", content.FieldKindText, map[string]any{"maxlength": float64(3)}),
			"much too long", "field_length",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := content.Values{test.field.Key: test.value}

			err := held.Validate([]content.Field{test.field})

			if !errors.Is(err, content.ErrFieldBounds) {
				t.Fatalf("Validate() error = %v, want %v", err, content.ErrFieldBounds)
			}
			if code, _ := content.CodeOf(err); code != test.code {
				t.Errorf("code = %q, want %q", code, test.code)
			}
		})
	}
}

func TestValuesAcceptWhatTheBoundsAllow(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		field content.Field
		value any
	}{
		"a number inside the bounds": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"min": float64(1), "max": float64(10)}),
			float64(5),
		},
		"a number sitting on min": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"min": float64(5)}), float64(5),
		},
		"a number sitting on max": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"max": float64(5)}), float64(5),
		},
		"a text exactly as long as maxlength": {
			bounded(t, "subtitle", content.FieldKindText, map[string]any{"maxlength": float64(3)}), "abc",
		},
		"a text counted in letters not bytes": {
			bounded(t, "subtitle", content.FieldKindText, map[string]any{"maxlength": float64(3)}), "áéí",
		},
		"a field carrying no bounds": {
			bounded(t, "rating", content.FieldKindNumber, nil), float64(9000),
		},
		"a cleared value skips the bounds": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"min": float64(10)}), nil,
		},
		"step is never enforced": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"step": float64(5)}), float64(3),
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := content.Values{test.field.Key: test.value}

			if err := held.Validate([]content.Field{test.field}); err != nil {
				t.Errorf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestParkedValuesSkipTheBounds(t *testing.T) {
	t.Parallel()

	held := bounded(t, "rating", content.FieldKindNumber, map[string]any{"min": float64(10)})
	parked := content.Values{"rating": float64(5)}

	if err := parked.ValidateShape([]content.Field{held}); err != nil {
		t.Errorf("ValidateShape() error = %v, want the buffer to park what the bounds refuse", err)
	}

	wrong := content.Values{"rating": "five"}

	if err := wrong.ValidateShape([]content.Field{held}); !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("ValidateShape() error = %v, want %v, shape still applies", err, content.ErrFieldShape)
	}
}
