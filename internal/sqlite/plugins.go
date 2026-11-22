// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// PluginStore meets the plugin record store contract.
var _ pluginkit.PluginStore = (*PluginStore)(nil)

// PluginStore keeps the state of each plugin, and the step and error of a pause, in the site's SQLite file.
type PluginStore struct {
	queries *db.Queries
}

// NewPluginStore returns a [PluginStore] on the site's one SQLite handle.
func NewPluginStore(handle *sql.DB) *PluginStore {
	return &PluginStore{queries: db.New(handle)}
}

// Records returns every plugin record, keyed by plugin id, in one read.
func (s *PluginStore) Records(ctx context.Context) (map[string]pluginkit.PluginRecord, error) {
	rows, err := s.queries.ListPlugins(ctx)
	if err != nil {
		return nil, failed("reading the plugin records", err)
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
		return failed(fmt.Sprintf("activating the plugin %q", id), err)
	}
	return nil
}

// Pause marks the plugin id paused at step with reason, its own error, adding its record when it has none.
func (s *PluginStore) Pause(ctx context.Context, id string, step pluginkit.Step, reason string) error {
	paused := db.PausePluginParams{ID: id, LastError: reason, Step: string(step)}
	if err := s.queries.PausePlugin(ctx, paused); err != nil {
		return failed(fmt.Sprintf("pausing the plugin %q", id), err)
	}
	return nil
}
