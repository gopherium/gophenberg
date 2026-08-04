// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log/slog"

	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/themehost"
)

// startTheme returns the theme the public site is served through, and how to stop it.
func startTheme(settings runConfig, logger *slog.Logger) (server.Theme, func(), error) {
	loaded, err := themehost.Load(settings.themesDir, settings.theme)
	if err != nil {
		return nil, func() {}, err
	}
	if loaded == nil {
		logger.Info("serving with the built-in renderer", "mode", "renderer")
		return nil, func() {}, nil
	}
	supervisor := themehost.NewSupervisor(themehost.SupervisorConfig{
		Theme:   loaded,
		NodeBin: settings.nodeBin,
		APIAddr: settings.addr,
		Logger:  logger,
	})
	supervisor.Start()
	logger.Info("serving through a theme", "mode", "theme", "theme", loaded.Name, "version", loaded.Version)
	return supervisor, supervisor.Stop, nil
}
