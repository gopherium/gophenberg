// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"cmp"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/updater"
)

func TestReachedDatabaseMarksAStartOfTheBuildAnUpdateSwitchedTo(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, switchedTo)

	if err := f.started().ReachedDatabase(); err != nil {
		t.Fatalf("ReachedDatabase() = %v, want the start marked", err)
	}

	want := switchedTo
	want.Attempts, want.DatabaseReached = 1, true
	f.wantState(want)
}

func TestReachedDatabaseLeavesAStateThatNamesNoStartOfThisBuildAlone(t *testing.T) {
	t.Parallel()

	for name, state := range map[string]updater.State{
		"a kept update":             {Step: updater.StepCommitted, From: running, To: offered},
		"a switch to another build": {Step: updater.StepSwitched, From: running, To: "0.22.0"},
		"a job that stopped":        {Step: updater.StepBackup, From: running, To: offered},
		"an install never updated":  {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newGuardFixture(t, state)

			if err := f.started().ReachedDatabase(); err != nil {
				t.Fatalf("ReachedDatabase() = %v, want nil", err)
			}

			f.wantState(state)
		})
	}
}

func TestReachedDatabaseMarksNothingOnAnInstallWithNoRoot(t *testing.T) {
	t.Parallel()

	guard, err := updater.Start(settingsOf(t, map[string]string{}), offered, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatalf("Start() with no install root = %v, want nil", err)
	}

	if err := guard.ReachedDatabase(); err != nil {
		t.Errorf("ReachedDatabase() with no install root = %v, want nil", err)
	}
}

// spoiledState is one way to spoil the state of an install root, with what a command that marks the database says.
type spoiledState struct {
	spoil func(t *testing.T, layout updater.Layout)
	says  string
}

// spoiledStates are the ways to spoil the state of an install root that a command marking the database cannot get past.
var spoiledStates = map[string]spoiledState{
	"a state folder it cannot lock": {
		spoil: func(t *testing.T, layout updater.Layout) {
			t.Helper()
			denied(t, filepath.Dir(layout.StateFile()), 0o300)
		},
		says: "updater: lock the update state",
	},
	"a state file it cannot read": {
		spoil: func(t *testing.T, layout updater.Layout) {
			t.Helper()
			if err := os.WriteFile(layout.StateFile(), []byte("step: switched"), 0o644); err != nil {
				t.Fatalf("writing the state file: %v", err)
			}
		},
		says: "updater: read the update state",
	},
	"a state folder it cannot write": {
		spoil: func(t *testing.T, layout updater.Layout) {
			t.Helper()
			denied(t, filepath.Dir(layout.StateFile()), 0o500)
		},
		says: "updater: write the update state",
	},
}

func TestReachedDatabaseRefusesAStateItCannotLockReadOrWrite(t *testing.T) {
	t.Parallel()

	for name, tc := range spoiledStates {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newGuardFixture(t, switchedTo)
			guard := f.started()
			tc.spoil(t, f.layout)

			err := guard.ReachedDatabase()

			wantRefused(t, err, tc.says)
		})
	}
}

func TestMarkDatabaseReachedMarksTheUpdateThatSwitchedToTheBuildThatRuns(t *testing.T) {
	t.Parallel()

	for name, state := range map[string]updater.State{
		"a switch that waits for its check": switchedTo,
		"an update that failed":             {Step: updater.StepFailed, From: running, To: offered, LongJump: true},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newGuardFixture(t, state)

			if err := updater.MarkDatabaseReached(settingsOf(t, f.env), offered); err != nil {
				t.Fatalf("MarkDatabaseReached() = %v, want the update marked", err)
			}

			want := state
			want.DatabaseReached = true
			f.wantState(want)
		})
	}
}

