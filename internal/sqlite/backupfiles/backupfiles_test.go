// SPDX-License-Identifier: Apache-2.0

package backupfiles_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
)

// takenAt is the time the backups of these tests were taken at, in UTC.
var takenAt = time.Date(2026, time.November, 10, 9, 15, 0, 0, time.UTC)

// plant writes an empty file called name into folder.
func plant(t *testing.T, folder, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(folder, name), nil, 0o600); err != nil {
		t.Fatalf("planting %s: %v", name, err)
	}
}

// held returns the name of every entry folder holds, in name order.
func held(t *testing.T, folder string) []string {
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

// named returns the backup the file name tells, failing t when it tells none.
func named(t *testing.T, name string) backupfiles.Backup {
	t.Helper()
	backup, ok := backupfiles.Read(name)
	if !ok {
		t.Fatalf("Read(%q) tells no backup", name)
	}
	return backup
}

// names returns the name of each backup listed.
func names(listed []backupfiles.Backup) []string {
	out := make([]string, 0, len(listed))
	for _, backup := range listed {
		out = append(out, backup.Name())
	}
	return out
}

func TestABackupNameCarriesItsVersionTimeAndKind(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		backup backupfiles.Backup
		want   string
	}{
		{backupfiles.Backup{Version: "0.20.0", At: takenAt, Kind: backupfiles.Update},
			"0.20.0-20261110T091500Z-update.sqlite"},
		{backupfiles.Backup{Version: "0.21.0", At: takenAt, Kind: backupfiles.Scheduled},
			"0.21.0-20261110T091500Z-scheduled.sqlite"},
		{backupfiles.Backup{Version: "0.21.0", At: takenAt, Kind: backupfiles.Manual},
			"0.21.0-20261110T091500Z-manual.sqlite"},
		{backupfiles.Backup{Version: backupfiles.UnknownVersion, At: takenAt, Kind: backupfiles.Update},
			"unknown-20261110T091500Z-update.sqlite"},
		{backupfiles.Backup{Version: "0.20.0", At: takenAt.In(time.FixedZone("UTC+2", 7200)), Kind: backupfiles.Manual},
			"0.20.0-20261110T091500Z-manual.sqlite"},
		{backupfiles.Backup{Version: "0.21.0", At: takenAt, Kind: backupfiles.Scheduled, Fingerprint: "9f2c4e1a7b3d5c60"},
			"0.21.0-20261110T091500Z-scheduled-9f2c4e1a7b3d5c60.sqlite"},
	} {
		if got := tc.backup.Name(); got != tc.want {
			t.Errorf("%+v.Name() = %q, want %q", tc.backup, got, tc.want)
		}
	}
}

func TestABackupNameIsReadFromTheRight(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]backupfiles.Backup{
		"0.20.0-20261110T091500Z-update.sqlite":         {Version: "0.20.0", At: takenAt, Kind: backupfiles.Update},
		"0.21.0-rc.1-20261110T091500Z-scheduled.sqlite": {Version: "0.21.0-rc.1", At: takenAt, Kind: backupfiles.Scheduled},
		"0.21.0-x-y-20261110T091500Z-manual.sqlite":     {Version: "0.21.0-x-y", At: takenAt, Kind: backupfiles.Manual},
		"unknown-20261110T091500Z-update.sqlite": {
			Version: backupfiles.UnknownVersion, At: takenAt, Kind: backupfiles.Update,
		},
		"0.21.0-rc.1-20261110T091500Z-scheduled-9f2c4e1a7b3d5c60.sqlite": {
			Version: "0.21.0-rc.1", At: takenAt, Kind: backupfiles.Scheduled, Fingerprint: "9f2c4e1a7b3d5c60",
		},
	} {
		got, ok := backupfiles.Read(name)
		if !ok || got != want {
			t.Errorf("Read(%q) = %+v, %v, want %+v, true", name, got, ok, want)
		}
		if got.Name() != name {
			t.Errorf("Read(%q).Name() = %q, want the name it was read from", name, got.Name())
		}
	}
}

