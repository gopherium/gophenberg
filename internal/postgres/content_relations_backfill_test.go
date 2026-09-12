// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestMigrationsCopyTheTargetsAnItemPointsAtIntoItsFields(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	events := storedCategory(t, store, "Events", author)
	filed := fileUnder(t, store, mustCreate(t, store, "Filed", author), events.ID, news.ID)
	url := pool.Config().ConnString()
	if err := postgres.MigrateDownTo(t.Context(), url, 22); err != nil {
		t.Fatalf("rolling back to before the copy: %v, want nil", err)
	}
	pointed := []string{events.ID.String(), news.ID.String()}
	if held := unfiled(t, pool, filed.ID); !slices.Equal(held, pointed) {
		t.Fatalf("the index holds %v, want the targets the copy reads, %v", held, pointed)
	}

	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	var targets []string
	err := pool.QueryRow(t.Context(),
		`SELECT fields -> 'categories' FROM core.content WHERE id = $1`, filed.ID).Scan(&targets)
	if err != nil {
		t.Fatalf("reading the copied targets: %v, want nil", err)
	}
	if !slices.Equal(targets, pointed) {
		t.Errorf("fields.categories = %v, want the targets in the order they were stored, %v", targets, pointed)
	}
	var pointing bool
	err = pool.QueryRow(t.Context(),
		`SELECT fields ? 'categories' FROM core.content WHERE id = $1`, news.ID).Scan(&pointing)
	if err != nil {
		t.Fatalf("reading an item pointing at nothing: %v, want nil", err)
	}
	if pointing {
		t.Error("an item pointing at nothing gained a relation key, want its fields left alone")
	}
}

// unfiled takes the relation value out of the item's fields, returning the targets the index still holds.
func unfiled(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) []string {
	t.Helper()
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content SET fields = fields - 'categories' WHERE id = $1`, id); err != nil {
		t.Fatalf("clearing the value the copy writes: %v, want nil", err)
	}
	var held []string
	err := pool.QueryRow(t.Context(),
		`SELECT array_agg(to_id::text ORDER BY position) FROM core.content_relations WHERE from_id = $1`,
		id).Scan(&held)
	if err != nil {
		t.Fatalf("reading the rows the copy reads: %v, want nil", err)
	}
	return held
}
