// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/sdk"
)

const unreachableDatabaseURL = "postgres://postgres:gophenberg@localhost:9/postgres?sslmode=disable&connect_timeout=1"

// testGetenv returns a getenv double backed by env.
func testGetenv(env map[string]string) func(string) string {
	return func(key string) string {
		return env[key]
	}
}

// emptyDatabaseURL returns the URL of a fresh unmigrated test database.
func emptyDatabaseURL(t *testing.T) string {
	t.Helper()
	return pgtestdb.Custom(t, testdb.Config(), pgtestdb.NoopMigrator{}).URL()
}

// noPlugins registers no plugins.
func noPlugins(_ sdk.Deps) ([]sdk.Plugin, error) {
	return []sdk.Plugin{}, nil
}

func TestRunValidatesItsEnvironment(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]string{
		"missing database url": {},
		"malformed database url": {
			"GOPHENBERG_DATABASE_URL": "not a url \x00",
		},
		"unreachable database": {
			"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		},
		"malformed trusted proxies": {
			"GOPHENBERG_DATABASE_URL":    unreachableDatabaseURL,
			"GOPHENBERG_TRUSTED_PROXIES": "not-a-cidr",
		},
	}

	for testName, env := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			err := run(t.Context(), testGetenv(env), io.Discard, noPlugins)

			if err == nil {
				t.Fatal("run() error = nil, want a failure")
			}
		})
	}
}

func TestRunReportsPluginRegistrationFailure(t *testing.T) {
	t.Parallel()

	errRegister := errors.New("register exploded")
	env := map[string]string{"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t)}

	err := run(t.Context(), testGetenv(env), io.Discard, func(_ sdk.Deps) ([]sdk.Plugin, error) {
		return nil, errRegister
	})

	if !errors.Is(err, errRegister) {
		t.Errorf("run() error = %v, want %v in its chain", err, errRegister)
	}
}

func TestRegisterPluginsReportsAPluginThatRefusesItsEnvironment(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_FEED_ITEMS": "banana"}

	plugins, err := registerPlugins(sdk.Deps{Getenv: testGetenv(env)})

	if err == nil {
		t.Fatal("registerPlugins() error = nil, want the feed cap refused")
	}
	if plugins != nil {
		t.Errorf("registerPlugins() = %v, want nil on failure", plugins)
	}
}

func TestRegisterPluginsWiresEveryManifestedPlugin(t *testing.T) {
	t.Parallel()

	plugins, err := registerPlugins(sdk.Deps{Getenv: testGetenv(nil)})

	if err != nil {
		t.Fatalf("registerPlugins() error = %v, want nil", err)
	}
	if len(plugins) != 1 || plugins[0].ID() != "feed" {
		t.Errorf("registerPlugins() = %v, want the feed plugin alone", plugins)
	}
}

func TestRunReportsPluginStartFailure(t *testing.T) {
	t.Parallel()

	errBoot := errors.New("boot exploded")
	env := map[string]string{"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t)}

	err := run(t.Context(), testGetenv(env), io.Discard, func(_ sdk.Deps) ([]sdk.Plugin, error) {
		return []sdk.Plugin{failingPlugin{err: errBoot}}, nil
	})

	if !errors.Is(err, errBoot) {
		t.Errorf("run() error = %v, want %v in its chain", err, errBoot)
	}
}

func TestRunStartsAndShutsDownCleanly(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":         "localhost:0",
		"GOPHENBERG_WEB_DIR":      t.TempDir(),
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := run(ctx, testGetenv(env), cancelOnListen{cancel: cancel}, noPlugins); err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
}

func TestRunReportsCoreMigrationFailure(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	if err := authkitpg.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("pre-migrating auth: %v", err)
	}
	if err := postgres.Migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("pre-migrating core: %v", err)
	}
	forgetCoreMigrations(t, databaseURL)
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}

	err := run(t.Context(), testGetenv(env), io.Discard, noPlugins)

	if err == nil {
		t.Fatal("run() error = nil, want a core migration failure")
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

	runErr := run(t.Context(), testGetenv(env), io.Discard, noPlugins)

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
		"GOPHENBERG_WEB_DIR":      t.TempDir(),
		"GOPHENBERG_MEDIA_DIR":    t.TempDir(),
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	if err := run(ctx, testGetenv(env), cancelOnListen{cancel: cancel}, noPlugins); err != nil {
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

	err := run(t.Context(), testGetenv(env), io.Discard, noPlugins)

	if err == nil {
		t.Fatal("run() error = nil, want the pinned theme reported")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %v, want it to name the theme", err)
	}
}
