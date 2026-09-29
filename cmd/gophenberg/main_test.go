// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"slices"
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

	if err == nil || !strings.Contains(err.Error(), "plugin feed: ") {
		t.Fatalf("registerPlugins() error = %v, want the feed cap refused and the plugin named", err)
	}
	if len(plugins) != 0 {
		t.Errorf("registerPlugins() = %v, want no plugin registered beside the failure", plugins)
	}
}

func TestRegisterPluginsWiresEveryManifestedPlugin(t *testing.T) {
	t.Parallel()

	plugins, err := registerPlugins(sdk.Deps{Getenv: testGetenv(nil)})

	if err != nil {
		t.Fatalf("registerPlugins() error = %v, want nil", err)
	}
	ids := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		ids = append(ids, plugin.ID())
	}
	if !slices.Contains(ids, "feed") {
		t.Errorf("registerPlugins() = %v, want the feed plugin among them", ids)
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

func TestDispatchPrintsTheCommandsWhenAskedForHelp(t *testing.T) {
	t.Parallel()

	for _, asked := range []string{"help", "-h", "-help", "--help"} {
		t.Run(asked, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := dispatch(t.Context(), []string{asked}, strings.NewReader(""), &stdout, &stderr)

			if code != 0 || stderr.Len() != 0 {
				t.Fatalf("dispatch(%q) = %d with stderr %q, want 0 and nothing on stderr", asked, code, stderr.String())
			}
			for _, line := range []string{
				"gophenberg serve", "gophenberg createadmin", "gophenberg grantrole", "gophenberg seed",
				"gophenberg help", "Pass -h to a subcommand for its own flags.",
			} {
				if !strings.Contains(stdout.String(), line) {
					t.Errorf("dispatch(%q) printed %q, want it to hold %q", asked, stdout.String(), line)
				}
			}
		})
	}
}

func TestDispatchRefusesAMisusedCommandBeforeTouchingAnything(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args []string
		want string
	}{
		"an unknown command": {
			[]string{"hepl"}, `gophenberg: unknown command "hepl", want createadmin, grantrole, seed or serve` + "\n",
		},
		"serve with an argument":         {[]string{"serve", "now"}, "gophenberg: serve takes no arguments\n"},
		"seed with an argument":          {[]string{"seed", "now"}, "gophenberg: seed takes no arguments\n"},
		"seed with the end of its flags": {[]string{"seed", "--"}, "gophenberg: seed takes no arguments\n"},
		"seed with a flag it does not know": {
			[]string{"seed", "-now"}, "gophenberg: seed: flag provided but not defined: -now\n",
		},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := dispatch(t.Context(), tc.args, strings.NewReader(""), &stdout, &stderr)

			if code != 2 || stderr.String() != tc.want || stdout.Len() != 0 {
				t.Errorf("dispatch(%q) = %d, stdout %q, stderr %q, want 2 and %q", tc.args, code, stdout.String(),
					stderr.String(), tc.want)
			}
		})
	}
}

func TestDispatchPrintsTheSeedHelpWithoutSeeding(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := dispatch(t.Context(), []string{"seed", "-h"}, strings.NewReader(""), &stdout, &stderr)

	if code != 0 || !strings.Contains(stdout.String(), "gophenberg seed") || stderr.Len() != 0 {
		t.Errorf("dispatch(seed -h) = %d, stdout %q, stderr %q, want 0 and the seed help", code, stdout.String(),
			stderr.String())
	}
}

func TestDispatchPrintsTheAccountCommandFlagsWithoutADatabase(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args []string
		want string
	}{
		"createadmin": {[]string{"createadmin", "-h"}, "Usage of createadmin:"},
		"grantrole":   {[]string{"grantrole", "--help"}, "Usage of grantrole:"},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := dispatch(t.Context(), tc.args, strings.NewReader(""), &stdout, &stderr)

			if code != 0 || !strings.Contains(stdout.String(), tc.want) || stderr.Len() != 0 {
				t.Errorf("dispatch(%q) = %d, stdout %q, stderr %q, want 0 and its flags", tc.args, code,
					stdout.String(), stderr.String())
			}
		})
	}
}

func TestReportAnswersTheExitCodeOfAnError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err    error
		code   int
		stderr string
	}{
		"no error":               {nil, 0, ""},
		"a help request":         {fmt.Errorf("createadmin: %w", flag.ErrHelp), 0, ""},
		"a failure":              {errors.New("database unreachable"), 1, "gophenberg: database unreachable\n"},
		"a refused command line": {misuse{"seed takes no arguments"}, 2, "gophenberg: seed takes no arguments\n"},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			var stderr bytes.Buffer
			code := report(tc.err, &stderr)

			if code != tc.code || stderr.String() != tc.stderr {
				t.Errorf("report(%v) = %d with stderr %q, want %d and %q", tc.err, code, stderr.String(), tc.code,
					tc.stderr)
			}
		})
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
