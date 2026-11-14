// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// newUserSettingStore returns a reader setting store over a migrated database and its pool.
func newUserSettingStore(t *testing.T) (*postgres.UserSettingStore, *pgxpool.Pool) {
	t.Helper()
	pool := migratedPool(t)
	return postgres.NewUserSettingStore(pool), pool
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

func TestAReaderSettingSaveBehindAHeldSettingAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	store, pool := newUserSettingStore(t)
	reader := addAuthor(t, pool, contenttest.DefaultAuthor)
	if err := store.Save(t.Context(), reader, "locale", "es-ES"); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}
	holdInRival(t, pool.Config().ConnString(), "SELECT key FROM core.user_settings FOR UPDATE")
	timed := postgres.NewUserSettingStore(lockTimedPool(t, pool.Config().ConnString()))

	err := timed.Save(t.Context(), reader, "locale", "fr-FR")

	if !errors.Is(err, dbkit.ErrBusy) || !strings.HasPrefix(fmt.Sprint(err), `postgres: writing the setting "locale": `) {
		t.Errorf("Save() behind a held setting error = %v, want dbkit.ErrBusy under the write", err)
	}
}
