// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// kindCases are the cases over storing the open field kinds and the relation targets they name.
var kindCases = []Case{
	{"TheStoreHoldsAChoiceField", theStoreHoldsAChoiceField},
	{"TheStoreNamesTheTargetAFieldPointsAtInVain", theStoreNamesTheTargetAFieldPointsAtInVain},
	{"TheStoreHoldsAManyMedia", theStoreHoldsAManyMedia},
}

// theStoreHoldsAChoiceField stores a choice field and lists back its kind and its choices.
func theStoreHoldsAChoiceField(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	group, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	declared := FieldOn(t, "", "style", content.FieldKindChoice, "")
	declared.Settings = map[string]any{
		"choices": []any{
			map[string]any{"value": "ipa", "label": "IPA"},
			map[string]any{"value": "stout", "label": "Stout"},
		},
		"presentation": "radio",
	}

	created, err := s.Types.CreateFieldInGroup(t.Context(), group.ID, declared, nil)

	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	if created.Kind != content.FieldKindChoice {
		t.Errorf("Kind = %q, want %q stored", created.Kind, content.FieldKindChoice)
	}
	groups, err := s.Types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := GroupOf(groups, group.ID)
	if !found || len(held.Fields) != 1 {
		t.Fatalf("groups = %+v, want the stored field listed", groups)
	}
	if held.Fields[0].Kind != content.FieldKindChoice {
		t.Errorf("listed kind = %q, want %q read back", held.Fields[0].Kind, content.FieldKindChoice)
	}
	if !reflect.DeepEqual(held.Fields[0].Settings, declared.Settings) {
		t.Errorf("listed settings = %v, want %v read back", held.Fields[0].Settings, declared.Settings)
	}
}

// theStoreNamesTheTargetAFieldPointsAtInVain refuses a relation field whose target type the store does not hold.
func theStoreNamesTheTargetAFieldPointsAtInVain(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	group, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	declared := FieldOn(t, "", "engine", content.FieldKindRelation, "nosuchtype")

	_, err = s.Types.CreateFieldInGroup(t.Context(), group.ID, declared, nil)

	if !errors.Is(err, content.ErrTargetUnknown) {
		t.Errorf("CreateFieldInGroup() error = %v, want %v", err, content.ErrTargetUnknown)
	}
}

// theStoreHoldsAManyMedia stores a media field holding many and lists it back holding many.
func theStoreHoldsAManyMedia(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	group, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	declared := FieldOn(t, "", "gallery", content.FieldKindMedia, "")
	declared.Many = true

	created, err := s.Types.CreateFieldInGroup(t.Context(), group.ID, declared, nil)

	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	if !created.Many {
		t.Errorf("Many = false, want the media field holding many stored")
	}
	groups, err := s.Types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := GroupOf(groups, group.ID)
	if !found || len(held.Fields) != 1 || !held.Fields[0].Many {
		t.Fatalf("groups = %+v, want the many media listed", groups)
	}
}
