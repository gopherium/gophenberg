// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
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

// linkTo returns a link named backups in a fresh folder of t that names folder.
func linkTo(t *testing.T, folder string) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), "backups")
	if err := os.Symlink(folder, link); err != nil {
		t.Fatalf("linking the backups folder to another folder: %v", err)
	}
	return link
}

// wantLeftOpen fails t unless folder keeps the mode 755 and holds nothing, as a backup that changed nothing leaves it.
func wantLeftOpen(t *testing.T, folder string) {
	t.Helper()
	if got := mode(t, folder); got != 0o755 {
		t.Errorf("the open folder has the mode %o, want the 755 it had", got)
	}
	if got := folderHolds(t, folder); len(got) != 0 {
		t.Errorf("the open folder holds %q, want no copy started", got)
	}
}

// manualBackup takes a manual backup of a fresh migrated file into folder.
func manualBackup(t *testing.T, folder string) (sqlite.TakenBackup, error) {
	t.Helper()
	return sqlite.TakeBackup(t.Context(), migrated(t), sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Manual})
}

// wantFolderNamed fails t unless err is the planted failure to keep folder from other users.
func wantFolderNamed(t *testing.T, err error, folder string) {
	t.Helper()
	want := "sqlite: keep the backups folder " + folder + " from other users: "
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("TakeBackup() error = %v, want one under %q", err, want)
	}
}

func TestABackupClosesTheFolderALinkNames(t *testing.T) {
	sqlite.RunAsUser(t)
	elsewhere := openFolder(t)
	link := linkTo(t, elsewhere)

	taken := takeBackup(t, migrated(t), sqlite.BackupRequest{Folder: link, Kind: backupfiles.Manual})

	if got := mode(t, elsewhere); got != 0o700 {
		t.Errorf("the folder the link names has the mode %o, want 700", got)
	}
	if _, err := os.Stat(filepath.Join(elsewhere, filepath.Base(taken.Path))); err != nil {
		t.Errorf("the backup in the folder the link names: %v, want it there", err)
	}
}

func TestABackupAsRootStopsAtAnOpenFolderAndLeavesItAsItIs(t *testing.T) {
	sqlite.RunAsRoot(t)
	folder := openFolder(t)

	_, err := manualBackup(t, folder)

	wantFolderNamed(t, err, folder)
	if err == nil || !strings.Contains(err.Error(), "a run as root") {
		t.Errorf("TakeBackup() as root error = %v, want one that says a run as root left the folder", err)
	}
	wantLeftOpen(t, folder)
}

func TestABackupAsRootLeavesTheFolderALinkNamesAsItIs(t *testing.T) {
	sqlite.RunAsRoot(t)
	elsewhere := openFolder(t)
	link := linkTo(t, elsewhere)

	_, err := manualBackup(t, link)

	wantFolderNamed(t, err, link)
	wantLeftOpen(t, elsewhere)
}

func TestABackupLeavesAFolderOnlyItsOwnerOpensAsItIs(t *testing.T) {
	sqlite.FailModes(t, errPlanted)
	folder := backupFolder(t)
	plantBackups(t, folder)

	_, err := manualBackup(t, folder)

	if !errors.Is(err, errPlanted) || !strings.HasPrefix(err.Error(), "sqlite: keep the backup "+folder+"/") {
		t.Errorf("TakeBackup() error = %v, want the mode of the copy failing and the folder left as it is", err)
	}
}

func TestABackupStopsWhenItCannotCloseAnOpenFolder(t *testing.T) {
	sqlite.RunAsUser(t)
	sqlite.FailModes(t, errPlanted)
	folder := openFolder(t)
	plantBackups(t, folder, "0.19.0-20200101T031500Z-update.sqlite")

	taken, err := manualBackup(t, folder)

	wantFolderNamed(t, err, folder)
	if !errors.Is(err, errPlanted) || taken.Path != "" {
		t.Errorf("TakeBackup() = %+v, %v, want no path and the planted failure", taken, err)
	}
	if got := folderHolds(t, folder); !slices.Equal(got, []string{"0.19.0-20200101T031500Z-update.sqlite"}) {
		t.Errorf("the folder holds %q, want no copy started", got)
	}
}

func TestABackupNamesALinkToAFolderThatIsGone(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)
	if err := os.Symlink(filepath.Join(t.TempDir(), "gone"), folder); err != nil {
		t.Fatalf("linking the backups folder to a folder that is gone: %v", err)
	}

	_, err := manualBackup(t, folder)

	wantFolderNamed(t, err, folder)
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
