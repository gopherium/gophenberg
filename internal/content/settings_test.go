// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestEveryKindAcceptsEmptySettings(t *testing.T) {
	t.Parallel()

	kinds := []content.FieldKind{
		content.FieldKindText, content.FieldKindNumber, content.FieldKindBoolean,
		content.FieldKindDate, content.FieldKindMedia, content.FieldKindRelation,
	}
	for _, kind := range kinds {
		if err := content.ValidateSettings(kind, nil); err != nil {
			t.Errorf("ValidateSettings(%s, nil) error = %v, want nil", kind, err)
		}
		if err := content.ValidateSettings(kind, map[string]any{}); err != nil {
			t.Errorf("ValidateSettings(%s, empty) error = %v, want nil", kind, err)
		}
	}
}

func TestEachKindAcceptsItsOwnSettings(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		kind     content.FieldKind
		settings map[string]any
	}{
		"text takes its vocabulary": {content.FieldKindText, map[string]any{
			"default": "unnamed", "instructions": "Say who wrote it.",
			"placeholder": "Maria Perez", "maxlength": float64(80),
		}},
		"number takes its vocabulary": {content.FieldKindNumber, map[string]any{
			"default": float64(5), "instructions": "One to ten.",
			"placeholder": "5", "min": float64(1), "max": float64(10), "step": float64(0.5),
		}},
		"boolean takes its vocabulary": {content.FieldKindBoolean, map[string]any{
			"default": true, "instructions": "Tick when it is in stock.",
		}},
		"date takes instructions": {content.FieldKindDate, map[string]any{
			"instructions": "The day it was sold.",
		}},
		"media takes instructions": {content.FieldKindMedia, map[string]any{
			"instructions": "A cover image.",
		}},
		"relation takes instructions": {content.FieldKindRelation, map[string]any{
			"instructions": "The categories it files under.",
		}},
		"whole numbers arrive as ints too": {content.FieldKindNumber, map[string]any{
			"min": 1, "max": 10, "default": 5,
		}},
		"maxlength arrives as an int too": {content.FieldKindText, map[string]any{
			"maxlength": 80,
		}},
		"choice takes its vocabulary": {content.FieldKindChoice, map[string]any{
			"instructions": "The style it pours as.", "default": "ipa",
			"choices": []any{
				map[string]any{"value": "ipa", "label": "IPA"},
				map[string]any{"value": "stout", "label": "Stout"},
			},
			"multiple": false, "presentation": "select", "allow_null": true, "allow_custom": false,
		}},
		"choice multiple takes a list default": {content.FieldKindChoice, map[string]any{
			"multiple": true, "default": []any{"ipa", "stout"},
			"choices": []any{
				map[string]any{"value": "ipa", "label": "IPA"},
				map[string]any{"value": "stout", "label": "Stout"},
			},
		}},
		"choice takes an empty choices list": {content.FieldKindChoice, map[string]any{
			"choices": []any{},
		}},
		"choice takes a stranger default when custom is allowed": {content.FieldKindChoice, map[string]any{
			"allow_custom": true, "default": "porter",
			"choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
		}},
		"text takes the email variant": {content.FieldKindText, map[string]any{
			"variant": "email",
		}},
		"text takes the url variant": {content.FieldKindText, map[string]any{
			"variant": "url",
		}},
		"text takes the textarea variant": {content.FieldKindText, map[string]any{
			"variant": "textarea", "maxlength": 500,
		}},
		"number takes the range presentation": {content.FieldKindNumber, map[string]any{
			"presentation": "range", "min": 1, "max": 10,
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := content.ValidateSettings(test.kind, test.settings); err != nil {
				t.Errorf("ValidateSettings() error = %v, want nil", err)
			}
		})
	}
}

