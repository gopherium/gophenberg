// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/gouncer/authkit"
	"github.com/gopherium/gouncer/authkit/ratelimit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/mediahost"
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

	built, err := compose(ctx, composeOf(settings, getenv), plugins)
	if err != nil {
		return err
	}
	defer built.close()
	host := hostOf(built.registered)
	if built.failed != nil {
		return errors.Join(
			fmt.Errorf("register plugins: %w", built.failed), stopPlugins(ctx, host, settings.serving.StopGrace))
	}
	if err := migrate(ctx, settings.databaseURL); err != nil {
		return errors.Join(err, stopPlugins(ctx, host, settings.serving.StopGrace))
	}

	reaper := authkit.NewReaper(built.users, authkit.ReaperConfig{Logger: logger})
	reaper.Start()
	defer reaper.Stop()

	walked, err := declareTypes(ctx, built.registry, built.registered, logger)
	if err != nil {
		return errors.Join(fmt.Errorf("declare plugin types: %w", err), stopPlugins(ctx, host, settings.serving.StopGrace))
	}

	if err := host.Start(ctx, settings.serving.StopGrace); err != nil {
		return fmt.Errorf("start plugins: %w", err)
	}

	themes, stopTheme, err := startTheme(ctx, settings, built.settings, logger)
	if err != nil {
		return errors.Join(err, stopPlugins(ctx, host, settings.serving.StopGrace))
	}
	defer stopTheme()

	cfg := serverConfig(settings, built, host, walked, logger)
	cfg.Theme, cfg.Themes = themes.Holder(), themes
	if settings.mediaDir != "" {
		cfg.Media = recoveredLibrary(ctx, mediaConfigFrom(settings, built.settings), built.media.Saved, logger)
		cfg.MediaStore = built.library
		cfg.MediaFiles = os.DirFS(settings.mediaDir)
	}

	return gonsole.Serve(ctx, httpServerFrom(settings, server.NewServer(cfg)), settings.serving, host.Stop, logger)
}

// serverConfig returns the server settings over what compose built, the theme and the media left to the caller.
func serverConfig(
	settings runConfig, built site, host *pluginkit.Host, walked definitions.Walked, logger *slog.Logger,
) server.Config {
	cfg := server.Config{
		Users:             built.users,
		Content:           built.content,
		Types:             built.types,
		Registry:          built.registry,
		Plugins:           host.Routes(),
		PluginPublicPaths: host.PublicPaths(),
		Version:           version.Version(),
		TrustedProxies:    settings.trustedProxies,
		PublicURL:         settings.publicURL,
		Logger:            logger,
		SiteTitle:         settings.siteTitle,
		Settings:          built.settings,
		Readers:           built.readers,
		Cache:             cachePolicyFrom(settings),
		ThemeTimeout:      settings.themeProxyTimeout,

		DefinitionsImportCap: settings.definitionsImportCap,
		UploadTimeout:        settings.uploadTimeout,
		Declarations:         walked,
		Lists:                settings.lists,
	}
	if settings.webDir != "" {
		cfg.Web = os.DirFS(settings.webDir)
	}
	return cfg
}

// runConfig carries the environment-derived settings of the server.
type runConfig struct {
	databaseURL    string
	addr           string
	webDir         string
	siteTitle      string
	trustedProxies []string
	publicURL      *url.URL
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
	uploadTimeout      time.Duration

	definitionsImportCap int64
	fieldDepth           int

	cacheAssetMaxAge                 time.Duration
	cacheMediaMaxAge                 time.Duration
	cacheContentSharedMaxAge         time.Duration
	cacheContentStaleWhileRevalidate time.Duration

	serving gonsole.Timeouts

	lists server.ListSettings
}

// servingDefaults are the HTTP timeouts and shutdown graces a site runs under when its environment names none.
var servingDefaults = gonsole.Timeouts{
	ReadHeader: 10 * time.Second, Read: 30 * time.Second, Idle: 120 * time.Second,
	Grace: 10 * time.Second, CancelGrace: 5 * time.Second, StopGrace: 5 * time.Second,
}

// settingsPrefix starts the name of every setting the program reads.
const settingsPrefix = "GOPHENBERG_"

// settingsEnv returns the reader of the settings under the program prefix.
func settingsEnv(getenv func(string) string) gonsole.Env {
	return gonsole.Env{Prefix: settingsPrefix, Getenv: getenv}
}

