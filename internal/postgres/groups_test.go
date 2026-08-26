// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// typedStore returns a type store, its pool and a seeded author over a migrated database.
func typedStore(t *testing.T) (*postgres.TypeStore, uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	_, author, pool := newContentStoreWithPool(t)
	return postgres.NewTypeStore(pool), author, pool
}

// storeType stores a content type built from the key.
func storeType(t *testing.T, store *postgres.TypeStore, key string) {
	t.Helper()
	built, err := content.NewType(key, "One "+key, "Many "+key, key+"s")
	if err != nil {
		t.Fatalf("NewType(%s) error = %v, want nil", key, err)
	}
	if _, err := store.Create(t.Context(), built); err != nil {
		t.Fatalf("Create(%s) error = %v, want nil", key, err)
	}
}

// declareTypedField stores a field on the type through the per type surface.
func declareTypedField(t *testing.T, store *postgres.TypeStore, typeKey, key string) content.Field {
	t.Helper()
	stored, err := store.CreateField(t.Context(), fieldOn(t, typeKey, key, content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateField(%s on %s) error = %v, want nil", key, typeKey, err)
	}
	return stored
}

// plantTyped stores a content row of the type and one revision, both carrying raw field values.
func plantTyped(t *testing.T, pool *pgxpool.Pool, author uuid.UUID, typeKey, slug, values string) {
	t.Helper()
	ctx := t.Context()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.content
		(id, type, status, slug, path, title, content, excerpt, author_id, created_at, updated_at, fields)
		VALUES ($1, $2, 'draft', $3, $3, 'Planted', '', '', $4, now(), now(), $5)`,
		id, typeKey, slug, author, values,
	); err != nil {
		t.Fatalf("planting the %s row: %v, want nil", typeKey, err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO core.content_revisions
		(id, content_id, kind, author_id, title, content, excerpt, created_at, fields)
		VALUES ($1, $2, 'revision', $3, 'Planted', '', '', now(), $4)`,
		uuid.Must(uuid.NewV7()), id, author, values,
	); err != nil {
		t.Fatalf("planting the %s revision: %v, want nil", typeKey, err)
	}
}

// locationOf returns a one rule location naming the type.
func locationOf(typeKey string) content.Rules {
	return content.Rules{{{Source: "content_type", Operator: content.OperatorIs, Value: typeKey}}}
}

func TestCreateFieldRaisesADefaultGroupOnDemand(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")

	declared := declareTypedField(t, store, "car", "subtitle")

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	if len(groups) != 1 {
		t.Fatalf("ListGroups() holds %d groups, want the one raised on demand", len(groups))
	}
	raised := groups[0]
	if raised.Title != "One car fields" {
		t.Errorf("Title = %q, want the singular label plus fields", raised.Title)
	}
	if len(raised.Location) != 1 || len(raised.Location[0]) != 1 || raised.Location[0][0] != locationOf("car")[0][0] {
		t.Errorf("Location = %v, want one rule naming the type", raised.Location)
	}
	if !raised.Active {
		t.Error("Active = false, want a raised group active")
	}
	if len(raised.Fields) != 1 || raised.Fields[0].Key != "subtitle" {
		t.Errorf("Fields = %v, want the declared field inside", raised.Fields)
	}
	if declared.GroupID != raised.ID {
		t.Errorf("GroupID = %d, want the raised group %d", declared.GroupID, raised.ID)
	}
}

func TestCreateFieldJoinsTheGroupAlreadyNamingTheType(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")

	declareTypedField(t, store, "car", "mileage")

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	if len(groups) != 1 {
		t.Fatalf("ListGroups() holds %d groups, want both fields joining one group", len(groups))
	}
	if len(groups[0].Fields) != 2 || groups[0].Fields[1].Key != "mileage" {
		t.Errorf("Fields = %v, want mileage second by position", groups[0].Fields)
	}
}

func TestByKeyFlattensTheFieldsOfMatchingGroups(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")
	extras, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := store.CreateFieldInGroup(
		t.Context(), extras.ID, fieldOn(t, "", "trim", content.FieldKindText, ""),
	); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}

	held, err := store.ByKey(t.Context(), "car")

	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 2 || held.Fields[0].Key != "subtitle" || held.Fields[1].Key != "trim" {
		t.Fatalf("Fields = %v, want both groups flattened in group order", held.Fields)
	}
	if held.Fields[1].TypeKey != "car" {
		t.Errorf("TypeKey = %q, want the flattened field carrying its type", held.Fields[1].TypeKey)
	}
}

func TestAGroupThatStopsMatchingStopsServingItsFields(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")
	groups, err := store.ListGroups(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("ListGroups() = %v, %v, want the one raised group", groups, err)
	}
	idle := groups[0]
	idle.Active = false

	if _, err := store.UpdateGroup(t.Context(), idle); err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}

	held, err := store.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 0 {
		t.Errorf("Fields = %v, want an inactive group serving nothing", held.Fields)
	}
	listed, err := store.ListGroups(t.Context())
	if err != nil || len(listed) != 1 || len(listed[0].Fields) != 1 {
		t.Errorf("ListGroups() = %v, %v, want the inactive group still listed whole", listed, err)
	}
}

