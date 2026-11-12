// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"time"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ cron.Store = (*CronStore)(nil)

// CronStore keeps the site's cron jobs and their leases in the core schema.
type CronStore struct {
	queries *db.Queries
}

// NewCronStore returns a [CronStore] backed by pool.
func NewCronStore(pool *pgxpool.Pool) *CronStore {
	return &CronStore{queries: db.New(pool)}
}

// Slots returns where every stored job stands.
func (s *CronStore) Slots(ctx context.Context) ([]cron.Slot, error) {
	rows, err := s.queries.ListCronSlots(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: reading the cron jobs: %w", dbkitpg.Classify(err))
	}
	slots := make([]cron.Slot, 0, len(rows))
	for _, row := range rows {
		slots = append(slots, cron.Slot{
			Name: row.Name, NextRun: row.NextRunAt, LeasedUntil: cmp.Or(row.LeasedUntil, &time.Time{}).UTC(),
		})
	}
	return slots, nil
}

// Take claims the job for the lease when it is due and free, reporting whether it did.
func (s *CronStore) Take(ctx context.Context, lease cron.Lease) (cron.Lease, bool, error) {
	nextRun, err := s.queries.TakeCronJob(ctx, db.TakeCronJobParams{
		Name: lease.Name, TakenAt: lease.Taken, LeasedUntil: &lease.Until, LeaseToken: &lease.Token,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return cron.Lease{}, false, nil
	}
	if err != nil {
		return cron.Lease{}, false, fmt.Errorf("postgres: taking the cron job %q: %w", lease.Name, dbkitpg.Classify(err))
	}
	lease.NextRun = nextRun
	return lease, true, nil
}

// Settle frees the job and records how its run ended, refusing a lease another run took over.
func (s *CronStore) Settle(ctx context.Context, lease cron.Lease, outcome cron.Outcome) error {
	settled, err := s.queries.SettleCronJob(ctx, db.SettleCronJobParams{
		NextRunAt: outcome.NextRun, FinishedAt: &outcome.Finished, LastError: failureOf(outcome),
		Name: lease.Name, LeaseToken: &lease.Token,
	})
	if err != nil {
		return fmt.Errorf("postgres: settling the cron job %q: %w", lease.Name, dbkitpg.Classify(err))
	}
	if settled == 0 {
		return cron.ErrLeaseLost
	}
	return nil
}

// failureOf returns the failure an outcome records, nothing for a run that ended well.
func failureOf(outcome cron.Outcome) *string {
	if outcome.Failure == "" {
		return nil
	}
	return &outcome.Failure
}
