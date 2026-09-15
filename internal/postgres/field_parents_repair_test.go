// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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

// parentOfField reads the field row the field stands under.
func parentOfField(t *testing.T, pool *pgxpool.Pool, id int) int {
	t.Helper()
	var held int
	if err := pool.QueryRow(t.Context(),
		`SELECT parent_field_id FROM core.content_fields WHERE id = $1`, id).Scan(&held); err != nil {
		t.Fatalf("reading the parent of field %d: %v, want nil", id, err)
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

// sectionAcrossGroups stores two groups on the car type and a section in the second, returning them in that order.
func sectionAcrossGroups(t *testing.T, store *postgres.TypeStore) (content.Group, content.Group, content.Field) {
	t.Helper()
	source, err := store.CreateGroup(t.Context(), content.Group{Title: "Details", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Details) error = %v, want nil", err)
	}
	landing, err := store.CreateGroup(t.Context(), content.Group{Title: "Elsewhere", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Elsewhere) error = %v, want nil", err)
	}
	section, err := store.CreateFieldInGroup(
		t.Context(), landing.ID, fieldOn(t, "", "author", content.FieldKindSection, ""))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(author) error = %v, want nil", err)
	}
	return source, landing, section
}

func TestDeletingAGroupDropsTheTwinItStoresInsideAnotherGroupsContainer(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	source, landing, section := sectionAcrossGroups(t, store)
	name := declaredInside(t, store, section, "name", content.FieldKindText)
	plantTwin(t, pool, source.ID, section.ID, "name", string(content.FieldKindText), 1)
	plantTyped(t, pool, author, "car", "one-car", `{"author": {"name": "Maria Perez"}}`)

	if err := store.DeleteGroup(t.Context(), source.ID); err != nil {
		t.Fatalf("DeleteGroup(Details) error = %v, want nil", err)
	}

	if held := fieldsKeyed(t, pool, "name"); held != 1 {
		t.Fatalf("%d fields keyed name, want only the twin the section's own group stores", held)
	}
	if held := groupOfField(t, pool, name.ID); held != landing.ID {
		t.Errorf("name sits in group %d, want the twin declared inside the section kept in %d", held, landing.ID)
	}
	if held := storedFields(t, pool, "one-car"); held != `{"author": {"name": "Maria Perez"}}` {
		t.Errorf("car fields = %s, want the value the kept twin reads left alone", held)
	}
}

func TestDeletingAGroupFoldsWhatOnlyItsTwinContainerHolds(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	source, landing, section := sectionAcrossGroups(t, store)
	profile := declaredInside(t, store, section, "profile", content.FieldKindSection)
	declaredInside(t, store, profile, "title", content.FieldKindText)
	twin := plantTwin(t, pool, source.ID, section.ID, "profile", string(content.FieldKindSection), 1)
	bio := plantTwin(t, pool, source.ID, twin, "bio", string(content.FieldKindText), 2)
	plantTwin(t, pool, source.ID, twin, "title", string(content.FieldKindText), 2)
	values := `{"author": {"profile": {"bio": "Maria Perez", "title": "Maria Perez"}}}`
	plantTyped(t, pool, author, "car", "one-car", values)

	if err := store.DeleteGroup(t.Context(), source.ID); err != nil {
		t.Fatalf("DeleteGroup(Details) error = %v, want nil", err)
	}

	for _, key := range []string{"profile", "bio", "title"} {
		if held := fieldsKeyed(t, pool, key); held != 1 {
			t.Errorf("%d fields keyed %s, want one left standing", held, key)
		}
	}
	if held := parentOfField(t, pool, bio); held != profile.ID {
		t.Errorf("bio stands under field %d, want it moved under the profile kept, %d", held, profile.ID)
	}
	if held := groupOfField(t, pool, bio); held != landing.ID {
		t.Errorf("bio sits in group %d, want it carried into %d with its section", held, landing.ID)
	}
	if held := storedFields(t, pool, "one-car"); held != values {
		t.Errorf("car fields = %s, want the values the kept fields read left alone", held)
	}
	if held := keysByPosition(t, pool, profile.ID); !slices.Equal(held, []string{"title@1", "bio@2"}) {
		t.Errorf("the kept profile lists %v, want bio landing after title", held)
	}
}

// keysByPosition returns the key and position of every field standing under the parent, in listing order.
func keysByPosition(t *testing.T, pool *pgxpool.Pool, parent int) []string {
	t.Helper()
	rows, err := pool.Query(t.Context(),
		`SELECT key || '@' || position FROM core.content_fields WHERE parent_field_id = $1 ORDER BY position, id`,
		parent)
	if err != nil {
		t.Fatalf("listing the fields under %d: %v, want nil", parent, err)
	}
	held, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("reading the fields under %d: %v, want nil", parent, err)
	}
	return held
}

