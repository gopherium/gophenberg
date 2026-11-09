// SPDX-License-Identifier: Apache-2.0

package cron_test

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/cron/crontest"
)

// limitsSeen returns the tidy job under limit, recording the time its run had left and its lease's end.
func limitsSeen(store cron.Store, limit time.Duration, left *time.Duration, held *time.Time) cron.Job {
	return cron.Job{Name: "tidy", Every: time.Hour, TimeLimit: limit, Run: func(ctx context.Context, _ time.Time) error {
		deadline, _ := ctx.Deadline()
		*left = time.Until(deadline)
		slots, err := store.Slots(ctx)
		for _, slot := range slots {
			*held = slot.LeasedUntil
		}
		return err
	}}
}

func TestNewLeavesOutAJobWhoseTimeLimitIsBelowZero(t *testing.T) {
	t.Parallel()

	job := &counter{}
	r := newRig(cron.Job{Name: "tidy", Every: time.Hour, TimeLimit: -time.Second, Run: job.Run})

	r.runDue(t)
	if job.count() != 0 {
		t.Errorf("a job with a time limit below zero ran %d times, want it left out", job.count())
	}
	r.logged(t, "cron job left out")
}

func TestRunDueGivesAJobWithNoLimitOfItsOwnTheRunnersLimitAndLease(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var left time.Duration
		var held time.Time
		store := crontest.NewMemoryStore()
		r := rigOver(store, limitsSeen(store, 0, &left, &held))

		r.runDue(t)
		if want := start.Add(2 * time.Second); left != time.Second || !held.Equal(want) {
			t.Errorf("the job had %s to run under a lease to %s, want the runner's second and its lease to %s",
				left, held, want)
		}
	})
}

func TestRunDueRunsAJobPastTheRunnersLimitUpToItsOwn(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		r := newRig(cron.Job{Name: "tidy", Every: time.Hour, TimeLimit: 5 * time.Second,
			Run: func(ctx context.Context, _ time.Time) error {
				time.Sleep(3 * time.Second)
				return ctx.Err()
			}})

		if report := r.runDue(t); report != (cron.Report{Ran: 1}) {
			t.Errorf("RunDue() = %+v, want a three second run under a five second limit to finish", report)
		}
	})
}

func TestRunDueCutsAJobAtItsOwnLimit(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		var ran time.Duration
		r := newRig(cron.Job{Name: "tidy", Every: time.Hour, TimeLimit: 5 * time.Second,
			Run: func(ctx context.Context, _ time.Time) error {
				began := time.Now()
				<-ctx.Done()
				ran = time.Since(began)
				return ctx.Err()
			}})

		r.runDue(t)
		if failed := r.failure(t); ran != 5*time.Second || failed != context.DeadlineExceeded.Error() {
			t.Errorf("the job ran %s and recorded %q, want it cut at its own five seconds", ran, failed)
		}
	})
}

// wantLeased fails t unless a job limited to own under a runner of limit and lease runs own under a lease of hold.
func wantLeased(t *testing.T, limit, lease, own, hold time.Duration) {
	t.Helper()
	synctest.Test(t, func(t *testing.T) {
		var left time.Duration
		var held time.Time
		store := crontest.NewMemoryStore()
		runner := cron.New(cron.Config{
			Jobs: []cron.Job{limitsSeen(store, own, &left, &held)}, Store: store, Clock: newClock().Now,
			TimeLimit: limit, Lease: lease,
		})

		if _, err := runner.RunDue(t.Context()); err != nil {
			t.Fatalf("RunDue() error = %v", err)
		}
		if want := start.Add(hold); left != own || !held.Equal(want) {
			t.Errorf("under a %s limit and a %s lease the job had %s to run under a lease to %s, want %s and %s",
				limit, lease, left, held, own, want)
		}
	})
}

func TestRunDueLeasesAJobWithItsOwnLimitForThatLimitPlusTheGap(t *testing.T) {
	t.Parallel()

	wantLeased(t, time.Second, 2*time.Second, 5*time.Second, 6*time.Second)
	wantLeased(t, time.Second, 2*time.Second, 500*time.Millisecond, 1500*time.Millisecond)
	wantLeased(t, time.Second, 5*time.Second, 3*time.Second, 7*time.Second)
}

func TestRunDueGivesUpTheTakeOfAJobWithItsOwnLimitOnceItsLeaseOrTheRunnersEnds(t *testing.T) {
	t.Parallel()

	for _, limits := range [][2]time.Duration{
		{100 * time.Millisecond, 1100 * time.Millisecond}, {5 * time.Second, 2 * time.Second},
	} {
		own, waited := limits[0], limits[1]
		synctest.Test(t, func(t *testing.T) {
			job := &counter{}
			r := rigOver(stalledTake{crontest.NewMemoryStore()},
				cron.Job{Name: "tidy", Every: time.Hour, TimeLimit: own, Run: job.Run})
			began := time.Now()

			if report := r.runDue(t); report != (cron.Report{Failed: 1}) || job.count() != 0 {
				t.Errorf("RunDue() = %+v with %d runs, want the stalled take given up and no run", report, job.count())
			}
			if took := time.Since(began); took != waited {
				t.Errorf("RunDue() gave up the take of a job limited to %s after %s, want %s", own, took, waited)
			}
		})
	}
}
