// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// specsDoorsOnPost declares a specs section on the post type holding a doors text field.
func specsDoorsOnPost(t *testing.T, store *postgres.TypeStore) content.Field {
	t.Helper()
	section := sectionOn(t, "specs")
	section.TypeKey = "post"
	stored, err := store.CreateField(t.Context(), section)
	if err != nil {
		t.Fatalf("CreateField(section specs) error = %v, want nil", err)
	}
	doors, err := store.CreateSubField(
		t.Context(), stored.ID, fieldOn(t, "", "doors", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateSubField(doors) error = %v, want nil", err)
	}
	return doors
}

// lockedRevisionRow opens a rival transaction holding the revision row lock until committed.
func lockedRevisionRow(t *testing.T, pool *pgxpool.Pool, id any) pgx.Tx {
	t.Helper()
	locker, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatalf("acquiring the locking connection: %v", err)
	}
	t.Cleanup(locker.Release)
	tx, err := locker.Begin(t.Context())
	if err != nil {
		t.Fatalf("opening the locking transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(t.Context(),
		`SELECT id FROM core.content_revisions WHERE id = $1 FOR UPDATE`, id); err != nil {
		t.Fatalf("locking the autosave row: %v", err)
	}
	return tx
}

// sweepBlockedOnRevisions waits until a revision sweep statement stands blocked on a row lock.
func sweepBlockedOnRevisions(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(t.Context(),
			`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'
			AND query LIKE '%UPDATE core.content_revisions%'`).Scan(&waiting); err != nil {
			t.Fatalf("polling pg_stat_activity: %v", err)
		}
		if waiting > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the sweep never blocked on the locked autosave row")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// doorsSweptFrom asserts the doors value is gone from the swept document.
func doorsSweptFrom(t *testing.T, held content.Values) {
	t.Helper()
	specs, ok := held["specs"].(map[string]any)
	if !ok {
		t.Fatalf("specs = %v, want the section object kept", held["specs"])
	}
	if _, still := specs["doors"]; still {
		t.Errorf("specs = %v, want the doors swept from the autosave", specs)
	}
}

func TestDeletingASubFieldKeepsAConcurrentAutosaveEdit(t *testing.T) {
	t.Parallel()

	contentStore, author, pool := newContentStoreWithPool(t)
	typeStore := postgres.NewTypeStore(pool)
	doors := specsDoorsOnPost(t, typeStore)
	created := mustCreate(t, contentStore, "Raced", author)
	buffer := created
	buffer.Fields = content.Values{
		"specs": map[string]any{"doors": "five"},
		"note":  "original",
	}
	saved, err := contentStore.SaveAutosave(t.Context(), mustAutosave(t, buffer, author))
	if err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
	rival := lockedRevisionRow(t, pool, saved.ID)
	deleted := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		deleted <- typeStore.DeleteSubField(ctx, doors.ID)
	}()
	sweepBlockedOnRevisions(t, pool)

	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content_revisions SET fields = $2 WHERE id = $1`,
		saved.ID, `{"specs": {"doors": "five"}, "note": "fresh"}`); err != nil {
		t.Fatalf("writing the rival autosave: %v", err)
	}
	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the rival autosave: %v", err)
	}
	if err := <-deleted; err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	var held content.Values
	if err := pool.QueryRow(t.Context(),
		`SELECT fields FROM core.content_revisions WHERE id = $1`, saved.ID).Scan(&held); err != nil {
		t.Fatalf("reading the swept autosave: %v", err)
	}
	if held["note"] != "fresh" {
		t.Errorf("note = %v, want the concurrent autosave edit kept, full fields = %v", held["note"], held)
	}
	doorsSweptFrom(t, held)
}
