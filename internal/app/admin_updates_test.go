// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"
	"testing"
	"testing/synctest"

	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/updater"
)

// asked is the start of an update every runner test asks for.
var asked = server.UpdateStart{Version: "99.0.0"}

// idleUpdates returns a runner of the updates administrators start that runs none yet, on a serve run of its own.
func idleUpdates(t *testing.T) *adminUpdates {
	t.Helper()
	serving, restart := context.WithCancelCause(t.Context())
	t.Cleanup(func() { restart(nil) })
	return &adminUpdates{
		job: updater.Job{Store: &heldSettings{values: map[string]string{}}}, env: settingsEnv(testkit.Getenv(nil)),
		serving: serving, restart: restart, logger: testLogger(io.Discard),
	}
}

func TestAdminUpdatesRefuseAStartOnceTheServeRunEnded(t *testing.T) {
	t.Parallel()

	updates := idleUpdates(t)
	if err := updates.ended(nil); err != nil {
		t.Fatalf("ended() = %v, want nil with no restart asked", err)
	}

	err := updates.start(t.Context(), asked)

	if !errors.Is(err, updater.ErrUpdateRuns) {
		t.Errorf("start() once the serve run ended = %v, want %v", err, updater.ErrUpdateRuns)
	}
}

func TestAdminUpdatesReportASettingStoreTheyCannotReadAsItIs(t *testing.T) {
	t.Parallel()

	updates := idleUpdates(t)
	updates.job.Store = &unreadSettings{}

	for range 2 {
		err := updates.start(t.Context(), asked)

		if !errors.Is(err, errSettings) || errors.Is(err, server.ErrUpdateRefused) {
			t.Errorf("start() over a store it cannot read = %v, want %v and no start the checks refused", err,
				errSettings)
		}
	}
}

func TestAdminUpdatesEndTheServeRunOnlyOnceTheUpdateUnderWayEnded(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		updates := idleUpdates(t)
		if !updates.take() {
			t.Fatal("take() = false on a serve run with no update, want the update under way")
		}
		ended := make(chan error, 1)
		go func() { ended <- updates.ended(nil) }()
		synctest.Wait()

		select {
		case err := <-ended:
			t.Fatalf("ended() = %v while an update was under way, want it to wait for the update", err)
		default:
		}
		updates.free()
		if err := <-ended; err != nil {
			t.Errorf("ended() = %v once the update ended, want nil with no restart asked", err)
		}
	})
}

func TestAdminUpdatesSayAnUpdateRunsFromItsStartUntilItEnds(t *testing.T) {
	t.Parallel()

	var none *adminUpdates
	if none.updating() {
		t.Error("updating() with no runner = true, want false")
	}
	updates := idleUpdates(t)
	if updates.updating() {
		t.Error("updating() before any start = true, want false")
	}
	if !updates.take() {
		t.Fatal("take() = false on a serve run with no update, want the update under way")
	}
	if !updates.updating() {
		t.Error("updating() while an update runs = false, want true")
	}
	updates.free()
	if updates.updating() {
		t.Error("updating() once the update ended = true, want false")
	}
}

func TestAdminUpdatesRestartOnlyOnceTheSwitchStands(t *testing.T) {
	t.Parallel()

	stands := updater.SwitchedError{Release: "99.0.0", Err: errSettings}
	for name, tc := range map[string]struct {
		state   updater.State
		err     error
		restart string
	}{
		"a switch that ended":                       {state: updater.State{To: "99.0.0"}, restart: "99.0.0"},
		"a switch that stands though a step failed": {err: stands, restart: "99.0.0"},
		"a failure before the switch":               {err: errSettings},
	} {
		updates := idleUpdates(t)

		updates.settle(tc.state, tc.err)

		var restart restartError
		asked := errors.As(context.Cause(updates.serving), &restart)
		if asked != (tc.restart != "") || restart.release != tc.restart {
			t.Errorf("settle() after %s asked the restart %q (%t), want %q", name, restart.release, asked, tc.restart)
		}
	}
}

func TestAdminUpdatesEndTheServeRunWithTheRestartAnUpdateAsked(t *testing.T) {
	t.Parallel()

	updates := idleUpdates(t)
	served := errors.New("the server stopped")
	updates.restart(restartError{release: "99.0.0"})

	err := updates.ended(served)

	if !errors.Is(err, served) || !errors.As(err, &restartError{}) || exitCodeOf(err) != exitRestart {
		t.Errorf("ended() after a restart was asked = %v, want %v beside the restart and exit %d", err, served,
			exitRestart)
	}
}

func TestRefusedStartMarksWhatTheChecksRefusedAndLeavesTheRest(t *testing.T) {
	t.Parallel()

	checked := errors.Join(errors.New("updater: the update to Gophenberg 99.0.0 waits for its first start"),
		updater.ErrRefused)
	runs := errors.Join(errors.New("take it"), updater.ErrUpdateRuns)
	failed := errors.New("updater: read the database server: failed to connect to host=db.example.com user=site")

	if err := refusedStart(checked); !errors.Is(err, server.ErrUpdateRefused) || err.Error() != checked.Error() {
		t.Errorf("refusedStart(%v) = %v, want it marked as refused with the same reason", checked, err)
	}
	if held, _ := content.DetailsOf(refusedStart(checked)); held["reason"] != checked.Error() {
		t.Errorf("refusedStart(%v) holds %v, want the reason the admin is shown", checked, held)
	}
	for _, unmarked := range []error{runs, failed} {
		if err := refusedStart(unmarked); err != unmarked {
			t.Errorf("refusedStart(%v) = %v, want it as it is", unmarked, err)
		}
	}
	if err := refusedStart(nil); err != nil {
		t.Errorf("refusedStart(nil) = %v, want nil", err)
	}
}
