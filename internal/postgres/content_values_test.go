// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// declareField adds a field definition to the post type so its items may hold values under the key.
func declareField(t *testing.T, pool *pgxpool.Pool, key string, kind content.FieldKind) {
	t.Helper()
	types := postgres.NewTypeStore(pool)
	if _, err := types.CreateField(t.Context(), fieldOn(t, "post", key, kind, "")); err != nil {
		t.Fatalf("declaring the %q field: %v, want nil", key, err)
	}
}

func TestContentStoreWaitsForAFieldDeletionInFlight(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "color", content.FieldKindText)
	created := mustCreate(t, store, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	removing, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("opening the removing transaction: %v, want nil", err)
	}
	defer func() { _ = removing.Rollback(context.Background()) }()
	if _, err := removing.Exec(
		t.Context(), `DELETE FROM core.content_fields WHERE key = 'color'`,
	); err != nil {
		t.Fatalf("removing the definition: %v, want nil", err)
	}
	written := make(chan error, 1)

	go func() {
		_, err := store.Update(context.Background(), created, created.CreatedAt, nil, 0)
		written <- err
	}()

	select {
	case err := <-written:
		t.Fatalf("Update() returned %v while the definition was being removed, want it waiting", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := removing.Commit(t.Context()); err != nil {
		t.Fatalf("committing the removal: %v, want nil", err)
	}
	if err := <-written; !errors.Is(err, content.ErrUnknownField) {
		t.Fatalf("Update() error = %v, want %v once the field was gone", err, content.ErrUnknownField)
	}
}

func TestDeleteFieldInGroupWaitsForAContentWriteHoldingTheDefinition(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "color", content.FieldKindText)
	types := postgres.NewTypeStore(pool)
	group := groupHolding(t, types, "color")
	created := mustCreate(t, store, "Hello world", author)
	held, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("opening the holding transaction: %v, want nil", err)
	}
	defer func() { _ = held.Rollback(context.Background()) }()
	if _, err := held.Exec(
		t.Context(), `SELECT key FROM core.content_fields FOR KEY SHARE`,
	); err != nil {
		t.Fatalf("holding the field definitions: %v, want nil", err)
	}
	swept := make(chan error, 1)

	go func() { swept <- types.DeleteFieldInGroup(context.Background(), group, "color", nil) }()

	select {
	case err := <-swept:
		t.Fatalf("DeleteFieldInGroup() returned %v while a content write held the definitions, want it waiting", err)
	case <-time.After(300 * time.Millisecond):
	}
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	if _, err := store.Update(t.Context(), created, created.CreatedAt, nil, 0); err != nil {
		t.Fatalf("writing the value: %v, want nil", err)
	}
	if err := held.Commit(t.Context()); err != nil {
		t.Fatalf("committing the holding transaction: %v, want nil", err)
	}
	if err := <-swept; err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil once the write finished", err)
	}
	if got := storedFields(t, pool, created.Slug); got != `{}` {
		t.Errorf("the item holds %s, want the sweep to have caught the value the write added", got)
	}
}
