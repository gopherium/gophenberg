// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestFieldSettingsSurviveTheStore(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	declared := fieldOn(t, "", "rating", content.FieldKindNumber, "")
	declared.Settings = map[string]any{"min": float64(1), "max": float64(10), "instructions": "One to ten."}

	created, err := store.CreateFieldInGroup(t.Context(), group.ID, declared)

	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(created.Settings, declared.Settings) {
		t.Errorf("created settings = %v, want %v answered back", created.Settings, declared.Settings)
	}
	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := groupOf(groups, group.ID)
	if !found || len(held.Fields) != 1 {
		t.Fatalf("groups = %+v, want the stored field listed", groups)
	}
	if !reflect.DeepEqual(held.Fields[0].Settings, declared.Settings) {
		t.Errorf("listed settings = %v, want %v read back", held.Fields[0].Settings, declared.Settings)
	}
}

func TestAFieldWithoutSettingsReadsBackWithNone(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	declared := declareTypedField(t, store, "car", "subtitle")

	groups, err := store.ListGroups(t.Context())

	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := groupOf(groups, declared.GroupID)
	if !found || len(held.Fields) != 1 {
		t.Fatalf("groups = %+v, want the declared field listed", groups)
	}
	if held.Fields[0].Settings != nil {
		t.Errorf("settings = %v, want none so the wire keeps omitting them", held.Fields[0].Settings)
	}
}

func TestUpdateFieldInGroupCarriesSettings(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	declared := declareTypedField(t, store, "car", "subtitle")
	declared.Settings = map[string]any{"maxlength": float64(80)}

	updated, err := store.UpdateFieldInGroup(t.Context(), declared.GroupID, declared, declared.UpdatedAt)

	if err != nil {
		t.Fatalf("UpdateFieldInGroup() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(updated.Settings, declared.Settings) {
		t.Errorf("updated settings = %v, want %v stored", updated.Settings, declared.Settings)
	}
	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, _ := groupOf(groups, declared.GroupID)
	if len(held.Fields) != 1 || !reflect.DeepEqual(held.Fields[0].Settings, declared.Settings) {
		t.Errorf("listed settings = %v, want the stored bounds read back", held.Fields)
	}
}

func TestRollingBackPastSettingsLeavesTheFieldStanding(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declared := fieldOn(t, "", "rating", content.FieldKindNumber, "")
	declared.Settings = map[string]any{"min": float64(1)}
	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := store.CreateFieldInGroup(t.Context(), group.ID, declared); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	url := pool.Config().ConnString()

	if err := postgres.MigrateDownTo(t.Context(), url, 13); err != nil {
		t.Fatalf("MigrateDownTo(13) error = %v, want nil", err)
	}
	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := groupOf(groups, group.ID)
	if !found || len(held.Fields) != 1 || held.Fields[0].Key != "rating" {
		t.Fatalf("groups = %+v, want the field standing after the walk", groups)
	}
	if held.Fields[0].Settings != nil {
		t.Errorf("settings = %v, want the rollback to have dropped them", held.Fields[0].Settings)
	}
}

// groupOf returns the group carrying the identifier, and whether one does.
func groupOf(groups []content.Group, id int) (content.Group, bool) {
	for _, held := range groups {
		if held.ID == id {
			return held, true
		}
	}
	return content.Group{}, false
}
