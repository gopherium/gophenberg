// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"fmt"

	"github.com/gopherium/framework/pluginkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// PluginStore meets the plugin record store contract.
var _ pluginkit.PluginStore = (*PluginStore)(nil)

// PluginStore keeps the state and last failure of each plugin in the core schema.
type PluginStore struct {
	queries *db.Queries
}

// NewPluginStore returns a [PluginStore] backed by pool.
func NewPluginStore(pool *pgxpool.Pool) *PluginStore {
	return &PluginStore{queries: db.New(pool)}
}

// Records returns every plugin record, keyed by plugin id, in one read.
func (s *PluginStore) Records(ctx context.Context) (map[string]pluginkit.PluginRecord, error) {
	rows, err := s.queries.ListPlugins(ctx)
	if err != nil {
		return nil, fmt.Errorf("postgres: reading the plugin records: %w", err)
	}
	records := make(map[string]pluginkit.PluginRecord, len(rows))
	for _, row := range rows {
		records[row.ID] = pluginkit.PluginRecord{
			ID: row.ID, State: pluginkit.PluginState(row.State), LastError: row.LastError,
		}
	}
	return records, nil
}

// Activate marks the plugin id active and clears its error, adding its record when it has none.
func (s *PluginStore) Activate(ctx context.Context, id string) error {
	if err := s.queries.ActivatePlugin(ctx, id); err != nil {
		return fmt.Errorf("postgres: activating the plugin %q: %w", id, err)
	}
	return nil
}

// Pause marks the plugin id paused with the text of its failure, adding its record when it has none.
func (s *PluginStore) Pause(ctx context.Context, id, failure string) error {
	if err := s.queries.PausePlugin(ctx, db.PausePluginParams{ID: id, LastError: failure}); err != nil {
		return fmt.Errorf("postgres: pausing the plugin %q: %w", id, err)
	}
	return nil
}
