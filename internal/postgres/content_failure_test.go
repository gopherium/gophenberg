// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

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

func TestContentStoreListReportsARejectedQuery(t *testing.T) {
	t.Parallel()

	store, _ := newContentStore(t)

	_, _, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: -1})

	if err == nil {
		t.Error("List() with a negative page size error = nil, want a failure")
	}
}

func TestContentStoreListServesAPageBeyondWhatAnOffsetHolds(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	mustCreate(t, store, "Only Post", author)

	rows, total, err := store.List(t.Context(), content.Filter{
		Type:    content.TypePost,
		OrderBy: content.OrderByDate,
		Order:   content.OrderDesc,
		Page:    30000000,
		PerPage: 100,
	})

	if err != nil {
		t.Fatalf("List() on a page past the offset range error = %v, want nil", err)
	}
	if len(rows) != 0 {
		t.Errorf("List() returned %d rows, want none that far out", len(rows))
	}
	if total != 1 {
		t.Errorf("total = %d, want the count of matching posts", total)
	}
}

func TestContentStoreReportsExhaustedSlugSuffixes(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	for range 20 {
		mustCreate(t, store, "Crowded", author)
	}
	spare := mustCreate(t, store, "Spare Title", author)
	edited := spare
	edited.Slug = "crowded"

	_, updateErr := store.Update(t.Context(), edited, spare.UpdatedAt, nil, 0)

	if !errors.Is(updateErr, content.ErrSlugTaken) {
		t.Errorf("Update() error = %v, want %v", updateErr, content.ErrSlugTaken)
	}
}

func TestContentStoreReportsASlugTakenEvenUnderTheIdentifiedOne(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	for range 20 {
		mustCreate(t, store, "Crowded", author)
	}
	crowded := mustPost(t, "Crowded", author)
	decoy := "crowded-" + strings.ReplaceAll(crowded.ID.String(), "-", "")
	_, err := pool.Exec(t.Context(),
		`INSERT INTO core.content
		(id, type, status, slug, path, title, content, excerpt, author_id, created_at, updated_at)
		VALUES ($1, 'post', 'draft', $2, $2, 'Decoy', '', '', $3, now(), now())`,
		uuid.Must(uuid.NewV7()), decoy, author,
	)
	if err != nil {
		t.Fatalf("inserting the decoy: %v", err)
	}

	_, createErr := store.Create(t.Context(), crowded)

	if !errors.Is(createErr, content.ErrSlugTaken) {
		t.Errorf("Create() error = %v, want %v", createErr, content.ErrSlugTaken)
	}
}

func TestContentStoreCreatesUnderACrowdedSlugAnyway(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
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
