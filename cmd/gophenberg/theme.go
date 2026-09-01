// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"log/slog"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// supervisionFrom returns how an activated theme is run under the settings the environment named.
func supervisionFrom(settings runConfig, logger *slog.Logger) themehost.SupervisorConfig {
	return themehost.SupervisorConfig{
		NodeBin:      settings.nodeBin,
		APIAddr:      settings.addr,
		Logger:       logger,
		ReadyTimeout: settings.themeReadyTimeout,
		Backoff:      settings.themeBackoff,
		MaxBackoff:   settings.themeMaxBackoff,
		MaxAttempts:  settings.themeStartAttempts,
		StopGrace:    settings.themeStopGrace,
	}
}

// startTheme returns the theme manager the public site is served through, and how to stop it.
func startTheme(
	ctx context.Context,
	settings runConfig,
	store themehost.Settings,
	logger *slog.Logger,
) (*themehost.Manager, func(), error) {
	manager := themehost.NewManager(themehost.ManagerConfig{
		Library:     themehost.NewLibrary(settings.themesDir),
		Settings:    store,
		Pinned:      settings.theme,
		Supervision: supervisionFrom(settings, logger),
	})
	if err := manager.Boot(ctx); err != nil {
		return nil, func() {}, err
	}
	return manager, manager.Close, nil
}
