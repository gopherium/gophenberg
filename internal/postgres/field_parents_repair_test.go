// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestMigrationsCarrySubFieldsIntoTheirContainersGroup(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	source, err := store.CreateGroup(t.Context(), content.Group{Title: "Details", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Details) error = %v, want nil", err)
	}
	section, err := store.CreateFieldInGroup(
		t.Context(), source.ID, fieldOn(t, "", "author", content.FieldKindSection, ""))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(author) error = %v, want nil", err)
	}
	name := declaredInside(t, store, section, "name", content.FieldKindText)
	rows := declaredInside(t, store, section, "rows", content.FieldKindRepeater)
	title := declaredInside(t, store, rows, "title", content.FieldKindText)
	landing, err := store.CreateGroup(t.Context(), content.Group{Title: "Elsewhere", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Elsewhere) error = %v, want nil", err)
	}
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content_fields SET group_id = $1 WHERE id = $2`, landing.ID, section.ID); err != nil {
		t.Fatalf("moving the section alone, as the old move did: %v, want nil", err)
	}
	twin, planted := plantSubField(t, pool, landing.ID, &section.ID, "name")
	if planted != nil {
		t.Fatalf("planting a second name under the moved section: %v, want nil", planted)
	}
	url := pool.Config().ConnString()
	if err := postgres.MigrateDownTo(t.Context(), url, 23); err != nil {
		t.Fatalf("rolling back to before the repair: %v, want nil", err)
	}

	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	for _, carried := range []struct {
		what string
		id   int
	}{{"the repeater", rows.ID}, {"the field inside the repeater", title.ID}} {
		if held := groupOfField(t, pool, carried.id); held != landing.ID {
			t.Errorf("%s sits in group %d, want it carried into %d with its section", carried.what, held, landing.ID)
		}
	}
	var twins int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM core.content_fields WHERE parent_field_id = $1 AND key = 'name'`,
		section.ID).Scan(&twins); err != nil {
		t.Fatalf("counting the fields named name under the section: %v, want nil", err)
	}
	if twins != 1 {
		t.Errorf("the section holds %d fields named name, want the twins collapsed to one", twins)
	}
	if held := groupOfField(t, pool, twin); held != landing.ID {
		t.Errorf("the surviving name sits in group %d, want the one already in %d kept", held, landing.ID)
	}
	var stale int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM core.content_fields WHERE id = $1`, name.ID).Scan(&stale); err != nil {
		t.Fatalf("looking for the stale name: %v, want nil", err)
	}
	if stale != 0 {
		t.Errorf("the stale name still exists, want the twin left in the old group taken away")
	}
}