func TestANameThatIsNoBackupIsNotRead(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"0.20.0-20261110T091500Z.dump",
		"0.20.0-20261110T091500Z-update.sqlite.dbkit-snapshot.partial",
		".dbkit-snapshot.lock",
		"0.20.0-20261110T091500Z-weekly.sqlite",
		"0.20.0-20261110-update.sqlite",
		"20261110T091500Z-update.sqlite",
		"-20261110T091500Z-update.sqlite",
		"0.20-20261110T091500Z-update.sqlite",
		"latest-20261110T091500Z-update.sqlite",
		"0.20.0-20261110T091500Z-update-9f2c4e1a7b3d5c60.sqlite",
		"0.20.0-20261110T091500Z-manual-9f2c4e1a7b3d5c60.sqlite",
		"0.20.0-20261110T091500Z-scheduled-9f2c4e1a7b3d5c6.sqlite",
		"0.20.0-20261110T091500Z-scheduled-9F2C4E1A7B3D5C60.sqlite",
		"0.20.0-20261110T091500Z-scheduled-9f2c4e1a7b3d5c6z.sqlite",
		"0.20.0-20261110T091500.5Z-update.sqlite",
		"0.20.0-20261110T091500,5Z-update.sqlite",
		"update.sqlite",
		"site.db",
	} {
		if got, ok := backupfiles.Read(name); ok {
			t.Errorf("Read(%q) = %+v, true, want it left out", name, got)
		}
	}
}

func TestABackupNameKeepsTheFirstSixteenCharactersOfItsFingerprint(t *testing.T) {
	t.Parallel()

	backup := backupfiles.Backup{
		Version: "0.21.0", At: takenAt, Kind: backupfiles.Scheduled,
		Fingerprint: "9f2c4e1a7b3d5c60" + "0123456789abcdef0123456789abcdef0123456789abcdef",
	}

	want := "0.21.0-20261110T091500Z-scheduled-9f2c4e1a7b3d5c60.sqlite"
	if got := backup.Name(); got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
	if read, ok := backupfiles.Read(backup.Name()); !ok || read.Fingerprint != "9f2c4e1a7b3d5c60" {
		t.Errorf("Read(Name()) = %+v, %v, want the first 16 characters of the fingerprint", read, ok)
	}
}

func TestABackupNameNeverCarriesTheCommit(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"0.20.0+3b96de93-20261110T091500Z-update.sqlite",
		"0.20.0-3b96de93+dirty-20261110T091500Z-update.sqlite",
	} {
		if got, ok := backupfiles.Read(name); ok {
			t.Errorf("Read(%q) = %+v, true, want a name with a commit left out", name, got)
		}
	}
}

func TestTheListingReadsOnlyTheBackupsOfAFile(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	for _, name := range []string{
		"0.19.0-20261101T091500Z.dump",
		"0.20.0-20261102T091500Z-update.sqlite",
		"0.20.0-20261104T091500Z-scheduled-9f2c4e1a7b3d5c60.sqlite",
		"0.20.0-20261103T091500Z-manual.sqlite",
		"0.20.0-20261105T091500Z-manual.sqlite.dbkit-snapshot.partial",
		".dbkit-snapshot.lock",
		"notes.txt",
	} {
		plant(t, folder, name)
	}
	if err := os.Mkdir(filepath.Join(folder, "0.20.0-20261106T091500Z-update.sqlite"), 0o700); err != nil {
		t.Fatalf("making a folder named like a backup: %v", err)
	}

	listed, err := backupfiles.Folder(folder).List()

	want := []string{
		"0.20.0-20261104T091500Z-scheduled-9f2c4e1a7b3d5c60.sqlite",
		"0.20.0-20261103T091500Z-manual.sqlite",
		"0.20.0-20261102T091500Z-update.sqlite",
	}
	if err != nil || !slices.Equal(names(listed), want) {
		t.Errorf("List() = %q, %v, want %q, the newest first", names(listed), err, want)
	}
}