// itemSlugged returns the identity of the item carrying the slug.
func itemSlugged(t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(t.Context(), `SELECT id::text FROM core.content WHERE slug = $1`, slug).Scan(&id); err != nil {
		t.Fatalf("reading the item %s: %v, want nil", slug, err)
	}
	return id
}

func TestDeletingAGroupFoldsTwoTwinsIntoOneKeptFieldOnce(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	source, landing, section := sectionAcrossGroups(t, store)
	profile := declaredInside(t, store, section, "profile", content.FieldKindSection)
	declaredInside(t, store, profile, "bio", content.FieldKindSection)
	twin := plantTwin(t, pool, source.ID, section.ID, "profile", string(content.FieldKindSection), 1)
	carried := plantTwin(t, pool, landing.ID, twin, "bio", string(content.FieldKindSection), 2)
	plantTwin(t, pool, landing.ID, carried, "note", string(content.FieldKindText), 3)
	left := plantTwin(t, pool, source.ID, twin, "bio", string(content.FieldKindSection), 2)
	plantTwin(t, pool, source.ID, left, "note", string(content.FieldKindText), 3)

	if err := store.DeleteGroup(t.Context(), source.ID); err != nil {
		t.Fatalf("DeleteGroup(Details) error = %v, want nil", err)
	}

	for _, key := range []string{"profile", "bio", "note"} {
		if held := fieldsKeyed(t, pool, key); held != 1 {
			t.Errorf("%d fields keyed %s, want one left standing", held, key)
		}
	}
}

func TestDeletingAGroupKeepsTheItemsADroppedRelationTwinIndexed(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "category")
	source, landing, section := sectionAcrossGroups(t, store)
	kept := plantTwin(t, pool, landing.ID, section.ID, "wrote", string(content.FieldKindRelation), 1)
	twin := plantTwin(t, pool, source.ID, section.ID, "wrote", string(content.FieldKindRelation), 1)
	plantTyped(t, pool, author, "car", "one-car", `{}`)
	plantTyped(t, pool, author, "car", "two-car", `{}`)
	indexUnder(t, pool, twin, "two-car")

	if err := store.DeleteGroup(t.Context(), source.ID); err != nil {
		t.Fatalf("DeleteGroup(Details) error = %v, want nil", err)
	}

	if held := indexRowsOf(t, pool, kept); !slices.Equal(held, []string{"3@2020-01-02@false"}) {
		t.Errorf("index rows under the kept twin = %v, want the row the dropped twin held as it stood", held)
	}
}

func TestDeletingAGroupKeepsAnIndexRowBothTwinsHold(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "category")
	source, landing, section := sectionAcrossGroups(t, store)
	kept := plantTwin(t, pool, landing.ID, section.ID, "wrote", string(content.FieldKindRelation), 1)
	twin := plantTwin(t, pool, source.ID, section.ID, "wrote", string(content.FieldKindRelation), 1)
	plantTyped(t, pool, author, "car", "one-car", `{}`)
	plantTyped(t, pool, author, "car", "two-car", `{}`)
	indexUnder(t, pool, kept, "two-car")
	indexUnder(t, pool, twin, "two-car")

	if err := store.DeleteGroup(t.Context(), source.ID); err != nil {
		t.Fatalf("DeleteGroup(Details) error = %v, want nil", err)
	}

	if held := indexRowsOf(t, pool, kept); !slices.Equal(held, []string{"3@2020-01-02@false"}) {
		t.Errorf("index rows under the kept twin = %v, want the one row both twins held", held)
	}
}

