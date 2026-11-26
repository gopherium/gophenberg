// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/updater"
)

// adminUpdates runs the updates administrators start from the admin, one at a time, on the context of the serve run.
type adminUpdates struct {
	job     updater.Job
	env     gonsole.Env
	serving context.Context
	restart context.CancelCauseFunc
	logger  *slog.Logger
	mu      sync.Mutex
	busy    bool
	stopped bool
	running sync.WaitGroup
}

// start runs the checks of the update asked for under ctx, then the rest of it on its own, refusing what they refuse.
func (a *adminUpdates) start(ctx context.Context, asked server.UpdateStart) error {
	if !a.take() {
		return updater.ErrUpdateRuns
	}
	planned, err := a.plan(ctx, asked)
	if err != nil {
		a.free()
		return err
	}
	go a.finish(planned)
	return nil
}

// take marks an update under way, reporting false while one runs or once the serve run ended.
func (a *adminUpdates) take() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.busy || a.stopped {
		return false
	}
	a.busy = true
	a.running.Add(1)
	return true
}

// updating reports whether an update runs now, from its start until the server stops for its switch, false on none.
func (a *adminUpdates) updating() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.busy
}

// free marks that no update runs, so the end of the serve run stops waiting for it.
func (a *adminUpdates) free() {
	a.mu.Lock()
	a.busy = false
	a.mu.Unlock()
	a.running.Done()
}

// plan returns the update asked for once every check that needs no download passed, marking what a check refused.
func (a *adminUpdates) plan(ctx context.Context, asked server.UpdateStart) (*updater.Planned, error) {
	job := a.job
	job.To, job.Retry = asked.Version, asked.Retry
	job, err := withTheme(ctx, a.env, job)
	if err != nil {
		return nil, err
	}
	planned, err := job.Plan(ctx)
	return planned, refusedStart(err)
}

// refusedStart returns err marked as a start the checks refused, with their reason, or as it is when no check refused.
func refusedStart(err error) error {
	if !errors.Is(err, updater.ErrRefused) {
		return err
	}
	return content.Refuse(server.ErrUpdateRefused, string(updater.ReasonOf(err)), err.Error(),
		content.Details{"reason": err.Error()})
}

// finish runs the rest of the update planned on the serve run's context, ending the run once it switched builds.
func (a *adminUpdates) finish(planned *updater.Planned) {
	defer a.free()
	a.settle(planned.Apply(a.serving))
}

// settle ends the serve run to start the new build once the switch stands, logging a failure before it otherwise.
func (a *adminUpdates) settle(state updater.State, err error) {
	var stands updater.SwitchedError
	switch {
	case errors.As(err, &stands):
		a.logger.Warn("switched builds but a step after failed, so the server stops to start the new one", "to",
			stands.Release, "error", err)
		a.restart(restartError{release: stands.Release})
	case err != nil:
		a.logger.Warn("the update an administrator started failed", "error", err)
	default:
		a.logger.Info("switched builds, so the server stops to start the new one", "from", state.From, "to", state.To)
		a.restart(restartError{release: state.To})
	}
}

// ended waits for the update under way, then returns served with the restart an update that switched asks for.
func (a *adminUpdates) ended(served error) error {
	a.mu.Lock()
	a.stopped = true
	a.mu.Unlock()
	a.running.Wait()
	var restart restartError
	if errors.As(context.Cause(a.serving), &restart) {
		return errors.Join(served, restart)
	}
	return served
}
