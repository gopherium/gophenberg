// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/framework/pluginkit/pluginstoretest"
	"github.com/gopherium/framework/pluginkit/settings"
	"github.com/gopherium/framework/pluginkit/wire"

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/internal/pluginsettings"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/sdk"
)

// errDropped is the failure a rejecting store answers.
var errDropped = errors.New("connection dropped")

// rejectingStore is a plugin store in memory whose reads or writes fail when it is told.
type rejectingStore struct {
	*pluginstoretest.Memory
	reads, writes bool
}

// Records answers the records, or errDropped when reads fail.
func (s rejectingStore) Records(ctx context.Context) (map[string]pluginkit.PluginRecord, error) {
	if s.reads {
		return nil, errDropped
	}
	return s.Memory.Records(ctx)
}

// Activate activates id, or answers errDropped when writes fail.
func (s rejectingStore) Activate(ctx context.Context, id string) error {
	if s.writes {
		return errDropped
	}
	return s.Memory.Activate(ctx, id)
}

// Pause pauses id at step with reason, or answers errDropped when writes fail.
func (s rejectingStore) Pause(ctx context.Context, id string, step pluginkit.Step, reason string) error {
	if s.writes {
		return errDropped
	}
	return s.Memory.Pause(ctx, id, step, reason)
}

