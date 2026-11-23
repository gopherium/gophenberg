// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// cronStart is the moment the cron store tests take their first lease.
var cronStart = time.Date(2026, time.October, 13, 10, 0, 0, 0, time.UTC)

// takeDeadline is how long a cron take behind a held write runs before its context ends.
const takeDeadline = 200 * time.Millisecond

// lastFailureIn returns a way to read the failure a job's last run recorded in the file behind handle.
func lastFailureIn(handle *sql.DB) func(ctx context.Context, name string) (string, error) {
	return func(ctx context.Context, name string) (string, error) {
		var failed string
		lookup := "SELECT coalesce(last_error, '') FROM core_cron_jobs WHERE name = ?"
		err := handle.QueryRowContext(ctx, lookup, name).Scan(&failed)
		return failed, err
	}
}

// tidyLease returns a minute's lease on the tidy job taken at the first moment.
func tidyLease() cron.Lease {
	return cron.Lease{Name: "tidy", Token: "first", Taken: cronStart, Until: cronStart.Add(time.Minute)}
}

func TestCronStoreReportsAStatementThatFails(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, statement, prefix string
		call                    func(ctx context.Context, store *sqlite.CronStore) error
	}{
		{"a read names the cron jobs", db.ListCronSlots, "sqlite: reading the cron jobs: ",
			func(ctx context.Context, store *sqlite.CronStore) error {
				_, err := store.Slots(ctx)
				return err
			}},
		{"a take names its job", db.TakeCronJob, `sqlite: taking the cron job "tidy": `,
			func(ctx context.Context, store *sqlite.CronStore) error {
				_, _, err := store.Take(ctx, tidyLease())
				return err
			}},
		{"a settle names its job", db.SettleCronJob, `sqlite: settling the cron job "tidy": `,
			func(ctx context.Context, store *sqlite.CronStore) error {
				return store.Settle(ctx, tidyLease(), cron.Outcome{NextRun: cronStart.Add(time.Hour), Finished: cronStart})
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handle, faults := faultyHandle(t)
			faults.FailStatement(tc.statement, errPlanted)

			err := tc.call(t.Context(), sqlite.NewCronStore(handle))

			wantPlanted(t, err, tc.prefix)
			if errors.Is(err, cron.ErrLeaseLost) {
				t.Errorf("error = %v, want the failed statement never taken for a lost lease", err)
			}
		})
	}
}

func TestCronTakeBehindAnotherWriterAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle := impatientHandle(t)
	holdWrites(t, handle)

	_, taken, err := sqlite.NewCronStore(handle).Take(t.Context(), tidyLease())

	named := strings.HasPrefix(fmt.Sprint(err), `sqlite: taking the cron job "tidy": `)
	if taken || !errors.Is(err, dbkit.ErrBusy) || !named {
		t.Errorf("Take() behind another writer = %v, %v, want dbkit.ErrBusy under the take", taken, err)
	}
}

func TestCronTakeBehindAHeldWriteGivesUpOnceItsContextEnds(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	holdWrites(t, handle)
	ctx, cancel := context.WithTimeout(t.Context(), takeDeadline)
	defer cancel()
	began := time.Now()

	_, taken, err := sqlite.NewCronStore(handle).Take(ctx, tidyLease())

	waited := time.Since(began)
	if taken || !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Take() behind a held write = %v, %v, want context.DeadlineExceeded once its context ends", taken, err)
	}
	if waited > takeDeadline+waitMargin {
		t.Errorf("Take() behind a held write answered after %v, want about %v, well before the busy timeout %v",
			waited, takeDeadline, testSettings().BusyTimeout)
	}
}

func TestCronSettleBehindAnotherWriterAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle := impatientHandle(t)
	store := sqlite.NewCronStore(handle)
	held, ok, err := store.Take(t.Context(), tidyLease())
	if err != nil || !ok {
		t.Fatalf("Take() = %v, %v, want the lease handed over", ok, err)
	}
	holdWrites(t, handle)

	err = store.Settle(t.Context(), held, cron.Outcome{NextRun: cronStart.Add(time.Hour), Finished: cronStart})

	named := strings.HasPrefix(fmt.Sprint(err), `sqlite: settling the cron job "tidy": `)
	if !errors.Is(err, dbkit.ErrBusy) || !named || errors.Is(err, cron.ErrLeaseLost) {
		t.Errorf("Settle() behind another writer = %v, want dbkit.ErrBusy under the settle", err)
	}
}

func TestCronSlotsBehindAnExclusiveConnectionAnswerDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle := impatientHandle(t)
	store := sqlite.NewCronStore(handle)
	if _, ok, err := store.Take(t.Context(), tidyLease()); err != nil || !ok {
		t.Fatalf("Take() = %v, %v, want the lease handed over", ok, err)
	}
	owner := otherWriter(t, handle)
	for _, statement := range []string{"PRAGMA locking_mode=EXCLUSIVE", "BEGIN IMMEDIATE",
		"UPDATE core_cron_jobs SET last_error = 'held'", "COMMIT"} {
		if _, err := owner.ExecContext(t.Context(), statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}

	_, err := store.Slots(t.Context())

	named := strings.HasPrefix(fmt.Sprint(err), "sqlite: reading the cron jobs: ")
	if !errors.Is(err, dbkit.ErrBusy) || !named {
		t.Errorf("Slots() behind an exclusive connection = %v, want dbkit.ErrBusy under the read", err)
	}
}
