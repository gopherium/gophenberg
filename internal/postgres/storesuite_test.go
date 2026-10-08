// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// migratedPool returns a pool over a fresh migrated database, skipping the test in short mode.
func migratedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	cfg := pgtestdb.Custom(t, testdb.Config(), testdb.Migrator())
	pool, err := pgxpool.New(t.Context(), cfg.URL())
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// postgresStores returns fresh stores over a migrated database of their own.
func postgresStores(t *testing.T) contenttest.Stores {
	t.Helper()
	pool := migratedPool(t)
	return contenttest.Stores{
		Content: postgres.NewContentStore(pool),
		Types:   postgres.NewTypeStore(pool),
		AddAuthor: func(t *testing.T, name string) uuid.UUID {
			t.Helper()
			return addAuthor(t, pool, name)
		},
	}
}

func TestContentStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.Run(t, postgresStores)
}

func TestTypeStoreSuite(t *testing.T) {
	t.Parallel()

	contenttest.RunTypes(t, postgresStores)
}
