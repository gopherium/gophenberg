// SPDX-License-Identifier: Apache-2.0

// Package app runs the gophenberg command line and the site it serves over the plugins it is handed.
package app

import (
	"context"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole"
	accounts "github.com/gopherium/framework/gonsole/auth"

	"github.com/gopherium/gophenberg/internal/version"
	"github.com/gopherium/gophenberg/sdk"
)

// Program returns the gophenberg command line over getenv and the compiled plugins.
func Program(getenv func(string) string, plugins func(sdk.Deps) ([]sdk.Plugin, error)) gonsole.Program {
	roles := goncierge.New()
	accountCommands := accountConfig(roles, getenv)
	composed := &sitePool{}
	return gonsole.Program{
		Name:       "gophenberg",
		Title:      "Gophenberg",
		Version:    version.Version(),
		Footer:     "Every command is described at https://docs.gophenberg.org/self-hosting/commands/",
		Env:        settingsEnv(getenv),
		Database:   "DATABASE_URL",
		Open:       handleOpener(getenv),
		Serve:      postgresOnly(getenv, "serve", serve(plugins)),
		Validate:   validate(accountCommands),
		Migrations: migrations(getenv),
		Seed:       postgresOnly(getenv, "seed", composed.seed),
		Commands:   append(accounts.Commands(accountCommands), accounts.Records(accountCommands), aboutCommand()),
		Plugins:    loadPlugins(plugins, roles, composed),
		Authorize:  accounts.Authorize(accountCommands),
		Record:     accounts.Record(accountCommands),
	}
}

// serve returns the command that serves the site over the compiled plugins.
func serve(plugins func(sdk.Deps) ([]sdk.Plugin, error)) func(context.Context, gonsole.Call) error {
	return func(ctx context.Context, call gonsole.Call) error {
		return run(ctx, call.Env.Getenv, call.Stderr, plugins)
	}
}

// validate returns the check of every setting the site serves under, the SQLite ones on SQLite, and cfg's hooks.
func validate(cfg accounts.Config) func(context.Context, gonsole.Call) error {
	return func(_ context.Context, call gonsole.Call) error {
		settings, err := loadRunConfig(call.Env.Getenv)
		if err != nil {
			return err
		}
		if onSQLite(settings.databaseURL) {
			if err := checkSQLite(call.Env, settings.databaseURL); err != nil {
				return err
			}
		}
		return cfg.Validate(call.Env)
	}
}

// sitePool holds the PostgreSQL handle of the site the run's plugin registration composed, nil on SQLite or describing.
type sitePool struct {
	handle *dbkitpg.Handle
}

// seed stores the demo data on the pool of the site the run composed.
func (p *sitePool) seed(ctx context.Context, call gonsole.Call) error {
	return seedDemoData(ctx, p.handle.Pool, call.Env.Getenv, call.Stdout)
}
