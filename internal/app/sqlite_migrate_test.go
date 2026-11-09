// SPDX-License-Identifier: Apache-2.0

package app

import (
	"database/sql"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
)

const (
	// migratedLines is what migrate prints on a SQLite site, one line per schema step and one for the plugins.
	migratedLines = "migrated accounts\nmigrated records\nmigrated core\nmigrated plugins\n"
	// olderVersion is a version every build of this repository comes after.
	olderVersion = "0.0.1"
)

// versionLookups reads the versions each schema owner's version table holds, keyed by the table's name.
var versionLookups = map[string]string{
	"auth_goose_db_version":    "SELECT version_id FROM auth_goose_db_version ORDER BY version_id",
	"gonsole_goose_db_version": "SELECT version_id FROM gonsole_goose_db_version ORDER BY version_id",
	"core_goose_db_version":    "SELECT version_id FROM core_goose_db_version ORDER BY version_id",
}

// migratingSQLite returns the settings of a site on the SQLite file at path.
func migratingSQLite(path string) func(string) string {
	return testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": sqlitePrefix + path})
}

// openSQLiteFile returns a handle on the SQLite file at path under the default settings, closed when t ends.
func openSQLiteFile(t *testing.T, path string) *sql.DB {
	t.Helper()
	settings, err := sqlite.SettingsFrom(settingsEnv(testkit.Getenv(nil)))
	if err != nil {
		t.Fatalf("reading the default SQLite settings: %v", err)
	}
	db, err := sqlite.Open(sqlitePrefix+path, settings)
	if err != nil {
		t.Fatalf("opening the SQLite file: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("closing the SQLite file: %v", err)
		}
	})
	return db
}

// appliedVersions returns the versions each schema owner's version table holds, keyed by the table's name.
func appliedVersions(t *testing.T, db *sql.DB) map[string][]int64 {
	t.Helper()
	versions := make(map[string][]int64, len(versionLookups))
	for table, lookup := range versionLookups {
		versions[table] = versionsBy(t, db, lookup)
	}
	return versions
}

// versionsBy returns the versions lookup reads from db, in its order.
func versionsBy(t *testing.T, db *sql.DB, lookup string) []int64 {
	t.Helper()
	rows, err := db.QueryContext(t.Context(), lookup)
	if err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	defer func() { _ = rows.Close() }()
	var versions []int64
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			t.Fatalf("%s: %v", lookup, err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	return versions
}

// hasTable reports whether the SQLite file behind db holds the table named name.
func hasTable(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var found bool
	lookup := "SELECT EXISTS (SELECT 1 FROM sqlite_schema WHERE type = 'table' AND name = ?)"
	if err := db.QueryRowContext(t.Context(), lookup, name).Scan(&found); err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	return found
}

// appliedOne reports whether versions holds a migration past the version table's own starting row.
func appliedOne(versions []int64) bool {
	return slices.ContainsFunc(versions, func(version int64) bool { return version > 0 })
}

// olderMigratedSQLiteFile returns the path of a migrated SQLite file that records olderVersion as its last migration.
func olderMigratedSQLiteFile(t *testing.T) string {
	t.Helper()
	path := migratedSQLiteFile(t)
	execSQLiteFile(t, path, "UPDATE core_settings SET value = '"+olderVersion+"' WHERE key = 'migrated_by'")
	return path
}

// backupsBeside returns the backups folder beside the SQLite file at path, named for it.
func backupsBeside(path string) string {
	return path + ".backups"
}

// updateBackupsIn returns every update backup folder holds, the newest first.
func updateBackupsIn(t *testing.T, folder string) []backupfiles.Backup {
	t.Helper()
	backups, err := backupfiles.Folder(folder).List()
	if err != nil {
		t.Fatalf("listing the backups in %s: %v", folder, err)
	}
	return slices.DeleteFunc(backups, func(backup backupfiles.Backup) bool { return backup.Kind != backupfiles.Update })
}

// oneUpdateBackupIn returns the path of the one update backup folder holds, failing t unless it holds exactly one.
func oneUpdateBackupIn(t *testing.T, folder string) string {
	t.Helper()
	backups := updateBackupsIn(t, folder)
	if len(backups) != 1 {
		t.Fatalf("%s holds the update backups %+v, want one", folder, backups)
	}
	return filepath.Join(folder, backups[0].Name())
}

// moveBackupsOff removes every update backup from folder, as an owner who moves each copy off the server does.
func moveBackupsOff(t *testing.T, folder string) {
	t.Helper()
	for _, backup := range updateBackupsIn(t, folder) {
		if err := os.Remove(filepath.Join(folder, backup.Name())); err != nil {
			t.Fatalf("moving %s off the server: %v", backup.Name(), err)
		}
	}
}

// copiedBackup returns a handle on a copy of the backup at path, so that reading it leaves the backup as it was.
func copiedBackup(t *testing.T, path string) *sql.DB {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the backup: %v", err)
	}
	copied := filepath.Join(t.TempDir(), "backup.db")
	if err := os.WriteFile(copied, body, 0o600); err != nil {
		t.Fatalf("copying the backup: %v", err)
	}
	return openSQLiteFile(t, copied)
}