// httpServerFrom returns the HTTP server for the handler at the address and under the timeouts the settings name.
func httpServerFrom(settings runConfig, handler http.Handler) *http.Server {
	return gonsole.NewServer(settings.addr, handler, settings.serving)
}

// stopPlugins stops the plugin host within the grace, whether or not ctx has ended.
func stopPlugins(ctx context.Context, host *pluginkit.Host, grace time.Duration) error {
	stopping, cancel := context.WithTimeout(context.WithoutCancel(ctx), grace)
	defer cancel()
	return host.Stop(stopping)
}

// cacheWindowsFrom reads how long each kind of public answer may be kept.
func cacheWindowsFrom(env gonsole.Env, held *runConfig) error {
	for _, asked := range []struct {
		name     string
		fallback time.Duration
		into     *time.Duration
	}{
		{"CACHE_ASSET_MAX_AGE", server.DefaultAssetCacheMaxAge, &held.cacheAssetMaxAge},
		{"CACHE_MEDIA_MAX_AGE", server.DefaultMediaCacheMaxAge, &held.cacheMediaMaxAge},
		{"CACHE_CONTENT_SHARED_MAX_AGE", server.DefaultContentSharedMaxAge, &held.cacheContentSharedMaxAge},
		{"CACHE_CONTENT_STALE_WHILE_REVALIDATE", server.DefaultContentStaleWhileRevalidate,
			&held.cacheContentStaleWhileRevalidate},
	} {
		stood, err := standingSeconds(env, asked.name, asked.fallback)
		if err != nil {
			return err
		}
		*asked.into = stood
	}
	return nil
}

