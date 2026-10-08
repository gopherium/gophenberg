// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

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