// lastMigratedBy returns the build the SQLite file behind db records as the one that last migrated it.
func lastMigratedBy(t *testing.T, db *sql.DB) sqlite.Build {
	t.Helper()
	build, err := sqlite.LastMigratedBy(t.Context(), db)
	if err != nil {
		t.Fatalf("LastMigratedBy() error = %v, want nil", err)
	}
	return build
}

func TestMigrateBuildsEveryOwnersSchemaOnAnEmptySQLiteFile(t *testing.T) {
	t.Parallel()

	path := emptySQLiteFile(t)
	k := &keeper{}

	got := testkit.Run(t, Program(migratingSQLite(path), keeping(k)), "", "migrate")

	if got.Code != gonsole.ExitDone || got.Stdout != migratedLines {
		t.Fatalf("migrate = %d, stdout %q, stderr %q, want 0 and %q", got.Code, got.Stdout, got.Stderr, migratedLines)
	}
	want := []string{"site.db", "site.db.migrate.lock"}
	if names := folderNames(t, filepath.Dir(path)); !slices.Equal(names, want) {
		t.Errorf("the folder holds %v after migrate, want %v with the run's one handle closed", names, want)
	}
	db := openSQLiteFile(t, path)
	for table, versions := range appliedVersions(t, db) {
		if !appliedOne(versions) {
			t.Errorf("%s holds the versions %v after migrate, want at least one applied migration", table, versions)
		}
	}
	for _, table := range []string{"core_media", "core_cron_jobs"} {
		if !hasTable(t, db, table) {
			t.Errorf("the file holds no %s table after migrate, want the core schema", table)
		}
	}
	if _, err := os.Stat(path + ".migrate.lock"); err != nil {
		t.Errorf("the migration lock file after migrate: %v, want it beside the database file", err)
	}
	if !slices.Contains(k.steps(), "migrate") {
		t.Errorf("the plugin was asked %v, want its schema migrated", k.steps())
	}
}

func TestASecondMigrateOfASQLiteFileChangesNoVersion(t *testing.T) {
	t.Parallel()

	path := emptySQLiteFile(t)
	if first := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "migrate"); first.Code != gonsole.ExitDone {
		t.Fatalf("the first migrate = %d with stderr %q, want 0", first.Code, first.Stderr)
	}
	db := openSQLiteFile(t, path)
	before := appliedVersions(t, db)

	got := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "migrate")

	if got.Code != gonsole.ExitDone || got.Stdout != migratedLines {
		t.Fatalf("the second migrate = %d, stdout %q, stderr %q, want 0 and %q",
			got.Code, got.Stdout, got.Stderr, migratedLines)
	}
	if after := appliedVersions(t, db); !maps.EqualFunc(after, before, slices.Equal) {
		t.Errorf("the versions after a second migrate = %v, want the %v the first one left", after, before)
	}
}

func TestMigrateNamesABadSQLiteLockSettingBeforeTouchingTheFile(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT", "GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := emptySQLiteFile(t)
			env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": sqlitePrefix + path, name: "0"})

			got := testkit.Run(t, Program(env, noPlugins), "", "migrate")

			if want := "migrate accounts: " + name; got.Code != gonsole.ExitFailed || got.Stdout != "" ||
				!strings.Contains(got.Stderr, want) {
				t.Errorf("migrate = %d, stdout %q, stderr %q, want 1, nothing migrated and %q",
					got.Code, got.Stdout, got.Stderr, want)
			}
			if names := folderNames(t, filepath.Dir(path)); !slices.Equal(names, []string{"site.db"}) {
				t.Errorf("the folder holds %v after migrate, want only site.db with no lock file", names)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("reading the file: %v", err)
			}
			if info.Size() != 0 {
				t.Errorf("the file holds %d bytes after migrate, want 0", info.Size())
			}
		})
	}
}

