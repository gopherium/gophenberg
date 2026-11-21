// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"testing"

	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/peterldowns/pgtestdb"
	"github.com/pressly/goose/v3"

	"github.com/gopherium/gophenberg/internal/testdb"
)

// pauseStepVersion is the migration that keeps the step a paused plugin failed at apart from its error.
const pauseStepVersion = 29

// heldPlugin is one plugin record as the table holds it, each column read as text.
type heldPlugin struct {
	state, lastError, step string
}

// beforePauseSteps returns a database migrated to the shape before pause steps and the provider that carries it on.
func beforePauseSteps(t *testing.T) (*sql.DB, *goose.Provider) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	databaseURL := testdb.New(t, pgtestdb.NoopMigrator{})
	if err := authkitpg.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("auth Migrate() error = %v, want nil", err)
	}
	db := testdb.Open(t, databaseURL)
	provider := coreProvider(t, db)
	if _, err := provider.UpTo(t.Context(), pauseStepVersion-1); err != nil {
		t.Fatalf("migrating to the shape before pause steps: %v", err)
	}
	return db, provider
}

func TestPauseStepMigrationKeepsAPausedRecordAsItWasWithAnEmptyStep(t *testing.T) {
	t.Parallel()

	db, provider := beforePauseSteps(t)
	const stored = "pluginkit: notes register: the title is too long"
	if _, err := db.Exec(
		`INSERT INTO core.plugins (id, state, last_error) VALUES ('notes', 'paused', $1)`, stored,
	); err != nil {
		t.Fatalf("storing a paused plugin before the migration: %v", err)
	}

	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatalf("migrating the stored plugin: %v, want nil", err)
	}

	var held heldPlugin
	err := db.QueryRow(`SELECT state, last_error, step FROM core.plugins WHERE id = 'notes'`).
		Scan(&held.state, &held.lastError, &held.step)
	if want := (heldPlugin{state: "paused", lastError: stored}); err != nil || held != want {
		t.Errorf("the notes record = %+v (%v), want %+v, its old error kept and its step empty", held, err, want)
	}
}
