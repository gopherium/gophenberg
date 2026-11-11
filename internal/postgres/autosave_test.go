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
	"github.com/gopherium/gophenberg/internal/postgres/db"
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

// parkedAt stores a revision row of the kind over the item, credited to the author and saved at the moment.
func parkedAt(t *testing.T, pool *pgxpool.Pool, item, author uuid.UUID, kind string, at time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO core.content_revisions
		(id, content_id, kind, author_id, title, content, excerpt, fields, created_at)
		VALUES ($1, $2, $3, $4, 'Parked', '', '', '{}', $5)`,
		id, item, kind, author, at); err != nil {
		t.Fatalf("storing the %s: %v", kind, err)
	}
	return id
}

// savedAtOf returns when the revision row was saved.
func savedAtOf(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	if err := pool.QueryRow(t.Context(),
		`SELECT created_at FROM core.content_revisions WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatalf("reading the revision row %s: %v", id, err)
	}
	return at.UTC()
}

func TestRestampNewerAutosavesStampsOnlyTheAutosavesSavedSinceTheItemLastChanged(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Restamped", author)
	other := mustCreate(t, store, "Untouched", author)
	since := created.UpdatedAt
	stamp := since.Add(2 * time.Minute)
	rows := []struct {
		name string
		id   uuid.UUID
		want time.Time
	}{
		{"the newer autosave", parkedAt(t, pool, created.ID, author, "autosave", since.Add(time.Minute)), stamp},
		{
			"the autosave saved with the item",
			parkedAt(t, pool, created.ID, addAuthor(t, pool, "Equal Writer"), "autosave", since), stamp,
		},
		{
			"the older autosave",
			parkedAt(t, pool, created.ID, addAuthor(t, pool, "Older Writer"), "autosave", since.Add(-time.Hour)),
			since.Add(-time.Hour),
		},
		{
			"the newer revision",
			parkedAt(t, pool, created.ID, author, "revision", since.Add(time.Minute)), since.Add(time.Minute),
		},
		{
			"the newer autosave of another item",
			parkedAt(t, pool, other.ID, author, "autosave", other.UpdatedAt.Add(time.Minute)),
			other.UpdatedAt.Add(time.Minute),
		},
	}

	err := db.New(pool).RestampNewerAutosaves(t.Context(), db.RestampNewerAutosavesParams{
		SavedAt: stamp, ContentID: created.ID,
	})

	if err != nil {
		t.Fatalf("RestampNewerAutosaves() error = %v, want nil", err)
	}
	for _, row := range rows {
		if got := savedAtOf(t, pool, row.id); !got.Equal(row.want) {
			t.Errorf("%s is saved at %s, want %s", row.name, got, row.want)
		}
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
