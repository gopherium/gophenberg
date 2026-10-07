// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// categoryType, namedTargets, storedCategory, fileUnder and publishItem are the shared fixtures these tests use.
var (
	categoryType   = contenttest.CategoryType
	namedTargets   = contenttest.NamedTargets
	storedCategory = contenttest.StoredCategory
	fileUnder      = contenttest.FileUnder
	publishItem    = contenttest.PublishItem
)

// relatingStore returns a store, an author, the pool, and a post type carrying a relation field.
func relatingStore(t *testing.T) (*postgres.ContentStore, uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), categoryType()); err != nil {
		t.Fatalf("registering the category type: %v, want nil", err)
	}
	built, err := content.NewField(content.Field{
		TypeKey: "post", Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if _, err := types.CreateField(t.Context(), built); err != nil {
		t.Fatalf("declaring the relation: %v, want nil", err)
	}
	return store, author, pool
}

// visibilityOf reads the denormalized pair the term page scans.
func visibilityOf(t *testing.T, pool *pgxpool.Pool, fromID uuid.UUID) (bool, time.Time) {
	t.Helper()
	var visible bool
	var sortAt time.Time
	err := pool.QueryRow(
		t.Context(), `SELECT visible, sort_at FROM core.content_relations WHERE from_id = $1`, fromID,
	).Scan(&visible, &sortAt)
	if err != nil {
		t.Fatalf("reading the relation row: %v, want nil", err)
	}
	return visible, sortAt
}

func TestContentStoreHidesTheRelationsOfADraft(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)

	visible, sortAt := visibilityOf(t, pool, post.ID)

	if visible {
		t.Errorf("the relation of a draft is visible, want it hidden until the item is published")
	}
	if !sortAt.Equal(post.CreatedAt) {
		t.Errorf("sort_at = %v, want the unpublished item's created stamp %v", sortAt, post.CreatedAt)
	}
}

func TestContentStoreShowsTheRelationsOfAPublishedItem(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)
	version := post.UpdatedAt
	if err := post.Transition(content.StatusPublished); err != nil {
		t.Fatalf("Transition() error = %v, want nil", err)
	}

	published, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("publishing: %v, want nil", err)
	}

	visible, sortAt := visibilityOf(t, pool, post.ID)
	if !visible {
		t.Errorf("the relation of a published item is hidden, want it on the term page")
	}
	if !sortAt.Equal(*published.PublishedAt) {
		t.Errorf("sort_at = %v, want the published stamp %v", sortAt, *published.PublishedAt)
	}
}

func TestContentStoreHidesTheRelationsOfATrashedItem(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)
	version := post.UpdatedAt
	if err := post.Transition(content.StatusPublished); err != nil {
		t.Fatalf("Transition() error = %v, want nil", err)
	}
	if _, err := store.Update(t.Context(), post, version, nil, 0); err != nil {
		t.Fatalf("publishing: %v, want nil", err)
	}

	if _, err := store.Trash(t.Context(), post.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	if visible, _ := visibilityOf(t, pool, post.ID); visible {
		t.Errorf("the relation of a trashed item is visible, want it off the term page")
	}
}

func TestContentStoreHidesTheRelationsOfARestoredDraft(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)
	trashed, err := store.Trash(t.Context(), post.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if _, err := pool.Exec(
		t.Context(), `UPDATE core.content_relations SET visible = true WHERE from_id = $1`, post.ID,
	); err != nil {
		t.Fatalf("planting a visible relation: %v, want nil", err)
	}
	if visible, _ := visibilityOf(t, pool, post.ID); !visible {
		t.Fatal("the planted relation is hidden, want the test to start from the state the restore must clear")
	}

	if _, err := store.Restore(t.Context(), post.ID, trashed.UpdatedAt); err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}

	if visible, _ := visibilityOf(t, pool, post.ID); visible {
		t.Errorf("the relation of a restored draft is visible, want the restore to have hidden it")
	}
}

// deadlocked reports whether Postgres turned the statement away to break a lock cycle.
func deadlocked(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "40P01"
}

func TestContentStoreWritesRelationsWhileTheFieldIsDeleted(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	types := postgres.NewTypeStore(pool)
	news := storedCategory(t, store, "News", author)
	group := groupHolding(t, types, "categories")
	for round := range 60 {
		if round > 0 {
			built, err := content.NewField(content.Field{
				TypeKey: "post", Key: "categories", Label: "Categories",
				Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
			})
			if err != nil {
				t.Fatalf("NewField() error = %v, want nil", err)
			}
			if _, err := types.CreateField(t.Context(), built); err != nil {
				t.Fatalf("redeclaring the relation: %v, want nil", err)
			}
		}
		post := mustCreate(t, store, fmt.Sprintf("Hello world %d", round), author)
		post.Fields = content.Values{"categories": namedTargets([]uuid.UUID{news.ID})}
		post.UpdatedAt = time.Now().UTC()
		filed, err := store.Update(t.Context(), post, post.CreatedAt, nil, 0)
		if err != nil {
			t.Fatalf("filing the post: %v, want nil", err)
		}
		version := filed.UpdatedAt
		filed.Title = "Renamed"
		filed.UpdatedAt = time.Now().UTC()
		start := make(chan struct{})
		var wg sync.WaitGroup
		var written, swept error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, written = store.Update(t.Context(), filed, version, nil, 0)
		}()
		go func() {
			defer wg.Done()
			<-start
			time.Sleep(time.Duration(round) * 100 * time.Microsecond)
			swept = types.DeleteFieldInGroup(t.Context(), group, "categories", nil)
		}()
		close(start)
		wg.Wait()
		if deadlocked(written) || deadlocked(swept) {
			t.Fatalf("round %d deadlocked: write = %v, sweep = %v", round, written, swept)
		}
	}
}

func TestContentStoreReportsTargetsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := relatingStore(t)
	pool.Close()

	if _, err := store.TargetsByIDs(t.Context(), []uuid.UUID{uuid.New()}); err == nil {
		t.Error("TargetsByIDs() error = nil, want the closed pool reported")
	}
}
