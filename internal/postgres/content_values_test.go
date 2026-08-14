// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestContentStoreCarriesFieldValues(t *testing.T) {
	t.Parallel()

	store, author, _ := newContentStoreWithPool(t)
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

	store, author, _ := newContentStoreWithPool(t)
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
