// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
)

// holdingTypeRow opens a transaction holding the type row against any write, rolled back when the test ends.
func holdingTypeRow(t *testing.T, pool *pgxpool.Pool, key string) pgx.Tx {
	t.Helper()
	locker, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatalf("acquiring the rival connection: %v", err)
	}
	t.Cleanup(locker.Release)
	tx, err := locker.Begin(t.Context())
	if err != nil {
		t.Fatalf("opening the rival transaction: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	if _, err := tx.Exec(t.Context(),
		`SELECT key FROM core.content_types WHERE key = $1 FOR UPDATE`, key); err != nil {
		t.Fatalf("locking the type row: %v", err)
	}
	return tx
}

// blockedOnTheTypeRow waits until a statement stands blocked on the locked type row.
func blockedOnTheTypeRow(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		var waiting int
		if err := pool.QueryRow(t.Context(),
			`SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'
			AND query LIKE '%FROM core.content_types%FOR UPDATE%'`).Scan(&waiting); err != nil {
			t.Fatalf("polling pg_stat_activity: %v", err)
		}
		if waiting > 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the import never blocked on the locked type row")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// relabeling gives the envelope's type another singular label.
func relabeling(t *testing.T, envelope definitions.Envelope, key, label string) {
	t.Helper()
	for i := range envelope.Types {
		if envelope.Types[i].Key == key {
			envelope.Types[i].SingularLabel = label
			return
		}
	}
	t.Fatalf("the envelope holds no type %q", key)
}

func TestApplyLeavesATypeNestingWhenAnItemNestsWhileItRuns(t *testing.T) {
	t.Parallel()

	registry, items, author, pool := nestingSiteWithPool(t)
	recipe, err := registry.ByKey(t.Context(), "recipe")
	if err != nil {
		t.Fatalf("ByKey(recipe) error = %v, want nil", err)
	}
	bread := filed(t, items, recipe, nil, "Bread", author)
	envelope := exported(t, registry)
	flatteningRecipe(t, envelope)
	relabeling(t, envelope, content.TypePost, "Entry")
	rival := holdingTypeRow(t, pool, "recipe")
	if _, err := rival.Exec(t.Context(),
		`INSERT INTO core.content (id, type, status, slug, title, content, excerpt,
			author_id, created_at, updated_at, parent_id, path, fields)
		VALUES ($1, 'recipe', 'draft', 'roll', 'Roll', '', '', $2, now(), now(), $3, 'recipes/bread/roll', '{}')`,
		uuid.Must(uuid.NewV7()), author, bread.ID); err != nil {
		t.Fatalf("filing the rival child: %v", err)
	}
	imported := make(chan definitions.Outcome, 1)
	failed := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		outcome, err := definitions.Apply(ctx, registry, importing(envelope))
		imported <- outcome
		failed <- err
	}()
	blockedOnTheTypeRow(t, pool)

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the rival child: %v", err)
	}

	outcome := <-imported
	if err := <-failed; err != nil {
		t.Fatalf("Apply() error = %v, want the import finishing around the item filed meanwhile", err)
	}
	if !storedNesting(t, registry, "recipe") {
		t.Errorf("the recipe type stopped nesting, want it kept for the item filed meanwhile")
	}
	if !keptNesting(outcome.Skipped, "recipe") {
		t.Errorf("skipped = %+v, want the kept nesting named there", outcome.Skipped)
	}
	if held, _ := registry.ByKey(t.Context(), content.TypePost); held.SingularLabel != "Entry" {
		t.Errorf("the post type is labeled %q, want the import's other write kept", held.SingularLabel)
	}
}
