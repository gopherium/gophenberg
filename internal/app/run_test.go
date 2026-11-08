// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/internal/version"
	"github.com/gopherium/gophenberg/sdk"
)

const unreachableDatabaseURL = "postgres://postgres:gophenberg@localhost:9/postgres?sslmode=disable&connect_timeout=1"

// emptyDatabaseURL returns the URL of a fresh unmigrated test database.
func emptyDatabaseURL(t *testing.T) string {
	t.Helper()
	return testdb.New(t, pgtestdb.NoopMigrator{})
}

// noPlugins registers no plugins.
var noPlugins registrations

// listOf returns a registration for each plugin, its Register answering that plugin.
func listOf(plugins ...sdk.Plugin) registrations {
	listed := make(registrations, 0, len(plugins))
	for _, plugin := range plugins {
		listed = append(listed, pluginkit.Registration[sdk.Deps, sdk.Plugin]{
			ID: plugin.ID(), Register: func(sdk.Deps) (sdk.Plugin, error) { return plugin, nil },
		})
	}
	return listed
}

// listedAs returns the one registration of the plugin id, whose Register is register.
func listedAs(id string, register func(sdk.Deps) (sdk.Plugin, error)) registrations {
	return registrations{{ID: id, Register: register}}
}

// idle is a plugin under its own name that starts and stops with nothing to do.
type idle string

// ID returns the plugin's identifier.
func (p idle) ID() string {
	return string(p)
}

// Start readies nothing.
func (idle) Start(context.Context) error {
	return nil
}

// Stop stops nothing.
func (idle) Stop(context.Context) error {
	return nil
}

// handing returns the one registration of an idle plugin whose Register keeps what it is handed in handed.
func handing(handed *sdk.Deps) registrations {
	return listedAs("handed", func(deps sdk.Deps) (sdk.Plugin, error) {
		*handed = deps
		return idle("handed"), nil
	})
}

// recordsAt returns every plugin record the PostgreSQL database at databaseURL holds.
func recordsAt(t *testing.T, databaseURL string) map[string]pluginkit.PluginRecord {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	defer pool.Close()
	records, err := postgres.NewPluginStore(pool).Records(t.Context())
	if err != nil {
		t.Fatalf("reading the plugin records: %v", err)
	}
	return records
}

// run serves the site over the plugins as the serve command of this binary does, until ctx ends or serving fails.
func run(ctx context.Context, getenv func(string) string, stderr io.Writer, plugins registrations) error {
	return serveSite(ctx, getenv, stderr, binary{plugins: plugins, stamp: version.Stamp()})
}

// serveUntilListening serves the site over the plugins until it listens, answering how the serve ended.
func serveUntilListening(t *testing.T, env map[string]string, plugins registrations) error {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	env["GOPHENBERG_ADDR"] = "localhost:0"
	return run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel}, plugins)
}

func TestRunValidatesItsEnvironment(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]string{
		"missing database url": {},
		"malformed database url": {
			"GOPHENBERG_DATABASE_URL": "not a url \x00",
		},
		"unreachable database": {
			"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_DATABASE_WAIT": "0",
		},
		"malformed trusted proxies": {
			"GOPHENBERG_DATABASE_URL":    unreachableDatabaseURL,
			"GOPHENBERG_TRUSTED_PROXIES": "not-a-cidr",
		},
	}

	for testName, env := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			err := run(t.Context(), testkit.Getenv(env), io.Discard, noPlugins)

			if err == nil {
				t.Fatal("run() error = nil, want a failure")
			}
		})
	}
}

func TestRunPausesAPluginWhoseRegisterFailsAndServes(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)

	err := serveUntilListening(t, map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL},
		listedAs("notes", func(sdk.Deps) (sdk.Plugin, error) { return nil, errors.New("register exploded") }))

	if err != nil {
		t.Fatalf("run() error = %v, want the site served with the plugin paused", err)
	}
	want := pluginkit.PluginRecord{
		ID: "notes", State: pluginkit.PluginPaused, LastError: "pluginkit: notes register: register exploded",
	}
	if got := recordsAt(t, databaseURL)["notes"]; got != want {
		t.Errorf("the notes record = %+v, want %+v", got, want)
	}
}

