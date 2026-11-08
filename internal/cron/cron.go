// SPDX-License-Identifier: Apache-2.0

// Package cron runs named jobs once their interval has passed, each under a lease in a store.
package cron

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"time"
)

// Job is one named piece of work that falls due again each time its interval passes, under its own time limit if set.
type Job struct {
	Name      string
	Every     time.Duration
	TimeLimit time.Duration
	Run       func(ctx context.Context, now time.Time) error
}

// Slot is where one stored job stands: when it falls due next and until when a run holds it.
type Slot struct {
	Name        string
	NextRun     time.Time
	LeasedUntil time.Time
}

// Lease is one run's hold on one job, proven by its token.
type Lease struct {
	Name    string
	Token   string
	Taken   time.Time
	Until   time.Time
	NextRun time.Time
}

// Outcome is how one run ended and when its job falls due next.
type Outcome struct {
	NextRun  time.Time
	Finished time.Time
	Failure  string
}

// ErrLeaseLost marks a settle whose lease another run has taken over.
var ErrLeaseLost = errors.New("cron: lease lost")

// Store keeps each job's next run time and the lease that guards a run.
type Store interface {
	Slots(ctx context.Context) ([]Slot, error)
	Take(ctx context.Context, lease Lease) (Lease, bool, error)
	Settle(ctx context.Context, lease Lease, outcome Outcome) error
}

// Config is what a runner is built from: the jobs, their store, the clock and the limits.
type Config struct {
	Jobs      []Job
	Store     Store
	Clock     func() time.Time
	TimeLimit time.Duration
	Lease     time.Duration
	Logger    *slog.Logger
}

// Report counts what one pass over the cron jobs did.
type Report struct {
	Ran    int
	Failed int
	Held   int
}

// Runner runs a list of cron jobs, one at a time.
type Runner struct {
	jobs      []Job
	store     Store
	clock     func() time.Time
	timeLimit time.Duration
	lease     time.Duration
	logger    *slog.Logger
	refused   bool
}

// New returns a runner over cfg, leaving out each job it cannot run, and running none under a config it cannot use.
func New(cfg Config) *Runner {
	r := &Runner{
		store:     cfg.Store,
		clock:     cfg.Clock,
		timeLimit: cfg.TimeLimit,
		lease:     cfg.Lease,
		logger:    cmp.Or(cfg.Logger, slog.New(slog.DiscardHandler)),
	}
	if r.clock == nil {
		r.clock = time.Now
	}
	if reason := configRefusal(cfg); reason != "" {
		r.logger.Error("cron config refused", "reason", reason)
		r.refused = true
		return r
	}
	seen := make(map[string]bool, len(cfg.Jobs))
	for _, job := range cfg.Jobs {
		if reason := refusal(job, seen); reason != "" {
			r.logger.Warn("cron job left out", "job", job.Name, "reason", reason)
			continue
		}
		seen[job.Name] = true
		r.jobs = append(r.jobs, job)
	}
	return r
}

// configRefusal returns why a runner can run no job under cfg, or nothing when it can.
func configRefusal(cfg Config) string {
	switch {
	case cfg.Store == nil:
		return "no store"
	case cfg.TimeLimit <= 0:
		return "a time limit not above zero"
	case cfg.Lease <= cfg.TimeLimit:
		return "a lease not above the time limit"
	}
	return ""
}

// refusal returns why the runner cannot run job, or nothing when it can.
func refusal(job Job, seen map[string]bool) string {
	switch {
	case job.Name == "":
		return "no name"
	case seen[job.Name]:
		return "a second job of the name"
	case job.Every <= 0:
		return "an interval not above zero"
	case job.TimeLimit < 0:
		return "a time limit below zero"
	case job.Run == nil:
		return "no run"
	}
	return ""
}