func TestAKindRefusesASettingItDoesNotTake(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		kind     content.FieldKind
		settings map[string]any
	}{
		"a date takes no placeholder":  {content.FieldKindDate, map[string]any{"placeholder": "soon"}},
		"a boolean takes no min":       {content.FieldKindBoolean, map[string]any{"min": float64(1)}},
		"a text field takes no step":   {content.FieldKindText, map[string]any{"step": float64(1)}},
		"a media field has no default": {content.FieldKindMedia, map[string]any{"default": float64(3)}},
		"a made up name is refused":    {content.FieldKindNumber, map[string]any{"banana": true}},
		"a text field takes no choices": {content.FieldKindText, map[string]any{
			"choices": []any{map[string]any{"value": "a", "label": "A"}},
		}},
		"a number takes no variant":   {content.FieldKindNumber, map[string]any{"variant": "range"}},
		"a choice takes no maxlength": {content.FieldKindChoice, map[string]any{"maxlength": float64(3)}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.ValidateSettings(test.kind, test.settings)

			if !errors.Is(err, content.ErrSettingUnknown) {
				t.Fatalf("ValidateSettings() error = %v, want %v", err, content.ErrSettingUnknown)
			}
			if code, _ := content.CodeOf(err); code != "setting_unknown" {
				t.Errorf("code = %q, want setting_unknown", code)
			}
		})
	}
}

func TestASettingRefusesAValueOfTheWrongShape(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		kind     content.FieldKind
		settings map[string]any
	}{
		"instructions take a string":     {content.FieldKindText, map[string]any{"instructions": float64(4)}},
		"placeholder takes a string":     {content.FieldKindText, map[string]any{"placeholder": true}},
		"maxlength takes a whole number": {content.FieldKindText, map[string]any{"maxlength": float64(3.5)}},
		"maxlength stays above zero":     {content.FieldKindText, map[string]any{"maxlength": float64(0)}},
		"maxlength takes no words":       {content.FieldKindText, map[string]any{"maxlength": "eighty"}},
		"min takes a number":             {content.FieldKindNumber, map[string]any{"min": "low"}},
		"max takes a number":             {content.FieldKindNumber, map[string]any{"max": true}},
		"step stays above zero":          {content.FieldKindNumber, map[string]any{"step": float64(0)}},
		"step never goes negative":       {content.FieldKindNumber, map[string]any{"step": float64(-1)}},
		"a text default is a string":     {content.FieldKindText, map[string]any{"default": float64(5)}},
		"a number default is a number":   {content.FieldKindNumber, map[string]any{"default": "five"}},
		"a boolean default is a bool":    {content.FieldKindBoolean, map[string]any{"default": "yes"}},
		"a variant is one of its words":  {content.FieldKindText, map[string]any{"variant": "phone"}},
		"a number presentation is range": {content.FieldKindNumber, map[string]any{"presentation": "dial"}},
		"a choice presentation is one of four": {content.FieldKindChoice, map[string]any{
			"presentation": "carousel",
		}},
		"choices come as a list": {content.FieldKindChoice, map[string]any{"choices": "ipa, stout"}},
		"a choice pair needs its label": {content.FieldKindChoice, map[string]any{
			"choices": []any{map[string]any{"value": "ipa"}},
		}},
		"a choice pair needs its value": {content.FieldKindChoice, map[string]any{
			"choices": []any{map[string]any{"label": "IPA"}},
		}},
		"a choice pair takes nothing else": {content.FieldKindChoice, map[string]any{
			"choices": []any{map[string]any{"value": "ipa", "label": "IPA", "color": "amber"}},
		}},
		"a choice pair holds words": {content.FieldKindChoice, map[string]any{
			"choices": []any{map[string]any{"value": float64(1), "label": "One"}},
		}},
		"a choice pair value is never empty": {content.FieldKindChoice, map[string]any{
			"choices": []any{map[string]any{"value": "", "label": "Empty"}},
		}},
		"multiple takes a bool":     {content.FieldKindChoice, map[string]any{"multiple": "yes"}},
		"allow_null takes a bool":   {content.FieldKindChoice, map[string]any{"allow_null": "maybe"}},
		"allow_custom takes a bool": {content.FieldKindChoice, map[string]any{"allow_custom": 1}},
		"a choice default holds words": {content.FieldKindChoice, map[string]any{
			"default": []any{"ipa", float64(2)},
		}},
		"a choice default is words not a number": {content.FieldKindChoice, map[string]any{
			"default": float64(2),
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.ValidateSettings(test.kind, test.settings)

			if !errors.Is(err, content.ErrSettingShape) {
				t.Fatalf("ValidateSettings() error = %v, want %v", err, content.ErrSettingShape)
			}
			if code, _ := content.CodeOf(err); code != "setting_shape" {
				t.Errorf("code = %q, want setting_shape", code)
			}
		})
	}
}