func TestRunHandsThePluginsTheirSettingsReader(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t),
		"GOPHENBERG_FEED_ITEMS":   "7",
	}
	var handed sdk.Deps

	err := serveUntilListening(t, env, listedAs("feed", func(deps sdk.Deps) (sdk.Plugin, error) {
		handed = deps
		return nil, errors.New("stopped after the registration")
	}))

	if err != nil {
		t.Fatalf("run() error = %v, want the site served", err)
	}
	if handed.Env.Getenv == nil {
		t.Fatal("run() handed the plugins no settings reader")
	}
	if items := handed.Env.Within("FEED_").Value("ITEMS"); items != "7" {
		t.Errorf("Env reads FEED_ITEMS as %q, want 7 under the program prefix", items)
	}
}

func TestRunPausesAPluginWhoseStartFailsAndServes(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	plugin := failingPlugin{err: errors.New("boot exploded")}

	err := serveUntilListening(t, map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}, listOf(plugin))

	if err != nil {
		t.Fatalf("run() error = %v, want the site served with the plugin paused", err)
	}
	if got := recordsAt(t, databaseURL)[plugin.ID()]; got.State != pluginkit.PluginPaused ||
		!strings.Contains(got.LastError, "boot exploded") {
		t.Errorf("the record = %+v, want it paused with the start error", got)
	}
}

func TestRunStartsAndShutsDownCleanly(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":         "localhost:0",
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel}, noPlugins); err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
}

func TestRunReportsCoreMigrationFailure(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	if err := authkitpg.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("pre-migrating auth: %v", err)
	}
	if err := postgres.Migrate(t.Context(), testdb.Open(t, databaseURL)); err != nil {
		t.Fatalf("pre-migrating core: %v", err)
	}
	forgetCoreMigrations(t, databaseURL)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": databaseURL, "GOPHENBERG_ADDR": "localhost:0",
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	err := run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel}, noPlugins)

	if err == nil || !strings.HasPrefix(err.Error(), "postgres: apply migrations: ") {
		t.Fatalf("run() error = %v, want the core migration failure before the site listens", err)
	}
}

// forgetCoreMigrations clears the core migration lineage from the database at databaseURL.
func forgetCoreMigrations(t *testing.T, databaseURL string) {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	defer pool.Close()
	if _, err := pool.Exec(t.Context(), "DELETE FROM goose_db_version"); err != nil {
		t.Fatalf("clearing core migration lineage: %v", err)
	}
}

func TestRunReportsServeFailure(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("occupying a port: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":         listener.Addr().String(),
	}

	runErr := run(t.Context(), testkit.Getenv(env), io.Discard, noPlugins)

	if runErr == nil {
		t.Fatal("run() error = nil, want an address-in-use failure")
	}
}

// cancelOnListen cancels its context once the server logs that it is listening.
type cancelOnListen struct {
	cancel context.CancelFunc
}

// Write scans log output for the listening line.
func (w cancelOnListen) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "listening") {
		w.cancel()
	}
	return len(p), nil
}

// failingPlugin is a plugin whose Start returns the error it holds.
type failingPlugin struct {
	err error
}

// ID returns the plugin's identifier.
func (failingPlugin) ID() string {
	return "failing"
}

// Start returns the failure the plugin was built with.
func (p failingPlugin) Start(_ context.Context) error {
	return p.err
}

// Stop returns nil without stopping anything.
func (failingPlugin) Stop(_ context.Context) error {
	return nil
}

func TestRunServesMediaWhenADirectoryIsConfigured(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":         "localhost:0",
		"GOPHENBERG_MEDIA_DIR":    t.TempDir(),
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := run(ctx, testkit.Getenv(env), cancelOnListen{cancel: cancel}, noPlugins); err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
}

func TestRunReportsAPinnedThemeItCannotLoad(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":         "localhost:0",
		"GOPHENBERG_THEMES_DIR":   t.TempDir(),
		"GOPHENBERG_THEME":        "missing",
	}

	err := run(t.Context(), testkit.Getenv(env), io.Discard, noPlugins)

	if err == nil {
		t.Fatal("run() error = nil, want the pinned theme reported")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %v, want it to name the theme", err)
	}
}
