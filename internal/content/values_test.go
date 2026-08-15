// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// declared returns the field definitions a values test validates against.
func declared(t *testing.T) []content.Field {
	t.Helper()
	kinds := map[string]content.FieldKind{
		"color":   content.FieldKindText,
		"doors":   content.FieldKindNumber,
		"boxed":   content.FieldKindBoolean,
		"sold-on": content.FieldKindDate,
		"cover":   content.FieldKindMedia,
	}
	fields := make([]content.Field, 0, len(kinds))
	for key, kind := range kinds {
		built, err := content.NewField(content.Field{
			TypeKey: "post", Key: key, Label: strings.ToUpper(key), Kind: kind,
		})
		if err != nil {
			t.Fatalf("NewField(%q) error = %v, want nil", key, err)
		}
		fields = append(fields, built)
	}
	return fields
}

func TestValuesAcceptEveryScalarShape(t *testing.T) {
	t.Parallel()

	held := content.Values{
		"color":   "red",
		"doors":   float64(4),
		"boxed":   true,
		"sold-on": "2026-08-14",
		"cover":   float64(12),
	}

	if err := held.Validate(declared(t)); err != nil {
		t.Fatalf("Validate() error = %v, want the shapes accepted", err)
	}
}

func TestValuesAcceptAWholeNumberWrittenInGo(t *testing.T) {
	t.Parallel()

	held := content.Values{"doors": 4}

	if err := held.Validate(declared(t)); err != nil {
		t.Fatalf("Validate() error = %v, want an int accepted as a number", err)
	}
}

func TestValuesRefuseAnUnknownKey(t *testing.T) {
	t.Parallel()

	held := content.Values{"finish": "matte"}

	err := held.Validate(declared(t))

	if !errors.Is(err, content.ErrUnknownField) {
		t.Fatalf("Validate() error = %v, want %v", err, content.ErrUnknownField)
	}
	if !strings.Contains(err.Error(), "finish") {
		t.Errorf("Validate() error = %q, want the key named", err)
	}
}

func TestValuesRefuseTheWrongShape(t *testing.T) {
	t.Parallel()

	tests := map[string]content.Values{
		"a word where a number belongs":   {"doors": "many"},
		"a number where a word belongs":   {"color": float64(3)},
		"a word where a flag belongs":     {"boxed": "yes"},
		"a word that is not a date":       {"sold-on": "the fourteenth"},
		"a word where a media id belongs": {"cover": "12"},
	}
	for name, held := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := held.Validate(declared(t))

			if !errors.Is(err, content.ErrFieldShape) {
				t.Fatalf("Validate() error = %v, want %v", err, content.ErrFieldShape)
			}
		})
	}
}

func TestValuesNameTheFieldThatHoldsTheWrongShape(t *testing.T) {
	t.Parallel()

	held := content.Values{"doors": "many"}

	err := held.Validate(declared(t))

	if !strings.Contains(err.Error(), "doors") {
		t.Errorf("Validate() error = %q, want the field named", err)
	}
}

func TestValuesAcceptAClearedKey(t *testing.T) {
	t.Parallel()

	held := content.Values{"color": nil}

	if err := held.Validate(declared(t)); err != nil {
		t.Fatalf("Validate() error = %v, want a cleared value accepted", err)
	}
}

func TestValuesRefuseTargetsAmongTheScalars(t *testing.T) {
	t.Parallel()

	relation, err := content.NewField(content.Field{
		TypeKey: "post", Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	held := content.Values{"categories": []any{"019fb000-0000-7000-8000-000000000001"}}

	err = held.Validate([]content.Field{relation})

	if !errors.Is(err, content.ErrFieldShape) {
		t.Fatalf("Validate() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestValuesMergeSetsAndClears(t *testing.T) {
	t.Parallel()

	stored := content.Values{"color": "red", "doors": float64(4)}

	merged := stored.Merge(content.Values{"color": "blue", "boxed": true, "doors": nil})

	want := content.Values{"color": "blue", "boxed": true}
	if len(merged) != len(want) {
		t.Fatalf("Merge() = %v, want %v", merged, want)
	}
	for key, value := range want {
		if merged[key] != value {
			t.Errorf("Merge()[%q] = %v, want %v", key, merged[key], value)
		}
	}
}

func TestValuesMergeLeavesTheStoredValuesAlone(t *testing.T) {
	t.Parallel()

	stored := content.Values{"color": "red"}

	stored.Merge(content.Values{"color": "blue"})

	if stored["color"] != "red" {
		t.Errorf("the stored values hold %v, want the merge to have left them alone", stored["color"])
	}
}

func TestValuesMergeOverNothingHolds(t *testing.T) {
	t.Parallel()

	var stored content.Values

	merged := stored.Merge(content.Values{"color": "blue"})

	if merged["color"] != "blue" {
		t.Errorf("Merge() = %v, want the patch held", merged)
	}
}

func TestValuesFilledPassesWhenNothingIsRequired(t *testing.T) {
	t.Parallel()

	held := content.Values{}

	if err := content.Filled(held, nil, declared(t)); err != nil {
		t.Fatalf("Filled() error = %v, want nil", err)
	}
}

func TestValuesFilledRefusesAnEmptyRequiredField(t *testing.T) {
	t.Parallel()

	required := requiredColor(t)
	tests := map[string]content.Values{
		"the key is absent": {},
		"the key is null":   {"color": nil},
		"the key is empty":  {"color": ""},
	}
	for name, held := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.Filled(held, nil, []content.Field{required})

			if !errors.Is(err, content.ErrFieldRequired) {
				t.Fatalf("Filled() error = %v, want %v", err, content.ErrFieldRequired)
			}
			if !strings.Contains(err.Error(), "color") {
				t.Errorf("Filled() error = %q, want the field named", err)
			}
		})
	}
}

func TestValuesFilledAcceptsAFalseFlag(t *testing.T) {
	t.Parallel()

	flag, err := content.NewField(content.Field{
		TypeKey: "post", Key: "boxed", Label: "Boxed",
		Kind: content.FieldKindBoolean, Required: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	held := content.Values{"boxed": false}

	if err := content.Filled(held, nil, []content.Field{flag}); err != nil {
		t.Fatalf("Filled() error = %v, want a false flag to count as filled", err)
	}
}

func TestValuesFilledAcceptsAZero(t *testing.T) {
	t.Parallel()

	count, err := content.NewField(content.Field{
		TypeKey: "post", Key: "doors", Label: "Doors",
		Kind: content.FieldKindNumber, Required: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	held := content.Values{"doors": float64(0)}

	if err := content.Filled(held, nil, []content.Field{count}); err != nil {
		t.Fatalf("Filled() error = %v, want a zero to count as filled", err)
	}
}

// requiredColor returns a required text field to test the publish gate against.
func requiredColor(t *testing.T) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: "post", Key: "color", Label: "Color",
		Kind: content.FieldKindText, Required: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	return built
}
