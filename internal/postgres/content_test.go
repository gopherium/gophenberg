// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// postType, mustPost and mustCreate are the shared fixtures under the names these tests use.
var (
	postType   = contenttest.PostType
	mustPost   = contenttest.MustPost
	mustCreate = contenttest.MustCreate
)

// newContentStore returns a store over a migrated database and the id of a stored author.
func newContentStore(t *testing.T) (*postgres.ContentStore, uuid.UUID) {
	t.Helper()
	store, author, _ := newContentStoreWithPool(t)
	return store, author
}

// newContentStoreWithPool returns a store, the id of a stored author, and the pool behind them.
func newContentStoreWithPool(t *testing.T) (*postgres.ContentStore, uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	pool := migratedPool(t)
	return postgres.NewContentStore(pool), addAuthor(t, pool, contenttest.DefaultAuthor), pool
}

func TestContentStoreUpdateWrapsDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Wrapped", author)
	pool.Close()

	_, err := store.Update(t.Context(), created, created.UpdatedAt, nil, 0)

	if err == nil {
		t.Fatal("Update() on a closed pool error = nil, want a failure")
	}
	if !strings.Contains(err.Error(), "postgres: update content") {
		t.Errorf("Update() error = %q, want the update content prefix", err)
	}
}

func TestRestoreQueuedBehindARestoreThatPublishedIsRefused(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Hello World", author)
	if _, err := store.Trash(t.Context(), created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR UPDATE`, created.ID); err != nil {
		t.Fatalf("locking the post: %v", err)
	}
	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content SET status = 'published', published_at = now() WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("restoring and publishing the post: %v", err)
	}
	restored := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := store.Restore(ctx, created.ID, time.Now().UTC())
		restored <- err
	}()
	waitingOn(t, pool, "%core.content%")

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the publish: %v", err)
	}

	if err := <-restored; !errors.Is(err, content.ErrInvalidTransition) {
		t.Errorf("Restore() error = %v, want the post published first refusing a late restore", err)
	}
	held, err := store.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if held.Status != content.StatusPublished {
		t.Errorf("Status = %q, want the published post left published", held.Status)
	}
}

func TestRestoreReportsAnItemItCannotLock(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Hello World", author)
	if _, err := store.Trash(t.Context(), created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	sabotage(t, pool, "ALTER TABLE core.content DROP COLUMN parent_id CASCADE")

	if _, err := store.Restore(t.Context(), created.ID, time.Now().UTC()); err == nil ||
		!strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Restore() error = %v, want the unreadable lock reported", err)
	}
}
