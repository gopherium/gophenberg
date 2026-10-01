// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"

	"github.com/gopherium/framework/gonsole"
	accounts "github.com/gopherium/framework/gonsole/auth"

	"github.com/gopherium/gophenberg/internal/version"
	"github.com/gopherium/gophenberg/sdk"
)

// program returns the gophenberg command line over getenv and the compiled plugins.
func program(getenv func(string) string, plugins func(sdk.Deps) ([]sdk.Plugin, error)) gonsole.Program {
	return gonsole.Program{
		Name:       "gophenberg",
		Title:      "Gophenberg",
		Version:    version.Version(),
		Footer:     "Every command is described at https://docs.gophenberg.org/self-hosting/commands/",
		Env:        settingsEnv(getenv),
		Database:   "DATABASE_URL",
		Serve:      serve(plugins),
		Validate:   validate,
		Migrations: migrations(),
		Seed:       seedSite,
		Commands:   accounts.Commands(accounts.Config{Roles: fixedRoles}),
		Plugins:    loadPlugins(plugins),
	}
}

// serve returns the command that serves the site over the compiled plugins.
func serve(plugins func(sdk.Deps) ([]sdk.Plugin, error)) func(context.Context, gonsole.Call) error {
	return func(ctx context.Context, call gonsole.Call) error {
		return run(ctx, call.Env.Getenv, call.Stderr, plugins)
	}
}

// validate reads every setting the site serves under, naming the first it cannot stand.
func validate(_ context.Context, call gonsole.Call) error {
	_, err := loadRunConfig(call.Env.Getenv)
	return err
}

// seedSite stores the demo data in the database the call's settings name.
func seedSite(ctx context.Context, call gonsole.Call) error {
	return seedDemoData(ctx, call.Env.Getenv, call.Stdout)
}
