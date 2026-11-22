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

// PluginStore keeps the state of each plugin, and the step and error of a pause, in the core schema.
type PluginStore struct {
	queries *db.Queries
}

// NewPluginStore returns a [PluginStore] backed by pool.
func NewPluginStore(pool *pgxpool.Pool) *PluginStore {
	return &PluginStore{queries: db.New(classified{conn: pool})}
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
			ID: row.ID, State: pluginkit.PluginState(row.State), Step: pluginkit.Step(row.Step), LastError: row.LastError,
		}
	}
	return records, nil
}

// Activate marks the plugin id active and clears its step and its error, adding its record when it has none.
func (s *PluginStore) Activate(ctx context.Context, id string) error {
	if err := s.queries.ActivatePlugin(ctx, id); err != nil {
		return fmt.Errorf("postgres: activating the plugin %q: %w", id, err)
	}
	return nil
}

// Pause marks the plugin id paused at step with reason, its own error, adding its record when it has none.
func (s *PluginStore) Pause(ctx context.Context, id string, step pluginkit.Step, reason string) error {
	paused := db.PausePluginParams{ID: id, LastError: reason, Step: string(step)}
	if err := s.queries.PausePlugin(ctx, paused); err != nil {
		return fmt.Errorf("postgres: pausing the plugin %q: %w", id, err)
	}
	return nil
}
