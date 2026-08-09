// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// newSettingStore returns a store over a migrated database.
func newSettingStore(t *testing.T) *postgres.SettingStore {
	t.Helper()
	store, _ := newSettingStoreWithPool(t)
	return store
}

// newSettingStoreWithPool returns a store over a migrated database and its pool.
func newSettingStoreWithPool(t *testing.T) (*postgres.SettingStore, *pgxpool.Pool) {
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
	return postgres.NewSettingStore(pool), pool
}

func TestSettingReadsWhatItWrote(t *testing.T) {
	store := newSettingStore(t)

	if err := store.Set(t.Context(), "theme.active", "aurora"); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := store.Get(t.Context(), "theme.active")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "aurora" {
		t.Errorf("got %q, want the stored value", got)
	}
}

func TestSettingReadsAnUnsetKeyAsEmpty(t *testing.T) {
	store := newSettingStore(t)

	got, err := store.Get(t.Context(), "theme.active")

	if err != nil {
		t.Fatalf("want an unset key to read as empty, got %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSettingOverwritesAnExistingKey(t *testing.T) {
	store := newSettingStore(t)
	if err := store.Set(t.Context(), "theme.active", "driftwood"); err != nil {
		t.Fatalf("set the first value: %v", err)
	}

	if err := store.Set(t.Context(), "theme.active", "aurora"); err != nil {
		t.Fatalf("set the second value: %v", err)
	}

	got, err := store.Get(t.Context(), "theme.active")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "aurora" {
		t.Errorf("got %q, want the newer value", got)
	}
}

func TestSettingKeepsKeysApart(t *testing.T) {
	store := newSettingStore(t)
	if err := store.Set(t.Context(), "theme.active", "aurora"); err != nil {
		t.Fatalf("set the active theme: %v", err)
	}

	if err := store.Set(t.Context(), "theme.previous", "driftwood"); err != nil {
		t.Fatalf("set the previous theme: %v", err)
	}

	active, err := store.Get(t.Context(), "theme.active")
	if err != nil {
		t.Fatalf("get the active theme: %v", err)
	}
	if active != "aurora" {
		t.Errorf("got %q, want the other key untouched", active)
	}
}

func TestSettingReportsDatabaseFailures(t *testing.T) {
	store, pool := newSettingStoreWithPool(t)
	pool.Close()

	if _, err := store.Get(t.Context(), "theme.active"); err == nil {
		t.Error("Get() on a closed pool error = nil, want a failure")
	}
	if err := store.Set(t.Context(), "theme.active", "aurora"); err == nil {
		t.Error("Set() on a closed pool error = nil, want a failure")
	}
}

func TestSettingClearsAKey(t *testing.T) {
	store := newSettingStore(t)
	if err := store.Set(t.Context(), "theme.active", "aurora"); err != nil {
		t.Fatalf("set: %v", err)
	}

	if err := store.Set(t.Context(), "theme.active", ""); err != nil {
		t.Fatalf("clear: %v", err)
	}

	got, err := store.Get(t.Context(), "theme.active")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != "" {
		t.Errorf("got %q, want the key cleared", got)
	}
}
