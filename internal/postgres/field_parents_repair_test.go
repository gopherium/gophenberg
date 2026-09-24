// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// beforeTheSettle rolls the database behind the pool back to before the settle migration, so a twin can be stored.
func beforeTheSettle(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if err := postgres.MigrateDownTo(t.Context(), pool.Config().ConnString(), 24); err != nil {
		t.Fatalf("rolling back to before the settle: %v, want nil", err)
	}
}

// rerunRepair runs the repair migrations again on the database behind the pool.
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
		t.Context(), source.ID, fieldOn(t, "", "author", content.FieldKindSection, ""), nil)
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
	beforeTheSettle(t, pool)
	twin := plantTwin(t, pool, landing.ID, author.ID, "profile", string(content.FieldKindSection), 1)
	plantTwin(t, pool, landing.ID, twin, "title", string(content.FieldKindText), 2)

	rerunRepair(t, pool)

	for _, key := range []string{"bio", "title"} {
		if held := fieldsKeyed(t, pool, key); held != 1 {
			t.Errorf("%d fields keyed %s, want the one declared inside a profile kept", held, key)
		}
	}
	if held := soleFieldKeyed(t, pool, "profile"); held != twin {
		t.Errorf("the profile kept is %d, want the twin the section's group stores, %d", held, twin)
	}
	if held := parentOfField(t, pool, bio.ID); held != twin {
		t.Errorf("bio stands under field %d, want it folded under the profile kept, %d", held, twin)
	}
	if held := groupOfField(t, pool, bio.ID); held != landing.ID {
		t.Errorf("bio sits in group %d, want it carried into %d with the section at the top", held, landing.ID)
	}
	if held := fieldsKeyed(t, pool, "profile"); held != 1 || profile.ID == twin {
		t.Errorf("%d fields keyed profile, want the twin left behind in %d dropped", held, source.ID)
	}
}

// soleFieldKeyed returns the identity of the one field row carrying the key, failing when there is not exactly one.
func soleFieldKeyed(t *testing.T, pool *pgxpool.Pool, key string) int {
	t.Helper()
	if held := fieldsKeyed(t, pool, key); held != 1 {
		t.Fatalf("%d fields keyed %s, want exactly one", held, key)
	}
	var id int
	if err := pool.QueryRow(t.Context(),
		`SELECT id FROM core.content_fields WHERE key = $1`, key).Scan(&id); err != nil {
		t.Fatalf("reading the field keyed %s: %v, want nil", key, err)
	}
	return id
}

func TestMigrationsKeepTheTwinTheContainersGroupStores(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	source, landing, section := sectionAcrossGroups(t, store)
	name := declaredInside(t, store, section, "name", content.FieldKindText)
	beforeTheSettle(t, pool)
	plantTwin(t, pool, source.ID, section.ID, "name", string(content.FieldKindText), 1)
	plantTyped(t, pool, author, "car", "one-car", `{"author": {"name": "Maria Perez"}}`)

	rerunRepair(t, pool)

	if held := soleFieldKeyed(t, pool, "name"); held != name.ID {
		t.Errorf("the name kept is %d, want the twin declared inside the section, %d", held, name.ID)
	}
	if held := groupOfField(t, pool, name.ID); held != landing.ID {
		t.Errorf("name sits in group %d, want it in the section's group %d", held, landing.ID)
	}
	if held := storedFields(t, pool, "one-car"); held != `{"author": {"name": "Maria Perez"}}` {
		t.Errorf("car fields = %s, want the value the kept twin reads left alone", held)
	}
}

func TestMigrationsKeepTheOlderOfTwoTwinsStoredElsewhere(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	source, landing, section := sectionAcrossGroups(t, store)
	third, err := store.CreateGroup(t.Context(), content.Group{Title: "Third", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Third) error = %v, want nil", err)
	}
	beforeTheSettle(t, pool)
	older := plantTwin(t, pool, source.ID, section.ID, "name", string(content.FieldKindText), 1)
	plantTwin(t, pool, third.ID, section.ID, "name", string(content.FieldKindText), 1)

	rerunRepair(t, pool)

	if held := soleFieldKeyed(t, pool, "name"); held != older {
		t.Errorf("the name kept is %d, want the older twin, %d", held, older)
	}
	if held := groupOfField(t, pool, older); held != landing.ID {
		t.Errorf("name sits in group %d, want it carried into the section's group %d", held, landing.ID)
	}
}

