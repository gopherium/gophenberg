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

// PluginStore keeps the state and last failure of each plugin in the core tables of the site's SQLite file.
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
			ID: row.ID, State: pluginkit.PluginState(row.State), LastError: row.LastError,
		}
	}
	return records, nil
}

// Activate marks the plugin id active and clears its error, adding its record when it has none.
func (s *PluginStore) Activate(ctx context.Context, id string) error {
	if err := s.queries.ActivatePlugin(ctx, id); err != nil {
		return failed(fmt.Sprintf("activating the plugin %q", id), err)
	}
	return nil
}

// Pause marks the plugin id paused with the text of its failure, adding its record when it has none.
func (s *PluginStore) Pause(ctx context.Context, id, failure string) error {
	if err := s.queries.PausePlugin(ctx, db.PausePluginParams{ID: id, LastError: failure}); err != nil {
		return failed(fmt.Sprintf("pausing the plugin %q", id), err)
	}
	return nil
}
