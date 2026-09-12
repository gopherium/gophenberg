// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// pointingRows returns a repeater whose rows each point at posts through a relation holding many.
func pointingRows(t *testing.T, many bool) content.Field {
	t.Helper()
	rows, err := content.NewField(content.Field{
		TypeKey: "post", Key: "team", Label: "Team", Kind: content.FieldKindRepeater,
	})
	if err != nil {
		t.Fatalf("NewField(team) error = %v, want nil", err)
	}
	inside, err := content.NewSubField(content.Field{
		Key: "wrote", Label: "Wrote", Kind: content.FieldKindRelation, RelatesTo: "post", Many: many,
	}, content.FieldKindRepeater)
	if err != nil {
		t.Fatalf("NewSubField(wrote) error = %v, want nil", err)
	}
	rows.Fields = []content.Field{inside}
	return rows
}

func TestARelationInsideARowHoldsTheTargetsItNames(t *testing.T) {
	t.Parallel()

	first, second := uuid.Must(uuid.NewV7()).String(), uuid.Must(uuid.NewV7()).String()
	for name, value := range map[string]any{
		"two targets in a row":     []any{first, second},
		"one target in a row":      []any{first},
		"a row pointing nowhere":   []any{},
		"a row that never pointed": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := content.Values{"team": []any{map[string]any{"wrote": value}}}

			if err := held.Validate([]content.Field{pointingRows(t, true)}); err != nil {
				t.Errorf("Validate() error = %v, want the row's targets accepted", err)
			}
		})
	}
}

func TestARelationInsideARowRefusesWhatARelationRefuses(t *testing.T) {
	t.Parallel()

	target := uuid.Must(uuid.NewV7()).String()
	for name, test := range map[string]struct {
		many  bool
		value any
		code  string
	}{
		"a target that is no list":     {true, target, "field_shape_list"},
		"two targets where one stands": {false, []any{target, uuid.Must(uuid.NewV7()).String()}, "too_many_targets"},
		"a target that is no identity": {true, []any{"news"}, "field_shape_identity"},
		"a target named twice":         {true, []any{target, target}, "target_repeated"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			held := content.Values{"team": []any{map[string]any{"wrote": test.value}}}

			err := held.Validate([]content.Field{pointingRows(t, test.many)})

			if codeOf(err) != test.code {
				t.Errorf("Validate() error = %v, want %s", err, test.code)
			}
		})
	}
}

func TestARelationValueStandsAtTheTopOfAGroup(t *testing.T) {
	t.Parallel()

	field, err := content.NewField(content.Field{
		TypeKey: "post", Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	})
	if err != nil {
		t.Fatalf("NewField(categories) error = %v, want nil", err)
	}
	held := content.Values{"categories": []any{uuid.Must(uuid.NewV7()).String()}}

	if err := held.Validate([]content.Field{field}); err != nil {
		t.Errorf("Validate() error = %v, want the targets accepted as a value", err)
	}
}
