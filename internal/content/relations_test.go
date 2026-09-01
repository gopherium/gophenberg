// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// relatable returns a text field and a many relation field to split a patch against.
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

func TestSplitValuesDividesScalarsFromTargets(t *testing.T) {
	t.Parallel()

	first, second := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	patch := content.Values{
		"color":      "red",
		"categories": []any{first.String(), second.String()},
	}

	scalars, relations, err := content.SplitValues(patch, relatable(t, true))

	if err != nil {
		t.Fatalf("SplitValues() error = %v, want nil", err)
	}
	if len(scalars) != 1 || scalars["color"] != "red" {
		t.Errorf("SplitValues() scalars = %v, want the text value alone", scalars)
	}
	held := relations["categories"]
	if len(held) != 2 || held[0] != first || held[1] != second {
		t.Errorf("SplitValues() relations = %v, want both targets in order", held)
	}
}

func TestSplitValuesReadsANullAsAClearedRelation(t *testing.T) {
	t.Parallel()

	patch := content.Values{"categories": nil}

	_, relations, err := content.SplitValues(patch, relatable(t, true))

	if err != nil {
		t.Fatalf("SplitValues() error = %v, want nil", err)
	}
	held, named := relations["categories"]
	if !named || len(held) != 0 {
		t.Errorf("SplitValues() relations = %v, want the field named with no targets", relations)
	}
}

func TestSplitValuesRefusesAnUnknownKey(t *testing.T) {
	t.Parallel()

	_, _, err := content.SplitValues(content.Values{"finish": "matte"}, relatable(t, true))

	if !errors.Is(err, content.ErrUnknownField) {
		t.Fatalf("SplitValues() error = %v, want %v", err, content.ErrUnknownField)
	}
}

func TestSplitValuesRefusesATargetThatIsNotAList(t *testing.T) {
	t.Parallel()

	_, _, err := content.SplitValues(content.Values{"categories": "news"}, relatable(t, true))

	if !errors.Is(err, content.ErrFieldShape) {
		t.Fatalf("SplitValues() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestSplitValuesRefusesATargetThatIsNotAnIdentity(t *testing.T) {
	t.Parallel()

	patch := content.Values{"categories": []any{"news"}}

	_, _, err := content.SplitValues(patch, relatable(t, true))

	if !errors.Is(err, content.ErrFieldShape) {
		t.Fatalf("SplitValues() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestSplitValuesRefusesASecondTargetOnAOneField(t *testing.T) {
	t.Parallel()

	patch := content.Values{
		"categories": []any{uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String()},
	}

	_, _, err := content.SplitValues(patch, relatable(t, false))

	if !errors.Is(err, content.ErrTooManyTargets) {
		t.Fatalf("SplitValues() error = %v, want %v", err, content.ErrTooManyTargets)
	}
	if !strings.Contains(err.Error(), "categories") {
		t.Errorf("SplitValues() error = %q, want the field named", err)
	}
}

func TestSplitValuesAcceptsOneTargetOnAOneField(t *testing.T) {
	t.Parallel()

	patch := content.Values{"categories": []any{uuid.Must(uuid.NewV7()).String()}}

	_, relations, err := content.SplitValues(patch, relatable(t, false))

	if err != nil {
		t.Fatalf("SplitValues() error = %v, want nil", err)
	}
	if len(relations["categories"]) != 1 {
		t.Errorf("SplitValues() relations = %v, want the one target", relations)
	}
}

func TestSplitValuesRefusesARepeatedTarget(t *testing.T) {
	t.Parallel()

	same := uuid.Must(uuid.NewV7()).String()
	patch := content.Values{"categories": []any{same, same}}

	_, _, err := content.SplitValues(patch, relatable(t, true))

	if !errors.Is(err, content.ErrRepeatedTarget) {
		t.Fatalf("SplitValues() error = %v, want %v", err, content.ErrRepeatedTarget)
	}
}

func TestRelationsMergeReplacesANamedField(t *testing.T) {
	t.Parallel()

	first, second := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	stored := content.Relations{"categories": {first}, "series": {second}}

	merged := stored.Merge(content.Relations{"categories": {second}})

	if len(merged["categories"]) != 1 || merged["categories"][0] != second {
		t.Errorf("Merge() categories = %v, want the named field replaced", merged["categories"])
	}
	if len(merged["series"]) != 1 || merged["series"][0] != second {
		t.Errorf("Merge() series = %v, want the absent field left alone", merged["series"])
	}
}

func TestRelationsMergeLeavesTheStoredTargetsAlone(t *testing.T) {
	t.Parallel()

	first, second := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	stored := content.Relations{"categories": {first}}

	stored.Merge(content.Relations{"categories": {second}})

	if stored["categories"][0] != first {
		t.Errorf("the stored targets hold %v, want the merge to have left them alone", stored["categories"])
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

	for name, held := range map[string]content.Relations{
		"the field is absent": {},
		"the field is empty":  {"engine": {}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.Filled(content.Values{}, held, fields)

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
	held := content.Relations{"engine": {uuid.Must(uuid.NewV7())}}

	if err := content.Filled(content.Values{}, held, []content.Field{required}); err != nil {
		t.Fatalf("Filled() error = %v, want a held target to count as filled", err)
	}
}

func TestSelfTargetedRefusesAnItemPointingAtItself(t *testing.T) {
	t.Parallel()

	held := content.Content{ID: uuid.Must(uuid.NewV7()), Type: content.TypePost}
	held.Relations = content.Relations{"related": {held.ID}}

	err := held.SelfTargeted()

	if !errors.Is(err, content.ErrSelfTarget) {
		t.Fatalf("SelfTargeted() error = %v, want %v", err, content.ErrSelfTarget)
	}
	if !strings.Contains(err.Error(), "related") {
		t.Errorf("SelfTargeted() error = %q, want the field named", err)
	}
}

func TestSelfTargetedAcceptsAnotherItemOfItsOwnType(t *testing.T) {
	t.Parallel()

	held := content.Content{ID: uuid.Must(uuid.NewV7()), Type: content.TypePost}
	held.Relations = content.Relations{"related": {uuid.Must(uuid.NewV7())}}

	if err := held.SelfTargeted(); err != nil {
		t.Fatalf("SelfTargeted() error = %v, want a sibling of the same type accepted", err)
	}
}

func TestARelationRefusesATargetThatIsNotWritten(t *testing.T) {
	t.Parallel()

	fields := []content.Field{{
		TypeKey: content.TypePost, Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	}}

	_, _, err := content.SplitValues(content.Values{"categories": []any{42}}, fields)

	if !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("SplitValues() error = %v, want %v", err, content.ErrFieldShape)
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

			err := content.Filled(values, content.Relations{}, []content.Field{held})

			if !errors.Is(err, asked.want) {
				t.Errorf("Filled() error = %v, want %v", err, asked.want)
			}
		})
	}
}

func TestFilledRefusesARequiredSectionNobodyAnswered(t *testing.T) {
	t.Parallel()

	held := containerHolding(t, content.FieldKindSection, true)

	err := content.Filled(content.Values{"author": map[string]any{}}, content.Relations{}, []content.Field{held})

	if code, _ := content.CodeOf(err); code != "field_required" {
		t.Errorf("code = %q, want field_required, error %v", code, err)
	}
}
