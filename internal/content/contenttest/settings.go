// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// settingCases are the cases over the settings a field definition carries through the store.
var settingCases = []Case{
	{"FieldSettingsSurviveTheStore", fieldSettingsSurviveTheStore},
	{"AFieldWithoutSettingsReadsBackWithNone", aFieldWithoutSettingsReadsBackWithNone},
	{"UpdateFieldInGroupCarriesSettings", updateFieldInGroupCarriesSettings},
}

// fieldSettingsSurviveTheStore answers and lists back the settings a stored field was declared with.
func fieldSettingsSurviveTheStore(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	group, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	declared := FieldOn(t, "", "rating", content.FieldKindNumber, "")
	declared.Settings = map[string]any{"min": float64(1), "max": float64(10), "instructions": "One to ten."}

	created, err := s.Types.CreateFieldInGroup(t.Context(), group.ID, declared, nil)

	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(created.Settings, declared.Settings) {
		t.Errorf("created settings = %v, want %v answered back", created.Settings, declared.Settings)
	}
	groups, err := s.Types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := GroupOf(groups, group.ID)
	if !found || len(held.Fields) != 1 {
		t.Fatalf("groups = %+v, want the stored field listed", groups)
	}
	if !reflect.DeepEqual(held.Fields[0].Settings, declared.Settings) {
		t.Errorf("listed settings = %v, want %v read back", held.Fields[0].Settings, declared.Settings)
	}
}

// aFieldWithoutSettingsReadsBackWithNone lists a field declared without settings as holding none.
func aFieldWithoutSettingsReadsBackWithNone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	declared := DeclareTypedField(t, s.Types, "car", "subtitle")

	groups, err := s.Types.ListGroups(t.Context())

	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := GroupOf(groups, declared.GroupID)
	if !found || len(held.Fields) != 1 {
		t.Fatalf("groups = %+v, want the declared field listed", groups)
	}
	if held.Fields[0].Settings != nil {
		t.Errorf("settings = %v, want none so the wire keeps omitting them", held.Fields[0].Settings)
	}
}

// updateFieldInGroupCarriesSettings stores the settings an edit gives a field and lists them back.
func updateFieldInGroupCarriesSettings(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	declared := DeclareTypedField(t, s.Types, "car", "subtitle")
	declared.Settings = map[string]any{"maxlength": float64(80)}

	updated, err := s.Types.UpdateFieldInGroup(t.Context(), declared.GroupID, declared, declared.UpdatedAt, nil)

	if err != nil {
		t.Fatalf("UpdateFieldInGroup() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(updated.Settings, declared.Settings) {
		t.Errorf("updated settings = %v, want %v stored", updated.Settings, declared.Settings)
	}
	groups, err := s.Types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, _ := GroupOf(groups, declared.GroupID)
	if len(held.Fields) != 1 || !reflect.DeepEqual(held.Fields[0].Settings, declared.Settings) {
		t.Errorf("listed settings = %v, want the stored bounds read back", held.Fields)
	}
}
