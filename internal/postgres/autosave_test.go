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
)

// mustAutosave is the shared fixture under the name these tests use.
var mustAutosave = contenttest.MustAutosave

// addAuthor stores a second user and returns its id.
func addAuthor(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	author := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(t.Context(),
		`INSERT INTO auth.users (id, email, name, password_hash, disabled, created_at)
		VALUES ($1, $2, $3, 'hash', false, $4)`,
		author, author.String()+"@example.com", name, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("inserting author: %v", err)
	}
	return author
}

func TestTrashQueuedBehindAnAutosaveSweepsIt(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Contended", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`INSERT INTO core.content_revisions (id, content_id, kind, author_id, title, content, excerpt, fields, created_at)
		VALUES ($1, $2, 'autosave', $3, 'Parked', '', '', '{}', now())`,
		uuid.Must(uuid.NewV7()), created.ID, author); err != nil {
		t.Fatalf("parking the rival words: %v", err)
	}
	trashed := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := store.Trash(ctx, created.ID, time.Now().UTC())
		trashed <- err
	}()
	waitingOn(t, pool, "%FROM core.content p%FOR UPDATE%")

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the rival words: %v", err)
	}

	if err := <-trashed; err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if _, err := store.Autosave(t.Context(), created.ID, author); !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("Autosave() error = %v, want the words parked before the trash swept", err)
	}
}

func TestAutosaveQueuedBehindATrashIsRefused(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Contended", author)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR UPDATE`, created.ID); err != nil {
		t.Fatalf("locking the post: %v", err)
	}
	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content SET status = 'trash' WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("trashing the post: %v", err)
	}
	parked := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := store.SaveAutosave(ctx, mustAutosave(t, created, author))
		parked <- err
	}()
	waitingOn(t, pool, "%FROM core.content p%FOR UPDATE%")

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the trash: %v", err)
	}

	if err := <-parked; !errors.Is(err, content.ErrTrashed) {
		t.Errorf("SaveAutosave() error = %v, want the trash that landed first refusing it", err)
	}
}

func TestContentStoreAutosavesReportDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Doomed", author)
	autosave := mustAutosave(t, created, author)
	pool.Close()

	if _, err := store.SaveAutosave(t.Context(), autosave); err == nil {
		t.Error("SaveAutosave() on a closed pool error = nil, want a failure")
	}
	if _, err := store.Autosave(t.Context(), created.ID, author); err == nil {
		t.Error("Autosave() on a closed pool error = nil, want a failure")
	}
	if err := store.DeleteAutosave(t.Context(), created.ID, author); err == nil {
		t.Error("DeleteAutosave() on a closed pool error = nil, want a failure")
	}
}