func TestTheListingOfTwoBackupsTakenAtOnceIsSteady(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	plant(t, folder, "0.20.0-20261110T091500Z-update.sqlite")
	plant(t, folder, "0.20.0-20261110T091500Z-manual.sqlite")

	listed, err := backupfiles.Folder(folder).List()

	want := []string{"0.20.0-20261110T091500Z-update.sqlite", "0.20.0-20261110T091500Z-manual.sqlite"}
	if err != nil || !slices.Equal(names(listed), want) {
		t.Errorf("List() = %q, %v, want %q", names(listed), err, want)
	}
}

func TestTheListingOfAMissingFolderIsEmpty(t *testing.T) {
	t.Parallel()

	listed, err := backupfiles.Folder(filepath.Join(t.TempDir(), "backups")).List()

	if err != nil || len(listed) != 0 {
		t.Errorf("List() = %+v, %v, want none and no error", listed, err)
	}
}

func TestTheListingNamesAFolderItCannotRead(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "backups")
	plant(t, filepath.Dir(file), "backups")

	_, err := backupfiles.Folder(file).List()

	if err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("List() over a file error = %v, want the read error", err)
	}
}

func TestPruningKeepsTheNewestUpdateAndScheduledBackupsToOneCount(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	for _, name := range []string{
		"0.19.0-20261101T031500Z-update.sqlite",
		"0.19.0-20261102T031500Z-scheduled.sqlite",
		"0.20.0-20261103T031500Z-update.sqlite",
		"0.20.0-20261104T031500Z-scheduled.sqlite",
	} {
		plant(t, folder, name)
	}

	err := backupfiles.Folder(folder).Prune(2, named(t, "0.20.0-20261104T031500Z-scheduled.sqlite"))

	want := []string{"0.20.0-20261103T031500Z-update.sqlite", "0.20.0-20261104T031500Z-scheduled.sqlite"}
	if got := held(t, folder); err != nil || !slices.Equal(got, want) {
		t.Errorf("Prune() left %q with error %v, want %q", got, err, want)
	}
}

func TestPruningLeavesManualBackupsOutOfTheCount(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	for _, name := range []string{
		"0.20.0-20261101T031500Z-scheduled.sqlite",
		"0.20.0-20261102T031500Z-scheduled.sqlite",
		"0.20.0-20261103T031500Z-manual.sqlite",
		"0.20.0-20261104T031500Z-manual.sqlite",
	} {
		plant(t, folder, name)
	}

	err := backupfiles.Folder(folder).Prune(2, named(t, "0.20.0-20261102T031500Z-scheduled.sqlite"))

	want := []string{
		"0.20.0-20261101T031500Z-scheduled.sqlite", "0.20.0-20261102T031500Z-scheduled.sqlite",
		"0.20.0-20261103T031500Z-manual.sqlite", "0.20.0-20261104T031500Z-manual.sqlite",
	}
	if got := held(t, folder); err != nil || !slices.Equal(got, want) {
		t.Errorf("Prune() left %q with error %v, want %q", got, err, want)
	}
}

func TestPruningNeverDeletesAManualBackup(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	for _, name := range []string{
		"0.18.0-20250101T031500Z-manual.sqlite",
		"0.20.0-20261101T031500Z-update.sqlite",
		"0.20.0-20261102T031500Z-update.sqlite",
	} {
		plant(t, folder, name)
	}

	err := backupfiles.Folder(folder).Prune(1, named(t, "0.20.0-20261102T031500Z-update.sqlite"))

	want := []string{"0.18.0-20250101T031500Z-manual.sqlite", "0.20.0-20261102T031500Z-update.sqlite"}
	if got := held(t, folder); err != nil || !slices.Equal(got, want) {
		t.Errorf("Prune() left %q with error %v, want %q", got, err, want)
	}
}

