// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
)

// snapshotLock is the lock file dbkit keeps in a folder it copies a database into.
const snapshotLock = ".dbkit-snapshot.lock"

// backupFolder returns a backups folder no test made yet, beside a fresh folder of t.
func backupFolder(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "backups")
}

// takeBackup takes a backup of db as request asks, failing the test unless the copy lands.
func takeBackup(t *testing.T, db *sql.DB, request sqlite.BackupRequest) sqlite.TakenBackup {
	t.Helper()
	taken, err := sqlite.TakeBackup(t.Context(), db, request)
	if err != nil {
		t.Fatalf("TakeBackup(%+v) error = %v, want nil", request, err)
	}
	return taken
}

// plantBackups makes folder and writes an empty file under each name in it.
func plantBackups(t *testing.T, folder string, names ...string) {
	t.Helper()
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatalf("making %s: %v", folder, err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(folder, name), nil, 0o600); err != nil {
			t.Fatalf("planting %s: %v", name, err)
		}
	}
}

// folderHolds returns the name of every entry in folder, in name order.
func folderHolds(t *testing.T, folder string) []string {
	t.Helper()
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatalf("reading %s: %v", folder, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// mode returns the permission bits of the file or folder at path.
func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return info.Mode().Perm()
}

// otherGroup returns a group the test user belongs to besides its own, skipping the test when it has none.
func otherGroup(t *testing.T) int {
	t.Helper()
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatalf("reading the groups of the test user: %v", err)
	}
	for _, group := range groups {
		if group != os.Getegid() {
			return group
		}
	}
	t.Skip("the test user belongs to no group besides its own")
	return 0
}

// liveFile returns the path of the main file of db.
func liveFile(t *testing.T, db *sql.DB) string {
	t.Helper()
	var path string
	if err := db.QueryRowContext(t.Context(), "SELECT file FROM pragma_database_list WHERE name = 'main'").
		Scan(&path); err != nil {
		t.Fatalf("reading the database path: %v", err)
	}
	return path
}

func TestABackupLandsInItsFolderUnderItsName(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	recordMigratedBy(t, db, sqlite.Build{Version: "0.20.0", Commit: "3b96de93"})
	mustExec(t, db, "INSERT INTO core_settings (key, value) VALUES ('site.title', 'Kept in the backup')")
	folder := backupFolder(t)
	before := time.Now().Truncate(time.Second)

	taken := takeBackup(t, db, sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Update, Kept: 3})

	backup, named := backupfiles.Read(filepath.Base(taken.Path))
	if !named || filepath.Dir(taken.Path) != folder || backup.Version != "0.20.0" || backup.Kind != backupfiles.Update ||
		backup.At.Before(before) || backup.At.After(time.Now()) {
		t.Fatalf("TakeBackup().Path = %q, read as %+v, want an update backup of 0.20.0 taken now in %s",
			taken.Path, backup, folder)
	}
	copied, err := sqlite.Open("sqlite:"+taken.Path, testSettings())
	if err != nil {
		t.Fatalf("opening the backup: %v", err)
	}
	t.Cleanup(func() { _ = copied.Close() })
	if got := count(t, copied, "SELECT count(*) FROM core_settings WHERE key = 'site.title'"); got != 1 {
		t.Errorf("the backup holds %d rows of the site title, want the 1 the file held", got)
	}
	if taken.AfterCopy != nil || taken.Prune != nil {
		t.Errorf("TakeBackup() = %+v, want no error after the copy and none from the prune", taken)
	}
}

func TestABackupOfAFileThatRecordsNoBuildNamesItUnknown(t *testing.T) {
	t.Parallel()

	taken := takeBackup(t, migrated(t), sqlite.BackupRequest{Folder: backupFolder(t), Kind: backupfiles.Manual})

	if backup, _ := backupfiles.Read(filepath.Base(taken.Path)); backup.Version != backupfiles.UnknownVersion {
		t.Errorf("TakeBackup().Path = %q, want a name of the unknown version", taken.Path)
	}
}

func TestABackupMakesAMissingFolderOnlyItsOwnerOpens(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)

	takeBackup(t, migrated(t), sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})

	if got := mode(t, folder); got != 0o700 {
		t.Errorf("the backups folder has the mode %o, want 700", got)
	}
}

func TestABackupOnlyItsOwnerReads(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	if err := os.Chmod(liveFile(t, db), 0o644); err != nil {
		t.Fatalf("opening the live file to every reader: %v", err)
	}
	folder := backupFolder(t)
	if err := os.Mkdir(folder, 0o755); err != nil {
		t.Fatalf("making an open backups folder: %v", err)
	}

	taken := takeBackup(t, db, sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})

	if got := mode(t, taken.Path); got != 0o600 {
		t.Errorf("the backup has the mode %o, want 600", got)
	}
	if got := mode(t, folder); got != 0o755 {
		t.Errorf("the backups folder has the mode %o, want the 755 it had", got)
	}
}

