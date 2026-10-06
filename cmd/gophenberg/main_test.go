// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/app"
	"github.com/gopherium/gophenberg/sdk"
)

const unreachableDatabaseURL = "postgres://postgres:gophenberg@localhost:9/postgres?sslmode=disable&connect_timeout=1"

// settingsOf returns the settings reader the command line hands the plugins over env.
func settingsOf(env map[string]string) gonsole.Env {
	return app.Program(testkit.Getenv(env), registerPlugins).Env
}

// declaredNames returns every name the package's own files declare, imports aside, sorted.
func declaredNames(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package files: %v", err)
	}
	var names []string
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			names = append(names, namesOf(decl)...)
		}
	}
	slices.Sort(names)
	return names
}

// namesOf returns the names one declaration brings in, imports aside.
func namesOf(decl ast.Decl) []string {
	if fn, ok := decl.(*ast.FuncDecl); ok {
		return []string{fn.Name.Name}
	}
	gen, ok := decl.(*ast.GenDecl)
	if !ok {
		return nil
	}
	var names []string
	for _, spec := range gen.Specs {
		switch spec := spec.(type) {
		case *ast.TypeSpec:
			names = append(names, spec.Name.Name)
		case *ast.ValueSpec:
			for _, name := range spec.Names {
				names = append(names, name.Name)
			}
		}
	}
	return names
}

func TestTheMainPackageHoldsOnlyTheWiring(t *testing.T) {
	t.Parallel()

	names := declaredNames(t)

	if want := []string{"main", "registerPlugins"}; !slices.Equal(names, want) {
		t.Errorf("package main declares %v, want only %v, the rest belongs in internal/app", names, want)
	}
}

func TestRegisterPluginsReportsAPluginThatRefusesItsEnvironment(t *testing.T) {
	t.Parallel()

	settings := settingsOf(map[string]string{"GOPHENBERG_FEED_ITEMS": "banana"})

	plugins, err := registerPlugins(sdk.Deps{Getenv: settings.Getenv, Env: settings})

	if err == nil || !strings.Contains(err.Error(), "plugin feed: ") {
		t.Fatalf("registerPlugins() error = %v, want the feed cap refused and the plugin named", err)
	}
	ids := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		ids = append(ids, plugin.ID())
	}
	if slices.Contains(ids, "feed") {
		t.Errorf("registerPlugins() = %v, want the refused feed plugin left out", ids)
	}
}

func TestRegisterPluginsWiresEveryManifestedPlugin(t *testing.T) {
	t.Parallel()

	settings := settingsOf(nil)

	plugins, err := registerPlugins(sdk.Deps{Getenv: settings.Getenv, Env: settings})

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

func TestRegisterPluginsCommands(t *testing.T) {
	t.Parallel()

	program := app.Program(testkit.Getenv(nil), registerPlugins)
	call := gonsole.Call{Env: program.Env, Describe: true}

	loaded, err := program.Plugins(t.Context(), call)

	if err != nil || loaded.Failed != nil {
		t.Fatalf("Plugins() = %v with %v failed, want every compiled plugin registered", err, loaded.Failed)
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

func TestAFeedThatCannotRegisterShowsUnderNotLoaded(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_FEED_ITEMS":   "banana",
	})

	listed := testkit.Run(t, app.Program(env, registerPlugins), "", "list")
	checked := testkit.Run(t, app.Program(env, registerPlugins), "", "check")

	if listed.Code != gonsole.ExitDone || !strings.Contains(listed.Stdout, "\nNot loaded:\n") ||
		!strings.Contains(listed.Stdout, "GOPHENBERG_FEED_ITEMS") {
		t.Errorf("list = %d, stdout %q, want 0 and the feed under Not loaded", listed.Code, listed.Stdout)
	}
	if checked.Code != gonsole.ExitFailed || !strings.Contains(checked.Stderr, "GOPHENBERG_FEED_ITEMS") {
		t.Errorf("check = %d with stderr %q, want 1 and the feed setting named", checked.Code, checked.Stderr)
	}
}
