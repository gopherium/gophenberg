// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/sdk"
)

var (
	_ sdk.Migrator        = (*keeper)(nil)
	_ pluginkit.Seeder    = (*keeper)(nil)
	_ sdk.CommandProvider = (*keeper)(nil)
)

// keeper is a plugin that notes what the host asks of it and offers one command showing what it was handed.
type keeper struct {
	deps     sdk.Deps
	holdStop bool
	mu       sync.Mutex
	asked    []string
}

// ID returns the plugin's identifier.
func (k *keeper) ID() string {
	return "keeper"
}

// Start notes the start.
func (k *keeper) Start(context.Context) error {
	k.note("start")
	return nil
}

// Stop notes the stop, holding until ctx ends when the plugin holds its stops.
func (k *keeper) Stop(ctx context.Context) error {
	k.note("stop")
	if k.holdStop {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

// Migrate notes the migration.
func (k *keeper) Migrate(context.Context) error {
	k.note("migrate")
	return nil
}

// Seed notes the seeding.
func (k *keeper) Seed(context.Context) error {
	k.note("seed")
	return nil
}

// Commands returns the command printing the database address and whether a content reader was handed.
func (k *keeper) Commands() []sdk.Command {
	return []sdk.Command{{
		Name:    "keeper:show",
		Summary: "show what the plugin was handed",
		Run: func(_ context.Context, call sdk.Call) error {
			_, err := fmt.Fprintf(call.Stdout, "%s %t\n", k.deps.DatabaseURL, k.deps.Content != nil)
			return err
		},
	}}
}

// note records one thing the host asked.
func (k *keeper) note(step string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.asked = append(k.asked, step)
}

// steps returns what the host asked, in order.
func (k *keeper) steps() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return slices.Clone(k.asked)
}

// keeping returns a registration answering k over the Deps it is handed.
func keeping(k *keeper) func(sdk.Deps) ([]sdk.Plugin, error) {
	return func(deps sdk.Deps) ([]sdk.Plugin, error) {
		k.deps = deps
		return []sdk.Plugin{k}, nil
	}
}

func TestRegisterPluginsCommands(t *testing.T) {
	t.Parallel()

	call := gonsole.Call{Env: settingsEnv(testkit.Getenv(nil)), Describe: true}

	loaded, err := loadPlugins(registerPlugins)(t.Context(), call)

	if err != nil || loaded.Failed != nil {
		t.Fatalf("loadPlugins() = %v with %v failed, want every compiled plugin registered", err, loaded.Failed)
	}
	defer func() { _ = loaded.Release(context.WithoutCancel(t.Context())) }()
	for _, group := range loaded.Groups {
		for _, command := range group.Commands {
			if !strings.HasPrefix(command.Name, group.Namespace+":") {
				t.Errorf("plugin %s offers %q, want every command under its id", group.Namespace, command.Name)
			}
		}
	}
}

func TestLoadPluginsNamesWhatItCannotRead(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		env      map[string]string
		describe bool
		want     string
	}{
		"a stop grace that is not a duration": {
			map[string]string{"GOPHENBERG_SHUTDOWN_STOP_GRACE": "soon"}, true, "GOPHENBERG_SHUTDOWN_STOP_GRACE",
		},
		"a field depth past the most allowed": {
			map[string]string{"GOPHENBERG_FIELD_DEPTH": "1001"}, true, "GOPHENBERG_FIELD_DEPTH",
		},
		"no database address outside describe mode": {map[string]string{}, false, "GOPHENBERG_DATABASE_URL"},
		"a database address it cannot parse": {
			map[string]string{"GOPHENBERG_DATABASE_URL": "not a url \x00"}, false, "parse database url",
		},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			call := gonsole.Call{Env: settingsEnv(testkit.Getenv(tc.env)), Describe: tc.describe}

			_, err := loadPlugins(registerPlugins)(t.Context(), call)

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("loadPlugins() error = %v, want %s named", err, tc.want)
			}
		})
	}
}

func TestSeedYesAppliesAPluginSchemaAndStopsIt(t *testing.T) {
	t.Parallel()

	k := &keeper{}
	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t)})

	got := testkit.Run(t, program(env, keeping(k)), "", "seed", "-yes")

	if got.Code != gonsole.ExitDone {
		t.Fatalf("seed -yes = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	if want := []string{"migrate", "seed", "stop"}; !slices.Equal(k.steps(), want) {
		t.Errorf("the plugin was asked %v, want %v", k.steps(), want)
	}
}

func TestAPluginCommandSeesTheComposedDeps(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL})

	got := testkit.Run(t, program(env, keeping(&keeper{})), "", "keeper:show")

	if want := unreachableDatabaseURL + " true\n"; got.Code != gonsole.ExitDone || got.Stdout != want {
		t.Errorf("keeper:show = %d, stdout %q, stderr %q, want 0 and %q", got.Code, got.Stdout, got.Stderr, want)
	}
}

func TestAFeedThatCannotRegisterShowsUnderNotLoaded(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_FEED_ITEMS":   "banana",
	})

	listed := testkit.Run(t, program(env, registerPlugins), "", "list")
	checked := testkit.Run(t, program(env, registerPlugins), "", "check")

	if listed.Code != gonsole.ExitDone || !strings.Contains(listed.Stdout, "\nNot loaded:\n") ||
		!strings.Contains(listed.Stdout, "GOPHENBERG_FEED_ITEMS") {
		t.Errorf("list = %d, stdout %q, want 0 and the feed under Not loaded", listed.Code, listed.Stdout)
	}
	if checked.Code != gonsole.ExitFailed || !strings.Contains(checked.Stderr, "GOPHENBERG_FEED_ITEMS") {
		t.Errorf("check = %d with stderr %q, want 1 and the feed setting named", checked.Code, checked.Stderr)
	}
}

func TestReleaseEndsWithinTheStopGrace(t *testing.T) {
	t.Parallel()

	k := &keeper{holdStop: true}
	call := gonsole.Call{Env: settingsEnv(testkit.Getenv(map[string]string{
		"GOPHENBERG_SHUTDOWN_STOP_GRACE": "100ms",
	})), Describe: true}
	loaded, err := loadPlugins(keeping(k))(t.Context(), call)
	if err != nil {
		t.Fatalf("loadPlugins() error = %v, want nil", err)
	}
	started := time.Now()

	err = loaded.Release(t.Context())

	if took := time.Since(started); err == nil || took > time.Second {
		t.Errorf("Release() = %v after %v, want it to give up within the grace", err, took)
	}
}
