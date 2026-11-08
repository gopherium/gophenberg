// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres"
)

// pluginStoreWithPool returns a plugin store over a fresh migrated database and the pool behind it.
func pluginStoreWithPool(t *testing.T) (*postgres.PluginStore, *pgxpool.Pool) {
	t.Helper()
	pool := migratedPool(t)
	return postgres.NewPluginStore(pool), pool
}

func TestPluginStoreReportsRecordsItCannotRead(t *testing.T) {
	t.Parallel()

	store, pool := pluginStoreWithPool(t)
	sabotage(t, pool, "ALTER TABLE core.plugins RENAME TO plugins_gone")

	_, err := store.Records(t.Context())
	if err == nil || !strings.Contains(err.Error(), "reading the plugin records") {
		t.Errorf("Records() error = %v, want the failed read named", err)
	}
}

func TestPluginStoreReportsAnActivationItCannotWrite(t *testing.T) {
	t.Parallel()

	store, pool := pluginStoreWithPool(t)
	sabotage(t, pool, "ALTER TABLE core.plugins RENAME TO plugins_gone")

	err := store.Activate(t.Context(), "events")
	if err == nil || !strings.Contains(err.Error(), `activating the plugin "events"`) {
		t.Errorf("Activate() error = %v, want the failed activation named", err)
	}
}

func TestPluginStoreReportsAPauseItCannotWrite(t *testing.T) {
	t.Parallel()

	store, pool := pluginStoreWithPool(t)
	sabotage(t, pool, "ALTER TABLE core.plugins RENAME TO plugins_gone")

	err := store.Pause(t.Context(), "events", "a setting is missing")
	if err == nil || !strings.Contains(err.Error(), `pausing the plugin "events"`) {
		t.Errorf("Pause() error = %v, want the failed pause named", err)
	}
}

func TestPluginMigrationRefusesAnUnknownState(t *testing.T) {
	t.Parallel()

	_, pool := pluginStoreWithPool(t)

	_, err := pool.Exec(t.Context(), "INSERT INTO core.plugins (id, state) VALUES ('events', 'broken')")
	if err == nil || !strings.Contains(err.Error(), "plugins_state_check") {
		t.Errorf("a plugin in an unknown state was stored with error %v, want the state check to refuse it", err)
	}
}
