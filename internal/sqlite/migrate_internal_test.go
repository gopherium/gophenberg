// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"testing"
	"time"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"
	authkitsqlite "github.com/gopherium/gouncer/authkit/sqlite"
	"github.com/pressly/goose/v3"
)

func TestMustSubPanicsOnAFolderOutsideTheTree(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("mustSub() did not panic, want a panic")
		}
	}()

	mustSub(migrations, "..")
}

func TestTheBaselineRollsBackOverAStoredPostAndAppliesAgain(t *testing.T) {
	t.Parallel()

	settings := Settings{
		BusyTimeout: 5 * time.Second, BusyPoll: 10 * time.Millisecond, CacheSize: 2048, MaxConns: 4,
		Synchronous: dbsqlite.SynchronousFull,
	}
	opts := MigrateOptions{LockWait: 5 * time.Second, LockPoll: 10 * time.Millisecond}
	db := sqlitetest.Open(t, settings.Options())
	if err := authkitsqlite.Migrate(t.Context(), db, authkitsqlite.MigrateOptions(opts)); err != nil {
		t.Fatalf("migrating the account tables: %v", err)
	}
	if err := Migrate(t.Context(), db, opts); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}
	for _, statement := range []string{
		"INSERT INTO auth_users (id, email, name, password_hash, disabled, created_at) VALUES " +
			"('019a0000-0000-7000-8000-000000000001', 'maria@example.com', 'Maria Perez', 'placeholder-hash', 0, 1)",
		"INSERT INTO core_content (id, type, status, slug, author_id, created_at, updated_at, path) VALUES " +
			"('019a0000-0000-7000-8000-0000000000c1', 'post', 'draft', 'hello', " +
			"'019a0000-0000-7000-8000-000000000001', 1, 1, 'hello')",
	} {
		if _, err := db.ExecContext(t.Context(), statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationSource,
		goose.WithTableName(versionTable), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatalf("building the migration runner: %v", err)
	}
	tables := `SELECT count(*) FILTER (WHERE name LIKE 'core\_%' ESCAPE '\' AND name <> ?), ` +
		`count(*) FILTER (WHERE name = 'auth_users') FROM sqlite_schema WHERE type = 'table'`

	if _, err := provider.DownTo(t.Context(), 0); err != nil {
		t.Fatalf("DownTo(0) over a stored post error = %v, want each table dropped after the tables that point at it", err)
	}

	var core, accounts int
	if err := db.QueryRowContext(t.Context(), tables, versionTable).Scan(&core, &accounts); err != nil {
		t.Fatalf("counting the tables: %v", err)
	}
	if core != 0 || accounts != 1 {
		t.Errorf("after the rollback %d core tables and %d account tables are left, want none and the one", core, accounts)
	}
	if err := Migrate(t.Context(), db, opts); err != nil {
		t.Fatalf("Migrate() after the rollback error = %v, want a clean reapply", err)
	}
	if err := db.QueryRowContext(t.Context(), tables, versionTable).Scan(&core, &accounts); err != nil || core != 11 {
		t.Errorf("after the reapply %d core tables are there with error %v, want all 11", core, err)
	}
}