func TestABackupNamesAFolderItCannotMake(t *testing.T) {
	t.Parallel()

	folder := filepath.Join(t.TempDir(), "missing", "backups")

	_, err := sqlite.TakeBackup(t.Context(), migrated(t), sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})

	if err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("TakeBackup() into %s error = %v, want the folder it could not make", folder, err)
	}
}

func TestABackupNamesAHandleItCannotRead(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	_ = db.Close()

	_, err := sqlite.TakeBackup(t.Context(), db, sqlite.BackupRequest{Folder: backupFolder(t), Kind: backupfiles.Manual})

	if err == nil {
		t.Error("TakeBackup() on a closed handle error = nil, want one")
	}
}

func TestAFolderMadeAsRootTakesTheOwnerOfTheDatabaseFile(t *testing.T) {
	sqlite.RunAsRoot(t)
	db := migrated(t)
	folder := backupFolder(t)

	takeBackup(t, db, sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})

	var file, made syscall.Stat_t
	if err := errors.Join(syscall.Stat(liveFile(t, db), &file), syscall.Stat(folder, &made)); err != nil {
		t.Fatalf("reading the owners: %v", err)
	}
	if made.Uid != file.Uid || made.Gid != file.Gid {
		t.Errorf("the backups folder belongs to %d:%d, want the %d:%d of the database file",
			made.Uid, made.Gid, file.Uid, file.Gid)
	}
}

func TestAFolderMadeAsRootNamesAFileItCannotRead(t *testing.T) {
	sqlite.RunAsRoot(t)
	db := migrated(t)
	_ = db.Close()
	folder := backupFolder(t)

	_, err := sqlite.TakeBackup(t.Context(), db, sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})

	if want := "read the owner of the database file"; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("TakeBackup() as root on a closed handle error = %v, want one naming %q", err, want)
	}
	if _, err := os.Lstat(folder); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the backups folder after a root run that could not read the owner: %v, want none made", err)
	}
}

func TestAFolderMadeAsRootTakesTheGroupOfTheDatabaseFile(t *testing.T) {
	sqlite.RunAsRoot(t)
	db := migrated(t)
	group := otherGroup(t)
	if err := os.Chown(liveFile(t, db), -1, group); err != nil {
		t.Fatalf("moving the database file to the group %d: %v", group, err)
	}
	folder := backupFolder(t)

	takeBackup(t, db, sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})

	var made syscall.Stat_t
	if err := syscall.Stat(folder, &made); err != nil || int(made.Gid) != group {
		t.Errorf("the backups folder belongs to the group %d (%v), want the group %d of the database file",
			made.Gid, err, group)
	}
}

func TestGivingAFolderRefusesALinkInItsPlace(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "elsewhere")
	link := filepath.Join(dir, "backups")
	if err := errors.Join(os.Mkdir(target, 0o700), os.Symlink(target, link)); err != nil {
		t.Fatalf("planting a link where the folder was: %v", err)
	}
	var owner syscall.Stat_t
	if err := syscall.Stat(target, &owner); err != nil {
		t.Fatalf("reading the owner: %v", err)
	}

	if err := sqlite.GiveFolder(link, owner); err == nil {
		t.Error("GiveFolder() through a link error = nil, want a refusal")
	}
	if _, err := os.Lstat(link); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the link after the refusal: %v, want it removed", err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Errorf("the folder the link named: %v, want it left alone", err)
	}
}

func TestABackupWhoseNameAnotherRunTookAnswersErrSnapshotRunning(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)
	now := time.Now()
	for second := range 5 {
		taken := backupfiles.Backup{
			Version: backupfiles.UnknownVersion, At: now.Add(time.Duration(second) * time.Second), Kind: backupfiles.Manual,
		}
		plantBackups(t, folder, taken.Name())
	}

	_, err := sqlite.TakeBackup(t.Context(), migrated(t), sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})

	if !errors.Is(err, dbsqlite.ErrSnapshotRunning) || !strings.Contains(err.Error(), "another run took the backup") {
		t.Errorf("TakeBackup() of a name another run took error = %v, want ErrSnapshotRunning", err)
	}
}

