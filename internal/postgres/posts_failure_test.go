// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/post"
)

func TestPostStoreReportsDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
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
	if _, _, err := store.List(t.Context(), post.Filter{Type: post.TypePost, Page: 1, PerPage: 10}); err == nil {
		t.Error("List() on a closed pool error = nil, want a failure")
	}
	if _, err := store.Update(t.Context(), stored, nil, 0); err == nil {
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

func TestPostStoreListReportsARejectedQuery(t *testing.T) {
	t.Parallel()

	store, _ := newPostStore(t)

	_, _, err := store.List(t.Context(), post.Filter{Type: post.TypePost, Page: 1, PerPage: -1})

	if err == nil {
		t.Error("List() with a negative page size error = nil, want a failure")
	}
}

func TestPostStoreReportsExhaustedSlugSuffixes(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	for range 20 {
		mustCreate(t, store, "Crowded", author)
	}
	spare := mustCreate(t, store, "Spare Title", author)

	_, createErr := store.Create(t.Context(), mustPost(t, "Crowded", author))
	edited := spare
	edited.Slug = "crowded"
	_, updateErr := store.Update(t.Context(), edited, nil, 0)

	if !errors.Is(createErr, post.ErrSlugTaken) {
		t.Errorf("Create() error = %v, want %v", createErr, post.ErrSlugTaken)
	}
	if !errors.Is(updateErr, post.ErrSlugTaken) {
		t.Errorf("Update() error = %v, want %v", updateErr, post.ErrSlugTaken)
	}
}
