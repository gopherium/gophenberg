// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
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

// backingUpThrough returns a command line on the SQLite file at path whose one command is database:backup over take.
func backingUpThrough(
	path string, take func(context.Context, *sql.DB, sqlite.BackupRequest) (sqlite.TakenBackup, error),
) gonsole.Program {
	getenv := migratingSQLite(path)
	return gonsole.Program{
		Name: "gophenberg", Env: settingsEnv(getenv), Database: siteAddressSetting, Open: handleOpener(getenv),
		Commands: []gonsole.Command{onDemandBackup{take: take}.command()},
	}
}

// manualBackupsIn returns every manual backup folder holds, the newest first.
func manualBackupsIn(t *testing.T, folder string) []backupfiles.Backup {
	t.Helper()
	backups, err := backupfiles.Folder(folder).List()
	if err != nil {
		t.Fatalf("listing the backups in %s: %v", folder, err)
	}
	return slices.DeleteFunc(backups, func(backup backupfiles.Backup) bool { return backup.Kind != backupfiles.Manual })
}

// oneManualBackupIn returns the one manual backup folder holds, failing t unless it holds exactly one.
func oneManualBackupIn(t *testing.T, folder string) backupfiles.Backup {
	t.Helper()
	backups := manualBackupsIn(t, folder)
	if len(backups) != 1 {
		t.Fatalf("%s holds the manual backups %+v, want one", folder, backups)
	}
	return backups[0]
}

// backedUpLine returns the line database:backup prints for a backup it put at path.
func backedUpLine(path string) string {
	return "backed up the database to " + path + "\n"
}

func TestDatabaseBackupPrintsWhereItPutTheCopy(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)

	got := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "database:backup")

	if got.Code != gonsole.ExitDone || got.Stderr != "" {
		t.Fatalf("database:backup = %d with stderr %q, want 0 and nothing on stderr", got.Code, got.Stderr)
	}
	backup := filepath.Join(backupsBeside(path), oneManualBackupIn(t, backupsBeside(path)).Name())
	if want := backedUpLine(backup); got.Stdout != want {
		t.Errorf("database:backup printed %q, want %q", got.Stdout, want)
	}
	if held := lastMigratedBy(t, copiedBackup(t, backup)); held != thisBuild() {
		t.Errorf("the backup records %+v as its last migration, want the file's %+v", held, thisBuild())
	}
}

func TestDatabaseBackupPrintsACheckpointError(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	copied := filepath.Join(backupsBeside(path), "copy.sqlite")
	take, _ := stubbedTake(sqlite.TakenBackup{Path: copied, AfterCopy: errAfterCopy}, nil)

	got := testkit.Run(t, backingUpThrough(path, take), "", "database:backup")

	want := "the backup stands, though a step after the copy failed: checkpoint failure\n"
	if got.Code != gonsole.ExitDone || got.Stdout != backedUpLine(copied) || got.Stderr != want {
		t.Errorf("database:backup = %d, stdout %q, stderr %q, want 0, %q and %q on a line of its own",
			got.Code, got.Stdout, got.Stderr, backedUpLine(copied), want)
	}
}

func TestDatabaseBackupNeverMigrates(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)

	got := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "database:backup")

	if got.Code != gonsole.ExitDone {
		t.Fatalf("database:backup = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	if held := lastMigratedBy(t, openSQLiteFile(t, path)); held.Version != olderVersion {
		t.Errorf("the file records %+v after database:backup, want the %s it held", held, olderVersion)
	}
	if backups := updateBackupsIn(t, backupsBeside(path)); len(backups) != 0 {
		t.Errorf("the backups folder holds the update backups %+v, want none taken before a migration", backups)
	}
	backup := oneManualBackupIn(t, backupsBeside(path))
	if want := backedUpLine(filepath.Join(backupsBeside(path), backup.Name())); got.Stdout != want {
		t.Errorf("database:backup printed %q, want only %q", got.Stdout, want)
	}
	if backup.Version != olderVersion {
		t.Errorf("the manual backup is named for %s, want the %s that last migrated the file", backup.Version, olderVersion)
	}
}

