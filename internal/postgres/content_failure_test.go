// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestContentStoreReportsDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	stored := mustCreate(t, store, "Doomed", author)
	pending := mustPost(t, "Never Stored", author)
	now := time.Now().UTC()
	pool.Close()

	if _, err := store.Create(t.Context(), pending); err == nil {
		t.Error("Create() on a closed pool error = nil, want a failure")
	}
	if _, err := store.ByID(t.Context(), stored.ID); err == nil {
		t.Error("ByID() on a closed pool error = nil, want a failure")
	}
	if _, err := store.PublishedByPath(t.Context(), "hello-world"); err == nil {
		t.Error("PublishedByPath() on a closed pool error = nil, want a failure")
	}
	if _, _, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10}); err == nil {
		t.Error("List() on a closed pool error = nil, want a failure")
	}
	narrowed := content.Filter{
		Type: content.TypePost, Page: 1, PerPage: 10, Fields: map[string]any{"price": float64(10)},
	}
	if _, _, err := store.List(t.Context(), narrowed); err == nil {
		t.Error("List() narrowed by fields on a closed pool error = nil, want a failure")
	}
	if _, err := store.Update(t.Context(), stored, stored.UpdatedAt, nil, 0); err == nil {
		t.Error("Update() on a closed pool error = nil, want a failure")
	}
	if _, err := store.Trash(t.Context(), stored.ID, now); err == nil {
		t.Error("Trash() on a closed pool error = nil, want a failure")
	}
	if _, err := store.Restore(t.Context(), stored.ID, now); err == nil {
		t.Error("Restore() on a closed pool error = nil, want a failure")
	}
	if err := store.Delete(t.Context(), stored.ID); err == nil {
		t.Error("Delete() on a closed pool error = nil, want a failure")
	}
}

func TestContentStoreRestoreReportsAWriteTheDatabaseRefuses(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Hello World", author)
	trashed, err := store.Trash(t.Context(), created.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	raiseOn(t, pool, "core.content", "UPDATE")

	_, err = store.Restore(t.Context(), created.ID, time.Now().UTC())

	if err == nil || !strings.Contains(err.Error(), "sabotaged") {
		t.Errorf("Restore() error = %v, want the refused write reported", err)
	}
	held, err := store.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if held.Status != content.StatusTrash || held.Slug != trashed.Slug {
		t.Errorf("held as %q under %q, want it left in the trash under %q", held.Status, held.Slug, trashed.Slug)
	}
}

func TestContentStoreListReportsARejectedQuery(t *testing.T) {
	t.Parallel()

	store, _ := newContentStore(t)

	_, _, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: -1})

	if err == nil {
		t.Error("List() with a negative page size error = nil, want a failure")
	}
}
