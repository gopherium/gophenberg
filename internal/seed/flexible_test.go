// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestFlexibleDeclaresTheLayoutsAndTheFieldsTheyHold(t *testing.T) {
	t.Parallel()

	types := &holdingTypeStore{}

	if err := Flexible(t.Context(), content.NewRegistry(types)); err != nil {
		t.Fatalf("Flexible() error = %v, want nil", err)
	}

	if len(types.declared) != 1 || types.declared[0].Key != FeaturesFieldKey {
		t.Fatalf("the store holds %+v, want the flexible declared", types.declared)
	}
	layouts := heldUnder(types.inside, types.declared[0].ID)
	if len(layouts) != 2 || layouts[0].Key != "hero" || layouts[1].Key != "quote" {
		t.Fatalf("the flexible holds %+v, want the hero and the quote", layouts)
	}
	for _, layout := range layouts {
		if layout.Kind != content.FieldKindLayout {
			t.Errorf("%q holds %q, want a layout", layout.Key, layout.Kind)
		}
		if len(heldUnder(types.inside, layout.ID)) == 0 {
			t.Errorf("the layout %q holds no fields, want the ones it declares", layout.Key)
		}
	}
}

func TestFlexibleLeavesAFlexibleTheTypeAlreadyCarries(t *testing.T) {
	t.Parallel()

	types := &holdingTypeStore{}
	registry := content.NewRegistry(types)
	if err := Flexible(t.Context(), registry); err != nil {
		t.Fatalf("the first seeding: %v, want nil", err)
	}
	stored := len(types.inside)

	if err := Flexible(t.Context(), registry); err != nil {
		t.Fatalf("Flexible() again error = %v, want nil", err)
	}

	if len(types.declared) != 1 || len(types.inside) != stored {
		t.Errorf("the store holds %d fields and %d inside, want the second seeding to declare none",
			len(types.declared), len(types.inside))
	}
}

func TestFlexibleReportsWhatItCannotDeclare(t *testing.T) {
	t.Parallel()

	for name, registry := range map[string]*content.Registry{
		"the type it cannot read":    content.NewRegistry(&categoryTypeStore{listErr: errStub}),
		"the field it cannot store":  content.NewRegistry(&categoryTypeStore{createFieldErr: errStub}),
		"the layout it cannot store": content.NewRegistry(&categoryTypeStore{subFieldErr: errStub}),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Flexible(t.Context(), registry)

			if !errors.Is(err, errStub) {
				t.Errorf("Flexible() error = %v, want %v", err, errStub)
			}
		})
	}
}

func TestFlexibleReportsAFieldALayoutCannotHold(t *testing.T) {
	t.Parallel()

	types := &refusingSubFieldStore{holdingTypeStore: holdingTypeStore{}, after: 1}

	err := Flexible(t.Context(), content.NewRegistry(types))

	if !errors.Is(err, errStub) {
		t.Errorf("Flexible() error = %v, want %v", err, errStub)
	}
}

func TestFeaturesFieldStandsAtTheTopOfThePostType(t *testing.T) {
	t.Parallel()

	held := FeaturesField()

	if held.Kind != content.FieldKindFlexible {
		t.Errorf("Kind = %q, want %q", held.Kind, content.FieldKindFlexible)
	}
	if held.TypeKey != content.TypePost {
		t.Errorf("TypeKey = %q, want the post type", held.TypeKey)
	}
	if err := held.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want the seeded flexible to stand", err)
	}
}

func TestFeatureLayoutsStandInsideAFlexibleAndHoldTheirOwnFields(t *testing.T) {
	t.Parallel()

	layouts := FeatureLayouts()

	if len(layouts) != 2 {
		t.Fatalf("the flexible declares %d layouts, want the hero and the quote", len(layouts))
	}
	for _, layout := range layouts {
		if _, err := content.NewSubField(layout, content.FieldKindFlexible); err != nil {
			t.Errorf("NewSubField(%q) error = %v, want it to stand inside a flexible", layout.Key, err)
		}
		if len(layout.Fields) == 0 {
			t.Errorf("the layout %q declares no fields, want the ones it holds", layout.Key)
		}
		for _, inside := range layout.Fields {
			if _, err := content.NewSubField(inside, content.FieldKindLayout); err != nil {
				t.Errorf("NewSubField(%q) error = %v, want it to stand inside a layout", inside.Key, err)
			}
		}
	}
}

// heldUnder returns the stored fields standing under the parent the identity names.
func heldUnder(stored []content.Field, parentID int) []content.Field {
	var inside []content.Field
	for _, held := range stored {
		if held.ParentID == parentID {
			inside = append(inside, held)
		}
	}
	return inside
}

// refusingSubFieldStore stores sub fields until the count is reached, then reports the scripted failure.
type refusingSubFieldStore struct {
	holdingTypeStore
	after int
}

// CreateSubField stores the declaration until the count is reached, then reports the scripted failure.
func (s *refusingSubFieldStore) CreateSubField(
	ctx context.Context, parentID int, f content.Field,
) (content.Field, error) {
	if len(s.inside) >= s.after {
		return content.Field{}, errStub
	}
	return s.holdingTypeStore.CreateSubField(ctx, parentID, f)
}
