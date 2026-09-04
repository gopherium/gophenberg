// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestCreateGroupMintsAKeyFromTheTitle(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")

	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Event details", Location: locationOf("car")})

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

func TestCreateGroupKeepsTwoSameTitlesApartByKey(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	first, err := store.CreateGroup(t.Context(), content.Group{Title: "Details", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(first) error = %v, want nil", err)
	}

	second, err := store.CreateGroup(t.Context(), content.Group{Title: "Details", Location: locationOf("car")})

	if err != nil {
		t.Fatalf("CreateGroup(second) error = %v, want nil", err)
	}
	if first.Key != "details" || second.Key != "details-2" {
		t.Errorf("keys = %q and %q, want details and details-2", first.Key, second.Key)
	}
}

func TestCreateGroupKeepsAKeyItWasGiven(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")

	group, err := store.CreateGroup(t.Context(), content.Group{
		Key: "event-info", Title: "Event details", Location: locationOf("car"), Origin: "events",
	})

	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if group.Key != "event-info" || group.Origin != "events" {
		t.Errorf("Key, Origin = %q, %q, want the event-info and events it was given", group.Key, group.Origin)
	}
	listed, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	if held, found := groupOf(listed, group.ID); !found || held.Origin != "events" {
		t.Errorf("ListGroups() holds origin %q for the group, want events read back", held.Origin)
	}
}

func TestCreateGroupRefusesAKeyAnotherGroupHolds(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	if _, err := store.CreateGroup(t.Context(), content.Group{
		Key: "event-info", Title: "Event details", Location: locationOf("car"),
	}); err != nil {
		t.Fatalf("CreateGroup(first) error = %v, want nil", err)
	}

	_, err := store.CreateGroup(t.Context(), content.Group{
		Key: "event-info", Title: "Other details", Location: locationOf("car"),
	})

	if !errors.Is(err, content.ErrGroupKeyTaken) {
		t.Errorf("CreateGroup(second) error = %v, want ErrGroupKeyTaken", err)
	}
}

func TestCreateStoresTheOriginATypeCarries(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	built, err := content.NewType("event", "Event", "Events", "events")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	built.Origin = "events"

	created, err := store.Create(t.Context(), built)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	held, err := store.ByKey(t.Context(), "event")
	if err != nil || created.Origin != "events" || held.Origin != "events" {
		t.Errorf("created, held origin = %q, %q (%v), want events stored and read back", created.Origin, held.Origin, err)
	}
}

func TestCreateFieldInGroupStoresTheOriginAFieldCarries(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "event")
	group, err := store.CreateGroup(t.Context(), content.Group{
		Key: "event-details", Title: "Event details", Location: locationOf("event"), Origin: "events",
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	section := fieldOn(t, "", "schedule", content.FieldKindSection, "")
	section.Origin = "events"

	parent, err := store.CreateFieldInGroup(t.Context(), group.ID, section)
	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	inner := fieldOn(t, "", "starts-at", content.FieldKindDate, "")
	inner.Origin = "events"
	child, err := store.CreateSubField(t.Context(), parent.ID, inner)
	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}

	if parent.Origin != "events" || child.Origin != "events" {
		t.Errorf("origins = %q, %q, want events on the field and the sub field", parent.Origin, child.Origin)
	}
	listed, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, _ := groupOf(listed, group.ID)
	if len(held.Fields) != 1 || held.Fields[0].Origin != "events" ||
		len(held.Fields[0].Fields) != 1 || held.Fields[0].Fields[0].Origin != "events" {
		t.Errorf("read back %+v, want events on the field and the sub field", held.Fields)
	}
}

func TestCreateFieldRefusesToRaiseAFieldInsideAGroupAPluginDeclared(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "event")
	if _, err := store.CreateGroup(t.Context(), content.Group{
		Key: "event-details", Title: "Event details", Location: locationOf("event"), Origin: "events",
	}); err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	_, err := store.CreateField(t.Context(), fieldOn(t, "event", "venue", content.FieldKindText, ""))

	if !errors.Is(err, content.ErrDefinitionReadOnly) {
		t.Errorf("CreateField() error = %v, want %v", err, content.ErrDefinitionReadOnly)
	}
}

func TestMigrationKeysExistingGroupsFromTheirTitles(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	url := pool.Config().ConnString()
	if err := postgres.MigrateDownTo(t.Context(), url, 18); err != nil {
		t.Fatalf("MigrateDownTo(18) error = %v, want nil", err)
	}
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO core.field_groups (title, location, position, active, created_at, updated_at)
		VALUES ('Details', '[]', 1, true, now(), now()),
		       ('Details', '[]', 2, true, now(), now()),
		       ('!!!', '[]', 3, true, now(), now()),
		       ('2024 plans', '[]', 4, true, now(), now()),
		       ('Maria''s picks', '[]', 5, true, now(), now())`,
	); err != nil {
		t.Fatalf("seeding the pre migration groups: %v, want nil", err)
	}

	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	want := []string{"details", "details-2", "untitled", "group-2024-plans", "marias-picks"}
	if len(groups) != len(want) {
		t.Fatalf("ListGroups() holds %d groups, want %d", len(groups), len(want))
	}
	for i, group := range groups {
		if group.Key != want[i] {
			t.Errorf("groups[%d].Key = %q, want %q", i, group.Key, want[i])
		}
		if group.Origin != "" {
			t.Errorf("groups[%d].Origin = %q, want none for a group that predates plugins", i, group.Origin)
		}
	}
}
