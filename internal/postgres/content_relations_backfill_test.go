// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"slices"
	"testing"

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

	if err := postgres.Migrate(t.Context(), url); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	var targets []string
	err := pool.QueryRow(t.Context(),
		`SELECT fields -> 'categories' FROM core.content WHERE id = $1`, filed.ID).Scan(&targets)
	if err != nil {
		t.Fatalf("reading the copied targets: %v, want nil", err)
	}
	if want := []string{events.ID.String(), news.ID.String()}; !slices.Equal(targets, want) {
		t.Errorf("fields.categories = %v, want the targets in the order they were stored, %v", targets, want)
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
