// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

var (
	// errAnotherBackup is what a stubbed copy answers while another backup writes into its folder.
	errAnotherBackup = fmt.Errorf("sqlite: another run took the backup: %w", dbsqlite.ErrSnapshotRunning)
	// errRunEnded is the cause a stubbed copy ends its run with.
	errRunEnded = errors.New("run ended")
	// errAfterCopy is the failure a stubbed copy reports after its copy landed.
	errAfterCopy = errors.New("checkpoint failure")
)

// patientWait is a lock wait no test reaches, polled often.
var patientWait = sqlite.MigrateOptions{LockWait: time.Minute, LockPoll: time.Millisecond}

// ruleAnswer is one answer of a stubbed backup rule.
type ruleAnswer struct {
	needed bool
	err    error
}

// stubbedRule returns a backup rule that gives answers in turn, the last one again once they run out, and its reads.
func stubbedRule(answers ...ruleAnswer) (func(context.Context, *sql.DB, sqlite.Build) (bool, error), *int) {
	reads := 0
	return func(context.Context, *sql.DB, sqlite.Build) (bool, error) {
		answer := answers[min(reads, len(answers)-1)]
		reads++
		return answer.needed, answer.err
	}, &reads
}

// stubbedTake returns a copy that answers taken and err, and the count of its calls.
func stubbedTake(
	taken sqlite.TakenBackup, err error,
) (func(context.Context, *sql.DB, sqlite.BackupRequest) (sqlite.TakenBackup, error), *int) {
	calls := 0
	return func(context.Context, *sql.DB, sqlite.BackupRequest) (sqlite.TakenBackup, error) {
		calls++
		return taken, err
	}, &calls
}

// countedApply returns a schema step run that answers nil, and the count of its calls.
func countedApply() (func(context.Context, *sql.DB, sqlite.MigrateOptions) error, *int) {
	calls := 0
	return func(context.Context, *sql.DB, sqlite.MigrateOptions) error {
		calls++
		return nil
	}, &calls
}

// migratingWith returns the settings of a site on the SQLite file at path, with the other settings given.
func migratingWith(path string, settings map[string]string) func(string) string {
	env := map[string]string{"GOPHENBERG_DATABASE_URL": sqlitePrefix + path}
	maps.Copy(env, settings)
	return testkit.Getenv(env)
}

// blockBackupsFolder plants a plain file where the backups folder beside the SQLite file at path goes.
func blockBackupsFolder(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(backupsBeside(path), nil, 0o600); err != nil {
		t.Fatalf("planting a file where the backups folder goes: %v", err)
	}
}

// noBackupsFolderBeside fails t when a backups folder stands beside the SQLite file at path.
func noBackupsFolderBeside(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(backupsBeside(path)); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the backups folder beside the file: %v, want none made", err)
	}
}

func TestTheBackupFolderIsTheDataFolderTheRootOrBesideTheFile(t *testing.T) {
	t.Parallel()

	address := "sqlite:/srv/site/site.db"
	for _, tc := range []struct {
		layout updater.Layout
		want   string
	}{
		{updater.Layout{Root: "/srv/root", Data: "/srv/data"}, "/srv/data/backups"},
		{updater.Layout{Root: "/srv/root"}, "/srv/root/backups"},
		{updater.Layout{}, "/srv/site/backups"},
	} {
		if got := backupsFolder(tc.layout, address); got != tc.want {
			t.Errorf("backupsFolder(%+v, %q) = %q, want %q", tc.layout, address, got, tc.want)
		}
	}
}

