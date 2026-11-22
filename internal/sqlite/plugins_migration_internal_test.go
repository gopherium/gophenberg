// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"database/sql"
	"testing"
	"time"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"
	authkitsqlite "github.com/gopherium/gouncer/authkit/sqlite"
	"github.com/pressly/goose/v3"
)

// pauseStepVersion is the migration that keeps the step a paused plugin failed at apart from its error.
const pauseStepVersion = 29

// heldPlugin is one plugin record as the table holds it, each column read as text.
type heldPlugin struct {
	state, lastError, step string
}

// beforePauseSteps returns a file migrated to the shape before pause steps and the runner that carries it on.
func beforePauseSteps(t *testing.T) (*sql.DB, *goose.Provider) {
	t.Helper()
	settings := Settings{
		BusyTimeout: 5 * time.Second, BusyPoll: 10 * time.Millisecond, CacheSize: 2048, MaxConns: 4,
		Synchronous: dbsqlite.SynchronousFull,
	}
	db := sqlitetest.Open(t, settings.Options())
	opts := authkitsqlite.MigrateOptions{LockWait: 5 * time.Second, LockPoll: 10 * time.Millisecond}
	if err := authkitsqlite.Migrate(t.Context(), db, opts); err != nil {
		t.Fatalf("migrating the account tables: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationSource,
		goose.WithTableName(versionTable), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatalf("building the migration runner: %v", err)
	}
	if _, err := provider.UpTo(t.Context(), pauseStepVersion-1); err != nil {
		t.Fatalf("migrating to the shape before pause steps: %v", err)
	}
	return db, provider
}

func TestPauseStepMigrationKeepsAPausedRecordAsItWasWithAnEmptyStep(t *testing.T) {
	t.Parallel()

	db, provider := beforePauseSteps(t)
	const stored = "pluginkit: notes register: the title is too long"
	if _, err := db.ExecContext(t.Context(),
		"INSERT INTO core_plugins (id, state, last_error) VALUES ('notes', 'paused', ?)", stored,
	); err != nil {
		t.Fatalf("storing a paused plugin before the migration: %v", err)
	}

	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatalf("migrating the stored plugin: %v, want nil", err)
	}

	var held heldPlugin
	err := db.QueryRowContext(t.Context(), "SELECT state, last_error, step FROM core_plugins WHERE id = 'notes'").
		Scan(&held.state, &held.lastError, &held.step)
	if want := (heldPlugin{state: "paused", lastError: stored}); err != nil || held != want {
		t.Errorf("the notes record = %+v (%v), want %+v, its old error kept and its step empty", held, err, want)
	}
}
