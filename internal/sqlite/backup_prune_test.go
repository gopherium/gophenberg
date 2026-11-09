// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
)

// olderBackups are the update and scheduled backups these tests plant before a new one is taken.
var olderBackups = []string{"0.19.0-20200101T031500Z-update.sqlite", "0.19.0-20200102T031500Z-scheduled.sqlite"}

// takenIn returns a backup taken into folder under name, its copy in place.
func takenIn(t *testing.T, folder, name string) sqlite.TakenBackup {
	t.Helper()
	plantBackups(t, folder, name)
	return sqlite.TakenBackup{Path: filepath.Join(folder, name)}
}

func TestAnUpdateBackupLeavesTheOlderBackupsUntilItsPrune(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)
	plantBackups(t, folder, olderBackups...)
	request := sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Update, Kept: 1}

	taken := takeBackup(t, migrated(t), request)

	want := append(append([]string{snapshotLock}, olderBackups...), filepath.Base(taken.Path))
	if got := folderHolds(t, folder); taken.Prune != nil || !slices.Equal(got, want) {
		t.Fatalf("an update backup left %q with the prune error %v, want %q", got, taken.Prune, want)
	}
	if err := sqlite.PruneBackups(request, taken); err != nil {
		t.Fatalf("PruneBackups() error = %v, want nil", err)
	}
	want = []string{snapshotLock, filepath.Base(taken.Path)}
	if got := folderHolds(t, folder); !slices.Equal(got, want) {
		t.Errorf("PruneBackups() left %q, want %q, the backup just taken alone in the count", got, want)
	}
}

func TestPruningAfterACopyThatDidNotLandCleanlyDeletesNothing(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)
	plantBackups(t, folder, olderBackups...)
	taken := takenIn(t, folder, "0.20.0-20200103T031500Z-update.sqlite")
	taken.AfterCopy = errPlanted

	err := sqlite.PruneBackups(sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Update, Kept: 1}, taken)

	want := append(slices.Clone(olderBackups), "0.20.0-20200103T031500Z-update.sqlite")
	if got := folderHolds(t, folder); err != nil || !slices.Equal(got, want) {
		t.Errorf("PruneBackups() after a failed checkpoint left %q with error %v, want %q", got, err, want)
	}
}

func TestPruningForAManualBackupDeletesNothing(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)
	plantBackups(t, folder, olderBackups...)
	taken := takenIn(t, folder, "0.20.0-20200103T031500Z-manual.sqlite")

	err := sqlite.PruneBackups(sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual}, taken)

	want := append(slices.Clone(olderBackups), "0.20.0-20200103T031500Z-manual.sqlite")
	if got := folderHolds(t, folder); err != nil || !slices.Equal(got, want) {
		t.Errorf("PruneBackups() for a manual backup left %q with error %v, want %q", got, err, want)
	}
}

func TestPruningNamesAFolderItCannotList(t *testing.T) {
	t.Parallel()

	file := backupFolder(t)
	plantBackups(t, filepath.Dir(file), filepath.Base(file))
	request := sqlite.BackupRequest{Folder: file, Kind: backupfiles.Update, Kept: 1}

	if err := sqlite.PruneBackups(request, sqlite.TakenBackup{Path: filepath.Join(file, olderBackups[0])}); err == nil {
		t.Error("PruneBackups() over a file error = nil, want the listing error")
	}
}
