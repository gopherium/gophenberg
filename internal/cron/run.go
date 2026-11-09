// SPDX-License-Identifier: Apache-2.0

package cron

import (
	"cmp"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"slices"
	"time"
)

// RunDue runs each job that is due and free in turn until the store fails, erring only when it cannot read the slots.
func (r *Runner) RunDue(ctx context.Context) (Report, error) {
	var report Report
	if r.refused {
		return report, nil
	}
	read, cancel := context.WithTimeout(ctx, r.lease)
	slots, err := r.store.Slots(read)
	cancel()
	if err != nil {
		r.logger.Error("cron jobs unread", "error", err)
		return report, fmt.Errorf("cron: read the slots: %w", err)
	}
	ready, held := r.plan(slots, r.clock())
	report.Held = held
	for _, job := range ready {
		if ctx.Err() != nil {
			break
		}
		if !r.runOne(ctx, job, &report) {
			break
		}
	}
	return report, nil
}

// plan returns the jobs whose time has come, in the order they run, and how many of them another run holds.
func (r *Runner) plan(slots []Slot, now time.Time) ([]Job, int) {
	stored := make(map[string]Slot, len(slots))
	for _, slot := range slots {
		stored[slot.Name] = slot
	}
	var ready []Job
	held := 0
	for _, job := range r.jobs {
		slot := stored[job.Name]
		if slot.NextRun.After(now) {
			continue
		}
		if slot.LeasedUntil.After(now) {
			held++
			continue
		}
		ready = append(ready, job)
	}
	slices.SortStableFunc(ready, func(a, b Job) int {
		return cmp.Or(stored[a.Name].NextRun.Compare(stored[b.Name].NextRun), cmp.Compare(a.Name, b.Name))
	})
	return ready, held
}

// runOne takes, runs and settles one job, counting how it ended, and reports whether the store can serve the next.
func (r *Runner) runOne(ctx context.Context, job Job, report *Report) bool {
	limit, hold := r.limits(job)
	taken := r.clock()
	lease, ok, err := r.take(ctx, Lease{Name: job.Name, Token: rand.Text(), Taken: taken, Until: taken.Add(hold)})
	if err != nil {
		report.Failed++
		r.logger.Warn("cron job failed", "job", job.Name, "error", err)
		return false
	}
	if !ok {
		report.Held++
		return true
	}
	failure := r.attempt(ctx, job, lease, limit)
	outcome := r.outcome(ctx, job, lease, failure)
	settled := r.settle(ctx, lease, outcome)
	served := settled == nil || errors.Is(settled, ErrLeaseLost)
	if settled != nil && outcome.Failure == "" {
		report.Failed++
		return served
	}
	r.count(job, outcome.Finished.Sub(taken), outcome, report)
	return served
}

// limits returns how long job may run and how long its lease holds, the runner's own for a job with no limit.
func (r *Runner) limits(job Job) (time.Duration, time.Duration) {
	if job.TimeLimit == 0 {
		return r.timeLimit, r.lease
	}
	return job.TimeLimit, job.TimeLimit + r.lease - r.timeLimit
}

// take asks the store for lease, giving up once lease would have ended or the runner's lease has passed.
func (r *Runner) take(ctx context.Context, lease Lease) (Lease, bool, error) {
	bounded, cancel := context.WithTimeout(ctx, min(r.lease, lease.Until.Sub(lease.Taken)))
	defer cancel()
	return r.store.Take(bounded, lease)
}

// count tallies how one run ended and how long it took, and logs it.
func (r *Runner) count(job Job, took time.Duration, outcome Outcome, report *Report) {
	if outcome.Failure == "" {
		report.Ran++
		r.logger.Info("cron job ran", "job", job.Name, "took", took)
		return
	}
	report.Failed++
	r.logger.Warn("cron job failed", "job", job.Name, "took", took, "error", outcome.Failure)
}

// attempt runs job within limit and the lease, turning a panic or a return past either into an error.
func (r *Runner) attempt(ctx context.Context, job Job, lease Lease, limit time.Duration) (err error) {
	limited, cancel := context.WithTimeout(ctx, min(limit, lease.Until.Sub(r.clock())))
	defer cancel()
	defer func() {
		if caught := recover(); caught != nil {
			err = fmt.Errorf("panic: %v", caught)
		}
	}()
	if err = job.Run(limited, lease.Taken); err != nil {
		return err
	}
	return overrun(ctx, limited)
}

// overrun returns why a run that came back cleanly still ran past its limit, nothing when its caller ended it.
func overrun(caller, limited context.Context) error {
	if caller.Err() != nil {
		return nil
	}
	return limited.Err()
}

// outcome returns how a run ended, keeping a run that its context cut short due again at once.
func (r *Runner) outcome(ctx context.Context, job Job, lease Lease, failure error) Outcome {
	finished := r.clock()
	if failure == nil {
		return Outcome{NextRun: nextRun(lease.NextRun, job.Every, finished), Finished: finished}
	}
	if ctx.Err() != nil {
		return Outcome{NextRun: lease.NextRun, Finished: finished, Failure: failure.Error()}
	}
	return Outcome{NextRun: nextRun(lease.NextRun, job.Every, finished), Finished: finished, Failure: failure.Error()}
}

// settle records the outcome past the end of ctx, bounded by the lease, and returns why it could not.
func (r *Runner) settle(ctx context.Context, lease Lease, outcome Outcome) error {
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), r.lease)
	defer cancel()
	err := r.store.Settle(bounded, lease, outcome)
	if errors.Is(err, ErrLeaseLost) {
		r.logger.Warn("cron job lease lost", "job", lease.Name)
		return err
	}
	if err != nil {
		r.logger.Error("cron job unsettled", "job", lease.Name, "error", err)
	}
	return err
}

// NextRun returns when the job called name falls due next in store, the zero time while store holds no slot of it.
func NextRun(ctx context.Context, store Store, name string) (time.Time, error) {
	slots, err := store.Slots(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("cron: read the slots: %w", err)
	}
	for _, slot := range slots {
		if slot.Name == name {
			return slot.NextRun, nil
		}
	}
	return time.Time{}, nil
}

// nextRun returns the first time after now on the rhythm that started at from.
func nextRun(from time.Time, every time.Duration, now time.Time) time.Time {
	next := from.Add(every)
	if next.After(now) {
		return next
	}
	return from.Add((now.Sub(from)/every + 1) * every)
}
