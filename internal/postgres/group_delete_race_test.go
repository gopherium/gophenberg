// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
)

// savingRival opens a rival transaction holding every field row as a running save does, until committed.
func savingRival(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	saver, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatalf("acquiring the saving connection: %v", err)
	}
	t.Cleanup(saver.Release)
	tx, err := saver.Begin(t.Context())
	if err != nil {
		t.Fatalf("opening the saving transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(t.Context(), `SELECT key FROM core.content_fields ORDER BY key FOR KEY SHARE`); err != nil {
		t.Fatalf("holding the field rows: %v", err)
	}
	return tx
}

// blockedOnALock waits until a statement of the database stands blocked on a lock.
func blockedOnALock(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(t.Context(),
			`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(&waiting); err != nil {
			t.Fatalf("polling pg_stat_activity: %v", err)
		}
		if waiting > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the group delete never blocked on the running save")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// targetsIndexedUnder returns the slugs of the items the field's index rows point at.
func targetsIndexedUnder(t *testing.T, pool *pgxpool.Pool, field int) []string {
	t.Helper()
	rows, err := pool.Query(t.Context(),
		`SELECT c.slug FROM core.content_relations r JOIN core.content c ON c.id = r.to_id
		WHERE r.field_id = $1 ORDER BY c.slug`, field)
	if err != nil {
		t.Fatalf("listing the targets under %d: %v, want nil", field, err)
	}
	held, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("reading the targets under %d: %v, want nil", field, err)
	}
	return held
}

func TestDeletingAGroupKeepsTheIndexAConcurrentSaveWrites(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "category")
	source, landing, section := sectionAcrossGroups(t, store)
	kept := plantTwin(t, pool, landing.ID, section.ID, "wrote", string(content.FieldKindRelation), 1)
	twin := plantTwin(t, pool, source.ID, section.ID, "wrote", string(content.FieldKindRelation), 1)
	for _, slug := range []string{"one-car", "news-car", "sports-car"} {
		plantTyped(t, pool, author, "car", slug, `{}`)
	}
	indexUnder(t, pool, twin, "news-car")
	rival := savingRival(t, pool)
	if _, err := rival.Exec(t.Context(),
		`DELETE FROM core.content_relations WHERE from_id = $1`, itemSlugged(t, pool, "one-car")); err != nil {
		t.Fatalf("clearing the saved index rows: %v", err)
	}
	if _, err := rival.Exec(t.Context(),
		`INSERT INTO core.content_relations (from_id, field_id, to_id, sort_at)
		VALUES ($1, $2, $4, now()), ($1, $3, $4, now())`,
		itemSlugged(t, pool, "one-car"), twin, kept, itemSlugged(t, pool, "sports-car")); err != nil {
		t.Fatalf("writing the saved index rows: %v", err)
	}
	deleted := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		deleted <- store.DeleteGroup(ctx, source.ID)
	}()
	blockedOnALock(t, pool)

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the save: %v", err)
	}
	if err := <-deleted; err != nil {
		t.Fatalf("DeleteGroup(Details) error = %v, want nil", err)
	}

	if held := targetsIndexedUnder(t, pool, kept); !slices.Equal(held, []string{"sports-car"}) {
		t.Errorf("the kept twin points at %v, want only what the save wrote", held)
	}
}
