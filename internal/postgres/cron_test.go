// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// cronStart is the moment the cron store tests take their first lease.
var cronStart = time.Date(2026, time.October, 13, 10, 0, 0, 0, time.UTC)

// cronStoreWithPool returns a cron store over a fresh migrated database and the pool behind it.
func cronStoreWithPool(t *testing.T) (*postgres.CronStore, *pgxpool.Pool) {
	t.Helper()
	pool := migratedPool(t)
	return postgres.NewCronStore(pool), pool
}

// lastFailureIn returns a way to read the failure a job's last run recorded in the database behind pool.
func lastFailureIn(pool *pgxpool.Pool) func(ctx context.Context, name string) (string, error) {
	return func(ctx context.Context, name string) (string, error) {
		var failed string
		err := pool.QueryRow(ctx, "SELECT coalesce(last_error, '') FROM core.cron_jobs WHERE name = $1", name).Scan(&failed)
		return failed, err
	}
}

// tidyLease returns a minute's lease on the tidy job taken at the first moment.
func tidyLease() cron.Lease {
	return cron.Lease{Name: "tidy", Token: "first", Taken: cronStart, Until: cronStart.Add(time.Minute)}
}

func TestCronStoreReportsSlotsItCannotRead(t *testing.T) {
	t.Parallel()

	store, pool := cronStoreWithPool(t)
	sabotage(t, pool, "ALTER TABLE core.cron_jobs RENAME TO cron_jobs_gone")

	if _, err := store.Slots(t.Context()); err == nil || !strings.Contains(err.Error(), "reading the cron jobs") {
		t.Errorf("Slots() error = %v, want the failed read named", err)
	}
}

func TestCronStoreReportsATakeItCannotWrite(t *testing.T) {
	t.Parallel()

	store, pool := cronStoreWithPool(t)
	sabotage(t, pool, "ALTER TABLE core.cron_jobs RENAME TO cron_jobs_gone")

	_, ok, err := store.Take(t.Context(), tidyLease())
	if ok || err == nil || !strings.Contains(err.Error(), `taking the cron job "tidy"`) {
		t.Errorf("Take() = %v, %v, want the failed take named", ok, err)
	}
}

func TestCronStoreReportsASettleItCannotWrite(t *testing.T) {
	t.Parallel()

	store, pool := cronStoreWithPool(t)
	lease, ok, err := store.Take(t.Context(), tidyLease())
	if err != nil || !ok {
		t.Fatalf("Take() = %v, %v, want the lease handed over", ok, err)
	}
	sabotage(t, pool, "ALTER TABLE core.cron_jobs RENAME TO cron_jobs_gone")

	err = store.Settle(t.Context(), lease, cron.Outcome{NextRun: cronStart.Add(time.Hour), Finished: cronStart})
	named := err != nil && strings.Contains(err.Error(), `settling the cron job "tidy"`)
	if !named || errors.Is(err, cron.ErrLeaseLost) {
		t.Errorf("Settle() error = %v, want the failed write named and not taken for a lost lease", err)
	}
}

func TestCronMigrationKeepsTheLeaseWhole(t *testing.T) {
	t.Parallel()

	_, pool := cronStoreWithPool(t)

	_, err := pool.Exec(t.Context(),
		"INSERT INTO core.cron_jobs (name, next_run_at, lease_token) VALUES ('tidy', now(), 'first')")
	if err == nil || !strings.Contains(err.Error(), "cron_jobs_lease_whole") {
		t.Errorf("a token with no lease end was stored with error %v, want the lease check to refuse it", err)
	}
}

func TestCronTakeBehindALockedTableAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	_, pool := cronStoreWithPool(t)
	holdInRival(t, pool.Config().ConnString(), "LOCK TABLE core.cron_jobs IN ACCESS EXCLUSIVE MODE")
	timed := postgres.NewCronStore(lockTimedPool(t, pool.Config().ConnString()))

	_, taken, err := timed.Take(t.Context(), tidyLease())

	named := err != nil && strings.Contains(err.Error(), `taking the cron job "tidy"`)
	if taken || !errors.Is(err, dbkit.ErrBusy) || !named {
		t.Errorf("Take() behind a locked table = %v, %v, want dbkit.ErrBusy under the take", taken, err)
	}
}