func TestSettingsAgreeWithEachOther(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		kind     content.FieldKind
		settings map[string]any
		agree    bool
	}{
		"min stays below max": {
			content.FieldKindNumber, map[string]any{"min": float64(10), "max": float64(5)}, false,
		},
		"a default respects max": {
			content.FieldKindNumber, map[string]any{"default": float64(50), "max": float64(10)}, false,
		},
		"a default respects min": {
			content.FieldKindNumber, map[string]any{"default": float64(1), "min": float64(5)}, false,
		},
		"a text default fits maxlength": {
			content.FieldKindText, map[string]any{"default": "too long", "maxlength": float64(3)}, false,
		},
		"a default may sit exactly on min": {
			content.FieldKindNumber, map[string]any{"default": float64(5), "min": float64(5)}, true,
		},
		"a default may sit exactly on max": {
			content.FieldKindNumber, map[string]any{"default": float64(10), "max": float64(10)}, true,
		},
		"min may equal max": {
			content.FieldKindNumber, map[string]any{"min": float64(7), "max": float64(7)}, true,
		},
		"a default may fill maxlength": {
			content.FieldKindText, map[string]any{"default": "abc", "maxlength": float64(3)}, true,
		},
		"an email default has to read as one": {
			content.FieldKindText, map[string]any{"variant": "email", "default": "nobody"}, false,
		},
		"an email default that reads as one stands": {
			content.FieldKindText,
			map[string]any{"variant": "email", "default": "maria@example.com"}, true,
		},
		"a web address default has to read as one": {
			content.FieldKindText, map[string]any{"variant": "url", "default": "example.com"}, false,
		},
		"a text area default is never a format": {
			content.FieldKindText, map[string]any{"variant": "textarea", "default": "anything"}, true,
		},
		"a choice default sits among its choices": {
			content.FieldKindChoice, map[string]any{
				"default": "ipa", "choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}, true,
		},
		"a choice default outside its choices disagrees": {
			content.FieldKindChoice, map[string]any{
				"default": "porter", "choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}, false,
		},
		"a custom choice default may wander": {
			content.FieldKindChoice, map[string]any{
				"allow_custom": true, "default": "porter",
				"choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}, true,
		},
		"a single default never comes as a list": {
			content.FieldKindChoice, map[string]any{
				"default": []any{"ipa"}, "choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}, false,
		},
		"a multiple default always comes as a list": {
			content.FieldKindChoice, map[string]any{
				"multiple": true, "default": "ipa",
				"choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}, false,
		},
		"a multiple default checks every member": {
			content.FieldKindChoice, map[string]any{
				"multiple": true, "default": []any{"ipa", "porter"},
				"choices": []any{map[string]any{"value": "ipa", "label": "IPA"}},
			}, false,
		},
		"a field listing nothing takes any default": {
			content.FieldKindChoice, map[string]any{"default": "ipa", "choices": []any{}}, true,
		},
		"a default with no choices listed stands": {
			content.FieldKindChoice, map[string]any{"default": "ipa"}, true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.ValidateSettings(test.kind, test.settings)

			if test.agree {
				if err != nil {
					t.Fatalf("ValidateSettings() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, content.ErrSettingBounds) {
				t.Fatalf("ValidateSettings() error = %v, want %v", err, content.ErrSettingBounds)
			}
			if code, _ := content.CodeOf(err); code != "setting_bounds" {
				t.Errorf("code = %q, want setting_bounds", code)
			}
		})
	}
}

func TestNewFieldCarriesAndValidatesSettings(t *testing.T) {
	t.Parallel()

	carried, err := content.NewField(content.Field{
		TypeKey: "post", Key: "rating", Label: "Rating", Kind: content.FieldKindNumber,
		Settings: map[string]any{"min": float64(1), "max": float64(10)},
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if carried.Settings["max"] != float64(10) {
		t.Errorf("Settings = %v, want the bounds carried through", carried.Settings)
	}

	_, err = content.NewField(content.Field{
		TypeKey: "post", Key: "rating", Label: "Rating", Kind: content.FieldKindNumber,
		Settings: map[string]any{"maxlength": float64(80)},
	})

	if !errors.Is(err, content.ErrSettingUnknown) {
		t.Fatalf("NewField() error = %v, want %v", err, content.ErrSettingUnknown)
	}
}
