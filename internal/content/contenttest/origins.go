// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// originCases are the cases over the keys groups carry and the plugin origins types, groups and fields keep.
var originCases = []Case{
	{"CreateGroupMintsAKeyFromTheTitle", createGroupMintsAKeyFromTheTitle},
	{"CreateGroupKeepsTwoSameTitlesApartByKey", createGroupKeepsTwoSameTitlesApartByKey},
	{"CreateGroupKeepsAKeyItWasGiven", createGroupKeepsAKeyItWasGiven},
	{"CreateGroupRefusesAKeyAnotherGroupHolds", createGroupRefusesAKeyAnotherGroupHolds},
	{"CreateStoresTheOriginATypeCarries", createStoresTheOriginATypeCarries},
	{"CreateFieldInGroupStoresTheOriginAFieldCarries", createFieldInGroupStoresTheOriginAFieldCarries},
}

// createGroupMintsAKeyFromTheTitle mints the key of a group the site made from its title and gives it no origin.
func createGroupMintsAKeyFromTheTitle(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")

	group, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Event details", Location: LocationOf("car")})

	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if group.Key != "event-details" {
		t.Errorf("Key = %q, want event-details minted from the title", group.Key)
	}
	if group.Origin != "" {
		t.Errorf("Origin = %q, want none for a group the site made", group.Origin)
	}
}

// createGroupKeepsTwoSameTitlesApartByKey suffixes the key minted for a second group carrying the same title.
func createGroupKeepsTwoSameTitlesApartByKey(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	first, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Details", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(first) error = %v, want nil", err)
	}

	second, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Details", Location: LocationOf("car")})

	if err != nil {
		t.Fatalf("CreateGroup(second) error = %v, want nil", err)
	}
	if first.Key != "details" || second.Key != "details-2" {
		t.Errorf("keys = %q and %q, want details and details-2", first.Key, second.Key)
	}
}

// createGroupKeepsAKeyItWasGiven stores the key and the origin a group was given and reads the origin back.
func createGroupKeepsAKeyItWasGiven(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")

	group, err := s.Types.CreateGroup(t.Context(), content.Group{
		Key: "event-info", Title: "Event details", Location: LocationOf("car"), Origin: "events",
	})

	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if group.Key != "event-info" || group.Origin != "events" {
		t.Errorf("Key, Origin = %q, %q, want the event-info and events it was given", group.Key, group.Origin)
	}
	listed, err := s.Types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	if held, found := GroupOf(listed, group.ID); !found || held.Origin != "events" {
		t.Errorf("ListGroups() holds origin %q for the group, want events read back", held.Origin)
	}
}

// createGroupRefusesAKeyAnotherGroupHolds refuses a group given the key another group already holds.
func createGroupRefusesAKeyAnotherGroupHolds(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	if _, err := s.Types.CreateGroup(t.Context(), content.Group{
		Key: "event-info", Title: "Event details", Location: LocationOf("car"),
	}); err != nil {
		t.Fatalf("CreateGroup(first) error = %v, want nil", err)
	}

	_, err := s.Types.CreateGroup(t.Context(), content.Group{
		Key: "event-info", Title: "Other details", Location: LocationOf("car"),
	})

	if !errors.Is(err, content.ErrGroupKeyTaken) {
		t.Errorf("CreateGroup(second) error = %v, want ErrGroupKeyTaken", err)
	}
}

// createStoresTheOriginATypeCarries stores the origin a type carries and reads it back.
func createStoresTheOriginATypeCarries(t *testing.T, s Stores) {
	built, err := content.NewType("event", "Event", "Events", "events")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	built.Origin = "events"

	created, err := s.Types.Create(t.Context(), built)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	held, err := s.Types.ByKey(t.Context(), "event")
	if err != nil || created.Origin != "events" || held.Origin != "events" {
		t.Errorf("created, held origin = %q, %q (%v), want events stored and read back", created.Origin, held.Origin, err)
	}
}

// createFieldInGroupStoresTheOriginAFieldCarries stores the origin a field and its sub field carry and reads it back.
func createFieldInGroupStoresTheOriginAFieldCarries(t *testing.T, s Stores) {
	StoreType(t, s.Types, "event")
	group, err := s.Types.CreateGroup(t.Context(), content.Group{
		Key: "event-details", Title: "Event details", Location: LocationOf("event"), Origin: "events",
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	section := FieldOn(t, "", "schedule", content.FieldKindSection, "")
	section.Origin = "events"

	parent, err := s.Types.CreateFieldInGroup(t.Context(), group.ID, section, nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	inner := FieldOn(t, "", "starts-at", content.FieldKindDate, "")
	inner.Origin = "events"
	child, err := s.Types.CreateSubField(t.Context(), parent.ID, inner, content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}

	if parent.Origin != "events" || child.Origin != "events" {
		t.Errorf("origins = %q, %q, want events on the field and the sub field", parent.Origin, child.Origin)
	}
	listed, err := s.Types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, _ := GroupOf(listed, group.ID)
	if len(held.Fields) != 1 || held.Fields[0].Origin != "events" ||
		len(held.Fields[0].Fields) != 1 || held.Fields[0].Fields[0].Origin != "events" {
		t.Errorf("read back %+v, want events on the field and the sub field", held.Fields)
	}
}