func TestAPostgreSQLProgramMigratesEveryStepByAddress(t *testing.T) {
	t.Parallel()

	program := Program(testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL}), noPlugins)

	names := make([]string, 0, len(program.Migrations))
	for _, step := range program.Migrations {
		if step.Run == nil || step.RunOn != nil {
			t.Errorf("step %q sets RunOn on a PostgreSQL address, want only Run so it takes the address", step.Name)
		}
		names = append(names, step.Name)
	}
	if want := []string{"accounts", "records", "core"}; !slices.Equal(names, want) {
		t.Errorf("the PostgreSQL steps are %v, want %v", names, want)
	}
}

func TestMigrateBacksUpAFileAnOlderVersionMigrated(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)

	got := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "migrate")

	if got.Code != gonsole.ExitDone || got.Stdout != migratedLines {
		t.Fatalf("migrate = %d, stdout %q, stderr %q, want 0 and %q", got.Code, got.Stdout, got.Stderr, migratedLines)
	}
	backup := oneUpdateBackupIn(t, backupsBeside(path))
	if named, _ := backupfiles.Read(filepath.Base(backup)); named.Version != olderVersion {
		t.Errorf("the update backup is named %s, want the %s that last migrated the file", backup, olderVersion)
	}
	if held := lastMigratedBy(t, copiedBackup(t, backup)); held.Version != olderVersion {
		t.Errorf("the update backup records %+v as its last migration, want the file as %s left it", held, olderVersion)
	}
	if held := lastMigratedBy(t, openSQLiteFile(t, path)); held != thisBuild() {
		t.Errorf("the file records %+v as its last migration after migrate, want %+v", held, thisBuild())
	}
}

func TestASecondMigrateOfOneVersionTakesNoBackup(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	if first := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "migrate"); first.Code != gonsole.ExitDone {
		t.Fatalf("the first migrate = %d with stderr %q, want 0", first.Code, first.Stderr)
	}
	oneUpdateBackupIn(t, backupsBeside(path))
	moveBackupsOff(t, backupsBeside(path))

	got := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "migrate")

	if got.Code != gonsole.ExitDone || got.Stdout != migratedLines {
		t.Fatalf("the second migrate = %d, stdout %q, stderr %q, want 0 and %q",
			got.Code, got.Stdout, got.Stderr, migratedLines)
	}
	if backups := updateBackupsIn(t, backupsBeside(path)); len(backups) != 0 {
		t.Errorf("the second migrate left the update backups %+v, want none", backups)
	}
}

func TestUpdateCheckOnANewVersionBacksUpOnce(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	site := offeringSite(t)
	env := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": sqlitePrefix + path, "GOPHENBERG_UPDATE_URL": site.address,
		"GOPHENBERG_TRUSTED_KEYS": site.trustedRoots(),
	})

	first := testkit.Run(t, Program(env, noPlugins), "", "update:check")
	backups := updateBackupsIn(t, backupsBeside(path))
	moveBackupsOff(t, backupsBeside(path))
	second := testkit.Run(t, Program(env, noPlugins), "", "update:check")

	for i, got := range []testkit.Result{first, second} {
		if got.Code != gonsole.ExitDone {
			t.Errorf("update:check call %d of 2 = %d with stderr %q, want 0", i+1, got.Code, got.Stderr)
		}
	}
	if len(backups) != 1 || backups[0].Version != olderVersion {
		t.Errorf("the first update:check left the update backups %+v, want one of %s", backups, olderVersion)
	}
	if again := updateBackupsIn(t, backupsBeside(path)); len(again) != 0 {
		t.Errorf("the second update:check left the update backups %+v, want none", again)
	}
}

func TestAFileWithANewerFloorStopsBeforeAnyBackup(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	execSQLiteFile(t, path, "UPDATE core_settings SET value = '"+newerMinimumVersion+"' WHERE key = 'minimum_version'")

	got := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "migrate")

	if want := "this database needs Gophenberg " + newerMinimumVersion + " or newer"; got.Code != gonsole.ExitFailed ||
		!strings.Contains(got.Stderr, want) {
		t.Fatalf("migrate = %d with stderr %q, want 1 and %q", got.Code, got.Stderr, want)
	}
	if _, err := os.Lstat(backupsBeside(path)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the backups folder after the refusal: %v, want none made", err)
	}
}
