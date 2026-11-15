// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
)

// openFolder makes a backups folder that every user may open and returns its path.
func openFolder(t *testing.T) string {
	t.Helper()
	folder := backupFolder(t)
	if err := errors.Join(os.Mkdir(folder, 0o755), os.Chmod(folder, 0o755)); err != nil {
		t.Fatalf("making an open backups folder: %v", err)
	}
	return folder
}

// readableMigrated returns a handle on a fresh migrated file every user may read.
func readableMigrated(t *testing.T) *sql.DB {
	t.Helper()
	db := migrated(t)
	if err := os.Chmod(liveFile(t, db), 0o644); err != nil {
		t.Fatalf("opening the live file to every reader: %v", err)
	}
	return db
}

// linkTo returns a link named backups in a fresh folder of t that names folder.
func linkTo(t *testing.T, folder string) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), "backups")
	if err := os.Symlink(folder, link); err != nil {
		t.Fatalf("linking the backups folder to another folder: %v", err)
	}
	return link
}

// wantLeftOpen fails t unless folder keeps the mode 755 it had.
func wantLeftOpen(t *testing.T, folder string) {
	t.Helper()
	if got := mode(t, folder); got != 0o755 {
		t.Errorf("the open folder has the mode %o, want the 755 it had", got)
	}
}

// manualBackup takes a manual backup of a fresh migrated file into folder.
func manualBackup(t *testing.T, folder string) (sqlite.TakenBackup, error) {
	t.Helper()
	return sqlite.TakeBackup(t.Context(), migrated(t), sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})
}

func TestABackupAsRootLandsInAnOpenFolderAndLeavesItAsItIs(t *testing.T) {
	sqlite.RunAsRoot(t)
	folder := openFolder(t)

	taken := takeBackup(t, readableMigrated(t), sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})

	if got := mode(t, taken.Path); got != 0o600 {
		t.Errorf("the backup taken as root of a file every user reads has the mode %o, want 600", got)
	}
	wantLeftOpen(t, folder)
}

func TestABackupAsRootLeavesTheFolderALinkNamesAsItIs(t *testing.T) {
	sqlite.RunAsRoot(t)
	elsewhere := openFolder(t)
	link := linkTo(t, elsewhere)

	taken := takeBackup(t, migrated(t), sqlite.BackupRequest{Folder: link, Kind: backupfiles.Manual})

	if _, err := os.Stat(filepath.Join(elsewhere, filepath.Base(taken.Path))); err != nil {
		t.Errorf("the backup in the folder the link names: %v, want it there", err)
	}
	wantLeftOpen(t, elsewhere)
}

func TestABackupNamesALinkToAFolderThatIsGone(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), folder); err != nil {
		t.Fatalf("linking the backups folder to a folder that is gone: %v", err)
	}

	_, err := manualBackup(t, folder)

	if want := "sqlite: take the backup " + folder + "/"; err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("TakeBackup() through a link to a folder that is gone error = %v, want one under %q", err, want)
	}
}

func TestABackupLeavesAFileWhereItsFolderGoesAsItIs(t *testing.T) {
	t.Parallel()

	file := backupFolder(t)
	if err := errors.Join(os.WriteFile(file, nil, 0o644), os.Chmod(file, 0o644)); err != nil {
		t.Fatalf("writing a file where the folder goes: %v", err)
	}

	if _, err := manualBackup(t, file); err == nil {
		t.Error("TakeBackup() into a file error = nil, want one")
	}
	if got := mode(t, file); got != 0o644 {
		t.Errorf("the file where the folder goes has the mode %o, want the 644 it had", got)
	}
}
