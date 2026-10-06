// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/sdk"
)

// loadPlugins returns the registration the command line runs the compiled plugins through, starting none.
func loadPlugins(
	plugins func(sdk.Deps) ([]sdk.Plugin, error),
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
		databaseURL, err := addressOf(call)
		if err != nil {
			return gonsole.Loaded{}, err
		}
		cfg := composeConfig{
			databaseURL: databaseURL, fieldDepth: depth, mediaDir: call.Env.Value("MEDIA_DIR"), getenv: call.Env.Getenv,
		}
		built, err := compose(ctx, cfg, plugins)
		if err != nil {
			return gonsole.Loaded{}, err
		}
		host := pluginkit.NewHost(built.registered...)
		return gonsole.Hosted(built.registered, host, built.failed, grace, built.pool.Close), nil
	}
}

// addressOf returns the database address the call's settings name, which a describing call may leave out.
func addressOf(call gonsole.Call) (string, error) {
	if call.Describe {
		return call.Env.Value("DATABASE_URL"), nil
	}
	return call.Env.Required("DATABASE_URL")
}
