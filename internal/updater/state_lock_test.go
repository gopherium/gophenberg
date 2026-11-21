// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/updater"
)

// takenBackMark is what a guard logs once another command took back the update while its start check ran.
const takenBackMark = "another command took back the update while its start check ran"

// lockWait is how long a test holds the state lock before it lets go, to see that a command waits for it.
const lockWait = 200 * time.Millisecond

// heldState takes the lock on the state folder of layout as each command does, returning what lets go of it.
func heldState(t *testing.T, layout updater.Layout) func() {
	t.Helper()
	folder, err := os.Open(filepath.Dir(layout.StateFile()))
	if err == nil {
		err = syscall.Flock(int(folder.Fd()), syscall.LOCK_EX)
	}
	if err != nil {
		t.Fatalf("locking the state folder: %v", err)
	}
	return func() { _ = folder.Close() }
}

// waitsForTheLock runs command while t holds the state lock, then writes newer and lets go, failing t on an early end.
func waitsForTheLock(t *testing.T, layout updater.Layout, newer updater.State, command func()) {
	t.Helper()
	read := make(chan struct{})
	close(read)
	waitsForTheLockOnceRead(t, layout, newer, read, command)
}

// waitsForTheLockOnceRead runs command while t holds the state lock, writes newer once read is closed and lets go.
func waitsForTheLockOnceRead(t *testing.T, layout updater.Layout, newer updater.State, read <-chan struct{},
	command func(),
) {
	t.Helper()
	release := heldState(t, layout)
	ended := make(chan struct{})
	go func() {
		defer close(ended)
		command()
	}()
	select {
	case <-ended:
		t.Error("the command ended before it read the state, want it to read the state and wait for the lock")
		release()
		return
	case <-read:
	}
	select {
	case <-ended:
		t.Error("the command ended while another command held the state lock, want it to wait for the lock")
	case <-time.After(lockWait):
	}
	writeState(t, layout, newer)
	release()
	<-ended
}

// settledServing serves guard with the check inside until it logs what became of the update, then stops it.
func settledServing(t *testing.T, f *guardFixture, guard *updater.Guard, inside func(context.Context) error) error {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	site := newTestSite(nil)
	ended := make(chan error, 1)
	go func() { ended <- guard.Serve(ctx, site.srv, inside, site.serveAt("127.0.0.1:0")) }()
	select {
	case <-f.logs.kept:
	case err := <-ended:
		t.Fatalf("Serve() = %v before it settled the update, want it serving", err)
	case <-time.After(30 * time.Second):
		t.Fatal("Serve() never settled the update")
	}
	cancel()
	return <-ended
}

// wantLeftTakenBack fails t unless the guard of f left the update as update:rollback took it back, saying so.
func (f *guardFixture) wantLeftTakenBack() {
	f.t.Helper()
	f.wantCurrent(running)
	f.wantState(takenBack)
	logs := f.logs.String()
	if !strings.Contains(logs, takenBackMark) || strings.Contains(logs, "kept the update") {
		f.t.Errorf("the guard logged %q, want it to say the update was taken back while its check ran", logs)
	}
}

func TestServeLeavesAnUpdateTakenBackWhileItsCheckRan(t *testing.T) {
	t.Parallel()

	errTheme := errors.New("themehost: the site chose the theme aurora, which does not serve the public site")
	for name, check := range map[string]struct {
		checked error
		logged  string
	}{
		"a check that passes": {logged: "step=rolled-back\n"},
		"a check that fails":  {checked: errTheme, logged: "step=rolled-back reason=\"" + errTheme.Error() + "\"\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newGuardFixture(t, switchedTo)
			settings := settingsOf(t, f.env)
			guard := f.started()
			rolledBack := make(chan error, 1)

			err := settledServing(t, f, guard, func(context.Context) error {
				_, err := updater.Rollback(settings)
				rolledBack <- err
				return check.checked
			})

			if err != nil {
				t.Errorf("Serve() = %v, want a clean stop once another command settled the update", err)
			}
			if err := <-rolledBack; err != nil {
				t.Errorf("Rollback() during the check = %v, want the update taken back", err)
			}
			f.wantLeftTakenBack()
			if logs := f.logs.String(); !strings.Contains(logs, check.logged) {
				t.Errorf("the guard logged %q, want it to end its line with %q", logs, check.logged)
			}
		})
	}
}

func TestServeWaitsForTheStateLockBeforeItSettlesTheUpdate(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, switchedTo)
	guard := f.started()
	release := heldState(t, f.layout)
	ended := make(chan error, 1)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	site := newTestSite(nil)
	go func() { ended <- guard.Serve(ctx, site.srv, passes, site.serveAt("127.0.0.1:0")) }()

	<-time.After(lockWait)
	pointLink(t, f.layout, updater.Current, running)
	writeState(t, f.layout, takenBack)
	release()

	select {
	case <-f.logs.kept:
	case <-time.After(30 * time.Second):
		t.Fatal("Serve() never settled the update")
	}
	cancel()
	if err := <-ended; err != nil {
		t.Errorf("Serve() = %v, want a clean stop once another command settled the update", err)
	}
	f.wantLeftTakenBack()
}