func TestPruningKeepsTheBackupJustTaken(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	for _, name := range []string{
		"0.20.0-20261101T031500Z-update.sqlite",
		"0.20.0-20261105T031500Z-scheduled.sqlite",
		"0.20.0-20261106T031500Z-scheduled.sqlite",
	} {
		plant(t, folder, name)
	}

	err := backupfiles.Folder(folder).Prune(1, named(t, "0.20.0-20261106T031500Z-scheduled.sqlite"))

	want := []string{"0.20.0-20261106T031500Z-scheduled.sqlite"}
	if got := held(t, folder); err != nil || !slices.Equal(got, want) {
		t.Errorf("Prune() left %q with error %v, want %q, the one just taken", got, err, want)
	}
}

func TestPruningNeverDeletesABackupNewerThanTheOneJustTaken(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	for _, name := range []string{
		"0.20.0-20261001T031500Z-update.sqlite",
		"0.20.0-20261101T031500Z-update.sqlite",
		"0.20.0-20261105T031500Z-scheduled.sqlite",
	} {
		plant(t, folder, name)
	}

	err := backupfiles.Folder(folder).Prune(1, named(t, "0.20.0-20261101T031500Z-update.sqlite"))

	want := []string{"0.20.0-20261101T031500Z-update.sqlite", "0.20.0-20261105T031500Z-scheduled.sqlite"}
	if got := held(t, folder); err != nil || !slices.Equal(got, want) {
		t.Errorf("Prune() left %q with error %v, want %q, a newer backup another run took kept", got, err, want)
	}
}

func TestPruningLeavesEveryFileThatIsNoBackup(t *testing.T) {
	t.Parallel()

	folder := t.TempDir()
	for _, name := range []string{
		"0.19.0-20261101T091500Z.dump",
		"0.20.0-20261101T031500Z-update.sqlite",
		"0.20.0-20261102T031500Z-update.sqlite",
		"0.20.0-20261103T031500Z-update.sqlite.dbkit-snapshot.partial",
		".dbkit-snapshot.lock",
	} {
		plant(t, folder, name)
	}

	err := backupfiles.Folder(folder).Prune(1, named(t, "0.20.0-20261102T031500Z-update.sqlite"))

	want := []string{
		".dbkit-snapshot.lock", "0.19.0-20261101T091500Z.dump", "0.20.0-20261102T031500Z-update.sqlite",
		"0.20.0-20261103T031500Z-update.sqlite.dbkit-snapshot.partial",
	}
	if got := held(t, folder); err != nil || !slices.Equal(got, want) {
		t.Errorf("Prune() left %q with error %v, want %q", got, err, want)
	}
}

func TestPruningReturnsARemoveThatFailed(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root removes files from a folder it cannot write to")
	}
	folder := t.TempDir()
	plant(t, folder, "0.20.0-20261101T031500Z-update.sqlite")
	plant(t, folder, "0.20.0-20261102T031500Z-update.sqlite")
	if err := os.Chmod(folder, 0o500); err != nil {
		t.Fatalf("making the folder read only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o700) })

	err := backupfiles.Folder(folder).Prune(1, named(t, "0.20.0-20261102T031500Z-update.sqlite"))

	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("Prune() in a read only folder error = %v, want the remove's permission error", err)
	}
}

func TestPruningAMissingFolderDoesNothing(t *testing.T) {
	t.Parallel()

	if err := backupfiles.Folder(filepath.Join(t.TempDir(), "backups")).Prune(1, backupfiles.Backup{}); err != nil {
		t.Errorf("Prune() of a missing folder error = %v, want nil", err)
	}
}

func TestPruningNamesAFolderItCannotRead(t *testing.T) {
	t.Parallel()

	file := filepath.Join(t.TempDir(), "backups")
	plant(t, filepath.Dir(file), "backups")

	if err := backupfiles.Folder(file).Prune(1, backupfiles.Backup{}); err == nil {
		t.Error("Prune() over a file error = nil, want the read error")
	}
}
