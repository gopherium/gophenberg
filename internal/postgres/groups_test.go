// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// typedStore returns a type store, its pool and a seeded author over a migrated database.
func typedStore(t *testing.T) (*postgres.TypeStore, uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	_, author, pool := newContentStoreWithPool(t)
	return postgres.NewTypeStore(pool), author, pool
}

// fieldOn, storeType, declareTypedField, declareOnType, fieldsGroupOf, locationOf and groupHolding are shared fixtures.
var (
	fieldOn           = contenttest.FieldOn
	storeType         = contenttest.StoreType
	declareTypedField = contenttest.DeclareTypedField
	declareOnType     = contenttest.DeclareOnType
	fieldsGroupOf     = contenttest.FieldsGroupOf
	locationOf        = contenttest.LocationOf
	groupHolding      = contenttest.GroupHolding
)

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

func TestUpdateGroupWritesNothingOnceTheRowMovedPastTheStampItHolds(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	read, err := store.CreateGroup(t.Context(), content.Group{Title: "Cars", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.field_groups SET title = 'Motors', updated_at = updated_at + interval '1 second' WHERE id = $1`,
		read.ID,
	); err != nil {
		t.Fatalf("moving the row past the stamp read: %v", err)
	}
	read.Title = "Vehicles"

	_, err = store.UpdateGroup(t.Context(), read, nil, nil)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateGroup() error = %v, want %v", err, content.ErrConflict)
	}
	if held := contenttest.GroupAt(t, store, read.ID); held.Title != "Motors" {
		t.Errorf("title = %q, want the row the other write left", held.Title)
	}
}

func TestMigrationCarriesFieldsIntoDefaultGroups(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	db := testdb.Open(t, pool.Config().ConnString())
	if err := postgres.MigrateDownTo(t.Context(), db, 12); err != nil {
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

	if err := postgres.Migrate(t.Context(), db); err != nil {
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