func TestServeStopsWhenItCannotReadTheStateOnceItsCheckEnds(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, switchedTo)
	guard := f.started()
	path := f.layout.StateFile()

	err := served(t, guard, newTestSite(nil), func(context.Context) error {
		return os.WriteFile(path, []byte("step: switched"), 0o644)
	})

	wantRefused(t, err, "updater: read the update state "+path)
	if errors.As(err, &updater.CheckError{}) {
		t.Errorf("Serve() = %v, want a state it cannot read told from a failed start check", err)
	}
	f.wantCurrent(offered)
}

func TestStartWaitsForTheStateLockAndChecksWhatTheStateThenNames(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, switchedTo)
	settings := settingsOf(t, f.env)
	var guard *updater.Guard
	var err error

	waitsForTheLock(t, f.layout, takenBack, func() {
		guard, err = updater.Start(settings, offered, slog.New(slog.DiscardHandler))
	})

	if err != nil {
		t.Fatalf("Start() = %v, want the start read", err)
	}
	site := newTestSite(nil)
	if err := guard.Serve(t.Context(), site.srv, passes, func(context.Context) error { return nil }); err != nil ||
		site.srv.BaseContext != nil {
		t.Errorf("Serve() = %v, want the serve run with no check of an update taken back", err)
	}
	f.wantState(takenBack)
}

// databaseMarks return, for each command that marks the database reached, the mark it writes on the install root of f.
var databaseMarks = map[string]func(f *guardFixture) func() error{
	"a start the guard checks": func(f *guardFixture) func() error {
		return f.started().ReachedDatabase
	},
	"a command that migrates": func(f *guardFixture) func() error {
		settings := settingsOf(f.t, f.env)
		return func() error { return updater.MarkDatabaseReached(settings, offered) }
	},
}

func TestTheDatabaseMarkWaitsForTheStateLockAndMarksWhatTheStateThenNames(t *testing.T) {
	t.Parallel()

	for name, mark := range databaseMarks {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			counted := switchedTo
			counted.Attempts = 2
			f := newGuardFixture(t, switchedTo)
			marks := mark(f)
			var err error

			waitsForTheLock(t, f.layout, counted, func() { err = marks() })

			if err != nil {
				t.Fatalf("the mark = %v, want nil", err)
			}
			marked := counted
			marked.DatabaseReached = true
			f.wantState(marked)
		})
	}
}

func TestReachedDatabaseRefusesAnUpdateTakenBackWhileItsStartWaitedForTheDatabase(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, switchedTo)
	guard := f.started()
	var err error

	waitsForTheLock(t, f.layout, takenBack, func() { err = guard.ReachedDatabase() })

	wantRefused(t, err, "updater: another command took back the update to Gophenberg 0.21.0 while this start "+
		"waited for the database, so it stops before it migrates")
	f.wantState(takenBack)
}

func TestRollbackWaitsForTheStateLockAndTakesBackWhatTheStateThenNames(t *testing.T) {
	t.Parallel()

	switched := switchedTo
	switched.Attempts = 1
	f := newGuardFixture(t, switched)
	settings := settingsOf(t, f.env)
	kept := switched
	kept.Step = updater.StepCommitted
	var state updater.State
	var err error

	waitsForTheLock(t, f.layout, kept, func() { state, err = updater.Rollback(settings) })

	f.wantTakenBack(state, err)
}

func TestApplyWaitsForTheStateLockAndStopsOnceAnotherCommandWroteTheState(t *testing.T) {
	f := newJobFixture(t)
	writeState(t, f.layout, committed)
	read := make(chan struct{})
	var once sync.Once
	f.site.onServe(func(string) { once.Do(func() { close(read) }) })
	job := f.job()
	var err error

	t.Parallel()

	waitsForTheLockOnceRead(t, f.layout, committedTakenBack, read, func() { _, err = job.Apply(t.Context()) })

	wantRefused(t, err, stateChanged)
	f.wantPutBack(committedTakenBack)
}

func TestRollbackRefusesAStateFolderItCannotLock(t *testing.T) {
	t.Parallel()

	f := newGuardFixture(t, switchedTo)
	folder := filepath.Dir(f.layout.StateFile())
	denied(t, folder, 0o300)

	_, err := updater.Rollback(settingsOf(t, f.env))

	wantRefused(t, err, "updater: lock the update state: open "+folder+": permission denied")
	f.wantCurrent(offered)
}
