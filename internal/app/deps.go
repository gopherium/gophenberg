// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/gopherium/gophenberg/internal/contentbridge"
	"github.com/gopherium/gophenberg/sdk"
)

// compose builds the pool, the stores, the registry and the registered plugins, migrating and starting nothing.
func compose(ctx context.Context, cfg composeConfig, plugins func(sdk.Deps) ([]sdk.Plugin, error)) (site, error) {
	built, err := openStores(ctx, cfg)
	if err != nil {
		return site{}, err
	}
	built.registered, built.failed = plugins(pluginDeps(cfg, built))
	return built, nil
}

// pluginDeps returns what the site lends every plugin, with a settings reader that is never nil.
func pluginDeps(cfg composeConfig, built site) sdk.Deps {
	getenv := cfg.getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	return sdk.Deps{
		DatabaseURL: cfg.databaseURL,
		Content:     contentbridge.New(built.content, built.registry, built.library, built.settings),
		Getenv:      getenv,
		Env:         settingsEnv(getenv),
	}
}
