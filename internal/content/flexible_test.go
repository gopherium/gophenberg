// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// layout returns a layout carrying the sub fields, built the way the registry builds one under a flexible.
func layout(t *testing.T, key string, settings map[string]any, subs ...content.Field) content.Field {
	t.Helper()
	built, err := content.NewSubField(content.Field{
		Key: key, Label: key, Kind: content.FieldKindLayout, Settings: settings,
	}, content.FieldKindFlexible)
	if err != nil {
		t.Fatalf("NewSubField(layout) error = %v, want nil", err)
	}
	built.Fields = subs
	return built
}

// features returns a flexible field holding a hero layout and a quote layout, both declaring a title.
func features(t *testing.T, settings map[string]any, heroSettings map[string]any) content.Field {
	t.Helper()
	return holding(t, content.FieldKindFlexible, "features", settings,
		layout(t, "hero", heroSettings,
			leaf(t, content.FieldKindText, "title", nil),
			leaf(t, content.FieldKindMedia, "image", nil)),
		layout(t, "quote", nil,
			leaf(t, content.FieldKindText, "title", nil),
			leaf(t, content.FieldKindText, "author", nil)))
}

// codeOf returns the code the refusal carries, empty when it carries none.
func codeOf(err error) string {
	code, _ := content.CodeOf(err)
	return code
}

func TestNewFieldAcceptsTheFlexibleKind(t *testing.T) {
	t.Parallel()

	built, err := content.NewField(content.Field{
		TypeKey: "post", Key: "features", Label: "Features", Kind: content.FieldKindFlexible,
	})

	if err != nil {
		t.Fatalf("NewField(flexible) error = %v, want nil", err)
	}
	if !built.Kind.Holds() {
		t.Errorf("Holds() = false, want a flexible to hold sub fields")
	}
}

func TestNewFieldRefusesALayoutStandingAlone(t *testing.T) {
	t.Parallel()

	_, err := content.NewField(content.Field{
		TypeKey: "post", Key: "hero", Label: "Hero", Kind: content.FieldKindLayout,
	})

	if !errors.Is(err, content.ErrFieldShape) || codeOf(err) != "field_layout_alone" {
		t.Errorf("NewField(layout) error = %v, want field_layout_alone", err)
	}
}

func TestARequiredLayoutIsRefused(t *testing.T) {
	t.Parallel()

	_, err := content.NewSubField(content.Field{
		Key: "hero", Label: "Hero", Kind: content.FieldKindLayout, Required: true,
	}, content.FieldKindFlexible)

	if !errors.Is(err, content.ErrFieldShape) || codeOf(err) != "field_never_required" {
		t.Errorf("NewSubField(required layout) error = %v, want field_never_required", err)
	}
}

func TestNewSubFieldTakesALayoutUnderAFlexible(t *testing.T) {
	t.Parallel()

	built, err := content.NewSubField(content.Field{
		Key: "hero", Label: "Hero", Kind: content.FieldKindLayout,
	}, content.FieldKindFlexible)

	if err != nil {
		t.Fatalf("NewSubField(layout under flexible) error = %v, want nil", err)
	}
	if !built.Kind.Holds() {
		t.Errorf("Holds() = false, want a layout to hold sub fields")
	}
}

func TestNewSubFieldRefusesALayoutUnderAnotherContainer(t *testing.T) {
	t.Parallel()

	for _, parent := range []content.FieldKind{content.FieldKindRepeater, content.FieldKindSection} {
		_, err := content.NewSubField(content.Field{
			Key: "hero", Label: "Hero", Kind: content.FieldKindLayout,
		}, parent)

		if !errors.Is(err, content.ErrFieldShape) || codeOf(err) != "field_layout_outside" {
			t.Errorf("NewSubField(layout under %s) error = %v, want field_layout_outside", parent, err)
		}
	}
}

func TestNewSubFieldRefusesAnythingButALayoutUnderAFlexible(t *testing.T) {
	t.Parallel()

	_, err := content.NewSubField(content.Field{
		Key: "title", Label: "Title", Kind: content.FieldKindText,
	}, content.FieldKindFlexible)

	if !errors.Is(err, content.ErrFieldShape) || codeOf(err) != "field_flexible_takes_layouts" {
		t.Errorf("NewSubField(text under flexible) error = %v, want field_flexible_takes_layouts", err)
	}
}

func TestLayoutSettingsTakeRowBounds(t *testing.T) {
	t.Parallel()

	if err := content.ValidateSettings(content.FieldKindLayout, map[string]any{
		"min": float64(1), "max": float64(3),
	}); err != nil {
		t.Errorf("ValidateSettings(layout bounds) error = %v, want nil", err)
	}
	if err := content.ValidateSettings(content.FieldKindLayout, map[string]any{
		"min": float64(3), "max": float64(1),
	}); err == nil {
		t.Error("ValidateSettings(min above max) error = nil, want the bounds refused")
	}
	if err := content.ValidateSettings(content.FieldKindFlexible, map[string]any{
		"min": float64(1),
	}); err != nil {
		t.Errorf("ValidateSettings(flexible min) error = %v, want nil", err)
	}
}

