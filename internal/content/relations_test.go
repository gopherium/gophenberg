// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// relatable returns a text field and a relation field to check a patch against.
func relatable(t *testing.T, many bool) []content.Field {
	t.Helper()
	color, err := content.NewField(content.Field{
		TypeKey: "post", Key: "color", Label: "Color", Kind: content.FieldKindText,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	categories, err := content.NewField(content.Field{
		TypeKey: "post", Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: many,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	return []content.Field{color, categories}
}

func TestValuesCarryTargetsBesideTheScalars(t *testing.T) {
	t.Parallel()

	first, second := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	patch := content.Values{
		"color":      "red",
		"categories": []any{first.String(), second.String()},
	}

	if err := patch.Validate(relatable(t, true)); err != nil {
		t.Fatalf("Validate() error = %v, want the targets carried beside the text value", err)
	}
}

func TestValuesReadANullRelationAsCleared(t *testing.T) {
	t.Parallel()

	patch := content.Values{"categories": nil}

	if err := patch.Validate(relatable(t, true)); err != nil {
		t.Fatalf("Validate() error = %v, want a cleared relation accepted", err)
	}
}

func TestValuesRefuseATargetThatIsNotAList(t *testing.T) {
	t.Parallel()

	err := content.Values{"categories": "news"}.Validate(relatable(t, true))

	if !errors.Is(err, content.ErrFieldShape) {
		t.Fatalf("Validate() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestValuesRefuseATargetThatIsNotAnIdentity(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{
		"a word":     []any{"news"},
		"a number":   []any{42},
		"nothing at": []any{nil},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.Values{"categories": value}.Validate(relatable(t, true))

			if !errors.Is(err, content.ErrFieldShape) {
				t.Fatalf("Validate() error = %v, want %v", err, content.ErrFieldShape)
			}
		})
	}
}

func TestValuesRefuseASecondTargetOnAOneField(t *testing.T) {
	t.Parallel()

	patch := content.Values{
		"categories": []any{uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String()},
	}

	err := patch.Validate(relatable(t, false))

	if !errors.Is(err, content.ErrTooManyTargets) {
		t.Fatalf("Validate() error = %v, want %v", err, content.ErrTooManyTargets)
	}
	if !strings.Contains(err.Error(), "categories") {
		t.Errorf("Validate() error = %q, want the field named", err)
	}
}

func TestValuesAcceptOneTargetOnAOneField(t *testing.T) {
	t.Parallel()

	patch := content.Values{"categories": []any{uuid.Must(uuid.NewV7()).String()}}

	if err := patch.Validate(relatable(t, false)); err != nil {
		t.Fatalf("Validate() error = %v, want the one target accepted", err)
	}
}

func TestValuesRefuseARepeatedTarget(t *testing.T) {
	t.Parallel()

	same := uuid.Must(uuid.NewV7()).String()
	patch := content.Values{"categories": []any{same, same}}

	err := patch.Validate(relatable(t, true))

	if !errors.Is(err, content.ErrRepeatedTarget) {
		t.Fatalf("Validate() error = %v, want %v", err, content.ErrRepeatedTarget)
	}
}

func TestFilledCountsAnEmptyRelationAsEmpty(t *testing.T) {
	t.Parallel()

	required, err := content.NewField(content.Field{
		TypeKey: "post", Key: "engine", Label: "Engine",
		Kind: content.FieldKindRelation, RelatesTo: "engine-type", Required: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	fields := []content.Field{required}

	for name, held := range map[string]content.Values{
		"the field is absent": {},
		"the field is empty":  {"engine": []any{}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.Filled(held, fields)

			if !errors.Is(err, content.ErrFieldRequired) {
				t.Fatalf("Filled() error = %v, want %v", err, content.ErrFieldRequired)
			}
		})
	}
}

func TestFilledAcceptsAHeldTarget(t *testing.T) {
	t.Parallel()

	required, err := content.NewField(content.Field{
		TypeKey: "post", Key: "engine", Label: "Engine",
		Kind: content.FieldKindRelation, RelatesTo: "engine-type", Required: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	held := content.Values{"engine": []any{uuid.Must(uuid.NewV7()).String()}}

	if err := content.Filled(held, []content.Field{required}); err != nil {
		t.Fatalf("Filled() error = %v, want a held target to count as filled", err)
	}
}

func TestSelfTargetedRefusesAnItemPointingAtItself(t *testing.T) {
	t.Parallel()

	fields := []content.Field{{
		TypeKey: content.TypePost, Key: "related", Label: "Related",
		Kind: content.FieldKindRelation, RelatesTo: content.TypePost, Many: true,
	}}
	held := content.Content{ID: uuid.Must(uuid.NewV7()), Type: content.TypePost}
	held.Fields = content.Values{"related": []any{held.ID.String()}}

	err := held.SelfTargeted(fields)

	if !errors.Is(err, content.ErrSelfTarget) {
		t.Fatalf("SelfTargeted() error = %v, want %v", err, content.ErrSelfTarget)
	}
	if !strings.Contains(err.Error(), "related") {
		t.Errorf("SelfTargeted() error = %q, want the field named", err)
	}
}

func TestSelfTargetedAcceptsAnotherItemOfItsOwnType(t *testing.T) {
	t.Parallel()

	fields := []content.Field{{
		TypeKey: content.TypePost, Key: "related", Label: "Related",
		Kind: content.FieldKindRelation, RelatesTo: content.TypePost, Many: true,
	}}
	held := content.Content{ID: uuid.Must(uuid.NewV7()), Type: content.TypePost}
	held.Fields = content.Values{"related": []any{uuid.Must(uuid.NewV7()).String()}}

	if err := held.SelfTargeted(fields); err != nil {
		t.Fatalf("SelfTargeted() error = %v, want a sibling of the same type accepted", err)
	}
}

func TestSelfTargetedReportsAValueNoRelationHolds(t *testing.T) {
	t.Parallel()

	fields := []content.Field{{
		TypeKey: content.TypePost, Key: "related", Label: "Related",
		Kind: content.FieldKindRelation, RelatesTo: content.TypePost, Many: true,
	}}
	held := content.Content{ID: uuid.Must(uuid.NewV7()), Fields: content.Values{"related": "news"}}

	if err := held.SelfTargeted(fields); !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("SelfTargeted() error = %v, want %v", err, content.ErrFieldShape)
	}
}

// relationsKeyed returns a relation at the top and one inside a repeater, each carrying its own identity.
func relationsKeyed() []content.Field {
	return []content.Field{
		{ID: 1, TypeKey: "post", Key: "color", Label: "Color", Kind: content.FieldKindText},
		{ID: 2, TypeKey: "post", Key: "categories", Label: "Categories",
			Kind: content.FieldKindRelation, RelatesTo: "category", Many: true},
		{ID: 3, TypeKey: "post", Key: "team", Label: "Team", Kind: content.FieldKindRepeater,
			Fields: []content.Field{{ID: 4, Key: "filed", Label: "Filed",
				Kind: content.FieldKindRelation, RelatesTo: "category", Many: true}}},
	}
}

func TestHeldIdentitiesNamesEveryTargetTheValuesPointAt(t *testing.T) {
	t.Parallel()

	first, second := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	values := content.Values{
		"color":      "red",
		"categories": []any{first.String(), second.String()},
	}

	held := content.HeldIdentities(relationsKeyed(), values)

	if len(held[2]) != 2 || !held[2][first] || !held[2][second] {
		t.Errorf("HeldIdentities() = %v, want both targets named under the field naming them", held)
	}
}

func TestHeldIdentitiesNamesNothingForValuesPointingNowhere(t *testing.T) {
	t.Parallel()

	for name, values := range map[string]content.Values{
		"values nobody filled in":     nil,
		"a relation standing empty":   {"categories": []any{}},
		"a value no relation holds":   {"categories": "news"},
		"a scalar beside no relation": {"color": "red"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if held := content.HeldIdentities(relationsKeyed(), values); len(held[2]) != 0 {
				t.Errorf("HeldIdentities() = %v, want nothing named", held)
			}
		})
	}
}

func TestHeldIdentitiesKeepsEachFieldsTargetsApart(t *testing.T) {
	t.Parallel()

	top, inside := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	values := content.Values{
		"color":      "red",
		"categories": []any{top.String()},
		"team": []any{
			map[string]any{"filed": []any{inside.String()}},
			map[string]any{},
		},
	}

	held := content.HeldIdentities(relationsKeyed(), values)

	if !held[2][top] || !held[4][inside] {
		t.Errorf("HeldIdentities() = %v, want each field naming the target it holds", held)
	}
	if held[2][inside] || held[4][top] {
		t.Errorf("HeldIdentities() = %v, want no field naming a target another field holds", held)
	}
}

// containerHolding returns a container of the kind, holding one required text sub field.
func containerHolding(t *testing.T, kind content.FieldKind, required bool) content.Field {
	t.Helper()
	inside, err := content.NewSubField(content.Field{Key: "name", Label: "Name", Kind: content.FieldKindText,
		Required: true}, kind)
	if err != nil {
		t.Fatalf("NewSubField() error = %v, want nil", err)
	}
	held, err := content.NewField(content.Field{TypeKey: content.TypePost, Key: "author", Label: "Author",
		Kind: kind, Required: required})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	held.Fields = []content.Field{inside}
	return held
}

func TestFilledReachesRequiredFieldsInsideAContainer(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		kind     content.FieldKind
		required bool
		value    any
		want     error
	}{
		"a section standing empty": {
			content.FieldKindSection, false, map[string]any{}, content.ErrFieldRequired,
		},
		"a section the author never opened": {
			content.FieldKindSection, false, nil, nil,
		},
		"a required section standing empty": {
			content.FieldKindSection, true, map[string]any{}, content.ErrFieldRequired,
		},
		"a section holding its answer": {
			content.FieldKindSection, false, map[string]any{"name": "Maria Perez"}, nil,
		},
		"a repeater row missing its answer": {
			content.FieldKindRepeater, false, []any{map[string]any{}}, content.ErrFieldRequired,
		},
		"a repeater standing empty": {
			content.FieldKindRepeater, false, []any{}, nil,
		},
		"a repeater holding its answer": {
			content.FieldKindRepeater, false, []any{map[string]any{"name": "Maria Perez"}}, nil,
		},
		"a section stored as a word, which the shape check refuses instead": {
			content.FieldKindSection, false, "typed", nil,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := containerHolding(t, asked.kind, asked.required)
			values := content.Values{}
			if asked.value != nil {
				values["author"] = asked.value
			}

			err := content.Filled(values, []content.Field{held})

			if !errors.Is(err, asked.want) {
				t.Errorf("Filled() error = %v, want %v", err, asked.want)
			}
		})
	}
}

func TestFilledRefusesARequiredSectionNobodyAnswered(t *testing.T) {
	t.Parallel()

	held := containerHolding(t, content.FieldKindSection, true)

	err := content.Filled(content.Values{"author": map[string]any{}}, []content.Field{held})

	if code, _ := content.CodeOf(err); code != "field_required" {
		t.Errorf("code = %q, want field_required, error %v", code, err)
	}
}