func TestABackupItCannotKeepFromOtherUsersIsRemoved(t *testing.T) {
	sqlite.FailModes(t, errPlanted)
	folder := backupFolder(t)
	plantBackups(t, folder, "0.19.0-20200101T031500Z-update.sqlite")

	taken, err := sqlite.TakeBackup(t.Context(), migrated(t),
		sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Update, Kept: 1})

	if !errors.Is(err, errPlanted) || taken.Path != "" {
		t.Errorf("TakeBackup() with a mode it could not set = %+v, %v, want no path and the planted failure", taken, err)
	}
	want := []string{snapshotLock, "0.19.0-20200101T031500Z-update.sqlite"}
	if got := folderHolds(t, folder); !slices.Equal(got, want) {
		t.Errorf("the folder holds %q, want %q, the copy removed and nothing pruned", got, want)
	}
}

func TestABackupKeepsACopyWhoseCheckpointFailed(t *testing.T) {
	t.Parallel()

	db, faults := faultyHandle(t)
	faults.FailStatement("PRAGMA wal_checkpoint(TRUNCATE)", errPlanted)
	folder := backupFolder(t)
	plantBackups(t, folder, "0.19.0-20200101T031500Z-update.sqlite")

	taken := takeBackup(t, db, sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Update, Kept: 1})

	if !errors.Is(taken.AfterCopy, errPlanted) {
		t.Errorf("TakeBackup().AfterCopy = %v, want the planted checkpoint failure", taken.AfterCopy)
	}
	want := []string{snapshotLock, "0.19.0-20200101T031500Z-update.sqlite", filepath.Base(taken.Path)}
	if got := folderHolds(t, folder); !slices.Equal(got, want) {
		t.Errorf("the folder holds %q, want %q, the copy kept and nothing pruned after a copy that did not land cleanly",
			got, want)
	}
}

func TestABackupIntoABusyFolderAnswersErrSnapshotRunning(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)
	plantBackups(t, folder)
	lock, err := os.OpenFile(filepath.Join(folder, snapshotLock), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("opening the snapshot lock: %v", err)
	}
	t.Cleanup(func() { _ = lock.Close() })
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatalf("holding the snapshot lock: %v", err)
	}

	_, err = sqlite.TakeBackup(t.Context(), migrated(t), sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})

	if !errors.Is(err, dbsqlite.ErrSnapshotRunning) {
		t.Errorf("TakeBackup() into a busy folder error = %v, want ErrSnapshotRunning", err)
	}
}

func TestAnUpdateOrScheduledBackupPrunesBothKindsToOneCount(t *testing.T) {
	t.Parallel()

	for _, kind := range []backupfiles.Kind{backupfiles.Update, backupfiles.Scheduled} {
		folder := backupFolder(t)
		plantBackups(t, folder,
			"0.19.0-20200101T031500Z-update.sqlite", "0.19.0-20200102T031500Z-scheduled.sqlite",
			"0.19.0-20200103T031500Z-scheduled.sqlite", "0.19.0-20200104T101500Z-manual.sqlite")

		taken := takeBackup(t, migrated(t), sqlite.BackupRequest{Folder: folder, Kind: kind, Kept: 2})

		want := []string{
			snapshotLock, "0.19.0-20200103T031500Z-scheduled.sqlite", "0.19.0-20200104T101500Z-manual.sqlite",
			filepath.Base(taken.Path),
		}
		if got := folderHolds(t, folder); taken.Prune != nil || !slices.Equal(got, want) {
			t.Errorf("a %s backup left %q with the prune error %v, want %q", kind, got, taken.Prune, want)
		}
	}
}

func TestAManualBackupPrunesNothing(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)
	planted := []string{"0.19.0-20200101T031500Z-update.sqlite", "0.19.0-20200102T031500Z-scheduled.sqlite"}
	plantBackups(t, folder, planted...)

	taken := takeBackup(t, migrated(t), sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual, Kept: 1})

	want := append(append([]string{snapshotLock}, planted...), filepath.Base(taken.Path))
	if got := folderHolds(t, folder); !slices.Equal(got, want) {
		t.Errorf("a manual backup left %q, want %q", got, want)
	}
}

func TestABackupPrunesOnlyAfterTheCopyLands(t *testing.T) {
	t.Parallel()

	db, faults := faultyHandle(t)
	faults.FailStatement("VACUUM INTO ?", errPlanted)
	folder := backupFolder(t)
	planted := []string{"0.19.0-20200101T031500Z-update.sqlite", "0.19.0-20200102T031500Z-scheduled.sqlite"}
	plantBackups(t, folder, planted...)

	request := sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Update, Kept: 1}
	taken, err := sqlite.TakeBackup(t.Context(), db, request)

	if !errors.Is(err, errPlanted) || taken.Path != "" {
		t.Fatalf("TakeBackup() with a failed copy = %+v, %v, want no path and the planted failure", taken, err)
	}
	if got := folderHolds(t, folder); !slices.Equal(got, append([]string{snapshotLock}, planted...)) {
		t.Errorf("a failed copy left %q, want every older backup kept", got)
	}
}
