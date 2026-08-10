// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// startTheme returns the theme manager the public site is served through, and how to stop it.
func startTheme(
	ctx context.Context,
	settings runConfig,
	store themehost.Settings,
	logger *slog.Logger,
) (*themehost.Manager, func(), error) {
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:  themehost.NewLibrary(settings.themesDir),
		Settings: store,
		Pinned:   settings.theme,
		Supervision: themehost.SupervisorConfig{
			NodeBin: settings.nodeBin,
			APIAddr: settings.addr,
			Logger:  logger,
		},
	})
	if err := manager.Boot(ctx); err != nil {
		return nil, func() {}, err
	}
	return manager, manager.Close, nil
}
