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
		"a number the caller decoded as a small float": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"max": float64(10)}),
			float32(50), "field_max",
		},
		"a number the caller decoded as a small whole number": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"max": float64(10)}),
			int32(50), "field_max",
		},
		"a number the caller decoded as a wide whole number": {
			bounded(t, "rating", content.FieldKindNumber, map[string]any{"max": float64(10)}),
			int64(50), "field_max",
		},
		"a choice outside its list": {
			bounded(t, "style", content.FieldKindChoice, map[string]any{
				"choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}),
			"porter", "field_choice",
		},
		"a choice member outside its list": {
			bounded(t, "styles", content.FieldKindChoice, map[string]any{
				"multiple": true,
				"choices":  []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}),
			[]any{"ipa", "porter"}, "field_choice",
		},
		"an email that is not one": {
			bounded(t, "contact", content.FieldKindText, map[string]any{"variant": "email"}),
			"184467235", "field_format",
		},
		"a url that is not one": {
			bounded(t, "homepage", content.FieldKindText, map[string]any{"variant": "url"}),
			"gophenberg", "field_format",
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
		"a day carrying instructions, which set no bound": {
			bounded(t, "sold-on", content.FieldKindDate,
				map[string]any{"instructions": "The day it sold."}), "2026-08-29",
		},
		"a check carrying instructions, which set no bound": {
			bounded(t, "boxed", content.FieldKindBoolean,
				map[string]any{"instructions": "Whether it ships boxed."}), true,
		},
		"a picture carrying instructions, which set no bound": {
			bounded(t, "cover", content.FieldKindMedia,
				map[string]any{"instructions": "Pick a wide one."}), float64(7),
		},
		"a listed choice": {
			bounded(t, "style", content.FieldKindChoice, map[string]any{
				"choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}), "ipa",
		},
		"a stranger choice when custom is allowed": {
			bounded(t, "style", content.FieldKindChoice, map[string]any{
				"allow_custom": true,
				"choices":      []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}), "porter",
		},
		"a bare choice field takes any word": {
			bounded(t, "style", content.FieldKindChoice, nil), "porter",
		},
		"a multiple choice all listed": {
			bounded(t, "styles", content.FieldKindChoice, map[string]any{
				"multiple": true,
				"choices": []any{
					map[string]any{"value": "ipa", "label": "IPA"},
					map[string]any{"value": "stout", "label": "Stout"},
				},
			}), []any{"ipa", "stout"},
		},
		"an email that is one": {
			bounded(t, "contact", content.FieldKindText,
				map[string]any{"variant": "email"}), "maria@example.com",
		},
		"a url that is one": {
			bounded(t, "homepage", content.FieldKindText,
				map[string]any{"variant": "url"}), "https://example.com/beers",
		},
		"a textarea holds any words": {
			bounded(t, "notes", content.FieldKindText,
				map[string]any{"variant": "textarea"}), "several lines of notes",
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

func TestParkedValuesSkipMembershipAndFormat(t *testing.T) {
	t.Parallel()

	style := bounded(t, "style", content.FieldKindChoice, map[string]any{
		"choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
	})
	stranger := content.Values{"style": "porter"}

	if err := stranger.ValidateShape([]content.Field{style}); err != nil {
		t.Errorf("ValidateShape() error = %v, want the buffer to park a stranger choice", err)
	}

	contact := bounded(t, "contact", content.FieldKindText, map[string]any{"variant": "email"})
	unfinished := content.Values{"contact": "maria@"}

	if err := unfinished.ValidateShape([]content.Field{contact}); err != nil {
		t.Errorf("ValidateShape() error = %v, want the buffer to park a half typed email", err)
	}
}