func TestTheAnyRuleServesAGroupOnEveryType(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	shared, err := store.CreateGroup(t.Context(), content.Group{
		Title:    "Everywhere",
		Location: content.Rules{{{Source: "content_type", Operator: content.OperatorIs, Value: content.AnyContentType}}},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := store.CreateFieldInGroup(
		t.Context(), shared.ID, fieldOn(t, "", "footer", content.FieldKindText, ""),
	); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}

	for _, typeKey := range []string{"car", "book"} {
		held, err := store.ByKey(t.Context(), typeKey)
		if err != nil {
			t.Fatalf("ByKey(%s) error = %v, want nil", typeKey, err)
		}
		if len(held.Fields) != 1 || held.Fields[0].Key != "footer" {
			t.Errorf("ByKey(%s).Fields = %v, want the any group's field served", typeKey, held.Fields)
		}
	}
}

func TestDeleteFieldSweepsOnlyTheTypesItsGroupMatches(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	declareTypedField(t, store, "car", "subtitle")
	declareTypedField(t, store, "book", "subtitle")
	plantTyped(t, pool, author, "car", "one-car", `{"subtitle": "car words"}`)
	plantTyped(t, pool, author, "book", "one-book", `{"subtitle": "book words"}`)

	if err := store.DeleteField(t.Context(), "car", "subtitle"); err != nil {
		t.Fatalf("DeleteField() error = %v, want nil", err)
	}

	if held := storedFields(t, pool, "one-car"); held != "{}" {
		t.Errorf("car fields = %s, want the deleted field's value swept", held)
	}
	if held := storedFields(t, pool, "one-book"); held != `{"subtitle": "book words"}` {
		t.Errorf("book fields = %s, want the same named field of another type untouched", held)
	}
}

func TestDeleteGroupTakesItsFieldsAndValuesInOneSweep(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")
	declareTypedField(t, store, "car", "mileage")
	plantTyped(t, pool, author, "car", "one-car", `{"subtitle": "kept words", "mileage": 9}`)
	groups, err := store.ListGroups(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("ListGroups() = %v, %v, want the one raised group", groups, err)
	}

	if err := store.DeleteGroup(t.Context(), groups[0].ID); err != nil {
		t.Fatalf("DeleteGroup() error = %v, want nil", err)
	}

	if held := storedFields(t, pool, "one-car"); held != "{}" {
		t.Errorf("fields = %s, want every value of the group swept", held)
	}
	if listed, err := store.ListGroups(t.Context()); err != nil || len(listed) != 0 {
		t.Errorf("ListGroups() = %v, %v, want no group left", listed, err)
	}
	held, err := store.ByKey(t.Context(), "car")
	if err != nil || len(held.Fields) != 0 {
		t.Errorf("ByKey() = %v, %v, want the type left fieldless", held.Fields, err)
	}
}

func TestDeleteGroupReportsAMissingGroup(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)

	err := store.DeleteGroup(t.Context(), 12345)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("DeleteGroup() = %v, want %v", err, content.ErrGroupNotFound)
	}
}

func TestReorderGroupsSettlesTheFlattenedOrder(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")
	extras, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := store.CreateFieldInGroup(
		t.Context(), extras.ID, fieldOn(t, "", "trim", content.FieldKindText, ""),
	); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	groups, err := store.ListGroups(t.Context())
	if err != nil || len(groups) != 2 {
		t.Fatalf("ListGroups() = %v, %v, want both groups", groups, err)
	}

	if err := store.ReorderGroups(t.Context(), []int{groups[1].ID, groups[0].ID}); err != nil {
		t.Fatalf("ReorderGroups() error = %v, want nil", err)
	}

	held, err := store.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 2 || held.Fields[0].Key != "trim" {
		t.Errorf("Fields = %v, want the reordered group's field first", held.Fields)
	}
}

func TestMigrationCarriesFieldsIntoDefaultGroups(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	url := pool.Config().ConnString()
	if err := postgres.MigrateDownTo(t.Context(), url, 12); err != nil {
		t.Fatalf("MigrateDownTo(12) error = %v, want nil", err)
	}
	for _, seed := range []string{
		`INSERT INTO core.content_types
		(key, singular_label, plural_label, route_word, page_kind, created_at, updated_at)
		VALUES ('car', 'One car', 'Many car', 'cars', 'single', now(), now())`,
		`INSERT INTO core.content_fields
		(type_key, key, label, kind, many, required, position, created_at, updated_at)
		VALUES ('car', 'subtitle', 'Subtitle', 'text', false, true, 1, now(), now()),
		       ('car', 'mileage', 'Mileage', 'number', false, false, 2, now(), now())`,
	} {
		if _, err := pool.Exec(t.Context(), seed); err != nil {
			t.Fatalf("seeding the pre migration state: %v, want nil", err)
		}
	}

	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	if len(groups) != 1 {
		t.Fatalf("ListGroups() holds %d groups, want one default group for the fielded type", len(groups))
	}
	carried := groups[0]
	if carried.Title != "One car fields" {
		t.Errorf("Title = %q, want the singular label plus fields", carried.Title)
	}
	if len(carried.Fields) != 2 || carried.Fields[0].Key != "subtitle" || carried.Fields[1].Key != "mileage" {
		t.Errorf("Fields = %v, want both fields carried in their stored order", carried.Fields)
	}
	held, err := store.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 2 || held.Fields[0].Key != "subtitle" || held.Fields[0].Required != true {
		t.Errorf("flattened fields = %v, want the migrated fields served as before", held.Fields)
	}
}