func TestFlexibleTakesRowsOfDifferentLayouts(t *testing.T) {
	t.Parallel()

	held := content.Values{"features": []any{
		map[string]any{"hero": map[string]any{"title": "Welcome", "image": float64(4)}},
		map[string]any{"quote": map[string]any{"title": "They said", "author": "Maria Perez"}},
		map[string]any{"hero": map[string]any{"title": "Again"}},
	}}

	if err := held.Validate([]content.Field{features(t, nil, nil)}); err != nil {
		t.Errorf("Validate() error = %v, want the rows taken", err)
	}
}

func TestFlexibleRefusesARowThatIsNotAnObject(t *testing.T) {
	t.Parallel()

	held := content.Values{"features": []any{"hero"}}

	if err := held.Validate([]content.Field{features(t, nil, nil)}); !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestFlexibleRefusesARowNamingAnUnknownLayout(t *testing.T) {
	t.Parallel()

	held := content.Values{"features": []any{
		map[string]any{"banner": map[string]any{"title": "Nope"}},
	}}

	if err := held.Validate([]content.Field{features(t, nil, nil)}); !errors.Is(err, content.ErrUnknownField) {
		t.Errorf("Validate() error = %v, want %v", err, content.ErrUnknownField)
	}
}

func TestFlexibleRefusesARowNamingNoLayoutAtPublish(t *testing.T) {
	t.Parallel()

	held := content.Values{"features": []any{map[string]any{}}}
	fields := []content.Field{features(t, nil, nil)}

	if err := held.Validate(fields); codeOf(err) != "field_layout_missing" {
		t.Errorf("Validate() error = %v, want field_layout_missing", err)
	}
	if err := held.ValidateShape(fields); err != nil {
		t.Errorf("ValidateShape() error = %v, want a half built row parked", err)
	}
}

func TestFlexibleRefusesARowNamingTwoLayouts(t *testing.T) {
	t.Parallel()

	held := content.Values{"features": []any{
		map[string]any{"hero": map[string]any{}, "quote": map[string]any{}},
	}}
	fields := []content.Field{features(t, nil, nil)}

	if err := held.Validate(fields); codeOf(err) != "field_layout_several" {
		t.Errorf("Validate() error = %v, want field_layout_several", err)
	}
	if err := held.ValidateShape(fields); codeOf(err) != "field_layout_several" {
		t.Errorf("ValidateShape() error = %v, want the malformed row refused early", err)
	}
}

func TestFlexibleRefusesASubFieldTheLayoutDoesNotDeclare(t *testing.T) {
	t.Parallel()

	held := content.Values{"features": []any{
		map[string]any{"quote": map[string]any{"image": float64(4)}},
	}}

	if err := held.Validate([]content.Field{features(t, nil, nil)}); !errors.Is(err, content.ErrUnknownField) {
		t.Errorf("Validate() error = %v, want the image refused under a quote", err)
	}
}

func TestFlexibleRefusesFewerRowsOfALayoutThanItAsksFor(t *testing.T) {
	t.Parallel()

	held := content.Values{"features": []any{
		map[string]any{"quote": map[string]any{"title": "Only one"}},
	}}
	fields := []content.Field{features(t, nil, map[string]any{"min": float64(1)})}

	err := held.Validate(fields)

	if !errors.Is(err, content.ErrFieldBounds) || codeOf(err) != "field_rows_min" {
		t.Errorf("Validate() error = %v, want the hero minimum refused", err)
	}
	if details, _ := content.DetailsOf(err); details["field"] != "hero" {
		t.Errorf("details = %v, want the layout named", details)
	}
}

func TestFlexibleRefusesMoreRowsOfALayoutThanItTakes(t *testing.T) {
	t.Parallel()

	held := content.Values{"features": []any{
		map[string]any{"hero": map[string]any{"title": "One"}},
		map[string]any{"hero": map[string]any{"title": "Two"}},
	}}
	fields := []content.Field{features(t, nil, map[string]any{"max": float64(1)})}

	err := held.Validate(fields)

	if !errors.Is(err, content.ErrFieldBounds) || codeOf(err) != "field_rows_max" {
		t.Errorf("Validate() error = %v, want the hero maximum refused", err)
	}
}

func TestFlexibleCountsEveryRowAgainstItsOwnBounds(t *testing.T) {
	t.Parallel()

	held := content.Values{"features": []any{
		map[string]any{"hero": map[string]any{"title": "One"}},
		map[string]any{"quote": map[string]any{"title": "Two"}},
	}}
	fields := []content.Field{features(t, map[string]any{"max": float64(1)}, nil)}

	err := held.Validate(fields)

	if codeOf(err) != "field_rows_max" {
		t.Errorf("Validate() error = %v, want the flexible maximum refused across layouts", err)
	}
	if details, _ := content.DetailsOf(err); details["field"] != "features" {
		t.Errorf("details = %v, want the flexible named", details)
	}
}

func TestFlexibleRowHoldsAContainerOfItsOwn(t *testing.T) {
	t.Parallel()

	crew := holding(t, content.FieldKindRepeater, "crew", nil, leaf(t, content.FieldKindText, "name", nil))
	field := holding(t, content.FieldKindFlexible, "features", nil, layout(t, "team", nil, crew))
	held := content.Values{"features": []any{
		map[string]any{"team": map[string]any{"crew": []any{map[string]any{"name": "Maria Perez"}}}},
	}}

	if err := held.Validate([]content.Field{field}); err != nil {
		t.Errorf("Validate() error = %v, want the nested rows taken", err)
	}
}
