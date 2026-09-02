// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gouncer/authkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/gopherium/gouncer/authkit/ratelimit"
	"github.com/gopherium/pluginkit"

	"github.com/gopherium/gophenberg/internal/contentbridge"
	"github.com/gopherium/gophenberg/internal/mediahost"
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

	contentStore := postgres.NewContentStore(pool)
	registered, err := plugins(sdk.Deps{
		DatabaseURL: settings.databaseURL,
		Content:     contentbridge.New(contentStore),
		Getenv:      getenv,
	})
	if err != nil {
		return fmt.Errorf("register plugins: %w", err)
	}

	host := pluginkit.NewHost(registered...)
	if err := host.Start(ctx); err != nil {
		return fmt.Errorf("start plugins: %w", err)
	}

	settingStore := postgres.NewSettingStore(pool)
	themes, stopTheme, err := startTheme(ctx, settings, settingStore, logger)
	if err != nil {
		return err
	}
	defer stopTheme()

	cfg := server.Config{
		Users:             userStore,
		Content:           contentStore,
		Types:             postgres.NewTypeStore(pool),
		Plugins:           host.Routes(),
		PluginPublicPaths: host.PublicPaths(),
		Version:           version.Version(),
		TrustedProxies:    settings.trustedProxies,
		SiteTitle:         settings.siteTitle,
		Theme:             themes.Holder(),
		Themes:            themes,
		Settings:          settingStore,
		Readers:           postgres.NewUserSettingStore(pool),
		Cache:             cachePolicyFrom(settings),
		ThemeTimeout:      settings.themeProxyTimeout,
	}
	if settings.webDir != "" {
		cfg.Web = os.DirFS(settings.webDir)
	}
	if settings.mediaDir != "" {
		cfg.Media = mediahost.New(mediaConfigFrom(settings, settingStore))
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

	themeReadyTimeout  time.Duration
	themeBackoff       time.Duration
	themeMaxBackoff    time.Duration
	themeStopGrace     time.Duration
	themeProxyTimeout  time.Duration
	themeStartAttempts int
	mediaUploadCap     int64

	cacheAssetMaxAge                 time.Duration
	cacheMediaMaxAge                 time.Duration
	cacheContentSharedMaxAge         time.Duration
	cacheContentStaleWhileRevalidate time.Duration
}

// cacheWindowsFrom reads how long each kind of public answer may be kept.
func cacheWindowsFrom(getenv func(string) string, held *runConfig) error {
	for _, asked := range []struct {
		key      string
		fallback time.Duration
		into     *time.Duration
	}{
		{"GOPHENBERG_CACHE_ASSET_MAX_AGE", server.DefaultAssetCacheMaxAge, &held.cacheAssetMaxAge},
		{"GOPHENBERG_CACHE_MEDIA_MAX_AGE", server.DefaultMediaCacheMaxAge, &held.cacheMediaMaxAge},
		{"GOPHENBERG_CACHE_CONTENT_SHARED_MAX_AGE", server.DefaultContentSharedMaxAge,
			&held.cacheContentSharedMaxAge},
		{"GOPHENBERG_CACHE_CONTENT_STALE_WHILE_REVALIDATE", server.DefaultContentStaleWhileRevalidate,
			&held.cacheContentStaleWhileRevalidate},
	} {
		stood, err := standingSeconds(getenv(asked.key), asked.key, asked.fallback)
		if err != nil {
			return err
		}
		*asked.into = stood
	}
	return nil
}

// standingSeconds returns the whole seconds the raw value names, or the fallback when it names none.
func standingSeconds(raw, key string, fallback time.Duration) (time.Duration, error) {
	stood, err := standingDuration(raw, key, fallback)
	if err != nil {
		return 0, err
	}
	if stood%time.Second != 0 {
		return 0, fmt.Errorf("%s: must be whole seconds like 1h or 90s, got %q", key, raw)
	}
	return stood, nil
}

// cachePolicyFrom returns how long each kind of public answer may be kept.
func cachePolicyFrom(settings runConfig) server.CachePolicy {
	return server.CachePolicy{
		AssetMaxAge:                 settings.cacheAssetMaxAge,
		MediaMaxAge:                 settings.cacheMediaMaxAge,
		ContentSharedMaxAge:         settings.cacheContentSharedMaxAge,
		ContentStaleWhileRevalidate: settings.cacheContentStaleWhileRevalidate,
	}
}