func TestMigrationsFoldWhatOnlyATwinHoldsIntoTheTwinKept(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	source, landing, section := sectionAcrossGroups(t, store)
	profile := declaredInside(t, store, section, "profile", content.FieldKindSection)
	declaredInside(t, store, profile, "title", content.FieldKindText)
	beforeTheSettle(t, pool)
	twin := plantTwin(t, pool, source.ID, section.ID, "profile", string(content.FieldKindSection), 1)
	bio := plantTwin(t, pool, source.ID, twin, "bio", string(content.FieldKindText), 2)
	plantTwin(t, pool, source.ID, twin, "title", string(content.FieldKindText), 2)
	values := `{"author": {"profile": {"bio": "Maria Perez", "title": "Maria Perez"}}}`
	plantTyped(t, pool, author, "car", "one-car", values)

	rerunRepair(t, pool)

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

func TestMigrationsFoldTwinsStandingInsideTwinsOnce(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	source, landing, section := sectionAcrossGroups(t, store)
	profile := declaredInside(t, store, section, "profile", content.FieldKindSection)
	bio := declaredInside(t, store, profile, "bio", content.FieldKindSection)
	beforeTheSettle(t, pool)
	twin := plantTwin(t, pool, source.ID, section.ID, "profile", string(content.FieldKindSection), 1)
	carried := plantTwin(t, pool, landing.ID, twin, "bio", string(content.FieldKindSection), 2)
	plantTwin(t, pool, landing.ID, carried, "note", string(content.FieldKindText), 3)
	left := plantTwin(t, pool, source.ID, twin, "bio", string(content.FieldKindSection), 2)
	plantTwin(t, pool, source.ID, left, "note", string(content.FieldKindText), 3)

	rerunRepair(t, pool)

	for _, key := range []string{"profile", "bio", "note"} {
		if held := fieldsKeyed(t, pool, key); held != 1 {
			t.Errorf("%d fields keyed %s, want one left standing", held, key)
		}
	}
	if held := parentOfField(t, pool, soleFieldKeyed(t, pool, "note")); held != bio.ID {
		t.Errorf("note stands under field %d, want it under the bio the kept profile holds, %d", held, bio.ID)
	}
}

func TestMigrationsDropWhatATwinOfAnotherKindHolds(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	source, _, section := sectionAcrossGroups(t, store)
	profile := declaredInside(t, store, section, "profile", content.FieldKindText)
	beforeTheSettle(t, pool)
	twin := plantTwin(t, pool, source.ID, section.ID, "profile", string(content.FieldKindSection), 1)
	plantTwin(t, pool, source.ID, twin, "bio", string(content.FieldKindText), 2)

	rerunRepair(t, pool)

	if held := soleFieldKeyed(t, pool, "profile"); held != profile.ID {
		t.Errorf("the profile kept is %d, want the text field the section's group stores, %d", held, profile.ID)
	}
	if held := fieldsKeyed(t, pool, "bio"); held != 0 {
		t.Errorf("%d fields keyed bio, want what only a twin of another kind held dropped with it", held)
	}
}

func TestMigrationsKeepTheItemsADroppedRelationTwinIndexed(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "category")
	source, landing, section := sectionAcrossGroups(t, store)
	beforeTheSettle(t, pool)
	kept := plantTwin(t, pool, landing.ID, section.ID, "wrote", string(content.FieldKindRelation), 1)
	twin := plantTwin(t, pool, source.ID, section.ID, "wrote", string(content.FieldKindRelation), 1)
	plantTyped(t, pool, author, "car", "one-car", `{}`)
	plantTyped(t, pool, author, "car", "two-car", `{}`)
	indexUnder(t, pool, twin, "two-car")

	rerunRepair(t, pool)

	if held := soleFieldKeyed(t, pool, "wrote"); held != kept {
		t.Errorf("the relation kept is %d, want the twin the section's group stores, %d", held, kept)
	}
	if held := indexRowsOf(t, pool, kept); !slices.Equal(held, []string{"3@2020-01-02@false"}) {
		t.Errorf("index rows under the kept twin = %v, want the row the dropped twin held as it stood", held)
	}
}

