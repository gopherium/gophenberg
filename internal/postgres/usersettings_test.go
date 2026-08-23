// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// newUserSettingStore returns a reader setting store over a migrated database and its pool.
func newUserSettingStore(t *testing.T) (*postgres.UserSettingStore, *pgxpool.Pool) {
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
	return postgres.NewUserSettingStore(pool), pool
}

func TestAReaderSettingReadsWhatItWrote(t *testing.T) {
	store, _ := newUserSettingStore(t)
	reader := uuid.New()

	if err := store.Save(t.Context(), reader, "locale", "es-ES"); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	held, found, err := store.Lookup(t.Context(), reader, "locale")

	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if !found || held != "es-ES" {
		t.Errorf("Lookup() = %q, %v, want the language the reader chose", held, found)
	}
}

func TestAReaderSettingCarriesTheLatestWrite(t *testing.T) {
	store, _ := newUserSettingStore(t)
	reader := uuid.New()
	if err := store.Save(t.Context(), reader, "locale", "es-ES"); err != nil {
		t.Fatalf("the first Save() error = %v, want nil", err)
	}

	if err := store.Save(t.Context(), reader, "locale", "fr-FR"); err != nil {
		t.Fatalf("the second Save() error = %v, want nil", err)
	}

	held, _, err := store.Lookup(t.Context(), reader, "locale")
	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if held != "fr-FR" {
		t.Errorf("Lookup() = %q, want the language written last", held)
	}
}

func TestAReaderSettingIsTheReadersAlone(t *testing.T) {
	store, _ := newUserSettingStore(t)
	mine, theirs := uuid.New(), uuid.New()
	if err := store.Save(t.Context(), mine, "locale", "es-ES"); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}

	_, found, err := store.Lookup(t.Context(), theirs, "locale")

	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if found {
		t.Error("another reader's choice was answered, want each reader answered its own")
	}
}

func TestAReaderSettingAnswersNothingForAKeyItNeverHeld(t *testing.T) {
	store, _ := newUserSettingStore(t)

	held, found, err := store.Lookup(t.Context(), uuid.New(), "never-written")

	if err != nil {
		t.Fatalf("Lookup() error = %v, want nil", err)
	}
	if found || held != "" {
		t.Errorf("Lookup() = %q, %v, want nothing stored", held, found)
	}
}

func TestAReaderSettingReportsADatabaseItCannotReach(t *testing.T) {
	store, pool := newUserSettingStore(t)
	pool.Close()

	if _, _, err := store.Lookup(t.Context(), uuid.New(), "locale"); err == nil {
		t.Error("Lookup() error = nil, want the closed pool reported")
	}
	if err := store.Save(t.Context(), uuid.New(), "locale", "es-ES"); err == nil {
		t.Error("Save() error = nil, want the closed pool reported")
	}
}
