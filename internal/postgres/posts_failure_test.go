// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

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
	if _, err := store.PublishedBySlug(t.Context(), post.TypePost, stored.Slug); err == nil {
		t.Error("PublishedBySlug() on a closed pool error = nil, want a failure")
	}
	if _, _, err := store.List(t.Context(), post.Filter{Type: post.TypePost, Page: 1, PerPage: 10}); err == nil {
		t.Error("List() on a closed pool error = nil, want a failure")
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
	edited := spare
	edited.Slug = "crowded"

	_, updateErr := store.Update(t.Context(), edited, spare.UpdatedAt, nil, 0)

	if !errors.Is(updateErr, post.ErrSlugTaken) {
		t.Errorf("Update() error = %v, want %v", updateErr, post.ErrSlugTaken)
	}
}

func TestPostStoreReportsASlugTakenEvenUnderTheIdentifiedOne(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
	for range 20 {
		mustCreate(t, store, "Crowded", author)
	}
	crowded := mustPost(t, "Crowded", author)
	decoy := "crowded-" + strings.ReplaceAll(crowded.ID.String(), "-", "")
	_, err := pool.Exec(t.Context(),
		`INSERT INTO core.posts (id, type, status, slug, title, content, excerpt, author_id, created_at, updated_at)
		VALUES ($1, 'post', 'draft', $2, 'Decoy', '', '', $3, now(), now())`,
		uuid.Must(uuid.NewV7()), decoy, author,
	)
	if err != nil {
		t.Fatalf("inserting the decoy: %v", err)
	}

	_, createErr := store.Create(t.Context(), crowded)

	if !errors.Is(createErr, post.ErrSlugTaken) {
		t.Errorf("Create() error = %v, want %v", createErr, post.ErrSlugTaken)
	}
}

func TestPostStoreCreatesUnderACrowdedSlugAnyway(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	for range 20 {
		mustCreate(t, store, "Crowded", author)
	}

	created, err := store.Create(t.Context(), mustPost(t, "Crowded", author))

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if !strings.HasPrefix(created.Slug, "crowded-") {
		t.Errorf("Slug = %q, want one carrying the crowded stem", created.Slug)
	}
	if !strings.Contains(created.Slug, strings.ReplaceAll(created.ID.String(), "-", "")) {
		t.Errorf("Slug = %q, want it to carry the id of the post", created.Slug)
	}
}