func TestMigrationsFoldWhatTwoDroppedTwinsHoldIntoTheOneKept(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "category")
	source, _, section := sectionAcrossGroups(t, store)
	third, err := store.CreateGroup(t.Context(), content.Group{Title: "Third", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Third) error = %v, want nil", err)
	}
	profile := declaredInside(t, store, section, "profile", content.FieldKindSection)
	beforeTheSettle(t, pool)
	second := plantTwin(t, pool, source.ID, section.ID, "profile", string(content.FieldKindSection), 1)
	wroteSecond := plantTwin(t, pool, source.ID, second, "wrote", string(content.FieldKindRelation), 2)
	last := plantTwin(t, pool, third.ID, section.ID, "profile", string(content.FieldKindSection), 1)
	wroteLast := plantTwin(t, pool, third.ID, last, "wrote", string(content.FieldKindRelation), 2)
	for _, slug := range []string{"one-car", "two-car", "three-car"} {
		plantTyped(t, pool, author, "car", slug, `{}`)
	}
	indexUnder(t, pool, wroteSecond, "two-car")
	indexUnder(t, pool, wroteLast, "three-car")

	rerunRepair(t, pool)

	wrote := soleFieldKeyed(t, pool, "wrote")
	if held := parentOfField(t, pool, wrote); held != profile.ID {
		t.Errorf("wrote stands under field %d, want it under the profile kept, %d", held, profile.ID)
	}
	if held := indexRowsOf(t, pool, wrote); len(held) != 2 {
		t.Errorf("index rows under the wrote kept = %v, want the rows both dropped twins held", held)
	}
}

func TestMigrationsRefuseASecondSubFieldOfANameInsideOneContainer(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	source, _, section := sectionAcrossGroups(t, store)
	declaredInside(t, store, section, "name", content.FieldKindText)

	_, err := pool.Exec(t.Context(),
		`INSERT INTO core.content_fields (group_id, parent_field_id, key, label, kind, depth, created_at, updated_at)
		VALUES ($1, $2, 'name', 'Name', 'text', 1, now(), now())`, source.ID, section.ID)

	var refused *pgconn.PgError
	if !errors.As(err, &refused) || refused.Code != "23505" {
		t.Fatalf("storing a second name inside the section: %v, want the unique rule refusing it", err)
	}
	if refused.ConstraintName != "content_fields_container_key_unique" {
		t.Errorf("constraint = %q, want content_fields_container_key_unique", refused.ConstraintName)
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
	beforeTheSettle(t, pool)
	plantTwin(t, pool, landing.ID, team, "filed", string(content.FieldKindRelation), 1)

	rerunRepair(t, pool)

	if pointing := pointedAtBy(t, store, news.ID); pointing != 1 {
		t.Errorf("%d items point at the category, want the filed post still found", pointing)
	}
	if held := groupOfField(t, pool, soleFieldKeyed(t, pool, "filed")); held != landing.ID {
		t.Errorf("filed sits in group %d, want the twin the repeater's group stores kept in %d", held, landing.ID)
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
		t.Context(), landing.ID, fieldOn(t, "", "author", content.FieldKindSection, ""), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(author) error = %v, want nil", err)
	}
	return source, landing, section
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

	if err := store.DeleteGroup(t.Context(), source.ID, nil); err != nil {
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
		t.Context(), source.ID, fieldOn(t, "", "author", content.FieldKindSection, ""), nil)
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
	beforeTheSettle(t, pool)
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
	if held := soleFieldKeyed(t, pool, "name"); held != twin {
		t.Errorf("the name kept is %d, want the twin the section's group stores, %d, and %d dropped", held, twin, name.ID)
	}
	if held := groupOfField(t, pool, twin); held != landing.ID {
		t.Errorf("the name kept sits in group %d, want it in the section's group %d", held, landing.ID)
	}
}
