// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"math"
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

// shaped returns a field of the kind, holding many when asked, carrying the settings.
func shaped(t *testing.T, kind content.FieldKind, many bool, settings map[string]any) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: "post", Key: "held", Label: "Held", Kind: kind, Many: many, Settings: settings,
	})
	if err != nil {
		t.Fatalf("NewField(%s) error = %v, want nil", kind, err)
	}
	return built
}

func TestValuesShapeTheChoiceKindAndAManyMedia(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		field content.Field
		value any
		holds bool
	}{
		"a choice holds a word": {
			shaped(t, content.FieldKindChoice, false, nil), "ipa", true,
		},
		"a choice refuses a list": {
			shaped(t, content.FieldKindChoice, false, nil), []any{"ipa"}, false,
		},
		"a choice refuses a number": {
			shaped(t, content.FieldKindChoice, false, nil), float64(2), false,
		},
		"a multiple choice holds a list of words": {
			shaped(t, content.FieldKindChoice, false, map[string]any{"multiple": true}),
			[]any{"ipa", "stout"}, true,
		},
		"a multiple choice refuses a bare word": {
			shaped(t, content.FieldKindChoice, false, map[string]any{"multiple": true}), "ipa", false,
		},
		"a multiple choice refuses a listed number": {
			shaped(t, content.FieldKindChoice, false, map[string]any{"multiple": true}),
			[]any{"ipa", float64(2)}, false,
		},
		"a many media holds a list of numbers": {
			shaped(t, content.FieldKindMedia, true, nil), []any{float64(1), float64(2)}, true,
		},
		"a many media refuses one bare number": {
			shaped(t, content.FieldKindMedia, true, nil), float64(3), false,
		},
		"a many media refuses a listed word": {
			shaped(t, content.FieldKindMedia, true, nil), []any{"cover"}, false,
		},
		"a single media still refuses a list": {
			shaped(t, content.FieldKindMedia, false, nil), []any{float64(1)}, false,
		},
		"a many media refuses the same item twice": {
			shaped(t, content.FieldKindMedia, true, nil), []any{float64(1), float64(1)}, false,
		},
		"a many media refuses the same item written two ways": {
			shaped(t, content.FieldKindMedia, true, nil), []any{float64(7), int64(7)}, false,
		},
		"a media refuses a part of an item": {
			shaped(t, content.FieldKindMedia, false, nil), float64(1.5), false,
		},
		"a media refuses an item below one": {
			shaped(t, content.FieldKindMedia, false, nil), float64(0), false,
		},
		"a media refuses an item before the first": {
			shaped(t, content.FieldKindMedia, false, nil), float64(-3), false,
		},
		"a media refuses an item past the last one storable": {
			shaped(t, content.FieldKindMedia, false, nil), float64(9223372036854775808), false,
		},
		"a media holds the largest item that stores": {
			shaped(t, content.FieldKindMedia, false, nil), float64(9223372036854774784), true,
		},
		"a media holds the last identity a caller writes whole": {
			shaped(t, content.FieldKindMedia, false, nil), int64(math.MaxInt64), true,
		},
		"a media refuses a whole identity before the first": {
			shaped(t, content.FieldKindMedia, false, nil), int64(0), false,
		},
		"a media holds an identity a caller wrote plainly": {
			shaped(t, content.FieldKindMedia, false, nil), 7, true,
		},
		"a media holds an identity a caller wrote narrowly": {
			shaped(t, content.FieldKindMedia, false, nil), int32(7), true,
		},
		"a media holds an identity that arrived narrow": {
			shaped(t, content.FieldKindMedia, false, nil), float32(7), true,
		},
		"a media refuses a plain identity before the first": {
			shaped(t, content.FieldKindMedia, false, nil), -1, false,
		},
		"a media refuses a narrow identity before the first": {
			shaped(t, content.FieldKindMedia, false, nil), int32(0), false,
		},
		"a many media refuses a part of an item": {
			shaped(t, content.FieldKindMedia, true, nil), []any{float64(1), float64(2.5)}, false,
		},
		"a number still holds a part of one": {
			shaped(t, content.FieldKindNumber, false, nil), float64(1.5), true,
		},
		"a multiple choice refuses the same answer twice": {
			shaped(t, content.FieldKindChoice, false, map[string]any{"multiple": true}),
			[]any{"ipa", "ipa"}, false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := content.Values{test.field.Key: test.value}

			err := held.Validate([]content.Field{test.field})

			if test.holds {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, content.ErrFieldShape) {
				t.Fatalf("Validate() error = %v, want %v", err, content.ErrFieldShape)
			}
		})
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

func TestValuesFilledRefusesAnEmptiedRequiredList(t *testing.T) {
	t.Parallel()

	gallery, err := content.NewField(content.Field{
		TypeKey: "post", Key: "gallery", Label: "Gallery",
		Kind: content.FieldKindMedia, Many: true, Required: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	held := content.Values{"gallery": []any{}}

	if err := content.Filled(held, nil, []content.Field{gallery}); !errors.Is(err, content.ErrFieldRequired) {
		t.Fatalf("Filled() error = %v, want %v, an emptied list holds nothing", err, content.ErrFieldRequired)
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

func TestValuesRefuseAKindTheCMSDoesNotHold(t *testing.T) {
	t.Parallel()

	held := content.Values{"oddity": "anything"}
	fields := []content.Field{{TypeKey: content.TypePost, Key: "oddity", Label: "Oddity", Kind: "sculpture"}}

	err := held.Validate(fields)

	if !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestADateFieldRefusesAValueThatIsNotWritten(t *testing.T) {
	t.Parallel()

	held := content.Values{"published": 20260823}
	fields := []content.Field{
		{TypeKey: content.TypePost, Key: "published", Label: "Published", Kind: content.FieldKindDate},
	}

	err := held.Validate(fields)

	if !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrFieldShape)
	}
}
