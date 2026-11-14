// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// emptyDB returns a handle on a fresh database holding no schema, skipping in short mode.
func emptyDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	return testdb.Open(t, testdb.New(t, pgtestdb.NoopMigrator{}))
}

// lockTimedHandle returns a database/sql handle on the database at url whose statements give up on a held lock.
func lockTimedHandle(t *testing.T, url string) *sql.DB {
	t.Helper()
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatalf("parsing the handle address: %v", err)
	}
	cfg.RuntimeParams["lock_timeout"] = "200ms"
	handle := stdlib.OpenDB(*cfg)
	t.Cleanup(func() { _ = handle.Close() })
	return handle
}

// raiseMinimumVersion raises the stored minimum version on db to minimum, failing the test on an error.
func raiseMinimumVersion(t *testing.T, db *sql.DB, minimum string) {
	t.Helper()
	if err := postgres.RaiseMinimumVersion(t.Context(), db, minimum); err != nil {
		t.Fatalf("RaiseMinimumVersion(%q) error = %v, want nil", minimum, err)
	}
}

func TestStoredMinimumVersionIsEmptyBeforeTheCoreTablesExist(t *testing.T) {
	t.Parallel()

	minimum, err := postgres.StoredMinimumVersion(t.Context(), emptyDB(t))

	if err != nil || minimum != "" {
		t.Errorf("StoredMinimumVersion() = %q, %v on a database without core tables, want an empty version and nil",
			minimum, err)
	}
}

func TestStoredMinimumVersionIsEmptyUntilABinaryRaisesIt(t *testing.T) {
	t.Parallel()

	minimum, err := postgres.StoredMinimumVersion(t.Context(), testdb.Open(t, migratedAddress(t)))

	if err != nil || minimum != "" {
		t.Errorf("StoredMinimumVersion() = %q, %v on a database no binary raised, want an empty version and nil",
			minimum, err)
	}
}

func TestRaiseMinimumVersionNeverLowersTheStoredOne(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		stored []string
		raise  string
		want   string
	}{
		{name: "a first minimum is stored", raise: "0.19.0", want: "0.19.0"},
		{name: "an older minimum is raised", stored: []string{"0.19.0"}, raise: "0.20.0", want: "0.20.0"},
		{name: "a newer minimum stays", stored: []string{"0.21.0"}, raise: "0.20.0", want: "0.21.0"},
		{name: "the same minimum stays", stored: []string{"0.20.0"}, raise: "0.20.0", want: "0.20.0"},
		{name: "numbers order the versions, not text", stored: []string{"0.9.0"}, raise: "0.10.0", want: "0.10.0"},
		{name: "a release raises its candidate's minimum", stored: []string{"0.20.0-rc.1"}, raise: "0.20.0",
			want: "0.20.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db := testdb.Open(t, migratedAddress(t))
			for _, minimum := range tc.stored {
				raiseMinimumVersion(t, db, minimum)
			}

			raiseMinimumVersion(t, db, tc.raise)

			if got, err := postgres.StoredMinimumVersion(t.Context(), db); err != nil || got != tc.want {
				t.Errorf("StoredMinimumVersion() = %q, %v after raising to %q, want %q", got, err, tc.raise, tc.want)
			}
		})
	}
}

func TestRaiseFromLeavesAMinimumVersionAnotherBinaryStoredSince(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t, migratedAddress(t))
	raiseMinimumVersion(t, db, "0.20.0")

	raised, err := postgres.RaiseFrom(t.Context(), db, "0.19.0", "0.21.0")

	if err != nil || raised {
		t.Errorf("RaiseFrom() over a stale 0.19.0 = %v, %v, want nothing raised", raised, err)
	}
	if got, _ := postgres.StoredMinimumVersion(t.Context(), db); got != "0.20.0" {
		t.Errorf("StoredMinimumVersion() = %q, want the 0.20.0 stored since kept", got)
	}
	if raised, err := postgres.RaiseFrom(t.Context(), db, "0.20.0", "0.21.0"); err != nil || !raised {
		t.Errorf("RaiseFrom() over the stored 0.20.0 = %v, %v, want 0.21.0 raised", raised, err)
	}
}

func TestRaiseMinimumVersionFailsBeforeTheCoreTablesExist(t *testing.T) {
	t.Parallel()

	if err := postgres.RaiseMinimumVersion(t.Context(), emptyDB(t), "0.19.0"); err == nil {
		t.Error("RaiseMinimumVersion() on a database without core tables error = nil, want a failure")
	}
}

func TestMinimumVersionReportsDatabaseFailures(t *testing.T) {
	t.Parallel()

	db := testdb.Open(t, migratedAddress(t))
	_ = db.Close()

	if _, err := postgres.StoredMinimumVersion(t.Context(), db); err == nil {
		t.Error("StoredMinimumVersion() on a closed handle error = nil, want a failure")
	}
	if err := postgres.RaiseMinimumVersion(t.Context(), db, "0.19.0"); err == nil {
		t.Error("RaiseMinimumVersion() on a closed handle error = nil, want a failure")
	}
}

func TestStoredMinimumVersionBehindALockedTableAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	url := migratedAddress(t)
	holdInRival(t, url, "LOCK TABLE core.settings IN ACCESS EXCLUSIVE MODE")

	_, err := postgres.StoredMinimumVersion(t.Context(), lockTimedHandle(t, url))

	if !errors.Is(err, dbkit.ErrBusy) ||
		!strings.HasPrefix(fmt.Sprint(err), "postgres: read the stored minimum version: ") {
		t.Errorf("StoredMinimumVersion() behind a locked table error = %v, want dbkit.ErrBusy under the read", err)
	}
}

func TestRaiseMinimumVersionBehindAHeldRowAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	url := migratedAddress(t)
	holdInRival(t, url, "INSERT INTO core.settings (key, value) VALUES ('minimum_version', '0.19.0')")

	err := postgres.RaiseMinimumVersion(t.Context(), lockTimedHandle(t, url), "0.20.0")

	if !errors.Is(err, dbkit.ErrBusy) ||
		!strings.Contains(fmt.Sprint(err), "raise the stored minimum version to 0.20.0: ") {
		t.Errorf("RaiseMinimumVersion() behind a held row error = %v, want dbkit.ErrBusy under the raise", err)
	}
}
