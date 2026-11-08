// SPDX-License-Identifier: Apache-2.0

// Package app runs the gophenberg command line and the site it serves over the plugins it is handed.
package app

import (
	"context"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole"
	accounts "github.com/gopherium/framework/gonsole/auth"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/internal/installer"
	"github.com/gopherium/gophenberg/internal/version"
	"github.com/gopherium/gophenberg/sdk"
)

// Main runs the gophenberg command line over the arguments and the standard streams of the process, answering its code.
func Main(getenv func(string) string, plugins []pluginkit.Registration[sdk.Deps, sdk.Plugin]) int {
	exits := &exitCodes{}
	return exits.of(gonsole.Main(commandLine(getenv, plugins, exits, version.Stamp())))
}

// Program returns the gophenberg command line over getenv and the compiled plugins.
func Program(getenv func(string) string, plugins []pluginkit.Registration[sdk.Deps, sdk.Plugin]) gonsole.Program {
	return commandLine(getenv, plugins, &exitCodes{}, version.Stamp())
}

// commandLine returns the command line of a binary of the build stamp given, keeping in exits the code a run asks for.
func commandLine(getenv func(string) string, plugins registrations, exits *exitCodes, stamp string) gonsole.Program {
	roles := goncierge.New()
	accountCommands := accountConfig(roles, getenv)
	composed := &sitePool{}
	installs := append(updateCommands(getenv, exits, stamp), serverCommands(installer.RealMachine())...)
	installs = append(installs, databaseOwnerCommand(getenv))
	return gonsole.Program{
		Name:       "gophenberg",
		Title:      "Gophenberg",
		Version:    version.Version(),
		Footer:     "Every command is described at https://docs.gophenberg.org/self-hosting/commands/",
		Env:        settingsEnv(getenv),
		Database:   "DATABASE_URL",
		Open:       handleOpener(getenv),
		Serve:      postgresOnly(getenv, "serve", exits.keep(serve(binary{plugins: plugins, stamp: stamp}))),
		Validate:   validate(accountCommands),
		Migrations: migrations(getenv),
		Seed:       postgresOnly(getenv, "seed", composed.seed),
		Commands:   coreCommands(accountCommands, installs...),
		Plugins:    loadPlugins(plugins, roles, composed),
		Authorize:  accounts.Authorize(accountCommands),
		Record:     accounts.Record(accountCommands),
	}
}

// coreCommands returns the commands the core offers beside the plugins' own, the update and server commands given last.
func coreCommands(cfg accounts.Config, last ...gonsole.Command) []gonsole.Command {
	return append(append(accounts.Commands(cfg), accounts.Records(cfg), aboutCommand()), last...)
}

// serve returns the command that serves the site of the binary given.
func serve(compiled binary) func(context.Context, gonsole.Call) error {
	return func(ctx context.Context, call gonsole.Call) error {
		return serveSite(ctx, call.Env.Getenv, call.Stderr, compiled)
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
