// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"slices"
	"testing"
	"time"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// marginNoDiskHolds is a disk margin setting at its bound, more free space than any test disk has.
	marginNoDiskHolds = "1073741824"
	// spaceFolder is the backups folder the stubbed free space failures name.
	spaceFolder = "/srv/site/site.db.backups"
)

// shortOf returns the error of a copy into spaceFolder that lacked free space, wrapped as sqlite wraps it.
func shortOf(free, needed int64) error {
	return fmt.Errorf("sqlite: take the backup %s/copy.sqlite: %w", spaceFolder,
		&dbsqlite.SpaceError{Folder: spaceFolder, Needed: needed, Free: free})
}

// spaceWords returns the pattern of the words of a backup into folder that lacks free space, for any megabytes.
func spaceWords(folder string) string {
	return "the backups folder " + regexp.QuoteMeta(folder) + ` has \d+ MB free, and the backup needs \d+ MB for ` +
		"the copy and GOPHENBERG_UPDATE_DISK_MARGIN_MB, so free some disk space first, for example by moving " +
		"backups off the server"
}

func TestShortOfSpaceNamesTheFreeAndNeededMegabytesAndTheMargin(t *testing.T) {
	t.Parallel()

	names := settingsEnv(testkit.Getenv(nil))
	for name, tc := range map[string]struct {
		free, needed     int64
		freeMB, neededMB string
	}{
		"whole megabytes":                {120 << 20, 450 << 20, "120", "450"},
		"a part of a megabyte":           {120<<20 + 1<<20 - 1, 450<<20 + 1, "120", "451"},
		"one byte needed":                {0, 1, "0", "1"},
		"one byte short of a megabyte":   {1<<20 - 1, 1 << 20, "0", "1"},
		"the largest need dbkit returns": {0, math.MaxInt64, "0", "8796093022208"},
	} {
		got := shortOfSpace(names, shortOf(tc.free, tc.needed))

		want := "the backups folder " + spaceFolder + " has " + tc.freeMB + " MB free, and the backup needs " +
			tc.neededMB + " MB for the copy and GOPHENBERG_UPDATE_DISK_MARGIN_MB, so free some disk space first, " +
			"for example by moving backups off the server"
		if fmt.Sprint(got) != want || errors.Is(got, dbsqlite.ErrNoSpace) {
			t.Errorf("shortOfSpace() with %s = %v, want %q that wraps no dbkit error", name, got, want)
		}
	}
}

func TestShortOfSpaceLeavesEveryOtherErrorAsItIs(t *testing.T) {
	t.Parallel()

	names := settingsEnv(testkit.Getenv(nil))
	for name, err := range map[string]error{
		"no error": nil, "a plain error": errRead, "another run's backup": errAnotherBackup,
	} {
		if got := shortOfSpace(names, err); got != err {
			t.Errorf("shortOfSpace() with %s = %v, want %v as it is", name, got, err)
		}
	}
}

func TestDatabaseBackupNamesTheMarginWhenTheFolderLacksFreeSpace(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	env := migratingWith(path, map[string]string{"GOPHENBERG_UPDATE_DISK_MARGIN_MB": marginNoDiskHolds})

	got := testkit.Run(t, Program(env, noPlugins), "", "database:backup")

	want := regexp.MustCompile("^gophenberg: " + spaceWords(backupsBeside(path)) + "\n$")
	if got.Code != gonsole.ExitFailed || got.Stdout != "" || !want.MatchString(got.Stderr) {
		t.Fatalf("database:backup = %d, stdout %q, stderr %q, want 1, nothing backed up and %s",
			got.Code, got.Stdout, got.Stderr, want)
	}
	if held := folderNames(t, backupsBeside(path)); !slices.Equal(held, []string{".dbkit-snapshot.lock"}) {
		t.Errorf("the backups folder holds %q, want only the lock and no backup", held)
	}
}

func TestMigrateStopsBeforeMigratingWhenTheFolderLacksFreeSpace(t *testing.T) {
	t.Parallel()

	skip, this := "GOPHENBERG_SQLITE_SKIP_BACKUP_FOR", regexp.QuoteMeta(version.Version())
	for name, tc := range map[string]struct{ floor, tail string }{
		"a minimum version that stays": {version.Minimum(), ", or set " + skip + "=" + this +
			" to migrate this version with no backup"},
		"a rising minimum version": {olderVersion, ", and " + skip + " does not apply since " + this +
			" raises the minimum version of the file"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := olderMigratedSQLiteFile(t)
			execSQLiteFile(t, path, "UPDATE core_settings SET value = '"+tc.floor+"' WHERE key = 'minimum_version'")
			env := migratingWith(path, map[string]string{"GOPHENBERG_UPDATE_DISK_MARGIN_MB": marginNoDiskHolds})

			got := testkit.Run(t, Program(env, noPlugins), "", "migrate")

			want := regexp.MustCompile("^gophenberg: migrate accounts: take the backup before migrating: " +
				spaceWords(backupsBeside(path)) + tc.tail + "\n$")
			if got.Code != gonsole.ExitFailed || got.Stdout != "" || !want.MatchString(got.Stderr) {
				t.Fatalf("migrate = %d, stdout %q, stderr %q, want 1, nothing migrated and %s with the advice once",
					got.Code, got.Stdout, got.Stderr, want)
			}
			if backups := updateBackupsIn(t, backupsBeside(path)); len(backups) != 0 {
				t.Errorf("the backups folder holds the update backups %+v, want none", backups)
			}
			if held := lastMigratedBy(t, openSQLiteFile(t, path)); held.Version != olderVersion {
				t.Errorf("the file records %+v after the stopped backup, want the %s it held", held, olderVersion)
			}
		})
	}
}

func TestTheBackupJobNamesTheMarginWhenTheFolderLacksFreeSpace(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	schedule, err := backupScheduleFrom(settingsEnv(migratingSQLite(path)), updater.BackupSettings{DiskMargin: 1 << 50})
	if err != nil {
		t.Fatalf("backupScheduleFrom() error = %v, want nil", err)
	}
	job := onlyJob(t, backupJobs(schedule, openSQLiteFile(t, path), slog.New(slog.DiscardHandler)))

	err = job.Run(t.Context(), time.Now())

	if want := regexp.MustCompile("^" + spaceWords(backupsBeside(path)) + "$"); !want.MatchString(fmt.Sprint(err)) {
		t.Errorf("the backup job error = %v, want %s", err, want)
	}
}
