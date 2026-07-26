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

	databaseURL := getenv("GOPHENBERG_DATABASE_URL")
	if databaseURL == "" {
		return errors.New("GOPHENBERG_DATABASE_URL is required")
	}
	addr := getenv("GOPHENBERG_ADDR")
	if addr == "" {
		addr = "localhost:8081"
	}
	trustedProxies, err := parseTrustedProxies(getenv("GOPHENBERG_TRUSTED_PROXIES"))
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}
	defer pool.Close()

	if err := authkitpg.Migrate(ctx, databaseURL); err != nil {
		return err
	}
	if err := postgres.Migrate(ctx, databaseURL); err != nil {
		return err
	}

	userStore := authkitpg.NewUserStore(pool)
	reaper := authkit.NewReaper(userStore, authkit.ReaperConfig{Logger: logger})
	reaper.Start()
	defer reaper.Stop()

	registered, err := plugins(sdk.Deps{
		DatabaseURL: databaseURL,
		Posts:       emptyPostReader{},
		Getenv:      getenv,
	})
	if err != nil {
		return fmt.Errorf("register plugins: %w", err)
	}

	host := pluginkit.NewHost(registered...)
	if err := host.Start(ctx); err != nil {
		return fmt.Errorf("start plugins: %w", err)
	}

	cfg := server.Config{
		Users:             userStore,
		Plugins:           host.Routes(),
		PluginPublicPaths: host.PublicPaths(),
		Version:           version.Version(),
		TrustedProxies:    trustedProxies,
	}
	if webDir := getenv("GOPHENBERG_WEB_DIR"); webDir != "" {
		cfg.Web = os.DirFS(webDir)
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           server.NewServer(cfg),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.ListenAndServe()
	}()
	logger.Info("listening", "addr", addr)

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
