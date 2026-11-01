// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/sdk"
)

// loadPlugins returns the plugin registration, declaring roles into roles and keeping the site's handle in composed.
func loadPlugins(
	plugins func(sdk.Deps) ([]sdk.Plugin, error), roles *goncierge.Registry, composed *sitePool,
) func(context.Context, gonsole.Call) (gonsole.Loaded, error) {
	return func(ctx context.Context, call gonsole.Call) (gonsole.Loaded, error) {
		grace, err := call.Env.Duration("SHUTDOWN_STOP_GRACE", servingDefaults.StopGrace)
		if err != nil {
			return gonsole.Loaded{}, err
		}
		depth, err := fieldDepthFrom(call.Env.Getenv)
		if err != nil {
			return gonsole.Loaded{}, err
		}
		databaseURL, handle, err := databaseOf(ctx, call)
		if err != nil {
			return gonsole.Loaded{}, err
		}
		cfg := composeConfig{
			databaseURL: databaseURL, handle: handle, describe: call.Describe, fieldDepth: depth,
			mediaDir: mediaDirFrom(call.Env), getenv: call.Env.Getenv, roles: roles,
		}
		built, err := compose(cfg, plugins)
		if err != nil {
			return gonsole.Loaded{}, err
		}
		composed.handle = built.pgHandle
		host := hostOf(built.registered)
		return gonsole.Hosted(offerings(built.registered), host, built.failed, grace, built.close), nil
	}
}

// databaseOf returns the database address the call's settings name and the run's handle on SQLite outside describing.
func databaseOf(ctx context.Context, call gonsole.Call) (string, *sql.DB, error) {
	address, err := addressOf(call)
	if err != nil {
		return "", nil, err
	}
	if call.Describe || !onSQLite(address) {
		return address, nil, nil
	}
	handle, err := call.DB(ctx)
	return address, handle, err
}

// addressOf returns the database address the call's settings name, which a describing call may leave out.
func addressOf(call gonsole.Call) (string, error) {
	if call.Describe {
		return call.Env.Value("DATABASE_URL"), nil
	}
	return call.Env.Required("DATABASE_URL")
}
