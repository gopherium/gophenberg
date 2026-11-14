// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres"
)

// errChange is the failure a test's change answers with.
var errChange = errors.New("change failure")

// newSettingStoreWithPool returns a store over a migrated database and its pool.
func newSettingStoreWithPool(t *testing.T) (*postgres.SettingStore, *pgxpool.Pool) {
	t.Helper()
	pool := migratedPool(t)
	return postgres.NewSettingStore(pool), pool
}

// counted adds one to the count held, starting from none.
func counted(held map[string]string) (map[string]string, error) {
	count, _ := strconv.Atoi(held["count"])
	return map[string]string{"count": strconv.Itoa(count + 1)}, nil
}

// lockTimedSettings returns a setting store whose connections give up quickly on a held lock.
func lockTimedSettings(t *testing.T, pool *pgxpool.Pool) *postgres.SettingStore {
	t.Helper()
	return postgres.NewSettingStore(lockTimedPool(t, pool.Config().ConnString()))
}

// holdSettingsLock holds the settings update lock until the test ends.
func holdSettingsLock(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatalf("acquiring a connection: %v", err)
	}
	if _, err := conn.Exec(t.Context(), "SELECT pg_advisory_lock(hashtext('core.settings'))"); err != nil {
		conn.Release()
		t.Fatalf("holding the lock: %v", err)
	}
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_unlock(hashtext('core.settings'))"); err != nil {
			t.Errorf("releasing the lock: %v", err)
		}
		conn.Release()
	})
}

func TestSettingReportsDatabaseFailures(t *testing.T) {
	store, pool := newSettingStoreWithPool(t)
	pool.Close()

	if _, _, err := store.Lookup(t.Context(), "theme.active"); err == nil {
		t.Error("Lookup() on a closed pool error = nil, want a failure")
	}
	if err := store.Save(t.Context(), map[string]string{"theme.active": "aurora"}); err == nil {
		t.Error("Save() on a closed pool error = nil, want a failure")
	}
	if err := store.Update(t.Context(), []string{"count"}, counted); err == nil {
		t.Error("Update() on a closed pool error = nil, want a failure")
	}
}

func TestSettingUpdatesRunOneAfterAnother(t *testing.T) {
	t.Parallel()

	store, _ := newSettingStoreWithPool(t)
	var group sync.WaitGroup
	for range 20 {
		group.Go(func() {
			if err := store.Update(t.Context(), []string{"count"}, counted); err != nil {
				t.Errorf("Update() error = %v, want nil", err)
			}
		})
	}
	group.Wait()

	if got, _, err := store.Lookup(t.Context(), "count"); err != nil || got != "20" {
		t.Errorf("count after 20 updates at once = %q, %v, want 20", got, err)
	}
}

func TestSettingUpdateChangesTheStoredValuesOfItsKeys(t *testing.T) {
	t.Parallel()

	store, _ := newSettingStoreWithPool(t)
	if err := store.Save(t.Context(), map[string]string{"a": "1", "b": "2", "c": "3"}); err != nil {
		t.Fatalf("Save() error = %v, want nil", err)
	}
	var seen map[string]string
	change := func(held map[string]string) (map[string]string, error) {
		seen = held
		return map[string]string{"a": "10"}, nil
	}

	err := store.Update(t.Context(), []string{"b", "a", "absent"}, change)

	if want := map[string]string{"a": "1", "b": "2"}; err != nil || !maps.Equal(seen, want) {
		t.Errorf("Update() = %v with %v held, want nil with %v", err, seen, want)
	}
	for key, want := range map[string]string{"a": "10", "b": "2", "c": "3"} {
		if got, _, _ := store.Lookup(t.Context(), key); got != want {
			t.Errorf("%s = %q after the update, want %q", key, got, want)
		}
	}
}

func TestSettingUpdateStoresNothingWhenTheChangeFails(t *testing.T) {
	t.Parallel()

	store, _ := newSettingStoreWithPool(t)

	err := store.Update(t.Context(), []string{"count"}, func(map[string]string) (map[string]string, error) {
		return map[string]string{"count": "1"}, errChange
	})

	if !errors.Is(err, errChange) {
		t.Errorf("Update() error = %v, want the change's failure", err)
	}
	if _, found, _ := store.Lookup(t.Context(), "count"); found {
		t.Error("count is stored after a failed change, want nothing stored")
	}
}

func TestSettingUpdateReportsALockItCannotTake(t *testing.T) {
	t.Parallel()

	_, pool := newSettingStoreWithPool(t)
	holdSettingsLock(t, pool)

	if err := lockTimedSettings(t, pool).Update(t.Context(), []string{"count"}, counted); !errors.Is(err, dbkit.ErrBusy) {
		t.Errorf("Update() behind a held lock error = %v, want the lock failure as dbkit.ErrBusy", err)
	}
}

func TestSettingLookupBehindALockedTableAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	_, pool := newSettingStoreWithPool(t)
	holdInRival(t, pool.Config().ConnString(), "LOCK TABLE core.settings IN ACCESS EXCLUSIVE MODE")

	_, _, err := lockTimedSettings(t, pool).Lookup(t.Context(), "theme.active")

	if !errors.Is(err, dbkit.ErrBusy) ||
		!strings.HasPrefix(fmt.Sprint(err), `postgres: reading setting "theme.active": `) {
		t.Errorf("Lookup() behind a locked table error = %v, want dbkit.ErrBusy under the read", err)
	}
}

func TestSettingSaveWhoseCommitLosesItsSerializationAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	store, pool := newSettingStoreWithPool(t)
	loseSerializationAtCommit(t, pool, "core.settings")

	err := store.Save(t.Context(), map[string]string{"theme.active": "aurora"})

	if !errors.Is(err, dbkit.ErrBusy) || !strings.HasPrefix(fmt.Sprint(err), "postgres: writing settings: ") {
		t.Errorf("Save() whose commit loses its serialization error = %v, want dbkit.ErrBusy under the write", err)
	}
}

func TestSettingUpdateReportsValuesItCannotRead(t *testing.T) {
	t.Parallel()

	store, pool := newSettingStoreWithPool(t)
	sabotage(t, pool, "ALTER TABLE core.settings RENAME COLUMN value TO retired")

	if err := store.Update(t.Context(), []string{"count"}, counted); err == nil {
		t.Error("Update() over an unreadable table error = nil, want the read failure")
	}
}
