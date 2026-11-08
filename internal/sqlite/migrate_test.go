// SPDX-License-Identifier: Apache-2.0

//go:build linux || darwin

package sqlite_test

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

// schemaText returns every table and index definition db holds, in name order.
func schemaText(t *testing.T, db *sql.DB) string {
	t.Helper()
	var text string
	lookup := "SELECT group_concat(sql, char(10)) FROM (SELECT sql FROM sqlite_schema WHERE sql IS NOT NULL ORDER BY name)"
	if err := db.QueryRowContext(t.Context(), lookup).Scan(&text); err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	return text
}

// withAccounts returns a handle on a new file that holds only the account tables, closed when t ends.
func withAccounts(t *testing.T) *sql.DB {
	t.Helper()
	db := sqlitetest.Open(t, testSettings().Options())
	if err := migrateAccounts(t.Context(), db); err != nil {
		t.Fatalf("migrating the account tables: %v", err)
	}
	return db
}

// coreTables returns how many core tables db holds beside the core version table.
func coreTables(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	return count(t, db, `SELECT count(*) FROM sqlite_schema WHERE type = 'table' `+
		`AND name LIKE 'core\_%' ESCAPE '\' AND name <> 'core_goose_db_version'`)
}

// mainFile returns the path of the main database file of db.
func mainFile(t *testing.T, db *sql.DB) string {
	t.Helper()
	var path string
	lookup := "SELECT file FROM pragma_database_list WHERE name = 'main'"
	if err := db.QueryRowContext(t.Context(), lookup).Scan(&path); err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	return path
}

// migrationLock holds the migration lock file beside the main file of db until t ends.
func migrationLock(t *testing.T, db *sql.DB) {
	t.Helper()
	lock, err := os.OpenFile(mainFile(t, db)+".migrate.lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("opening the migration lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatalf("holding the migration lock: %v", err)
	}
}

func TestMigrateOptionsFallBackToTheDocumentedDefaults(t *testing.T) {
	t.Parallel()

	got, err := sqlite.MigrateOptionsFrom(environment(nil))
	if err != nil {
		t.Fatalf("MigrateOptionsFrom() error = %v, want nil", err)
	}

	want := sqlite.MigrateOptions{LockWait: 30 * time.Second, LockPoll: 250 * time.Millisecond}
	if got != want {
		t.Errorf("MigrateOptionsFrom() = %+v, want %+v", got, want)
	}
}

func TestMigrateOptionsReadEveryValueFromTheEnvironment(t *testing.T) {
	t.Parallel()

	got, err := sqlite.MigrateOptionsFrom(environment(map[string]string{
		"GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT": "2m",
		"GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL": "40ms",
	}))
	if err != nil {
		t.Fatalf("MigrateOptionsFrom() error = %v, want nil", err)
	}

	want := sqlite.MigrateOptions{LockWait: 2 * time.Minute, LockPoll: 40 * time.Millisecond}
	if got != want {
		t.Errorf("MigrateOptionsFrom() = %+v, want %+v", got, want)
	}
}

func TestMigrateOptionsRefuseABadValueByItsName(t *testing.T) {
	t.Parallel()

	for _, bad := range []struct{ name, value string }{
		{"GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT", "0s"},
		{"GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT", "-1s"},
		{"GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT", "soon"},
		{"GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL", "0s"},
		{"GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL", "-5ms"},
		{"GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL", "often"},
	} {
		_, err := sqlite.MigrateOptionsFrom(environment(map[string]string{bad.name: bad.value}))
		if err == nil || !strings.Contains(err.Error(), bad.name) {
			t.Errorf("MigrateOptionsFrom() with %s=%s error = %v, want one naming the setting", bad.name, bad.value, err)
		}
	}
}

func TestMigrateRecordsTheBaselineInTheCoreVersionTable(t *testing.T) {
	t.Parallel()

	db := migrated(t)

	core := count(t, db, "SELECT max(version_id) FROM core_goose_db_version")
	auth := count(t, db, "SELECT max(version_id) FROM auth_goose_db_version")
	if core != 28 || auth != 4 {
		t.Errorf("the highest versions are core %d and auth %d, want the 00028 plugins beside the 00004 auth one",
			core, auth)
	}
}

