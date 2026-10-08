// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// groupOf is the shared fixture under the name these tests use.
var groupOf = contenttest.GroupOf

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
	if _, err := store.CreateFieldInGroup(t.Context(), group.ID, declared, nil); err != nil {
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
