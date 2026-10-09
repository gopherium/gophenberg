// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"

	"github.com/gopherium/gophenberg/internal/contentbridge"
	"github.com/gopherium/gophenberg/sdk"
)

// compose builds the pool, the stores, the registry, the registered plugins and their database share, starting nothing.
func compose(ctx context.Context, cfg composeConfig, plugins func(sdk.Deps) ([]sdk.Plugin, error)) (site, error) {
	lane, err := laneSettingsFrom(cfg.getenv)
	if err != nil {
		return site{}, err
	}
	built, err := openStores(ctx, cfg)
	if err != nil {
		return site{}, err
	}
	built.lane = &pluginLane{}
	built.registered, built.failed = plugins(pluginDeps(cfg, built))
	if err := built.lane.open(built.pool, len(built.registered), lane); err != nil {
		built.close()
		return site{}, err
	}
	return built, nil
}

// close closes the plugins' database share and then the pool.
func (s site) close() {
	s.lane.close()
	s.pool.Close()
}

// pluginDeps returns what the site lends every plugin, with a settings reader that is never nil.
func pluginDeps(cfg composeConfig, built site) sdk.Deps {
	getenv := cfg.getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	return sdk.Deps{
		DatabaseURL: cfg.databaseURL,
		DB:          built.lane,
		Content:     contentbridge.New(built.content, built.registry, built.library, built.settings),
		Getenv:      getenv,
		Env:         sdk.Env{Prefix: settingsPrefix, Getenv: getenv},
	}
}