func TestMarkDatabaseReachedMarksTheStateInTheDataFolder(t *testing.T) {
	t.Parallel()

	failed := updater.State{Step: updater.StepFailed, From: running, To: offered, LongJump: true}
	f := newDataFixture(t, failed)

	if err := updater.MarkDatabaseReached(f.settings, offered); err != nil {
		t.Fatalf("MarkDatabaseReached() = %v, want the update marked", err)
	}

	if state := readState(t, f.layout); !state.DatabaseReached {
		t.Errorf("the state in the data folder is %+v, want the database marked reached", state)
	}
}

func TestMarkDatabaseReachedLeavesAStateThatNamesNoSwitchToTheBuildThatRunsAlone(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		state   updater.State
		running string
	}{
		"an install never updated":  {running: offered},
		"a kept update":             {state: updater.State{Step: updater.StepCommitted, From: running, To: offered}},
		"an update taken back":      {state: takenBack},
		"a job that stopped":        {state: updater.State{Step: updater.StepBackup, From: running, To: offered}},
		"a switch to another build": {state: switchedTo, running: "0.22.0"},
		"the build before a switch": {state: switchedTo, running: running},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newGuardFixture(t, tc.state)

			if err := updater.MarkDatabaseReached(settingsOf(t, f.env), cmp.Or(tc.running, offered)); err != nil {
				t.Fatalf("MarkDatabaseReached() = %v, want nil", err)
			}

			f.wantState(tc.state)
		})
	}
}

func TestMarkDatabaseReachedMarksNothingOnAnInstallWithNoRoot(t *testing.T) {
	folder := t.TempDir()
	writeState(t, updater.Layout{Root: folder}, switchedTo)
	t.Chdir(folder)

	if err := updater.MarkDatabaseReached(settingsOf(t, map[string]string{}), offered); err != nil {
		t.Errorf("MarkDatabaseReached() with no install root = %v, want nil", err)
	}

	if state := readState(t, updater.Layout{Root: folder}); !reflect.DeepEqual(state, switchedTo) {
		t.Errorf("the state file in the working folder holds %+v, want %+v left alone", state, switchedTo)
	}
}

func TestMarkDatabaseReachedRefusesAStateItCannotLockReadOrWrite(t *testing.T) {
	t.Parallel()

	for name, tc := range spoiledStates {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newGuardFixture(t, switchedTo)
			settings := settingsOf(t, f.env)
			tc.spoil(t, f.layout)

			err := updater.MarkDatabaseReached(settings, offered)

			wantRefused(t, err, tc.says)
		})
	}
}

func TestStartGoesBackWhenALongJumpFailsBeforeItsBuildReachedTheDatabase(t *testing.T) {
	t.Parallel()

	tried := switchedTo
	tried.Attempts, tried.LongJump, tried.Backup = 3, true, longJumpBackup
	f := newGuardFixture(t, tried)

	_, err := f.start()

	wantCheckFailed(t, err,
		"updater: Gophenberg 0.21.0 failed its start check after the update from 0.20.0: it tried to start 4 times",
		"updater: current names Gophenberg 0.20.0 again, which the next start runs")
	f.wantCurrent(running)
	want := tried
	want.Attempts, want.Refused = 4, []string{"0.19.5", offered}
	f.wantState(want)
}

func TestServeKeepsTheSiteDownWhenTheBuildOfALongJumpReachedTheDatabaseAndFailsItsCheck(t *testing.T) {
	t.Parallel()

	long := switchedTo
	long.LongJump, long.Backup = true, longJumpBackup
	f := newGuardFixture(t, long)
	guard := f.started()
	if err := guard.ReachedDatabase(); err != nil {
		t.Fatalf("ReachedDatabase() = %v, want the start marked", err)
	}

	err := served(t, guard, newTestSite(map[string]int{"/admin/": http.StatusInternalServerError}), passes)

	wantCheckFailed(t, err,
		"updater: Gophenberg 0.21.0 failed its start check after the update from 0.20.0: /admin/ answered 500, want 200",
		"so 0.20.0 cannot open the database and the site stays down")
	f.wantCurrent(offered)
	want := long
	want.Step, want.Attempts, want.Refused, want.DatabaseReached = updater.StepFailed, 1, []string{"0.19.5", offered}, true
	f.wantState(want)
}
