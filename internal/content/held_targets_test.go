// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// targetsUnder returns the targets the held fields carry under the key, and whether the key stands among them.
func targetsUnder(held []content.FieldTargets, key string) ([]uuid.UUID, bool) {
	for _, ft := range held {
		if ft.Field.Key == key {
			return ft.Targets, true
		}
	}
	return nil, false
}

// rowsHolding returns a repeater whose rows hold a relation, and a top level relation beside it.
func rowsHolding(t *testing.T) []content.Field {
	t.Helper()
	rows, err := content.NewField(content.Field{
		TypeKey: "post", Key: "team", Label: "Team", Kind: content.FieldKindRepeater,
	})
	if err != nil {
		t.Fatalf("NewField(team) error = %v, want nil", err)
	}
	inside, err := content.NewSubField(content.Field{
		Key: "wrote", Label: "Wrote", Kind: content.FieldKindRelation, RelatesTo: "post", Many: true,
	}, content.FieldKindRepeater)
	if err != nil {
		t.Fatalf("NewSubField(wrote) error = %v, want nil", err)
	}
	inside.ID = 7
	rows.ID = 5
	rows.Fields = []content.Field{inside}
	beside, err := content.NewField(content.Field{
		TypeKey: "post", Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	})
	if err != nil {
		t.Fatalf("NewField(categories) error = %v, want nil", err)
	}
	beside.ID = 3
	plain, err := content.NewField(content.Field{
		TypeKey: "post", Key: "venue", Label: "Venue", Kind: content.FieldKindText,
	})
	if err != nil {
		t.Fatalf("NewField(venue) error = %v, want nil", err)
	}
	plain.ID = 9
	return []content.Field{rows, beside, plain}
}

func TestHeldTargetsNamesEveryRelationTheTypeDeclares(t *testing.T) {
	t.Parallel()

	news := uuid.Must(uuid.NewV7())
	values := content.Values{"categories": []any{news.String()}, "venue": "Town hall"}

	held, err := content.HeldTargets(rowsHolding(t), values)

	if err != nil {
		t.Fatalf("HeldTargets() error = %v, want nil", err)
	}
	if len(held) != 2 {
		t.Fatalf("HeldTargets() = %+v, want the two relations alone, the text field left out", held)
	}
	filed, named := targetsUnder(held, "categories")
	if !named || len(filed) != 1 || filed[0] != news {
		t.Errorf("categories = %v, want the one target it names", filed)
	}
	inside, named := targetsUnder(held, "wrote")
	if !named || len(inside) != 0 {
		t.Errorf("wrote = %v, want no target where the rows name none", inside)
	}
}

func TestHeldTargetsGathersEveryRowOfARepeaterOnce(t *testing.T) {
	t.Parallel()

	first, second, third := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	values := content.Values{"team": []any{
		map[string]any{"wrote": []any{first.String(), second.String()}},
		map[string]any{"wrote": []any{second.String(), third.String()}},
	}}

	held, err := content.HeldTargets(rowsHolding(t), values)

	if err != nil {
		t.Fatalf("HeldTargets() error = %v, want nil", err)
	}
	inside, named := targetsUnder(held, "wrote")
	if !named || len(inside) != 3 || inside[0] != first || inside[1] != second || inside[2] != third {
		t.Errorf("wrote = %v, want each target once in the order the rows name them", inside)
	}
}

func TestHeldTargetsCarriesTheFieldTheStoreWritesUnder(t *testing.T) {
	t.Parallel()

	held, err := content.HeldTargets(rowsHolding(t), content.Values{})

	if err != nil {
		t.Fatalf("HeldTargets() error = %v, want nil", err)
	}
	for _, ft := range held {
		if ft.Field.ID == 0 || ft.Field.RelatesTo == "" {
			t.Errorf("held %+v, want the identity and the target type the store writes with", ft.Field)
		}
	}
}

func TestHeldTargetsReportsAValueNoRelationHolds(t *testing.T) {
	t.Parallel()

	values := content.Values{"team": []any{map[string]any{"wrote": []any{"news"}}}}

	_, err := content.HeldTargets(rowsHolding(t), values)

	if codeOf(err) != "field_shape_identity" {
		t.Errorf("HeldTargets() error = %v, want field_shape_identity", err)
	}
}

func TestSelfTargetedReadsARelationInsideARow(t *testing.T) {
	t.Parallel()

	itself := uuid.Must(uuid.NewV7())
	c := content.Content{ID: itself, Fields: content.Values{"team": []any{
		map[string]any{"wrote": []any{itself.String()}},
	}}}

	err := c.SelfTargeted(rowsHolding(t))

	if codeOf(err) != "target_is_self" {
		t.Errorf("SelfTargeted() error = %v, want target_is_self", err)
	}
}

func TestSelfTargetedLeavesAnItemPointingElsewhere(t *testing.T) {
	t.Parallel()

	c := content.Content{ID: uuid.Must(uuid.NewV7()), Fields: content.Values{
		"categories": []any{uuid.Must(uuid.NewV7()).String()},
	}}

	if err := c.SelfTargeted(rowsHolding(t)); err != nil {
		t.Errorf("SelfTargeted() error = %v, want nil", err)
	}
}
