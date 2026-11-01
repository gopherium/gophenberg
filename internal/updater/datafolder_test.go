// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"log/slog"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/updater"
)

// dataFixture is an install root and a data folder outside it, the settings naming both.
type dataFixture struct {
	layout   updater.Layout
	settings updater.Settings
}

// newDataFixture returns an install root that updated to the offered build, its state in a data folder.
func newDataFixture(t *testing.T, state updater.State) dataFixture {
	t.Helper()
	layout := updater.Layout{Root: t.TempDir(), Data: t.TempDir()}
	placeBuildIn(t, layout, running)
	placeBuildIn(t, layout, offered)
	pointLink(t, layout, updater.Previous, running)
	pointLink(t, layout, updater.Current, offered)
	writeState(t, layout, state)
	return dataFixture{layout: layout, settings: settingsOf(t, map[string]string{
		"GOPHENBERG_INSTALL_ROOT": layout.Root, "GOPHENBERG_DATA_DIR": layout.Data,
	})}
}

func TestSettingsFromReadsTheDataFolderInAnAbsoluteForm(t *testing.T) {
	t.Parallel()

	settings := settingsOf(t, map[string]string{
		"GOPHENBERG_INSTALL_ROOT": "/opt/gophenberg", "GOPHENBERG_DATA_DIR": "/srv/gophenberg-data/",
	})

	want := updater.Layout{Root: "/opt/gophenberg", Data: "/srv/gophenberg-data"}
	if got := settings.Layout(); got != want {
		t.Errorf("Layout() = %+v, want %+v", got, want)
	}
}

func TestSettingsFromRefusesADataFolderThatIsNotAbsolute(t *testing.T) {
	t.Parallel()

	_, err := updater.SettingsFrom(envOf(map[string]string{"GOPHENBERG_DATA_DIR": "gophenberg-data"}))

	wantRefused(t, err, `GOPHENBERG_DATA_DIR: must be an absolute path, got "gophenberg-data"`)
}

func TestStartCountsTheStartInTheDataFolder(t *testing.T) {
	t.Parallel()

	f := newDataFixture(t, updater.State{Step: updater.StepSwitched, From: running, To: offered})

	_, err := updater.Start(f.settings, offered, slog.New(slog.DiscardHandler))

	if err != nil {
		t.Fatalf("Start() = %v, want the start counted", err)
	}
	if state := readState(t, f.layout); state.Attempts != 1 {
		t.Errorf("the state in the data folder counts %d attempts, want 1", state.Attempts)
	}
}

func TestReadStatusListsTheBackupsOfTheDataFolder(t *testing.T) {
	t.Parallel()

	f := newDataFixture(t, updater.State{})
	keepFile(t, updater.Layout{Root: f.layout.Data}, "backups/0.20.0-20261020T120000Z.dump", "a dump")

	status, err := updater.ReadStatus(f.settings)

	if err != nil {
		t.Fatalf("ReadStatus() = %v, want nil", err)
	}
	want := []string{filepath.Join("backups", "0.20.0-20261020T120000Z.dump")}
	if status.Data != f.layout.Data || !reflect.DeepEqual(status.Backups, want) {
		t.Errorf("ReadStatus() = %+v, want the data folder and its backups %q", status, want)
	}
}

func TestReadStatusNamesTheInstallRootAsTheFolderThatKeepsTheBackupsWithNoDataFolder(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	status, err := updater.ReadStatus(settingsOf(t, map[string]string{"GOPHENBERG_INSTALL_ROOT": root}))

	if err != nil || status.Data != root {
		t.Errorf("ReadStatus() = %+v, %v, want the install root as the folder that keeps the backups", status, err)
	}
}

func TestReadStatusNamesTheBackupToRestoreInTheDataFolder(t *testing.T) {
	t.Parallel()

	f := newDataFixture(t, longJumped(updater.StepFailed))

	status, err := updater.ReadStatus(f.settings)

	if err != nil || !strings.Contains(status.Restore, filepath.Join(f.layout.Data, longJumpBackup)) {
		t.Errorf("ReadStatus() = %+v, %v, want the restore steps naming the data folder", status, err)
	}
}

func TestReadOverviewCountsTheBackupsOfTheDataFolder(t *testing.T) {
	t.Parallel()

	f := newDataFixture(t, updater.State{})
	keepFile(t, updater.Layout{Root: f.layout.Data}, "backups/0.20.0-20261020T120000Z.dump", "a dump")

	overview, err := updater.ReadOverview(f.settings)

	if err != nil || overview.Backups.Kept != 1 {
		t.Errorf("ReadOverview() = %+v, %v, want the backup of the data folder counted", overview, err)
	}
}