func TestABackupBeforeMigratingMakesTheDataFolderOnlyItsOwnerOpens(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	data := filepath.Join(t.TempDir(), "data")

	got := testkit.Run(t, Program(migratingWith(path, map[string]string{"GOPHENBERG_DATA_DIR": data}), noPlugins),
		"", "migrate")

	if got.Code != gonsole.ExitDone {
		t.Fatalf("migrate = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	oneUpdateBackupIn(t, filepath.Join(data, "backups"))
	if info, err := os.Stat(data); err != nil || info.Mode().Perm() != 0o700 {
		t.Errorf("the data folder after migrate: %v, %v, want one made with the mode 700", info, err)
	}
	noBackupsFolderBeside(t, path)
}

func TestABackupBeforeMigratingKeepsTheOneCount(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	scheduled, manual := "0.0.1-20200101T031500Z-scheduled.sqlite", "0.0.1-20200102T101500Z-manual.sqlite"
	if err := os.Mkdir(backupsBeside(path), 0o700); err != nil {
		t.Fatalf("making the backups folder: %v", err)
	}
	for _, name := range []string{scheduled, manual} {
		if err := os.WriteFile(filepath.Join(backupsBeside(path), name), nil, 0o600); err != nil {
			t.Fatalf("planting %s: %v", name, err)
		}
	}

	got := testkit.Run(t, Program(migratingWith(path, map[string]string{"GOPHENBERG_SQLITE_BACKUPS_KEEP": "1"}),
		noPlugins), "", "migrate")

	if got.Code != gonsole.ExitDone {
		t.Fatalf("migrate = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	backup := filepath.Base(oneUpdateBackupIn(t, backupsBeside(path)))
	want := []string{".dbkit-snapshot.lock", manual, backup}
	if held := folderNames(t, backupsBeside(path)); !slices.Equal(held, want) {
		t.Errorf("the backups folder holds %q, want %q with the scheduled backup pruned", held, want)
	}
}

func TestABadUpdateSettingStopsNoBackupBeforeMigrating(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	env := migratingWith(path, map[string]string{
		"GOPHENBERG_UPDATE_BACKUPS_KEEP": "0", "GOPHENBERG_UPDATE_DOWNLOAD_CAP_MB": "0",
	})

	got := testkit.Run(t, Program(env, noPlugins), "", "migrate")

	if got.Code != gonsole.ExitDone || got.Stdout != migratedLines {
		t.Fatalf("migrate = %d, stdout %q, stderr %q, want 0 and %q", got.Code, got.Stdout, got.Stderr, migratedLines)
	}
	oneUpdateBackupIn(t, backupsBeside(path))
}

func TestMigrateNamesABadBackupSetting(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]string{
		"GOPHENBERG_SQLITE_BACKUPS_KEEP": "0", "GOPHENBERG_INSTALL_ROOT": "relative/root",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := emptySQLiteFile(t)

			got := testkit.Run(t, Program(migratingWith(path, map[string]string{name: value}), noPlugins), "", "migrate")

			if want := "migrate accounts: " + name; got.Code != gonsole.ExitFailed || got.Stdout != "" ||
				!strings.Contains(got.Stderr, want) {
				t.Errorf("migrate = %d, stdout %q, stderr %q, want 1, nothing migrated and %q",
					got.Code, got.Stdout, got.Stderr, want)
			}
		})
	}
}

func TestAFailedBackupStopsTheMigrationAndNamesTheSkipSetting(t *testing.T) {
	t.Parallel()

	for name, block := range map[string]func(t *testing.T, path string) map[string]string{
		"a file where the backups folder goes": func(t *testing.T, path string) map[string]string {
			blockBackupsFolder(t, path)
			return nil
		},
		"a data folder it cannot make": func(_ *testing.T, path string) map[string]string {
			return map[string]string{"GOPHENBERG_DATA_DIR": filepath.Join(path, "data")}
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := olderMigratedSQLiteFile(t)
			env := migratingWith(path, block(t, path))

			got := testkit.Run(t, Program(env, noPlugins), "", "migrate")

			if got.Code != gonsole.ExitFailed || got.Stdout != "" {
				t.Fatalf("migrate = %d, stdout %q, stderr %q, want 1 and nothing migrated", got.Code, got.Stdout, got.Stderr)
			}
			for _, want := range []string{
				"migrate accounts: take the backup before migrating: ", "so free room by moving backups off the server",
				"or set GOPHENBERG_SQLITE_SKIP_BACKUP_FOR=" + version.Version() + " to migrate this version with no backup",
			} {
				if !strings.Contains(got.Stderr, want) {
					t.Errorf("migrate stderr %q, want it to carry %q", got.Stderr, want)
				}
			}
			if held := lastMigratedBy(t, openSQLiteFile(t, path)); held.Version != olderVersion {
				t.Errorf("the file records %+v after the failed backup, want the %s it held", held, olderVersion)
			}
		})
	}
}