func TestDatabaseBackupPrunesNoOtherBackup(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	update, scheduled := "0.0.1-20200101T031500Z-update.sqlite", "0.0.1-20200102T031500Z-scheduled.sqlite"
	plantBackups(t, path, update, scheduled)
	env := migratingWith(path, map[string]string{"GOPHENBERG_SQLITE_BACKUPS_KEEP": "1"})

	got := testkit.Run(t, Program(env, noPlugins), "", "database:backup")

	if got.Code != gonsole.ExitDone {
		t.Fatalf("database:backup = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	held := folderNames(t, backupsBeside(path))
	if !slices.Contains(held, update) || !slices.Contains(held, scheduled) {
		t.Fatalf("the backups folder holds %q, want %s and %s kept", held, update, scheduled)
	}
	want := []string{".dbkit-snapshot.lock", update, scheduled, oneManualBackupIn(t, backupsBeside(path)).Name()}
	if !slices.Equal(held, want) {
		t.Errorf("the backups folder holds %q, want %q with no backup pruned", held, want)
	}
}

func TestDatabaseBackupStaysThroughALaterPrune(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	env := migratingWith(path, map[string]string{"GOPHENBERG_SQLITE_BACKUPS_KEEP": "1"})
	if got := testkit.Run(t, Program(env, noPlugins), "", "database:backup"); got.Code != gonsole.ExitDone {
		t.Fatalf("database:backup = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	manual := oneManualBackupIn(t, backupsBeside(path))
	execSQLiteFile(t, path, "UPDATE core_settings SET value = '"+olderVersion+"' WHERE key = 'migrated_by'")

	if got := testkit.Run(t, Program(env, noPlugins), "", "migrate"); got.Code != gonsole.ExitDone {
		t.Fatalf("migrate = %d with stderr %q, want 0", got.Code, got.Stderr)
	}

	oneUpdateBackupIn(t, backupsBeside(path))
	if held := oneManualBackupIn(t, backupsBeside(path)); held != manual {
		t.Errorf("the manual backup after the prune is %+v, want %+v kept", held, manual)
	}
}

func TestDatabaseBackupMakesTheDataFolderOnlyItsOwnerOpens(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	data := filepath.Join(t.TempDir(), "data")

	got := testkit.Run(t, Program(migratingWith(path, map[string]string{"GOPHENBERG_DATA_DIR": data}), noPlugins),
		"", "database:backup")

	if got.Code != gonsole.ExitDone {
		t.Fatalf("database:backup = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	folder := filepath.Join(data, "backups")
	if want := backedUpLine(filepath.Join(folder, oneManualBackupIn(t, folder).Name())); got.Stdout != want {
		t.Errorf("database:backup printed %q, want %q", got.Stdout, want)
	}
	if info, err := os.Stat(data); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the data folder after database:backup: %v, %v, want one made with the mode 700", info, err)
	}
	noBackupsFolderBeside(t, path)
}

func TestDatabaseBackupOnPostgreSQLNamesPgDump(t *testing.T) {
	t.Parallel()

	data := filepath.Join(t.TempDir(), "data")
	env := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_DATA_DIR": data,
	})

	got := testkit.Run(t, Program(env, noPlugins), "", "database:backup")

	want := "gophenberg: database:backup copies a SQLite file only, so back up a PostgreSQL database with pg_dump, " +
		"as https://docs.gophenberg.org/self-hosting/updates-and-backups/#what-to-back-up shows\n"
	if got.Code != gonsole.ExitFailed || got.Stdout != "" || got.Stderr != want {
		t.Errorf("database:backup = %d, stdout %q, stderr %q, want 1 and %q", got.Code, got.Stdout, got.Stderr, want)
	}
	if _, err := os.Lstat(data); err == nil {
		t.Errorf("the data folder %s exists after the refusal, want none made", data)
	}
}

func TestDatabaseBackupNamesWhatStopsIt(t *testing.T) {
	t.Parallel()

	onFile := func(path string, settings map[string]string) map[string]string {
		env := map[string]string{"GOPHENBERG_DATABASE_URL": sqlitePrefix + path}
		maps.Copy(env, settings)
		return env
	}
	for name, stop := range map[string]func(t *testing.T) (map[string]string, string){
		"no database setting": func(*testing.T) (map[string]string, string) {
			return map[string]string{}, "GOPHENBERG_DATABASE_URL is required"
		},
		"a bad folder setting": func(t *testing.T) (map[string]string, string) {
			return onFile(migratedSQLiteFile(t), map[string]string{"GOPHENBERG_INSTALL_ROOT": "relative/root"}),
				"GOPHENBERG_INSTALL_ROOT"
		},
		"a missing file": func(t *testing.T) (map[string]string, string) {
			path := missingSQLiteFile(t)
			return onFile(path, nil), namingTheMissingFile(path)
		},
		"a data folder it cannot make": func(t *testing.T) (map[string]string, string) {
			path := migratedSQLiteFile(t)
			return onFile(path, map[string]string{"GOPHENBERG_DATA_DIR": filepath.Join(path, "data")}),
				"make the data folder"
		},
		"a file where the backups folder goes": func(t *testing.T) (map[string]string, string) {
			path := migratedSQLiteFile(t)
			blockBackupsFolder(t, path)
			return onFile(path, nil), "take the backup " + backupsBeside(path)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			env, want := stop(t)

			got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "database:backup")

			if got.Code != gonsole.ExitFailed || got.Stdout != "" || !strings.Contains(got.Stderr, want) {
				t.Errorf("database:backup = %d, stdout %q, stderr %q, want 1, nothing backed up and %q",
					got.Code, got.Stdout, got.Stderr, want)
			}
		})
	}
}
