// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"sync"
	"testing"
	"time"

	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/peterldowns/pgtestdb"
	"github.com/pressly/goose/v3"
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

	cfg := pgtestdb.Custom(t, testdb.Config(), pgtestdb.NoopMigrator{})
	if err := authkitpg.Migrate(t.Context(), cfg.URL()); err != nil {
		t.Fatalf("auth Migrate() error = %v, want nil", err)
	}

	if err := postgres.Migrate(t.Context(), cfg.URL()); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}
	if err := postgres.Migrate(t.Context(), cfg.URL()); err != nil {
		t.Fatalf("second Migrate() error = %v, want idempotent nil", err)
	}

	db, err := sql.Open("pgx", cfg.URL())
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()
	if !schemaExists(t, db, "core") {
		t.Fatal("core schema not found after Migrate()")
	}
}

func TestMigrateRollsBackAndReapplies(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}

	cfg := pgtestdb.Custom(t, testdb.Config(), pgtestdb.NoopMigrator{})
	if err := authkitpg.Migrate(t.Context(), cfg.URL()); err != nil {
		t.Fatalf("auth Migrate() error = %v, want nil", err)
	}
	if err := postgres.Migrate(t.Context(), cfg.URL()); err != nil {
		t.Fatalf("Migrate() error = %v, want nil", err)
	}
	db, err := sql.Open("pgx", cfg.URL())
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()
	source, err := fs.Sub(postgres.Migrations, "migrations")
	if err != nil {
		t.Fatalf("migrations subtree: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, source)
	if err != nil {
		t.Fatalf("goose provider: %v", err)
	}

	if _, err := provider.DownTo(t.Context(), 0); err != nil {
		t.Fatalf("DownTo(0) error = %v, want a clean rollback", err)
	}

	if schemaExists(t, db, "core") {
		t.Error("core schema still present after rollback, want it dropped")
	}
	if !schemaExists(t, db, "auth") {
		t.Error("auth schema missing after core rollback, want it untouched")
	}
	if err := postgres.Migrate(t.Context(), cfg.URL()); err != nil {
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
	cfg := pgtestdb.Custom(t, testdb.Config(), pgtestdb.NoopMigrator{})
	if err := authkitpg.Migrate(t.Context(), cfg.URL()); err != nil {
		t.Fatalf("auth Migrate() error = %v, want nil", err)
	}
	return cfg.URL()
}

func TestMigrateWaitsForTheMigrationLock(t *testing.T) {
	t.Parallel()

	databaseURL := accountsMigrated(t)
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()
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

	err = postgres.Migrate(ctx, databaseURL)

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
		runs.Go(func() { failures <- postgres.Migrate(t.Context(), databaseURL) })
	}
	runs.Wait()
	close(failures)

	for err := range failures {
		if err != nil {
			t.Errorf("Migrate() error = %v, want every run to succeed", err)
		}
	}
}

func TestMigrateRejectsMalformedURL(t *testing.T) {
	t.Parallel()

	if err := postgres.Migrate(t.Context(), "://not-a-url"); err == nil {
		t.Fatal("Migrate() error = nil, want a parse error")
	}
}

func TestMigrateReportsUnreachableDatabase(t *testing.T) {
	t.Parallel()

	err := postgres.Migrate(
		t.Context(),
		"postgres://postgres:gophenberg@localhost:9/postgres?sslmode=disable&connect_timeout=1",
	)

	if err == nil {
		t.Fatal("Migrate() error = nil, want a connection error")
	}
}