func TestAFailedBackupOnARisingFloorSaysTheSkipCannotHelp(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	execSQLiteFile(t, path, "UPDATE core_settings SET value = '"+olderVersion+"' WHERE key = 'minimum_version'")
	blockBackupsFolder(t, path)
	env := migratingWith(path, map[string]string{"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR": version.Version()})

	got := testkit.Run(t, Program(env, noPlugins), "", "migrate")

	if got.Code != gonsole.ExitFailed || got.Stdout != "" || strings.Contains(got.Stderr, "with no backup") {
		t.Fatalf("migrate = %d, stdout %q, stderr %q, want 1, nothing migrated and no offer to skip",
			got.Code, got.Stdout, got.Stderr)
	}
	for _, want := range []string{
		"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR does not apply since " + version.Version() +
			" raises the minimum version of the file", "so free room by moving backups off the server",
	} {
		if !strings.Contains(got.Stderr, want) {
			t.Errorf("migrate stderr %q, want it to carry %q", got.Stderr, want)
		}
	}
}

func TestTheSkipSettingLetsItsVersionMigrateWithNoBackup(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	env := migratingWith(path, map[string]string{"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR": version.Version()})

	got := testkit.Run(t, Program(env, noPlugins), "", "migrate")

	if got.Code != gonsole.ExitDone || got.Stdout != migratedLines {
		t.Fatalf("migrate = %d, stdout %q, stderr %q, want 0 and %q", got.Code, got.Stdout, got.Stderr, migratedLines)
	}
	noBackupsFolderBeside(t, path)
	if held := lastMigratedBy(t, openSQLiteFile(t, path)); held != thisBuild() {
		t.Errorf("the file records %+v as its last migration, want %+v", held, thisBuild())
	}
}

func TestTheSkipSettingNeverSkipsARisingFloor(t *testing.T) {
	t.Parallel()

	for name, floor := range map[string]string{
		"a minimum version below this binary's": "UPDATE core_settings SET value = '" + olderVersion +
			"' WHERE key = 'minimum_version'",
		"no minimum version": "DELETE FROM core_settings WHERE key = 'minimum_version'",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := olderMigratedSQLiteFile(t)
			execSQLiteFile(t, path, floor)
			env := migratingWith(path, map[string]string{"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR": version.Version()})

			got := testkit.Run(t, Program(env, noPlugins), "", "migrate")

			if got.Code != gonsole.ExitDone {
				t.Fatalf("migrate = %d with stderr %q, want 0", got.Code, got.Stderr)
			}
			oneUpdateBackupIn(t, backupsBeside(path))
		})
	}
}

func TestTheSkipSettingForAnotherVersionDoesNothing(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	env := migratingWith(path, map[string]string{"GOPHENBERG_SQLITE_SKIP_BACKUP_FOR": olderVersion})

	got := testkit.Run(t, Program(env, noPlugins), "", "migrate")

	if got.Code != gonsole.ExitDone {
		t.Fatalf("migrate = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	oneUpdateBackupIn(t, backupsBeside(path))
}

func TestABackupBeforeMigratingStopsWhenItCannotRecordTheCopy(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	execSQLiteFile(t, path, "CREATE TRIGGER refuse_the_backup_record BEFORE INSERT ON core_settings "+
		"WHEN NEW.key = 'backed_up_for' BEGIN SELECT RAISE(ABORT, 'planted refusal'); END")

	got := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "migrate")

	if want := "record the backup taken before migrating"; got.Code != gonsole.ExitFailed || got.Stdout != "" ||
		!strings.Contains(got.Stderr, want) {
		t.Fatalf("migrate = %d, stdout %q, stderr %q, want 1, nothing migrated and %q",
			got.Code, got.Stdout, got.Stderr, want)
	}
	oneUpdateBackupIn(t, backupsBeside(path))
}

func TestABackupBeforeMigratingCountsACopyWhoseLastStepsFailed(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	db := openSQLiteFile(t, path)
	take, _ := stubbedTake(sqlite.TakenBackup{
		Path: filepath.Join(backupsBeside(path), "copy.sqlite"), AfterCopy: errAfterCopy, Prune: errRead,
	}, nil)
	apply, applied := countedApply()
	run := beforeMigrating{needs: sqlite.NeedsBackup, take: take}.around(settingsEnv(migratingSQLite(path)), apply)

	if err := run(t.Context(), db, patientWait); err != nil || *applied != 1 {
		t.Fatalf("a run whose copy landed with later failures = %v after %d migrations, want nil after 1", err, *applied)
	}
	if needed, err := sqlite.NeedsBackup(t.Context(), db, thisBuild()); err != nil || needed {
		t.Errorf("NeedsBackup() after that copy = %t, %v, want the copy recorded as taken", needed, err)
	}
}

func TestARunThatMeetsAnotherRunsBackupNeverCopies(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	needs, reads := stubbedRule(ruleAnswer{needed: true}, ruleAnswer{needed: true}, ruleAnswer{})
	take, takes := stubbedTake(sqlite.TakenBackup{}, errAnotherBackup)
	apply, applied := countedApply()
	run := beforeMigrating{needs: needs, take: take}.around(settingsEnv(migratingSQLite(path)), apply)

	err := run(t.Context(), openSQLiteFile(t, path), patientWait)

	if err != nil || *takes != 1 || *reads != 3 || *applied != 1 {
		t.Errorf("a run that met another run's backup = %v after %d copies, %d reads of the rule and %d migrations, "+
			"want nil after 1, 3 and 1", err, *takes, *reads, *applied)
	}
}

func TestARunThatMeetsAnotherRunsBackupStopsAfterTheLockWait(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	needs, _ := stubbedRule(ruleAnswer{needed: true})
	take, takes := stubbedTake(sqlite.TakenBackup{}, errAnotherBackup)
	apply, applied := countedApply()
	run := beforeMigrating{needs: needs, take: take}.around(settingsEnv(migratingSQLite(path)), apply)

	err := run(t.Context(), openSQLiteFile(t, path),
		sqlite.MigrateOptions{LockWait: 50 * time.Millisecond, LockPoll: 10 * time.Millisecond})

	for _, want := range []string{
		backupsBeside(path), "GOPHENBERG_SQLITE_MIGRATE_LOCK_WAIT of 50ms", "GOPHENBERG_SQLITE_MIGRATE_LOCK_POLL of 10ms",
	} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("a run whose wait for another backup passed = %v, want an error naming %q", err, want)
		}
	}
	if *takes != 1 || *applied != 0 {
		t.Errorf("that run copied %d times and migrated %d times, want 1 and 0", *takes, *applied)
	}
}

