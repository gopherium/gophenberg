// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// pluginCall is one call of the plugin store with the statement it runs and the text its failure starts with.
type pluginCall struct {
	name, statement, prefix string
	call                    func(ctx context.Context, store *sqlite.PluginStore) error
}

// pluginWrites are the writes of the plugin store, each on the plugin events.
var pluginWrites = []pluginCall{
	{"an activation names its plugin", db.ActivatePlugin, `sqlite: activating the plugin "events": `,
		func(ctx context.Context, store *sqlite.PluginStore) error {
			return store.Activate(ctx, "events")
		}},
	{"a pause names its plugin", db.PausePlugin, `sqlite: pausing the plugin "events": `,
		func(ctx context.Context, store *sqlite.PluginStore) error {
			return store.Pause(ctx, "events", "a setting is missing")
		}},
}

func TestPluginStoreReportsAStatementThatFails(t *testing.T) {
	t.Parallel()

	read := pluginCall{"a read names the plugin records", db.ListPlugins, "sqlite: reading the plugin records: ",
		func(ctx context.Context, store *sqlite.PluginStore) error {
			_, err := store.Records(ctx)
			return err
		}}
	for _, tc := range append([]pluginCall{read}, pluginWrites...) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handle, faults := faultyHandle(t)
			faults.FailStatement(tc.statement, errPlanted)

			err := tc.call(t.Context(), sqlite.NewPluginStore(handle))

			wantPlanted(t, err, tc.prefix)
		})
	}
}

func TestPluginWritesBehindAnotherWriterAnswerDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	for _, tc := range pluginWrites {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			handle := impatientHandle(t)
			holdWrites(t, handle)

			err := tc.call(t.Context(), sqlite.NewPluginStore(handle))

			if !errors.Is(err, dbkit.ErrBusy) || !strings.HasPrefix(fmt.Sprint(err), tc.prefix) {
				t.Errorf("the write behind another writer = %v, want dbkit.ErrBusy under %q", err, tc.prefix)
			}
		})
	}
}

func TestPluginRecordsBehindAnExclusiveConnectionAnswerDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	handle := impatientHandle(t)
	store := sqlite.NewPluginStore(handle)
	if err := store.Activate(t.Context(), "events"); err != nil {
		t.Fatalf("Activate() error = %v, want the record added", err)
	}
	owner := otherWriter(t, handle)
	for _, statement := range []string{"PRAGMA locking_mode=EXCLUSIVE", "BEGIN IMMEDIATE",
		"UPDATE core_plugins SET last_error = 'held'", "COMMIT"} {
		if _, err := owner.ExecContext(t.Context(), statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}

	_, err := store.Records(t.Context())

	named := strings.HasPrefix(fmt.Sprint(err), "sqlite: reading the plugin records: ")
	if !errors.Is(err, dbkit.ErrBusy) || !named {
		t.Errorf("Records() behind an exclusive connection = %v, want dbkit.ErrBusy under the read", err)
	}
}
