// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gouncer/authkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/gopherium/gouncer/authkit/ratelimit"
	"github.com/gopherium/pluginkit"

	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/postbridge"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/version"
	"github.com/gopherium/gophenberg/sdk"
)

// run starts the server and serves until ctx is cancelled or serving fails.
func run(
	ctx context.Context,
	getenv func(string) string,
	stderr io.Writer,
	plugins func(sdk.Deps) ([]sdk.Plugin, error),
) error {
	logger := slog.New(slog.NewTextHandler(stderr, nil))

	settings, err := loadRunConfig(getenv)
	if err != nil {
		return err
	}

	pool, err := openDatabase(ctx, settings.databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	userStore := authkitpg.NewUserStore(pool)
	reaper := authkit.NewReaper(userStore, authkit.ReaperConfig{Logger: logger})
	reaper.Start()
	defer reaper.Stop()

	postStore := postgres.NewPostStore(pool)
	registered, err := plugins(sdk.Deps{
		DatabaseURL: settings.databaseURL,
		Posts:       postbridge.New(postStore),
		Getenv:      getenv,
	})
	if err != nil {
		return fmt.Errorf("register plugins: %w", err)
	}

	host := pluginkit.NewHost(registered...)
	if err := host.Start(ctx); err != nil {
		return fmt.Errorf("start plugins: %w", err)
	}

	themes, stopTheme, err := startTheme(ctx, settings, postgres.NewSettingStore(pool), logger)
	if err != nil {
		return err
	}
	defer stopTheme()

	cfg := server.Config{
		Users:             userStore,
		Posts:             postStore,
		Plugins:           host.Routes(),
		PluginPublicPaths: host.PublicPaths(),
		Version:           version.Version(),
		TrustedProxies:    settings.trustedProxies,
		SiteTitle:         settings.siteTitle,
		Theme:             themes.Holder(),
		Themes:            themes,
	}
	if settings.webDir != "" {
		cfg.Web = os.DirFS(settings.webDir)
	}
	if settings.mediaDir != "" {
		cfg.Media = mediahost.New(mediahost.Config{Dir: settings.mediaDir})
		cfg.MediaStore = postgres.NewMediaStore(pool)
		cfg.MediaFiles = os.DirFS(settings.mediaDir)
	}

	httpServer := &http.Server{
		Addr:              settings.addr,
		Handler:           server.NewServer(cfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	return serveUntilDone(ctx, httpServer, host, logger)
}

// openDatabase returns a migrated connection pool for the database at url.
func openDatabase(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if err := authkitpg.Migrate(ctx, url); err != nil {
		pool.Close()
		return nil, err
	}
	if err := postgres.Migrate(ctx, url); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// runConfig carries the environment-derived settings of the server.
type runConfig struct {
	databaseURL    string
	addr           string
	webDir         string
	siteTitle      string
	trustedProxies []string
	themesDir      string
	theme          string
	nodeBin        string
	mediaDir       string
}

// loadRunConfig reads the server settings from the environment.
func loadRunConfig(getenv func(string) string) (runConfig, error) {
	databaseURL := getenv("GOPHENBERG_DATABASE_URL")
	if databaseURL == "" {
		return runConfig{}, errors.New("GOPHENBERG_DATABASE_URL is required")
	}
	addr := getenv("GOPHENBERG_ADDR")
	if addr == "" {
		addr = "localhost:8081"
	}
	trustedProxies, err := parseTrustedProxies(getenv("GOPHENBERG_TRUSTED_PROXIES"))
	if err != nil {
		return runConfig{}, err
	}
	nodeBin := getenv("GOPHENBERG_NODE_BIN")
	if nodeBin == "" {
		nodeBin = "node"
	}
	return runConfig{
		databaseURL:    databaseURL,
		addr:           addr,
		webDir:         getenv("GOPHENBERG_WEB_DIR"),
		siteTitle:      getenv("GOPHENBERG_SITE_TITLE"),
		trustedProxies: trustedProxies,
		themesDir:      getenv("GOPHENBERG_THEMES_DIR"),
		theme:          getenv("GOPHENBERG_THEME"),
		nodeBin:        nodeBin,
		mediaDir:       getenv("GOPHENBERG_MEDIA_DIR"),
	}, nil
}

// serveUntilDone serves HTTP until ctx is cancelled or serving fails, then
// stops the plugin host.
func serveUntilDone(
	ctx context.Context,
	httpServer *http.Server,
	host *pluginkit.Host,
	logger *slog.Logger,
) error {
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.ListenAndServe()
	}()
	logger.Info("listening", "addr", httpServer.Addr)

	select {
	case err := <-serveErr:
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return errors.Join(fmt.Errorf("http server: %w", err), host.Stop(stopCtx))
	case <-ctx.Done():
	}

	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return errors.Join(httpServer.Shutdown(shutdownCtx), host.Stop(shutdownCtx))
}

// parseTrustedProxies parses raw into trusted-proxy CIDR ranges.
func parseTrustedProxies(raw string) ([]string, error) {
	prefixes, err := ratelimit.ParseTrustedProxies(raw)
	if err != nil {
		return nil, fmt.Errorf("GOPHENBERG_TRUSTED_PROXIES: %w", err)
	}
	return prefixes, nil
}