func TestMigrateDoesNothingOnASecondRun(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	schema := schemaText(t, db)
	versions := count(t, db, "SELECT count(*) FROM core_goose_db_version")

	err := sqlite.Migrate(t.Context(), db, testMigrate)

	if err != nil {
		t.Fatalf("a second Migrate() error = %v, want nil", err)
	}
	if got := count(t, db, "SELECT count(*) FROM core_goose_db_version"); got != versions {
		t.Errorf("core_goose_db_version holds %d rows after a second run, want the %d it held", got, versions)
	}
	if got := count(t, db, "SELECT count(*) FROM core_content_types"); got != 1 {
		t.Errorf("core_content_types holds %d rows after a second run, want the one seeded type", got)
	}
	if got := schemaText(t, db); got != schema {
		t.Errorf("the schema after a second run = %s, want it unchanged from %s", got, schema)
	}
}

func TestMigrateRefusesAFileWithoutTheAccountTables(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	err := sqlite.Migrate(t.Context(), db, testMigrate)

	if !errors.Is(err, sqlite.ErrNoAccounts) {
		t.Errorf("Migrate() before the account tables error = %v, want ErrNoAccounts", err)
	}
	if got := coreTables(t, db); got != 0 {
		t.Errorf("Migrate() before the account tables created %d core tables, want none", got)
	}
}

func TestMigrateReportsAHandleItCannotRead(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}

	err := sqlite.Migrate(t.Context(), db, testMigrate)

	if err == nil || errors.Is(err, sqlite.ErrNoAccounts) ||
		!strings.HasPrefix(err.Error(), "sqlite: look for the account tables: ") {
		t.Errorf("Migrate() on a closed handle error = %v, want the account lookup that failed named", err)
	}
}

func TestMigrateReportsAFileAnotherWriterHoldsAsBusy(t *testing.T) {
	t.Parallel()

	db := withAccounts(t)
	writer, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("Conn() error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = writer.Close() })
	if _, err := writer.ExecContext(t.Context(), "BEGIN IMMEDIATE"); err != nil {
		t.Fatalf("BEGIN IMMEDIATE error = %v, want nil", err)
	}
	impatient := testSettings()
	impatient.BusyTimeout, impatient.BusyPoll = time.Millisecond, time.Millisecond
	other := open(t, "sqlite:"+mainFile(t, db), impatient)

	err = sqlite.Migrate(t.Context(), other, testMigrate)

	if !errors.Is(err, dbkit.ErrBusy) {
		t.Errorf("Migrate() behind another writer error = %v, want dbkit.ErrBusy", err)
	}
}

func TestMigrateRefusesLockSettingsAtZero(t *testing.T) {
	t.Parallel()

	db := withAccounts(t)

	err := sqlite.Migrate(t.Context(), db, sqlite.MigrateOptions{})

	if err == nil || !strings.HasPrefix(err.Error(), "sqlite: apply migrations: ") {
		t.Errorf("Migrate() with no lock settings error = %v, want it refused under the sqlite prefix", err)
	}
	if got := coreTables(t, db); got != 0 {
		t.Errorf("Migrate() with no lock settings created %d core tables, want none", got)
	}
}

func TestMigrateGivesUpOnAMigrationLockHeldPastItsWait(t *testing.T) {
	t.Parallel()

	db := withAccounts(t)
	migrationLock(t, db)

	start := time.Now()
	err := sqlite.Migrate(t.Context(), db, sqlite.MigrateOptions{
		LockWait: 100 * time.Millisecond, LockPoll: 10 * time.Millisecond,
	})
	waited := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "migration lock") {
		t.Errorf("Migrate() behind a held lock error = %v, want the migration lock named", err)
	}
	if got := coreTables(t, db); got != 0 {
		t.Errorf("Migrate() behind a held lock created %d core tables, want none", got)
	}
	if waited < 100*time.Millisecond {
		t.Errorf("Migrate() gave up after %v, want it to wait out the lock wait of 100ms", waited)
	}
}
