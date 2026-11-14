// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"syscall"
	"testing"
	"time"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// driveFolder is the backups folder the stubbed drive failures name.
	driveFolder = "/mnt/c/gophenberg-data/backups"
	// noReplaceWords are the words of a backup into driveFolder on a drive that cannot move without replacing.
	noReplaceWords = "the drive of the backups folder " + driveFolder + " cannot move a file without replacing " +
		"one, so set GOPHENBERG_DATA_DIR to a folder on a local disk, then copy the backups off the server"
	// partialCopy is the copy dbkit writes into driveFolder before it moves it into place.
	partialCopy = driveFolder + "/copy.sqlite.dbkit-snapshot.partial"
	// leftCopyWords are the words that name partialCopy once the backup could not remove it.
	leftCopyWords = ", and delete the copy " + partialCopy + " the backup could not remove"
)

var (
	// errRemoveCopy is the failure of the removal of partialCopy after a move that could not keep a target.
	errRemoveCopy = &fs.PathError{Op: "remove", Path: partialCopy, Err: syscall.EBUSY}
	// errCloseLock is the failure of the close of the snapshot lock in driveFolder.
	errCloseLock = &fs.PathError{Op: "close", Path: driveFolder + "/.dbkit-snapshot.lock", Err: syscall.EIO}
)

// onSharedDrive returns the error of a copy into driveFolder on the file system named system, as sqlite wraps it.
func onSharedDrive(system string) error {
	return fmt.Errorf("sqlite: take the backup %s/copy.sqlite: %w", driveFolder,
		&dbsqlite.UnsupportedFileSystemError{Folder: driveFolder, FileSystem: system})
}

// withNoReplaceMove returns the error of a copy into driveFolder whose move cannot keep a target, as dbkit joins it.
func withNoReplaceMove(removed, unlocked error) error {
	moved := &dbsqlite.UnsupportedFileSystemError{Folder: driveFolder, Err: syscall.EINVAL}
	return fmt.Errorf("sqlite: take the backup %s/copy.sqlite: %w", driveFolder,
		errors.Join(errors.Join(moved, removed), unlocked))
}

// sharedDriveWords returns the words of a backup into driveFolder on the file system named system.
func sharedDriveWords(system string) string {
	return "the backups folder " + driveFolder + " is on a network, shared or FUSE drive (" + system + "), so set " +
		"GOPHENBERG_DATA_DIR to a folder on a local disk, then copy the backups off the server"
}

func TestNeedsLocalDiskNamesTheDriveAndTheDataFolderSetting(t *testing.T) {
	t.Parallel()

	names := settingsEnv(testkit.Getenv(nil))
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"a network drive": {onSharedDrive("9P"), sharedDriveWords("9P")},
		"a FUSE mount":    {onSharedDrive("FUSE"), sharedDriveWords("FUSE")},
		"a drive that cannot move a file without replacing one": {withNoReplaceMove(nil, nil), noReplaceWords},
		"a move whose lock did not close":                       {withNoReplaceMove(nil, errCloseLock), noReplaceWords},
		"a move whose copy stayed": {
			withNoReplaceMove(errRemoveCopy, nil), noReplaceWords + leftCopyWords,
		},
		"a move whose copy stayed and lock did not close": {
			withNoReplaceMove(errRemoveCopy, errCloseLock), noReplaceWords + leftCopyWords,
		},
	} {
		got := needsLocalDisk(names, tc.err)

		if fmt.Sprint(got) != tc.want || errors.Is(got, dbsqlite.ErrUnsupportedFileSystem) {
			t.Errorf("needsLocalDisk() with %s = %v, want %q that wraps no dbkit error", name, got, tc.want)
		}
	}
}

func TestNeedsLocalDiskLeavesEveryOtherErrorAsItIs(t *testing.T) {
	t.Parallel()

	names := settingsEnv(testkit.Getenv(nil))
	for name, err := range map[string]error{
		"no error": nil, "a plain error": errRead, "another run's backup": errAnotherBackup,
		"too little free space": shortOf(0, 1),
	} {
		if got := needsLocalDisk(names, err); got != err {
			t.Errorf("needsLocalDisk() with %s = %v, want %v as it is", name, got, err)
		}
	}
}

func TestDatabaseBackupNamesTheDataFolderWhenTheDriveCannotHoldABackup(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	take, _ := stubbedTake(sqlite.TakenBackup{}, onSharedDrive("9P"))

	got := testkit.Run(t, backingUpThrough(path, take), "", "database:backup")

	want := "gophenberg: " + sharedDriveWords("9P") + "\n"
	if got.Code != gonsole.ExitFailed || got.Stdout != "" || got.Stderr != want {
		t.Errorf("database:backup = %d, stdout %q, stderr %q, want 1, nothing on stdout and %q",
			got.Code, got.Stdout, got.Stderr, want)
	}
}

func TestMigrateNamesTheDataFolderWhenTheDriveCannotHoldABackup(t *testing.T) {
	t.Parallel()

	skip, this := "GOPHENBERG_SQLITE_SKIP_BACKUP_FOR", version.Version()
	for name, tc := range map[string]struct {
		floor        string
		err          error
		words, tails string
	}{
		"a network drive under a minimum version that stays": {
			version.Minimum(), onSharedDrive("NFS"), sharedDriveWords("NFS"),
			", or set " + skip + "=" + this + " to migrate this version with no backup",
		},
		"a drive that cannot move a file under a rising minimum version": {
			olderVersion, withNoReplaceMove(nil, nil), noReplaceWords,
			", and " + skip + " does not apply since " + this + " raises the minimum version of the file",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := olderMigratedSQLiteFile(t)
			execSQLiteFile(t, path, "UPDATE core_settings SET value = '"+tc.floor+"' WHERE key = 'minimum_version'")
			needs, _ := stubbedRule(ruleAnswer{needed: true})
			take, _ := stubbedTake(sqlite.TakenBackup{}, tc.err)
			apply, applied := countedApply()
			run := beforeMigrating{needs: needs, take: take}.around(settingsEnv(migratingSQLite(path)), apply)

			err := run(t.Context(), openSQLiteFile(t, path), patientWait)

			want := "take the backup before migrating: " + tc.words + tc.tails
			if fmt.Sprint(err) != want || *applied != 0 {
				t.Errorf("a run whose drive cannot hold a backup = %v after %d migrations, want %q with no disk "+
					"advice and 0", err, *applied, want)
			}
		})
	}
}

func TestTheBackupJobNamesTheDataFolderWhenTheDriveCannotHoldABackup(t *testing.T) {
	t.Parallel()

	take, _ := stubbedTake(sqlite.TakenBackup{}, withNoReplaceMove(nil, nil))
	schedule := scheduleBeside(t, scheduledFile, map[string]string{"GOPHENBERG_SQLITE_BACKUP_INTERVAL": "2h"})
	job := onlyJob(t, scheduledBackup{take: take, logger: slog.New(slog.DiscardHandler)}.jobs(schedule))

	if err := job.Run(t.Context(), time.Now()); fmt.Sprint(err) != noReplaceWords {
		t.Errorf("the backup job error = %v, want %q", err, noReplaceWords)
	}
}