// indexUnder stores an index row under the field pointing from the one car at the target.
func indexUnder(t *testing.T, pool *pgxpool.Pool, field int, to string) {
	t.Helper()
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO core.content_relations (from_id, field_id, to_id, position, sort_at, visible)
		VALUES ($1, $2, $3, 3, '2020-01-02T00:00:00Z', false)`,
		itemSlugged(t, pool, "one-car"), field, itemSlugged(t, pool, to)); err != nil {
		t.Fatalf("indexing the one car under field %d: %v, want nil", field, err)
	}
}

// indexRowsOf returns the position, ordering day and visibility of every index row under the field.
func indexRowsOf(t *testing.T, pool *pgxpool.Pool, field int) []string {
	t.Helper()
	rows, err := pool.Query(t.Context(),
		`SELECT position || '@' || to_char(sort_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') || '@' || visible
		FROM core.content_relations WHERE field_id = $1 ORDER BY position, to_id`, field)
	if err != nil {
		t.Fatalf("listing the index rows under %d: %v, want nil", field, err)
	}
	held, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("reading the index rows under %d: %v, want nil", field, err)
	}
	return held
}

func TestDeletingAGroupFoldsTwinsInsideItsTwinContainerOnce(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	source, _, section := sectionAcrossGroups(t, store)
	third, err := store.CreateGroup(t.Context(), content.Group{Title: "Third", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Third) error = %v, want nil", err)
	}
	profile := declaredInside(t, store, section, "profile", content.FieldKindSection)
	twin := plantTwin(t, pool, source.ID, section.ID, "profile", string(content.FieldKindSection), 1)
	plantTwin(t, pool, source.ID, twin, "tag", string(content.FieldKindText), 2)
	plantTwin(t, pool, third.ID, twin, "tag", string(content.FieldKindText), 2)

	if err := store.DeleteGroup(t.Context(), source.ID); err != nil {
		t.Fatalf("DeleteGroup(Details) error = %v, want nil", err)
	}

	var parents []int
	rows, err := pool.Query(t.Context(), `SELECT parent_field_id FROM core.content_fields WHERE key = 'tag'`)
	if err != nil {
		t.Fatalf("reading the tags: %v, want nil", err)
	}
	for rows.Next() {
		var parent int
		if err := rows.Scan(&parent); err != nil {
			t.Fatalf("scanning a tag: %v, want nil", err)
		}
		parents = append(parents, parent)
	}
	if len(parents) != 1 || parents[0] != profile.ID {
		t.Errorf("tags stand under %v, want one tag under the profile kept, %d", parents, profile.ID)
	}
}

func TestDeletingAGroupCarriesTheFieldsItStoresInsideAnotherGroupsContainer(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	source, landing, section := sectionAcrossGroups(t, store)
	rows := declaredInside(t, store, section, "rows", content.FieldKindRepeater)
	title := declaredInside(t, store, rows, "title", content.FieldKindText)
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content_fields SET group_id = $1 WHERE id = $2 OR id = $3`,
		source.ID, rows.ID, title.ID); err != nil {
		t.Fatalf("leaving the repeater behind, as a move before the repair did: %v, want nil", err)
	}
	plantTyped(t, pool, author, "car", "one-car", `{"author": {"rows": [{"title": "Maria Perez"}]}}`)

	if err := store.DeleteGroup(t.Context(), source.ID); err != nil {
		t.Fatalf("DeleteGroup(Details) error = %v, want nil", err)
	}

	for _, carried := range []struct {
		what string
		id   int
	}{{"the repeater", rows.ID}, {"the field inside the repeater", title.ID}} {
		if held := groupOfField(t, pool, carried.id); held != landing.ID {
			t.Errorf("%s sits in group %d, want it carried into %d with its section", carried.what, held, landing.ID)
		}
	}
	if held := storedFields(t, pool, "one-car"); held != `{"author": {"rows": [{"title": "Maria Perez"}]}}` {
		t.Errorf("car fields = %s, want the rows the carried fields read left alone", held)
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
