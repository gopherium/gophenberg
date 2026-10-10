// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestRollingBackPastOpenKindsDropsWhatTheOldCheckRefuses(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	for _, declared := range []content.Field{
		fieldOn(t, "", "style", content.FieldKindChoice, ""),
		fieldOn(t, "", "subtitle", content.FieldKindText, ""),
	} {
		if _, err := store.CreateFieldInGroup(t.Context(), group.ID, declared, nil); err != nil {
			t.Fatalf("CreateFieldInGroup(%s) error = %v, want nil", declared.Key, err)
		}
	}
	gallery := fieldOn(t, "", "gallery", content.FieldKindMedia, "")
	gallery.Many = true
	if _, err := store.CreateFieldInGroup(t.Context(), group.ID, gallery, nil); err != nil {
		t.Fatalf("CreateFieldInGroup(gallery) error = %v, want nil", err)
	}
	url := pool.Config().ConnString()

	if err := postgres.MigrateDownTo(t.Context(), url, 14); err != nil {
		t.Fatalf("MigrateDownTo(14) error = %v, want nil", err)
	}
	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := groupOf(groups, group.ID)
	if !found {
		t.Fatalf("groups = %+v, want the group standing after the walk", groups)
	}
	keys := make(map[string]content.Field, len(held.Fields))
	for _, f := range held.Fields {
		keys[f.Key] = f
	}
	if _, stands := keys["style"]; stands {
		t.Errorf("fields = %v, want the choice dropped by the rollback", keys)
	}
	if _, stands := keys["subtitle"]; !stands {
		t.Errorf("fields = %v, want the text field standing", keys)
	}
	if kept, stands := keys["gallery"]; !stands || kept.Many {
		t.Errorf("fields = %v, want the gallery standing and holding one", keys)
	}
}
