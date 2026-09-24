// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// twinOnCars declares a relation on the cars group pointing at cars, beside the maker the backlinks reads.
func twinOnCars(t *testing.T, registry *content.Registry, cars content.Group) {
	t.Helper()
	if _, err := registry.CreateFieldInGroup(t.Context(), cars.ID, content.Field{
		Key: "twin", Label: "Twin", Kind: content.FieldKindRelation, RelatesTo: "car",
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(twin) error = %v, want nil", err)
	}
}

// readingOnCars returns the source naming one relation of the cars group, stamped as the group's field stands now.
func readingOnCars(
	t *testing.T, registry *content.Registry, groupID int, key, relation string,
) map[string]content.Repoint {
	t.Helper()
	return map[string]content.Repoint{key: {
		Source:    content.Source{Group: "cars", Field: []string{relation}},
		UpdatedAt: topFieldIn(t, registry, groupID, key).UpdatedAt,
	}}
}

// locationOf returns the stored location of the group carrying the identity.
func locationOf(t *testing.T, registry *content.Registry, groupID int) content.Rules {
	t.Helper()
	groups, err := registry.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups() error = %v, want nil", err)
	}
	for _, g := range groups {
		if g.ID == groupID {
			return g.Location
		}
	}
	t.Fatalf("no group carries the identity %d", groupID)
	return nil
}

func TestRegistryMovesAGroupWithItsBacklinksPointedAnew(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	cars, makers := readableSource(t, registry)
	twinOnCars(t, registry, cars)
	makers.Location = namingType("car")

	if _, err := registry.UpdateGroupRepointed(
		t.Context(), makers, readingOnCars(t, registry, makers.ID, "linked-from", "twin"),
	); err != nil {
		t.Fatalf("UpdateGroupRepointed() error = %v, want the move taken with its backlinks", err)
	}

	if path := content.SourceFieldOf(backlinksIn(t, registry, makers.ID)); !slices.Equal(path, []string{"twin"}) {
		t.Errorf("SourceFieldOf() = %v, want the backlinks reading the twin", path)
	}
	if !locationOf(t, registry, makers.ID).Equal(namingType("car")) {
		t.Errorf("location = %v, want the group placed on cars", locationOf(t, registry, makers.ID))
	}
}

func TestRegistryWritesNothingWhenABacklinksIsPointedAtNothing(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	_, makers := readableSource(t, registry)
	makers.Location = namingType("car")

	_, err := registry.UpdateGroupRepointed(
		t.Context(), makers, readingOnCars(t, registry, makers.ID, "linked-from", "nothing"),
	)

	if codeOf(err) != "backlinks_source_unknown" {
		t.Errorf("UpdateGroupRepointed() error = %v, want backlinks_source_unknown", err)
	}
	if path := content.SourceFieldOf(backlinksIn(t, registry, makers.ID)); !slices.Equal(path, []string{"maker"}) {
		t.Errorf("SourceFieldOf() = %v, want the backlinks still reading the maker", path)
	}
	if !locationOf(t, registry, makers.ID).Equal(namingPost()) {
		t.Errorf("location = %v, want the group still on posts", locationOf(t, registry, makers.ID))
	}
}

func TestRegistryRefusesToPointAFieldChangedSinceItWasRead(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	cars, makers := readableSource(t, registry)
	twinOnCars(t, registry, cars)
	makers.Location = namingType("car")
	asked := readingOnCars(t, registry, makers.ID, "linked-from", "twin")
	stale := asked["linked-from"]
	stale.UpdatedAt = stale.UpdatedAt.Add(-time.Minute)
	asked["linked-from"] = stale

	_, err := registry.UpdateGroupRepointed(t.Context(), makers, asked)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateGroupRepointed() error = %v, want %v", err, content.ErrConflict)
	}
	if path := content.SourceFieldOf(backlinksIn(t, registry, makers.ID)); !slices.Equal(path, []string{"maker"}) {
		t.Errorf("SourceFieldOf() = %v, want the backlinks still reading the maker", path)
	}
	if !locationOf(t, registry, makers.ID).Equal(namingPost()) {
		t.Errorf("location = %v, want the group still on posts", locationOf(t, registry, makers.ID))
	}
}

func TestRegistryRefusesToPointAFieldTheGroupLacks(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	_, makers := readableSource(t, registry)

	_, err := registry.UpdateGroupRepointed(t.Context(), makers, map[string]content.Repoint{
		"ghost": {Source: content.Source{Group: "cars", Field: []string{"maker"}}},
	})

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("UpdateGroupRepointed() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestRegistryRefusesToPointAFieldThatReadsNoRelation(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	_, makers := readableSource(t, registry)
	if _, err := registry.CreateFieldInGroup(t.Context(), makers.ID, content.Field{
		Key: "note", Label: "Note", Kind: content.FieldKindText,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(note) error = %v, want nil", err)
	}

	_, err := registry.UpdateGroupRepointed(
		t.Context(), makers, readingOnCars(t, registry, makers.ID, "note", "maker"),
	)

	if codeOf(err) != "setting_unknown" {
		t.Errorf("UpdateGroupRepointed() error = %v, want setting_unknown", err)
	}
}

func TestRegistryKeepsTheSettingsABacklinksCarriesBesideItsSource(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	cars, makers := readableSource(t, registry)
	twinOnCars(t, registry, cars)
	held := backlinksIn(t, registry, makers.ID)
	held.Settings = map[string]any{
		content.SettingSourceGroup: "cars", content.SettingSourceField: []any{"maker"},
		content.SettingInstructions: "Every car made here",
	}
	if _, err := registry.UpdateFieldInGroup(t.Context(), makers.ID, held, held.UpdatedAt); err != nil {
		t.Fatalf("UpdateFieldInGroup() error = %v, want nil", err)
	}
	makers.Location = namingType("car")

	if _, err := registry.UpdateGroupRepointed(
		t.Context(), makers, readingOnCars(t, registry, makers.ID, "linked-from", "twin"),
	); err != nil {
		t.Fatalf("UpdateGroupRepointed() error = %v, want nil", err)
	}

	if said := backlinksIn(t, registry, makers.ID).Settings[content.SettingInstructions]; said != "Every car made here" {
		t.Errorf("instructions = %v, want them kept beside the new source", said)
	}
}
