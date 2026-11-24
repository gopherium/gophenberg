// SPDX-License-Identifier: Apache-2.0

package updater_test

import (
	"database/sql"
	"errors"
	"testing"

	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/internal/updater"
)

// emptyDatabase returns the address of a fresh database holding no schema, skipping in short mode.
func emptyDatabase(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	return testdb.New(t, pgtestdb.NoopMigrator{})
}

// serverMajor returns the PostgreSQL major of the server db connects to, failing t when it cannot read it.
func serverMajor(t *testing.T, db *sql.DB) int {
	t.Helper()
	var major int
	if err := db.QueryRowContext(t.Context(), "SELECT current_setting('server_version_num')::int / 10000").
		Scan(&major); err != nil {
		t.Fatalf("reading the major of the test server: %v", err)
	}
	return major
}

// databaseSize returns the size in bytes of the database db connects to, failing t when it cannot read it.
func databaseSize(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var size int64
	if err := db.QueryRowContext(t.Context(), "SELECT pg_database_size(current_database())").Scan(&size); err != nil {
		t.Fatalf("reading the size of the test database: %v", err)
	}
	return size
}

// locked takes the update lock on db, failing t when it cannot.
func locked(t *testing.T, db *sql.DB) func() {
	t.Helper()
	release, err := updater.Lock(t.Context(), db)
	if err != nil {
		t.Fatalf("Lock() = %v, want the update lock taken", err)
	}
	return release
}

func TestLockStopsASecondJobUntilTheFirstLetsGo(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t, emptyDatabase(t))
	release := locked(t, db)

	_, err := updater.Lock(t.Context(), db)

	if !errors.Is(err, updater.ErrUpdateRuns) {
		t.Errorf("Lock() while another job holds it = %v, want %v", err, updater.ErrUpdateRuns)
	}
	release()
	locked(t, db)()
}

func TestLockHeldSaysWhetherAJobHoldsTheUpdateLockWithoutTakingIt(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t, emptyDatabase(t))
	if held, err := updater.LockHeld(t.Context(), db); err != nil || held {
		t.Fatalf("LockHeld() with no job = %t, %v, want false, nil", held, err)
	}
	release := locked(t, db)

	held, err := updater.LockHeld(t.Context(), db)

	if err != nil || !held {
		t.Errorf("LockHeld() while a job holds the lock = %t, %v, want true, nil", held, err)
	}
	release()
	if held, err := updater.LockHeld(t.Context(), db); err != nil || held {
		t.Errorf("LockHeld() once the job let go = %t, %v, want false, nil", held, err)
	}
	locked(t, db)()
}

func TestLockHeldRefusesADatabaseItCannotReach(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t, emptyDatabase(t))
	_ = db.Close()

	_, err := updater.LockHeld(t.Context(), db)

	wantRefused(t, err, "updater: read the update lock")
}

func TestLockRefusesADatabaseItCannotReach(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t, emptyDatabase(t))
	_ = db.Close()

	_, err := updater.Lock(t.Context(), db)

	wantRefused(t, err, "updater: take the update lock")
}

func TestLockRefusesARoleThatMayNotTakeIt(t *testing.T) {
	t.Parallel()

	address := emptyDatabase(t)
	revoke := "REVOKE EXECUTE ON FUNCTION pg_try_advisory_lock(bigint) FROM PUBLIC"
	if _, err := testdb.Open(t, testdb.AsServer(t, address)).ExecContext(t.Context(), revoke); err != nil {
		t.Fatalf("revoking the advisory lock: %v", err)
	}

	_, err := updater.Lock(t.Context(), testdb.Open(t, address))

	wantRefused(t, err, "updater: take the update lock")
	wantRefused(t, err, "permission denied for function pg_try_advisory_lock")
}
