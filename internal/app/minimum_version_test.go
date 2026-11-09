// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/internal/version"
)

// newerMinimumVersion is a minimum version no binary of this repository reaches.
const newerMinimumVersion = "99.0.0"

// errRead is the failure a stubbed read of the database answers with.
var errRead = errors.New("read failure")

func TestMinimumVersionRefusalLetsThisBinaryOnADatabaseItMayServe(t *testing.T) {
	t.Parallel()

	for _, stored := range []string{"", "0.0.1", version.Minimum(), version.Version()} {
		if err := minimumVersionRefusal(stored); err != nil {
			t.Errorf("minimumVersionRefusal(%q) = %v, want nil", stored, err)
		}
	}
}

func TestMinimumVersionRefusalNamesTheVersionADatabaseNeeds(t *testing.T) {
	t.Parallel()

	err := minimumVersionRefusal(newerMinimumVersion)

	want := "this database needs Gophenberg 99.0.0 or newer and this is " + version.Version() +
		", so restore the backup taken before the update to run an older version"
	if err == nil || err.Error() != want {
		t.Errorf("minimumVersionRefusal(%q) = %v, want %q", newerMinimumVersion, err, want)
	}
}

func TestGuardMinimumVersionReportsAVersionItCouldNotRead(t *testing.T) {
	t.Parallel()

	if err := guardMinimumVersion("", errRead); !errors.Is(err, errRead) {
		t.Errorf("guardMinimumVersion() = %v, want the failure of the read", err)
	}
}

// minimumVersionLeftAt migrates the PostgreSQL database at address, then stores minimum as the minimum version there.
func minimumVersionLeftAt(t *testing.T, address, minimum string) {
	t.Helper()
	db := testdb.Open(t, address)
	if err := migrate(t.Context(), address, db); err != nil {
		t.Fatalf("migrate() error = %v, want nil", err)
	}
	if _, err := db.ExecContext(t.Context(),
		"UPDATE core.settings SET value = $1 WHERE key = 'minimum_version'", minimum); err != nil {
		t.Fatalf("storing the minimum version %s: %v", minimum, err)
	}
}

// storedMinimumVersionAt returns the minimum version the PostgreSQL database at address holds.
func storedMinimumVersionAt(t *testing.T, address string) string {
	t.Helper()
	minimum, err := postgres.StoredMinimumVersion(t.Context(), testdb.Open(t, address))
	if err != nil {
		t.Fatalf("StoredMinimumVersion() error = %v, want nil", err)
	}
	return minimum
}

func TestRunStoresTheMinimumVersionOfThisBinary(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL, "GOPHENBERG_ADDR": "localhost:0"}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel}, noPlugins); err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
	if got := storedMinimumVersionAt(t, databaseURL); got != version.Minimum() {
		t.Errorf("stored minimum version = %q after serving, want this binary's %q", got, version.Minimum())
	}
}

func TestTheCoreStepRecordsTheVersionThatMigrated(t *testing.T) {
	t.Parallel()

	address := emptySQLiteAddress(t)
	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": address})

	if got := testkit.Run(t, Program(env, noPlugins), "", "migrate"); got.Code != gonsole.ExitDone {
		t.Fatalf("migrate = %d with stderr %q, want 0", got.Code, got.Stderr)
	}

	db, err := openDefaultSQLite(address)
	if err != nil {
		t.Fatalf("opening the migrated file: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	got, err := sqlite.LastMigratedBy(t.Context(), db)
	if want := (sqlite.Build{Version: version.Version(), Commit: version.Commit()}); err != nil || got != want {
		t.Errorf("LastMigratedBy() = %+v, %v after migrate, want %+v", got, err, want)
	}
}

func TestRunStopsOnADatabaseANewerGophenbergMigrated(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	minimumVersionLeftAt(t, databaseURL, newerMinimumVersion)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL, "GOPHENBERG_ADDR": "localhost:0"}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel}, noPlugins)

	if err == nil || !strings.HasPrefix(err.Error(), "this database needs Gophenberg 99.0.0 or newer") {
		t.Fatalf("run() error = %v, want the refusal before the site listens", err)
	}
	if got := storedMinimumVersionAt(t, databaseURL); got != newerMinimumVersion {
		t.Errorf("stored minimum version = %q after the refusal, want %q kept", got, newerMinimumVersion)
	}
}
