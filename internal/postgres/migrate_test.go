// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/peterldowns/pgtestdb"
	"github.com/pressly/goose/v3/lock"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// schemaExists reports whether the named schema is present in db.
func schemaExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var found bool
	err := db.QueryRow(
		"SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)", name,
	).Scan(&found)
	if err != nil {
		t.Fatalf("querying schemata: %v", err)
	}
	return found
}

func TestMigrateCreatesCoreSchema(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}

	databaseURL := testdb.New(t, pgtestdb.NoopMigrator{})
	if err := authkitpg.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("auth Migrate() error = %v, want nil", err)
	}
	db := testdb.Open(t, databaseURL)

	if err := postgres.Migrate(t.Context(), db); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}
	if err := postgres.Migrate(t.Context(), db); err != nil {
		t.Fatalf("second Migrate() error = %v, want idempotent nil", err)
	}

	if !schemaExists(t, db, "core") {
		t.Fatal("core schema not found after Migrate()")
	}
}

func TestMigrateRollsBackAndReapplies(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}

	databaseURL := testdb.New(t, pgtestdb.NoopMigrator{})
	if err := authkitpg.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("auth Migrate() error = %v, want nil", err)
	}
	db := testdb.Open(t, databaseURL)
	if err := postgres.Migrate(t.Context(), db); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}

	if _, err := coreProvider(t, db).DownTo(t.Context(), 0); err != nil {
		t.Fatalf("DownTo(0) error = %v, want a clean rollback", err)
	}

	if schemaExists(t, db, "core") {
		t.Error("core schema still present after rollback, want it dropped")
	}
	if !schemaExists(t, db, "auth") {
		t.Error("auth schema missing after core rollback, want it untouched")
	}
	if err := postgres.Migrate(t.Context(), db); err != nil {
		t.Fatalf("Migrate() after rollback error = %v, want a clean reapply", err)
	}
	if !schemaExists(t, db, "core") {
		t.Error("core schema not found after reapply")
	}
}

// accountsMigrated returns the address of a fresh database holding the auth schema the core schema needs.
func accountsMigrated(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	databaseURL := testdb.New(t, pgtestdb.NoopMigrator{})
	if err := authkitpg.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("auth Migrate() error = %v, want nil", err)
	}
	return databaseURL
}

func TestMigrateWaitsForTheMigrationLock(t *testing.T) {
	t.Parallel()

	databaseURL := accountsMigrated(t)
	db := testdb.Open(t, databaseURL)
	holder, err := db.Conn(t.Context())
	if err != nil {
		t.Fatalf("taking a connection: %v", err)
	}
	defer func() { _ = holder.Close() }()
	if _, err := holder.ExecContext(t.Context(), "SELECT pg_advisory_lock($1)", lock.DefaultLockID); err != nil {
		t.Fatalf("holding the migration lock: %v", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
	defer cancel()

	err = postgres.Migrate(ctx, testdb.Open(t, databaseURL))

	if created := schemaExists(t, db, "core"); !errors.Is(err, context.DeadlineExceeded) || created {
		t.Errorf("Migrate() = %v with the core schema created %v, want the deadline and nothing applied", err, created)
	}
}

func TestMigrateLetsRunsAtOnceAllSucceed(t *testing.T) {
	t.Parallel()

	databaseURL := accountsMigrated(t)
	failures := make(chan error, 4)
	var runs sync.WaitGroup
	for range 4 {
		db := testdb.Open(t, databaseURL)
		runs.Go(func() { failures <- postgres.Migrate(t.Context(), db) })
	}
	runs.Wait()
	close(failures)

	for err := range failures {
		if err != nil {
			t.Errorf("Migrate() error = %v, want every run to succeed", err)
		}
	}
}

func TestMigrateBehindALockedVersionTableAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	url := migratedAddress(t)
	holdInRival(t, url, "LOCK TABLE goose_db_version IN ACCESS EXCLUSIVE MODE")

	err := postgres.Migrate(t.Context(), lockTimedHandle(t, url))

	if !errors.Is(err, dbkit.ErrBusy) || !strings.HasPrefix(fmt.Sprint(err), "postgres: apply migrations: ") {
		t.Errorf("Migrate() behind a locked version table error = %v, want dbkit.ErrBusy under the migrations", err)
	}
}

func TestMigrateReportsUnreachableDatabase(t *testing.T) {
	t.Parallel()

	err := postgres.Migrate(
		t.Context(),
		testdb.Open(t, "postgres://postgres:gophenberg@localhost:9/postgres?sslmode=disable&connect_timeout=1"),
	)

	if err == nil {
		t.Fatal("Migrate() error = nil, want a connection error")
	}
}
