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

func TestContentStoreFreezesTheValuesOfAGroupThatStoppedMatching(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "color", content.FieldKindText)
	types := postgres.NewTypeStore(pool)
	created := mustCreate(t, store, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	stored, err := store.Update(t.Context(), created, created.CreatedAt, nil, 0)
	if err != nil {
		t.Fatalf("storing the value: %v, want nil", err)
	}
	groups, err := types.ListGroups(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("ListGroups() = %v, %v, want the one raised group", groups, err)
	}
	resting := groups[0]
	resting.Active = false
	if _, err := types.UpdateGroup(t.Context(), resting); err != nil {
		t.Fatalf("resting the group: %v, want nil", err)
	}

	was := stored.UpdatedAt
	stored.Title = "Renamed while the group rests"
	stored.UpdatedAt = time.Now().UTC()
	settled, err := store.Update(t.Context(), stored, was, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want the item still editable while its group rests", err)
	}
	if settled.Title != "Renamed while the group rests" {
		t.Errorf("Title = %q, want the edit stored", settled.Title)
	}
	if held := storedFields(t, pool, settled.Slug); held != `{"color": "red"}` {
		t.Errorf("fields = %s, want the resting group's value frozen rather than swept", held)
	}
}

func TestContentStoreCarriesFieldValues(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "color", content.FieldKindText)
	declareField(t, pool, "doors", content.FieldKindNumber)
	created := mustCreate(t, store, "Hello world", author)
	created.Fields = content.Values{"color": "red", "doors": float64(4)}
	created.UpdatedAt = time.Now().UTC()

	updated, err := store.Update(t.Context(), created, created.CreatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Fields["color"] != "red" {
		t.Errorf("Update() fields = %v, want the values carried back", updated.Fields)
	}
	held, err := store.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if held.Fields["color"] != "red" || held.Fields["doors"] != float64(4) {
		t.Errorf("ByID() fields = %v, want both values stored", held.Fields)
	}
}

func TestContentStoreRefusesAValueWhoseFieldIsGone(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "color", content.FieldKindText)
	created := mustCreate(t, store, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	types := postgres.NewTypeStore(pool)
	if err := types.DeleteFieldInGroup(t.Context(), groupHolding(t, types, "color"), "color"); err != nil {
		t.Fatalf("deleting the field: %v, want nil", err)
	}

	_, err := store.Update(t.Context(), created, created.CreatedAt, nil, 0)

	if !errors.Is(err, content.ErrUnknownField) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrUnknownField)
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

	go func() { swept <- types.DeleteFieldInGroup(context.Background(), group, "color") }()

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

func TestContentStoreCreatesWithTheFieldValuesGiven(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "color", content.FieldKindText)
	post := mustPost(t, "Hello world", author)
	post.Fields = content.Values{"color": "red"}

	created, err := store.Create(t.Context(), post)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Fields["color"] != "red" {
		t.Errorf("Create() fields = %v, want the values the caller gave", created.Fields)
	}
	if got := storedFields(t, pool, created.Slug); got != `{"color": "red"}` {
		t.Errorf("the item holds %s, want the values the caller gave", got)
	}
}

func TestContentStoreCreateRefusesAValueNoFieldDeclares(t *testing.T) {
	t.Parallel()

	store, author, _ := newContentStoreWithPool(t)
	post := mustPost(t, "Hello world", author)
	post.Fields = content.Values{"undeclared": "red"}

	_, err := store.Create(t.Context(), post)

	if !errors.Is(err, content.ErrUnknownField) {
		t.Errorf("Create() error = %v, want %v", err, content.ErrUnknownField)
	}
}

func TestContentStoreStartsWithNoFieldValues(t *testing.T) {
	t.Parallel()

	store, author, _ := newContentStoreWithPool(t)

	created := mustCreate(t, store, "Hello world", author)

	if len(created.Fields) != 0 {
		t.Errorf("Create() fields = %v, want a fresh item to hold none", created.Fields)
	}
}

func TestContentStoreSnapshotsFieldValues(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareField(t, pool, "color", content.FieldKindText)
	created := mustCreate(t, store, "Hello world", author)
	created.Fields = content.Values{"color": "red"}
	created.UpdatedAt = time.Now().UTC()
	stored, err := store.Update(t.Context(), created, created.CreatedAt, nil, 0)
	if err != nil {
		t.Fatalf("filling the field: %v, want nil", err)
	}
	snapshot, err := content.NewRevision(stored, content.RevisionKindRevision, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	edited := stored
	edited.Fields = content.Values{"color": "blue"}
	edited.UpdatedAt = time.Now().UTC()

	if _, err := store.Update(t.Context(), edited, stored.UpdatedAt, &snapshot, 100); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	revisions, err := store.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("Revisions() = %d, want the snapshot stored", len(revisions))
	}
	held, err := store.RevisionByID(t.Context(), created.ID, revisions[0].ID)
	if err != nil {
		t.Fatalf("Revision() error = %v, want nil", err)
	}
	if held.Fields["color"] != "red" {
		t.Errorf("Revision() fields = %v, want the values as they stood", held.Fields)
	}
}

func TestContentStoreParksFieldValuesInAnAutosave(t *testing.T) {
	t.Parallel()

	store, author, _ := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Hello world", author)
	buffer := created
	buffer.Fields = content.Values{"color": "red"}
	autosave, err := content.NewRevision(buffer, content.RevisionKindAutosave, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}

	saved, err := store.SaveAutosave(t.Context(), autosave)

	if err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
	if saved.Fields["color"] != "red" {
		t.Errorf("SaveAutosave() fields = %v, want the buffer's values", saved.Fields)
	}
	held, err := store.Autosave(t.Context(), created.ID, author)
	if err != nil {
		t.Fatalf("Autosave() error = %v, want nil", err)
	}
	if held.Fields["color"] != "red" {
		t.Errorf("Autosave() fields = %v, want the parked values", held.Fields)
	}
}