// timingsFrom reads the durations, the attempt count and the upload cap the environment names.
func timingsFrom(getenv func(string) string) (runConfig, error) {
	held := runConfig{}
	for _, asked := range []struct {
		key      string
		fallback time.Duration
		into     *time.Duration
	}{
		{"GOPHENBERG_THEME_READY_TIMEOUT", 30 * time.Second, &held.themeReadyTimeout},
		{"GOPHENBERG_THEME_BACKOFF", 500 * time.Millisecond, &held.themeBackoff},
		{"GOPHENBERG_THEME_MAX_BACKOFF", 30 * time.Second, &held.themeMaxBackoff},
		{"GOPHENBERG_THEME_STOP_GRACE", 3 * time.Second, &held.themeStopGrace},
		{"GOPHENBERG_THEME_PROXY_TIMEOUT", 10 * time.Second, &held.themeProxyTimeout},
	} {
		stood, err := standingDuration(getenv(asked.key), asked.key, asked.fallback)
		if err != nil {
			return runConfig{}, err
		}
		*asked.into = stood
	}
	if held.themeMaxBackoff < held.themeBackoff {
		return runConfig{}, fmt.Errorf(
			"GOPHENBERG_THEME_MAX_BACKOFF: must stand at or above GOPHENBERG_THEME_BACKOFF, got %v", held.themeMaxBackoff)
	}
	attempts, err := standingCount(
		getenv("GOPHENBERG_THEME_START_ATTEMPTS"), "GOPHENBERG_THEME_START_ATTEMPTS", 5, maxStartAttempts)
	if err != nil {
		return runConfig{}, err
	}
	cap, err := standingMegabytes(getenv("GOPHENBERG_MEDIA_UPLOAD_CAP_MB"), "GOPHENBERG_MEDIA_UPLOAD_CAP_MB", 128)
	if err != nil {
		return runConfig{}, err
	}
	held.themeStartAttempts, held.mediaUploadCap = attempts, cap
	if err := cacheWindowsFrom(getenv, &held); err != nil {
		return runConfig{}, err
	}
	return held, nil
}

// maxUploadCapMB is the most megabytes an upload cap may name and still be held in bytes.
const maxUploadCapMB int64 = math.MaxInt64 >> 20

// standingMegabytes returns the bytes the raw megabyte value names, or the fallback when it names none.
func standingMegabytes(raw, key string, fallback int64) (int64, error) {
	if raw == "" {
		return fallback << 20, nil
	}
	stood, err := strconv.ParseInt(raw, 10, 64)
	if errors.Is(err, strconv.ErrRange) || stood > maxUploadCapMB {
		return 0, fmt.Errorf("%s: must stand at or below %d, got %q", key, maxUploadCapMB, raw)
	}
	if err != nil {
		return 0, fmt.Errorf("%s: must be a whole number, got %q", key, raw)
	}
	if stood < 1 {
		return 0, fmt.Errorf("%s: must stand above zero, got %q", key, raw)
	}
	return stood << 20, nil
}

// mediaConfigFrom returns the media library settings the environment named and the site chose.
func mediaConfigFrom(settings runConfig, store mediahost.Settings) mediahost.Config {
	return mediahost.Config{Dir: settings.mediaDir, MaxSize: settings.mediaUploadCap, Settings: store}
}

// standingDuration returns the duration the raw value names, or the fallback when it names none.
func standingDuration(raw, key string, fallback time.Duration) (time.Duration, error) {
	if raw == "" {
		return fallback, nil
	}
	stood, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: must be a duration like 30s, got %q", key, raw)
	}
	if stood <= 0 {
		return 0, fmt.Errorf("%s: must stand above zero, got %q", key, raw)
	}
	return stood, nil
}

// maxStartAttempts is how many times a theme that will not start may be tried again.
const maxStartAttempts = 1000

// standingCount returns the whole number the raw value names, or the fallback when it names none.
func standingCount(raw, key string, fallback, ceiling int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	stood, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s: must be a whole number, got %q", key, raw)
	}
	if stood < 1 {
		return 0, fmt.Errorf("%s: must stand above zero, got %q", key, raw)
	}
	if stood > ceiling {
		return 0, fmt.Errorf("%s: must stand at or below %d, got %q", key, ceiling, raw)
	}
	return stood, nil
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
	settings, err := timingsFrom(getenv)
	if err != nil {
		return runConfig{}, err
	}
	settings.databaseURL = databaseURL
	settings.addr = addr
	settings.webDir = getenv("GOPHENBERG_WEB_DIR")
	settings.siteTitle = getenv("GOPHENBERG_SITE_TITLE")
	settings.trustedProxies = trustedProxies
	settings.themesDir = getenv("GOPHENBERG_THEMES_DIR")
	settings.theme = getenv("GOPHENBERG_THEME")
	settings.nodeBin = nodeBin
	settings.mediaDir = getenv("GOPHENBERG_MEDIA_DIR")
	return settings, nil
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
