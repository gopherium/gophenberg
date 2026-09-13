// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// plantTwin stores a field row of the kind under the parent in the group and returns its identity.
func plantTwin(t *testing.T, pool *pgxpool.Pool, groupID, parentID int, key, kind string, depth int) int {
	t.Helper()
	var relatesTo *string
	if kind == string(content.FieldKindRelation) {
		category := "category"
		relatesTo = &category
	}
	var id int
	err := pool.QueryRow(t.Context(),
		`INSERT INTO core.content_fields
			(group_id, parent_field_id, key, label, kind, relates_to, many, depth, created_at, updated_at)
		VALUES ($1, $2, $3, $3, $4, $5, $6, $7, now(), now())
		RETURNING id`,
		groupID, parentID, key, kind, relatesTo, relatesTo != nil, depth).Scan(&id)
	if err != nil {
		t.Fatalf("planting %s under field %d: %v, want nil", key, parentID, err)
	}
	return id
}

// fieldsKeyed counts the field rows carrying the key.
func fieldsKeyed(t *testing.T, pool *pgxpool.Pool, key string) int {
	t.Helper()
	var held int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM core.content_fields WHERE key = $1`, key).Scan(&held); err != nil {
		t.Fatalf("counting the fields keyed %s: %v, want nil", key, err)
	}
	return held
}

// rerunRepair runs the repair migration again on the database behind the pool.
func rerunRepair(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	url := pool.Config().ConnString()
	if err := postgres.MigrateDownTo(t.Context(), url, 23); err != nil {
		t.Fatalf("rolling back to before the repair: %v, want nil", err)
	}
	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}
}

func TestMigrationsKeepTheFieldsInsideAContainerLeftTwice(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	source, err := store.CreateGroup(t.Context(), content.Group{Title: "Details", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Details) error = %v, want nil", err)
	}
	author, err := store.CreateFieldInGroup(
		t.Context(), source.ID, fieldOn(t, "", "author", content.FieldKindSection, ""))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(author) error = %v, want nil", err)
	}
	profile := declaredInside(t, store, author, "profile", content.FieldKindSection)
	bio := declaredInside(t, store, profile, "bio", content.FieldKindText)
	landing, err := store.CreateGroup(t.Context(), content.Group{Title: "Elsewhere", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Elsewhere) error = %v, want nil", err)
	}
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content_fields SET group_id = $1 WHERE id = $2`, landing.ID, author.ID); err != nil {
		t.Fatalf("moving the section alone, as the old move did: %v, want nil", err)
	}
	twin := plantTwin(t, pool, landing.ID, author.ID, "profile", string(content.FieldKindSection), 1)
	plantTwin(t, pool, landing.ID, twin, "title", string(content.FieldKindText), 2)

	rerunRepair(t, pool)

	for _, key := range []string{"bio", "title"} {
		if held := fieldsKeyed(t, pool, key); held != 1 {
			t.Errorf("%d fields keyed %s, want the one declared inside a profile kept", held, key)
		}
	}
	if held := fieldsKeyed(t, pool, "profile"); held != 2 {
		t.Errorf("%d fields keyed profile, want both twins left for the editor to settle", held)
	}
	if held := groupOfField(t, pool, bio.ID); held != landing.ID {
		t.Errorf("bio sits in group %d, want it carried into %d with the section at the top", held, landing.ID)
	}
	if held := groupOfField(t, pool, profile.ID); held != source.ID {
		t.Errorf("the twin left behind sits in group %d, want it left alone in %d", held, source.ID)
	}
}

func TestMigrationsKeepWhatARelationLeftTwicePointsAt(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	rowsPointing(t, pool)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	post := mustCreate(t, store, "Filed", author)
	post.Fields = rowsFiledUnder(news.ID)
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()
	filed, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("filing the post through the row: %v, want nil", err)
	}
	publishItem(t, store, filed)
	var team int
	if err := pool.QueryRow(t.Context(),
		`SELECT id FROM core.content_fields WHERE key = 'team' AND parent_field_id IS NULL`).Scan(&team); err != nil {
		t.Fatalf("reading the repeater: %v, want nil", err)
	}
	landing, err := postgres.NewTypeStore(pool).CreateGroup(
		t.Context(), content.Group{Title: "Elsewhere", Location: locationOf("post")})
	if err != nil {
		t.Fatalf("CreateGroup(Elsewhere) error = %v, want nil", err)
	}
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content_fields SET group_id = $1 WHERE id = $2`, landing.ID, team); err != nil {
		t.Fatalf("moving the repeater alone, as the old move did: %v, want nil", err)
	}
	plantTwin(t, pool, landing.ID, team, "filed", string(content.FieldKindRelation), 1)

	rerunRepair(t, pool)

	if pointing := pointedAtBy(t, store, news.ID); pointing != 1 {
		t.Errorf("%d items point at the category, want the filed post still found", pointing)
	}
	if held := fieldsKeyed(t, pool, "filed"); held != 2 {
		t.Errorf("%d fields keyed filed, want both twins left for the editor to settle", held)
	}
}

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

	rerunRepair(t, pool)

	for _, carried := range []struct {
		what string
		id   int
	}{{"the repeater", rows.ID}, {"the field inside the repeater", title.ID}} {
		if held := groupOfField(t, pool, carried.id); held != landing.ID {
			t.Errorf("%s sits in group %d, want it carried into %d with its section", carried.what, held, landing.ID)
		}
	}
	if held := fieldsKeyed(t, pool, "name"); held != 2 {
		t.Errorf("%d fields named name, want both twins left for the editor to settle", held)
	}
	if held := groupOfField(t, pool, name.ID); held != source.ID {
		t.Errorf("the twin left behind sits in group %d, want it left alone in %d", held, source.ID)
	}
	if held := groupOfField(t, pool, twin); held != landing.ID {
		t.Errorf("the twin added later sits in group %d, want it left alone in %d", held, landing.ID)
	}
}
