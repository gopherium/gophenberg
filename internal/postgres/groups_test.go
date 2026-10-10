// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// typedStore returns a type store, its pool and a seeded author over a migrated database.
func typedStore(t *testing.T) (*postgres.TypeStore, uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	_, author, pool := newContentStoreWithPool(t)
	return postgres.NewTypeStore(pool), author, pool
}

// storeType, declareTypedField, locationOf and groupHolding are the shared fixtures under the names these tests use.
var (
	storeType         = contenttest.StoreType
	declareTypedField = contenttest.DeclareTypedField
	locationOf        = contenttest.LocationOf
	groupHolding      = contenttest.GroupHolding
)

// createCarField stores a text field on the car type through the per type CreateField.
func createCarField(t *testing.T, store *postgres.TypeStore, key string) content.Field {
	t.Helper()
	stored, err := store.CreateField(t.Context(), fieldOn(t, "car", key, content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateField(%s on car) error = %v, want nil", key, err)
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

func TestCreateFieldRaisesADefaultGroupOnDemand(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")

	declared := createCarField(t, store, "subtitle")

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
	createCarField(t, store, "subtitle")

	createCarField(t, store, "mileage")

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

func TestUpdateGroupWaitsForAGroupWriteAlreadyUnderway(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	group, err := store.CreateGroup(t.Context(), content.Group{Title: "Cars", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	holding, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatalf("Begin() error = %v, want nil", err)
	}
	defer func() { _ = holding.Rollback(context.Background()) }()
	if _, err := holding.Exec(
		context.Background(), "SELECT pg_advisory_xact_lock(hashtext('core.field_groups'))",
	); err != nil {
		t.Fatalf("taking the lock: %v", err)
	}

	group.Title = "Motors"
	done := make(chan error, 1)
	go func() { _, err := store.UpdateGroup(context.Background(), group, nil, nil); done <- err }()

	select {
	case err := <-done:
		t.Fatalf("UpdateGroup() answered %v while another write held the lock, want it waiting", err)
	case <-time.After(250 * time.Millisecond):
	}
	if err := holding.Rollback(context.Background()); err != nil {
		t.Fatalf("Rollback() error = %v, want nil", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil once the lock is free", err)
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