// bootOver returns a start over store and no database, its plugins lent no share.
func bootOver(store pluginkit.PluginStore) boot {
	return boot{
		settings: runConfig{
			pluginStartBudget: time.Second, pluginHookBudget: time.Second, pluginHookSaveBudget: time.Second,
			pluginHookTimedFirings: 20, serving: servingDefaults,
		},
		built: site{
			pluginRecords: store, roles: goncierge.New(), defaultRole: role.Member, lane: &pluginLane{sqlite: true},
			accounts:       accountbridge.New(nil, accountbridge.Options{}),
			pluginSettings: pluginsettings.New(nil, testkit.Getenv(nil), nil),
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

// handlerless is an idle plugin whose Routes answers no handler.
type handlerless struct {
	idle
}

// Routes answers no handler.
func (handlerless) Routes() http.Handler {
	return nil
}

// unlisted is an idle plugin that adds head tags without listing the hook.
type unlisted struct {
	idle
}

// HeadTags adds no tag.
func (unlisted) HeadTags(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
	return nil, nil
}

// activeRecord returns the record of the plugin id active with no error.
func activeRecord(id string) pluginkit.PluginRecord {
	return pluginkit.PluginRecord{ID: id, State: pluginkit.PluginActive}
}

func TestStartPluginsStopsWhenThePluginStoreFails(t *testing.T) {
	t.Parallel()

	paused := pluginkit.PluginRecord{ID: "notes", State: pluginkit.PluginPaused, LastError: "a setting is missing"}
	for name, asked := range map[string]struct {
		store   rejectingStore
		plugins registrations
		want    string
	}{
		"a read": {rejectingStore{Memory: pluginstoretest.NewMemory(), reads: true}, listOf(idle("notes")),
			"pluginkit: read the plugin records: connection dropped"},
		"a new plugin": {rejectingStore{Memory: pluginstoretest.NewMemory(), writes: true}, listOf(idle("notes")),
			"pluginkit: activate notes: connection dropped"},
		"a role pause": {
			rejectingStore{Memory: pluginstoretest.NewMemory(activeRecord("notes")), writes: true},
			listOf(declaringRole("notes", "archivist", "manage_archive")),
			"pluginkit: pause notes: connection dropped",
		},
		"a type pause": {
			rejectingStore{Memory: pluginstoretest.NewMemory(activeRecord("holding")), writes: true},
			listOf(refusingDeclarer{holdingPlugin: newHoldingPlugin(), err: errors.New("declare exploded")}),
			"pluginkit: pause holding: connection dropped",
		},
		"a routes pause": {
			rejectingStore{Memory: pluginstoretest.NewMemory(activeRecord("notes")), writes: true},
			listOf(handlerless{idle: "notes"}), "pluginkit: pause notes: connection dropped",
		},
		"a hooks pause": {
			rejectingStore{Memory: pluginstoretest.NewMemory(activeRecord("notes")), writes: true},
			listOf(unlisted{idle: "notes"}), "pluginkit: pause notes: connection dropped",
		},
		"a plugin that starts again": {rejectingStore{Memory: pluginstoretest.NewMemory(paused), writes: true},
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
		ID: "turncoat", State: pluginkit.PluginPaused, Step: pluginkit.StepRoles, LastError: "panicked: boom",
	}
	if err != nil || records["turncoat"] != want {
		t.Errorf("Records() = %+v (%v), want %+v", records, err, want)
	}
}

func TestStepOfKeepsTheWholeErrorTheLogPrintsBesideThePluginsOwn(t *testing.T) {
	t.Parallel()

	returned := stepOf("notes", stepHooks, func() error { return errors.New("lists public.head-tags twice") })
	panicked := stepOf("notes", pluginkit.StepRoles, func() error { panic("boom") })

	if got := fmt.Sprint(returned.Err); got != "plugin notes hooks: lists public.head-tags twice" {
		t.Errorf("a returned error reads %q, want the plugin id and the step in front of it", got)
	}
	if returned.Reason != "lists public.head-tags twice" {
		t.Errorf("a returned error keeps the reason %q, want the plugin's own error alone", returned.Reason)
	}
	if got := fmt.Sprint(panicked.Err); got != "plugin notes roles panicked: boom" || !panicked.Panicked {
		t.Errorf("a panic reads %q with Panicked %v, want the plugin id and the step in front of it", got,
			panicked.Panicked)
	}
	if panicked.Reason != "panicked: boom" {
		t.Errorf("a panic keeps the reason %q, want the panic's value alone", panicked.Reason)
	}
}

// driftingError is an error whose text changes each time it is read.
type driftingError struct {
	reads *int
}

// Error returns a text naming how many times the error was read.
func (e driftingError) Error() string {
	*e.reads++
	return fmt.Sprintf("read number %d", *e.reads)
}

func TestStepOfReadsThePluginsErrorOnceForTheLogAndTheRecord(t *testing.T) {
	t.Parallel()

	drifting := driftingError{reads: new(int)}

	failure := stepOf("notes", stepTypes, func() error { return drifting })

	if got := fmt.Sprint(failure.Err); got != "plugin notes types: "+failure.Reason {
		t.Errorf("the log reads %q and the record %q, want one read of the plugin's error in both", got, failure.Reason)
	}
	if !errors.Is(failure.Err, drifting) {
		t.Errorf("the error %v leaves out the plugin's own, want it kept for errors.Is", failure.Err)
	}
}

func TestStartPluginsStopsWhenTheHookSettingsAreRefused(t *testing.T) {
	t.Parallel()

	start := bootOver(pluginstoretest.NewMemory())
	start.settings.pluginHookBudget = 0

	started, err := start.startPlugins(t.Context(), listOf(idle("notes")))

	if err == nil || !strings.Contains(err.Error(), "the budget must stand above zero") {
		t.Errorf("startPlugins() = %v, want the hook settings refused", err)
	}
	if started.host == nil {
		t.Error("startPlugins() answered no host, want one a stop can reach")
	}
}

func TestStartPluginsBuildsTheHooksOverTheStartedPluginsOnly(t *testing.T) {
	t.Parallel()

	start := bootOver(pluginstoretest.NewMemory())
	start.listed = listedHooks{byPlugin: map[string][]string{
		"heads": {sdk.HeadTagsHook}, "fails": {sdk.HeadTagsHook},
	}}

	started, err := start.startPlugins(t.Context(), listOf(heads{idle: "heads"}, failingHeads{heads{idle: "fails"}}))

	if err != nil {
		t.Fatalf("startPlugins() = %v, want the site to start", err)
	}
	got := started.hooks.HeadTags(t.Context(), sdk.PageMeta{})
	want := []sdk.HeadTag{{Tag: "meta", Attrs: []sdk.Attr{{Key: "name", Value: "heads"}}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("HeadTags() = %+v, want the tags of the started plugin alone %+v", got, want)
	}
}

func TestStartPluginsPausesAPluginWhoseSettingsAreRejectedAtRegister(t *testing.T) {
	t.Parallel()

	store := pluginstoretest.NewMemory()
	start := bootOver(store)
	start.built.pluginSettings = pluginsettings.New(map[string][]wire.Setting{
		"notes": {{Key: "per_page", Type: settings.TypeCount}},
	}, testkit.Getenv(nil), nil)

	_, err := start.startPlugins(t.Context(), listOf(idle("notes"), idle("gallery")))

	if err != nil {
		t.Fatalf("startPlugins() = %v, want the other plugins started", err)
	}
	records, err := store.Records(t.Context())
	want := map[string]pluginkit.PluginRecord{
		"notes": {
			ID: "notes", State: pluginkit.PluginPaused, Step: pluginkit.StepRegister,
			LastError: "settings[0] per_page: a count needs a default",
		},
		"gallery": {ID: "gallery", State: pluginkit.PluginActive},
	}
	if err != nil || !reflect.DeepEqual(records, want) {
		t.Errorf("Records() = %+v (%v), want %+v", records, err, want)
	}
}

// heads is an idle plugin that adds a meta tag named after its id.
type heads struct {
	idle
}

// HeadTags adds a meta tag named after the plugin's id.
func (h heads) HeadTags(context.Context, sdk.PageMeta) ([]sdk.HeadTag, error) {
	return []sdk.HeadTag{{Tag: "meta", Attrs: []sdk.Attr{{Key: "name", Value: h.ID()}}}}, nil
}

// failingHeads is a plugin that adds head tags and fails to start.
type failingHeads struct {
	heads
}

// Start fails.
func (failingHeads) Start(context.Context) error {
	return errors.New("no route table")
}

func TestDeclarePluginsRefusesACoreTableTheRegistryRefuses(t *testing.T) {
	t.Parallel()

	start := bootOver(rejectingStore{Memory: pluginstoretest.NewMemory()})

	_, err := start.declarePlugins(t.Context(), &pausing{}, []goncierge.Role{{Name: "Admin"}}, nil)

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