// standingSeconds returns the whole seconds the setting names, or the fallback when it names none.
func standingSeconds(env gonsole.Env, name string, fallback time.Duration) (time.Duration, error) {
	stood, err := env.Duration(name, fallback)
	if err != nil {
		return 0, err
	}
	if stood%time.Second != 0 {
		return 0, fmt.Errorf("%s: must be whole seconds like 1h or 90s, got %q", env.Key(name), env.Value(name))
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
func timingsFrom(env gonsole.Env) (runConfig, error) {
	held := runConfig{}
	for _, asked := range []struct {
		name     string
		fallback time.Duration
		into     *time.Duration
	}{
		{"THEME_READY_TIMEOUT", 30 * time.Second, &held.themeReadyTimeout},
		{"THEME_BACKOFF", 500 * time.Millisecond, &held.themeBackoff},
		{"THEME_MAX_BACKOFF", 30 * time.Second, &held.themeMaxBackoff},
		{"THEME_STOP_GRACE", 3 * time.Second, &held.themeStopGrace},
		{"THEME_PROXY_TIMEOUT", 10 * time.Second, &held.themeProxyTimeout},
		{"UPLOAD_TIMEOUT", server.DefaultUploadTimeout, &held.uploadTimeout},
	} {
		stood, err := env.Duration(asked.name, asked.fallback)
		if err != nil {
			return runConfig{}, err
		}
		*asked.into = stood
	}
	if held.themeMaxBackoff < held.themeBackoff {
		return runConfig{}, fmt.Errorf(
			"GOPHENBERG_THEME_MAX_BACKOFF: must stand at or above GOPHENBERG_THEME_BACKOFF, got %v", held.themeMaxBackoff)
	}
	attempts, err := env.Count("THEME_START_ATTEMPTS", 5, gonsole.AtMost(maxStartAttempts))
	if err != nil {
		return runConfig{}, err
	}
	uploadCap, err := env.Count("MEDIA_UPLOAD_CAP_MB", 128, gonsole.AtMost(maxUploadCapMB))
	if err != nil {
		return runConfig{}, err
	}
	importCap, err := env.Count("DEFINITIONS_IMPORT_CAP_KB", int(server.DefaultDefinitionsImportCap>>10),
		gonsole.AtMost(server.MaxDefinitionsImportCap>>10))
	if err != nil {
		return runConfig{}, err
	}
	held.themeStartAttempts = attempts
	held.mediaUploadCap, held.definitionsImportCap = int64(uploadCap)<<20, int64(importCap)<<10
	if err := cacheWindowsFrom(env, &held); err != nil {
		return runConfig{}, err
	}
	return held, nil
}

// maxUploadCapMB is the most megabytes an upload cap may name and still be held in bytes.
const maxUploadCapMB int64 = math.MaxInt64 >> 20

// mediaConfigFrom returns the media library settings the environment named and the site chose.
func mediaConfigFrom(settings runConfig, store mediahost.Settings) mediahost.Config {
	return mediahost.Config{Dir: settings.mediaDir, MaxSize: settings.mediaUploadCap, Settings: store}
}

// recoveredLibrary returns the media library once it deletes the files of the uploads a stop left unsaved.
func recoveredLibrary(
	ctx context.Context, cfg mediahost.Config, saved mediahost.Saved, logger *slog.Logger,
) *mediahost.Library {
	library := mediahost.New(cfg)
	deleted, err := library.Recover(ctx, time.Now(), saved)
	for _, file := range deleted {
		logger.Info("unsaved upload deleted", "file", file)
	}
	if err != nil {
		logger.Warn("unsaved uploads kept for the next start", "error", err)
	}
	return library
}

// declareTypes hands every declaring plugin a registrar over the registry, logging what a plugin could not claim.
func declareTypes(
	ctx context.Context, registry *content.Registry, plugins []sdk.Plugin, logger *slog.Logger,
) (definitions.Walked, error) {
	walked := definitions.Walked{}
	for _, plugin := range plugins {
		declarer, ok := plugin.(sdk.TypeDeclarer)
		if !ok {
			continue
		}
		registrar := definitions.New(registry, plugin.ID())
		if err := declarer.DeclareTypes(ctx, registrar); err != nil {
			return nil, fmt.Errorf("plugin %s: %w", plugin.ID(), err)
		}
		walked[plugin.ID()] = definitions.Walk{
			Declared: registrar.Declared(), Skipped: registrar.Skipped(), Kept: registrar.Kept(),
		}
		if kept := registrar.Kept(); len(kept) > 0 {
			logger.Warn("plugin declarations kept as they stand, the site's content holds them",
				"plugin", plugin.ID(), "keys", kept)
		}
		if skipped := registrar.Skipped(); len(skipped) > 0 {
			logger.Warn("plugin declarations skipped, another owner holds the key",
				"plugin", plugin.ID(), "keys", skipped)
		}
	}
	return walked, nil
}

// maxStartAttempts is how many times a theme that will not start may be tried again.
const maxStartAttempts = 1000

// maxFieldDepth is the most containers a site may let a field stand inside.
const maxFieldDepth = 1000

// fieldDepthFrom returns how many containers a field may stand inside, as the environment names it.
func fieldDepthFrom(getenv func(string) string) (int, error) {
	return settingsEnv(getenv).Count("FIELD_DEPTH", content.DefaultFieldDepth, gonsole.AtMost(maxFieldDepth))
}

// registryFrom returns the type registry over the store, holding the nesting limit depth names.
func registryFrom(depth int, store content.TypeStore) *content.Registry {
	return content.NewRegistry(store).WithFieldDepth(depth)
}

// loadRunConfig reads the server settings from the environment.
func loadRunConfig(getenv func(string) string) (runConfig, error) {
	env := settingsEnv(getenv)
	databaseURL, err := env.Required("DATABASE_URL")
	if err != nil {
		return runConfig{}, err
	}
	trustedProxies, err := gonsole.Parse(env, "TRUSTED_PROXIES", nil, ratelimit.ParseTrustedProxies)
	if err != nil {
		return runConfig{}, err
	}
	publicURL, err := gonsole.Parse(env, "PUBLIC_URL", nil, server.ParsePublicURL)
	if err != nil {
		return runConfig{}, err
	}
	settings, err := timingsFrom(env)
	if err != nil {
		return runConfig{}, err
	}
	if settings.fieldDepth, err = fieldDepthFrom(getenv); err != nil {
		return runConfig{}, err
	}
	if settings.serving, err = env.Timeouts(servingDefaults); err != nil {
		return runConfig{}, err
	}
	if settings.lists, err = listSettingsFrom(env); err != nil {
		return runConfig{}, err
	}
	settings.databaseURL = databaseURL
	settings.addr = valueOr(env, "ADDR", "localhost:8081")
	settings.webDir = env.Value("WEB_DIR")
	settings.siteTitle = env.Value("SITE_TITLE")
	settings.trustedProxies = trustedProxies
	settings.publicURL = publicURL
	settings.themesDir = env.Value("THEMES_DIR")
	settings.theme = env.Value("THEME")
	settings.nodeBin = valueOr(env, "NODE_BIN", "node")
	settings.mediaDir = env.Value("MEDIA_DIR")
	return settings, nil
}

// valueOr returns the setting's value, or fallback when it is empty.
func valueOr(env gonsole.Env, name, fallback string) string {
	if value := env.Value(name); value != "" {
		return value
	}
	return fallback
}
