// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/framework/pluginkit/pluginstoretest"

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/sdk"
)

// errDropped is the failure a refusing store answers.
var errDropped = errors.New("connection dropped")

// refusingStore is a plugin store in memory whose reads or writes fail when it is told.
type refusingStore struct {
	*pluginstoretest.Memory
	reads, writes bool
}

// Records answers the records, or errDropped when reads fail.
func (s refusingStore) Records(ctx context.Context) (map[string]pluginkit.PluginRecord, error) {
	if s.reads {
		return nil, errDropped
	}
	return s.Memory.Records(ctx)
}

// Activate activates id, or answers errDropped when writes fail.
func (s refusingStore) Activate(ctx context.Context, id string) error {
	if s.writes {
		return errDropped
	}
	return s.Memory.Activate(ctx, id)
}

// Pause pauses id, or answers errDropped when writes fail.
func (s refusingStore) Pause(ctx context.Context, id, failure string) error {
	if s.writes {
		return errDropped
	}
	return s.Memory.Pause(ctx, id, failure)
}

// bootOver returns a start over store and no database, its plugins lent no share.
func bootOver(store pluginkit.PluginStore) boot {
	return boot{
		settings: runConfig{pluginStartBudget: time.Second, serving: servingDefaults},
		built: site{
			pluginRecords: store, roles: goncierge.New(), defaultRole: role.Member, lane: &pluginLane{sqlite: true},
			accounts: accountbridge.New(nil, accountbridge.Options{}),
		},
		logger: testLogger(io.Discard),
	}
}

// turncoat is a plugin whose Roles breaks its ID and then panics.
type turncoat struct {
	broken bool
}

// ID returns the plugin's id, panicking once Roles broke it.
func (t *turncoat) ID() string {
	if t.broken {
		panic("the id is gone")
	}
	return "turncoat"
}

// Start starts nothing.
func (*turncoat) Start(context.Context) error {
	return nil
}

// Stop stops nothing.
func (*turncoat) Stop(context.Context) error {
	return nil
}

// Roles breaks the plugin's ID, then panics.
func (t *turncoat) Roles() []sdk.RoleDeclaration {
	t.broken = true
	panic("boom")
}

// activeRecord returns the record of the plugin id active with no error.
func activeRecord(id string) pluginkit.PluginRecord {
	return pluginkit.PluginRecord{ID: id, State: pluginkit.PluginActive}
}

func TestStartPluginsStopsWhenThePluginStoreFails(t *testing.T) {
	t.Parallel()

	paused := pluginkit.PluginRecord{ID: "notes", State: pluginkit.PluginPaused, LastError: "a setting is missing"}
	for name, asked := range map[string]struct {
		store   refusingStore
		plugins registrations
		want    string
	}{
		"a read": {refusingStore{Memory: pluginstoretest.NewMemory(), reads: true}, listOf(idle("notes")),
			"pluginkit: read the plugin records: connection dropped"},
		"a new plugin": {refusingStore{Memory: pluginstoretest.NewMemory(), writes: true}, listOf(idle("notes")),
			"pluginkit: activate notes: connection dropped"},
		"a role pause": {
			refusingStore{Memory: pluginstoretest.NewMemory(activeRecord("notes")), writes: true},
			listOf(declaringRole("notes", "archivist", "manage_archive")),
			"pluginkit: pause notes: connection dropped",
		},
		"a type pause": {
			refusingStore{Memory: pluginstoretest.NewMemory(activeRecord("holding")), writes: true},
			listOf(refusingDeclarer{holdingPlugin: newHoldingPlugin(), err: errors.New("declare exploded")}),
			"pluginkit: pause holding: connection dropped",
		},
		"a plugin that starts again": {refusingStore{Memory: pluginstoretest.NewMemory(paused), writes: true},
			listOf(idle("notes")), "pluginkit: activate notes: connection dropped"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			started, err := bootOver(asked.store).startPlugins(t.Context(), asked.plugins)

			if fmt.Sprint(err) != asked.want || !errors.Is(err, errDropped) {
				t.Errorf("startPlugins() = %v, want %q", err, asked.want)
			}
			if started.host == nil {
				t.Error("startPlugins() answered no host, want one a stop can reach")
			}
		})
	}
}

func TestStartPluginsPausesAPluginUnderTheIDItHadBeforeItsStepPanicked(t *testing.T) {
	t.Parallel()
	store := pluginstoretest.NewMemory()

	_, err := bootOver(store).startPlugins(t.Context(), listOf(&turncoat{}))

	if err != nil {
		t.Fatalf("startPlugins() = %v, want the site to start", err)
	}
	records, err := store.Records(t.Context())
	want := pluginkit.PluginRecord{
		ID: "turncoat", State: pluginkit.PluginPaused, LastError: "plugin turncoat roles panicked: boom",
	}
	if err != nil || records["turncoat"] != want {
		t.Errorf("Records() = %+v (%v), want %+v", records, err, want)
	}
}

func TestDeclarePluginsRefusesACoreTableTheRegistryRefuses(t *testing.T) {
	t.Parallel()

	start := bootOver(refusingStore{Memory: pluginstoretest.NewMemory()})

	_, _, err := start.declarePlugins(t.Context(), &pausing{}, []goncierge.Role{{Name: "Admin"}}, nil)

	if !errors.Is(err, goncierge.ErrInvalidName) {
		t.Errorf("declarePlugins() = %v, want the registry's name error", err)
	}
}

func TestServeStopsWhenThePoolLeavesTheStartedPluginsNoConnection(t *testing.T) {
	t.Parallel()

	getenv := capped(map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t), "GOPHENBERG_PLUGIN_DB_RESERVE": "3",
	}, 2)

	err := run(t.Context(), getenv, io.Discard, lending(&lender{id: "notes"}))

	want := "GOPHENBERG_PLUGIN_DB_RESERVE=3 leaves plugins no connection in a pool of 2"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("run() = %v, want %q", err, want)
	}
}