func TestARunThatMeetsAnotherRunsBackupStopsWhenItEnds(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	needs, _ := stubbedRule(ruleAnswer{needed: true})
	take := func(context.Context, *sql.DB, sqlite.BackupRequest) (sqlite.TakenBackup, error) {
		cancel(errRunEnded)
		return sqlite.TakenBackup{}, errAnotherBackup
	}
	apply, applied := countedApply()
	run := beforeMigrating{needs: needs, take: take}.around(settingsEnv(migratingSQLite(path)), apply)

	err := run(ctx, openSQLiteFile(t, path), sqlite.MigrateOptions{LockWait: time.Minute, LockPoll: time.Minute})

	if !errors.Is(err, errRunEnded) || *applied != 0 {
		t.Errorf("a run that ended while it waited for another backup = %v after %d migrations, want its cause and 0",
			err, *applied)
	}
}

func TestABackupBeforeMigratingReportsARuleItCouldNotRead(t *testing.T) {
	t.Parallel()

	for name, answers := range map[string][]ruleAnswer{
		"before the copy":                         {{err: errRead}},
		"while it waits for another run's backup": {{needed: true}, {err: errRead}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := olderMigratedSQLiteFile(t)
			needs, _ := stubbedRule(answers...)
			take, _ := stubbedTake(sqlite.TakenBackup{}, errAnotherBackup)
			apply, applied := countedApply()
			run := beforeMigrating{needs: needs, take: take}.around(settingsEnv(migratingSQLite(path)), apply)

			if err := run(t.Context(), openSQLiteFile(t, path), patientWait); !errors.Is(err, errRead) || *applied != 0 {
				t.Errorf("a run whose rule failed = %v after %d migrations, want the failure and 0", err, *applied)
			}
		})
	}
}

func TestABackupBeforeMigratingReportsAMinimumVersionItCouldNotRead(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	db, err := openDefaultSQLite(sqlitePrefix + path)
	if err != nil {
		t.Fatalf("opening the SQLite file: %v", err)
	}
	_ = db.Close()
	needs, _ := stubbedRule(ruleAnswer{needed: true})
	take, takes := stubbedTake(sqlite.TakenBackup{}, nil)
	apply, applied := countedApply()
	run := beforeMigrating{needs: needs, take: take}.around(settingsEnv(migratingSQLite(path)), apply)

	if err := run(t.Context(), db, patientWait); err == nil || *takes != 0 || *applied != 0 {
		t.Errorf("a run on a closed handle = %v after %d copies and %d migrations, want an error and 0 of each",
			err, *takes, *applied)
	}
}
