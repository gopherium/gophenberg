// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/gouncer/authkit/ratelimit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
	"github.com/gopherium/gophenberg/internal/webui"
	"github.com/gopherium/gophenberg/sdk"
)

// binary is what a gophenberg binary is built with: the plugins compiled into it and its build stamp.
type binary struct {
	plugins func(sdk.Deps) ([]sdk.Plugin, error)
	stamp   string
}

// serveSite starts the site of the binary given and serves it until ctx ends, serving fails or an update switched.
func serveSite(ctx context.Context, getenv func(string) string, stderr io.Writer, compiled binary) error {
	logger := slog.New(slog.NewTextHandler(stderr, nil))

	settings, guard, err := started(getenv, logger)
	if err != nil {
		return err
	}

	built, err := compose(composeOf(settings, getenv), compiled.plugins)
	if err != nil {
		return err
	}
	defer built.close()
	b := boot{
		settings: settings, guard: guard, built: built, host: hostOf(built.registered), logger: logger,
		stamp: compiled.stamp,
	}
	serving, restart := context.WithCancelCause(ctx)
	defer restart(nil)
	b.admin = b.updatesFromAdmin(serving, restart, settingsEnv(getenv))
	if err := b.ready(ctx); err != nil {
		return errors.Join(b.stopped(ctx, err), b.stopPlugins(ctx))
	}

	jobs := cronOf(settings.cron, built, logger, updateJobs(settings.updates, b.updates())...)

	walked, err := declareTypes(ctx, built.registry, built.registered, logger)
	if err != nil {
		return errors.Join(fmt.Errorf("declare plugin types: %w", err), b.stopPlugins(ctx))
	}

	if err := b.host.Start(ctx, settings.serving.StopGrace); err != nil {
		return fmt.Errorf("start plugins: %w", err)
	}

	themes, stopTheme, err := startTheme(ctx, settings, built.settings, logger)
	if err != nil {
		return errors.Join(err, b.stopPlugins(ctx))
	}
	defer stopTheme()

	cfg := b.serverConfig(walked)
	cfg.Theme, cfg.Themes = themes.Holder(), themes
	cfg.Cron, cfg.CronSecret, cfg.CronOnVisits = jobs.runs, settings.cron.secret, settings.cron.onVisits
	b.lendMedia(ctx, &cfg)

	srv := jobs.serverOf(settings, server.NewServer(cfg))
	return b.admin.ended(guard.Serve(serving, srv, themes.CheckChoice, func(ctx context.Context) error {
		stop := jobs.stop(b.host, settings.serving.StopGrace)
		return gonsole.ServeListening(ctx, srv, settings.serving, settings.listening, stop, logger)
	}))
}

// started reads the server settings, logs the runtime limits, then counts this start before the database opens.
func started(getenv func(string) string, logger *slog.Logger) (runConfig, *updater.Guard, error) {
	settings, err := loadRunConfig(getenv)
	if err != nil {
		return runConfig{}, nil, err
	}
	named, _ := settingsEnv(getenv).RuntimeLimits()
	logRuntimeLimits(logger, named, runtime.GOMAXPROCS(0), debug.SetMemoryLimit(-1))
	if err := settings.install.Layout().MakeData(); err != nil {
		return runConfig{}, nil, err
	}
	guard, err := updater.Start(settings.install, version.Version(), logger)
	return settings, guard, err
}

// boot is one start of the site: its settings, guard, what compose built, plugins, logger, stamp and admin updates.
type boot struct {
	settings runConfig
	guard    *updater.Guard
	built    site
	host     *pluginkit.Host
	logger   *slog.Logger
	stamp    string
	admin    *adminUpdates
}

// updates returns the checks for a newer Gophenberg, the install of the site and the updates administrators start.
func (b boot) updates() siteUpdates {
	updates := siteUpdatesOf(b.settings, b.built, b.stamp)
	updates.admin = b.admin
	return updates
}

// updatesFromAdmin returns what runs the updates administrators start on serving, ending it once one switched.
func (b boot) updatesFromAdmin(
	serving context.Context, restart context.CancelCauseFunc, env gonsole.Env,
) *adminUpdates {
	return &adminUpdates{
		job: updater.Job{
			Settings: b.settings.install, Check: b.settings.updates.config(0), Store: b.built.settings,
			DB: b.built.pgHandle.DB, DatabaseURL: b.settings.databaseURL, Stamp: b.stamp,
		},
		env: env, serving: serving, restart: restart, logger: b.logger,
	}
}

// ready refuses plugins that failed to register, then waits for the database and readies it.
func (b boot) ready(ctx context.Context) error {
	if b.built.failed != nil {
		return fmt.Errorf("register plugins: %w", b.built.failed)
	}
	return b.reachDatabase(ctx)
}

// reachDatabase waits for the database, marks it reached for the boot guard, then checks the role and migrates it.
func (b boot) reachDatabase(ctx context.Context) error {
	if err := b.settings.databaseWait.await(ctx, b.built.pgHandle.Pool.Ping, b.logger); err != nil {
		return err
	}
	if err := b.guard.ReachedDatabase(); err != nil {
		return err
	}
	return prepareDatabase(ctx, b.settings, b.built, b.logger)
}

// stopped returns nil and logs the stop when err is the end of ctx, and err otherwise.
func (b boot) stopped(ctx context.Context, err error) error {
	if !errors.Is(err, ctx.Err()) {
		return err
	}
	b.logger.Info("shutting down")
	return nil
}

// stopPlugins stops the plugins of the start within their stop grace, whether or not ctx has ended.
func (b boot) stopPlugins(ctx context.Context) error {
	return stopPlugins(ctx, b.host, b.settings.serving.StopGrace)
}

// lendMedia lends cfg the media library, its store and its files while the settings name a media folder.
func (b boot) lendMedia(ctx context.Context, cfg *server.Config) {
	if b.settings.mediaDir == "" {
		return
	}
	cfg.Media = recoveredLibrary(ctx, mediaConfigFrom(b.settings, b.built.settings), b.built.saved, b.logger)
	cfg.MediaStore = b.built.library
	cfg.MediaFiles = os.DirFS(b.settings.mediaDir)
}

// serverConfig returns the server settings over what compose built and the types walked, the theme and media aside.
func (b boot) serverConfig(walked definitions.Walked) server.Config {
	settings, built, host := b.settings, b.built, b.host
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
		Logger:            b.logger,
		SiteTitle:         settings.siteTitle,
		Settings:          built.settings,
		Readers:           built.readers,
		Cache:             cachePolicyFrom(settings),
		ThemeTimeout:      settings.themeProxyTimeout,

		DefinitionsImportCap: settings.definitionsImportCap,
		UploadTimeout:        settings.uploadTimeout,
		Declarations:         walked,
		Lists:                settings.lists,
		Roles:                built.roles,
		DefaultRole:          built.defaultRole,
		Avatars:              built.avatars,
		Updates:              b.updates(),
		UpdatesCheckAge:      settings.updates.screenAge,
		UpdatesScreen:        settings.updates.screen,
	}
	cfg.Web = webui.FS()
	if settings.webDir != "" {
		cfg.Web = os.DirFS(settings.webDir)
	}
	return cfg
}

// runConfig carries the environment-derived settings of the server.
type runConfig struct {
	databaseURL    string
	listening      gonsole.Listening
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

	mediaConcurrentImages int
	mediaMaxPixels        int

	definitionsImportCap int64
	fieldDepth           int

	cacheAssetMaxAge                 time.Duration
	cacheMediaMaxAge                 time.Duration
	cacheContentSharedMaxAge         time.Duration
	cacheContentStaleWhileRevalidate time.Duration
	cacheAdminAssetMaxAge            time.Duration

	serving gonsole.Timeouts

	lists server.ListSettings

	cron cronSettings

	superuser superuserPolicy

	databaseWait databaseWait

	updates updateSettings

	install updater.Settings
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
	return gonsole.NewServer(settings.listening.Address, handler, settings.serving)
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
		{"CACHE_ADMIN_ASSET_MAX_AGE", server.DefaultAdminAssetCacheMaxAge, &held.cacheAdminAssetMaxAge},
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
		AdminAssetMaxAge:            settings.cacheAdminAssetMaxAge,
	}
}

// timingsFrom reads the durations, the attempt count, the caps and the image limit the environment names.
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
	if err := mediaLimitsFrom(env, &held); err != nil {
		return runConfig{}, err
	}
	importCap, err := env.Count("DEFINITIONS_IMPORT_CAP_KB", int(server.DefaultDefinitionsImportCap>>10),
		gonsole.AtMost(server.MaxDefinitionsImportCap>>10))
	if err != nil {
		return runConfig{}, err
	}
	held.themeStartAttempts = attempts
	held.definitionsImportCap = int64(importCap) << 10
	if err := cacheWindowsFrom(env, &held); err != nil {
		return runConfig{}, err
	}
	return held, nil
}

// maxUploadCapMB is the most megabytes an upload cap may name and still be held in bytes.
const maxUploadCapMB int64 = math.MaxInt64 >> 20

// megapixel is the pixels in one megapixel.
const megapixel = 1_000_000

// maxMegapixels is the most megapixels a pixel budget may name and still be held in pixels.
const maxMegapixels = math.MaxInt / megapixel

// mediaLimitsFrom reads the upload cap, how many images decode at once and the pixel budget into held.
func mediaLimitsFrom(env gonsole.Env, held *runConfig) error {
	uploadCap, err := env.Count("MEDIA_UPLOAD_CAP_MB", 128, gonsole.AtMost(maxUploadCapMB))
	if err != nil {
		return err
	}
	images, err := env.Count("MEDIA_CONCURRENT_IMAGES", mediahost.DefaultConcurrentImages,
		gonsole.AtMost(mediahost.MaxConcurrentImages))
	if err != nil {
		return err
	}
	megapixels, err := env.Count("MEDIA_MAX_MEGAPIXELS", mediahost.DefaultMaxPixels/megapixel,
		gonsole.AtMost(maxMegapixels))
	if err != nil {
		return err
	}
	held.mediaUploadCap, held.mediaConcurrentImages = int64(uploadCap)<<20, images
	held.mediaMaxPixels = megapixels * megapixel
	return nil
}

// mediaConfigFrom returns the media library settings the environment named and the site chose.
func mediaConfigFrom(settings runConfig, store mediahost.Settings) mediahost.Config {
	return mediahost.Config{
		Dir: settings.mediaDir, MaxSize: settings.mediaUploadCap, Settings: store,
		ConcurrentImages: settings.mediaConcurrentImages, MaxPixels: settings.mediaMaxPixels,
	}
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
	if err := sectionsFrom(env, &settings); err != nil {
		return runConfig{}, err
	}
	settings.databaseURL = databaseURL
	settings.siteTitle = env.Value("SITE_TITLE")
	settings.trustedProxies = trustedProxies
	settings.publicURL = publicURL
	return settings, nil
}

// sectionsFrom reads the list, admin folder, cron, superuser, database wait, update, install and place settings.
func sectionsFrom(env gonsole.Env, held *runConfig) error {
	lists, err := listSettingsFrom(env)
	if err != nil {
		return err
	}
	held.lists = lists
	if held.webDir, err = webDirFrom(env); err != nil {
		return err
	}
	if held.cron, err = cronSettingsFrom(env); err != nil {
		return err
	}
	if held.superuser, err = superuserPolicyFrom(env); err != nil {
		return err
	}
	if held.databaseWait, err = databaseWaitFrom(env); err != nil {
		return err
	}
	if held.updates, err = updateSettingsFrom(env); err != nil {
		return err
	}
	if held.install, err = updater.SettingsFrom(env); err != nil {
		return err
	}
	return placesFrom(env, held)
}

// mediaDirFrom returns the media folder the settings name, else the one inside a data folder they name.
func mediaDirFrom(env gonsole.Env) string {
	named, data := env.Value("MEDIA_DIR"), env.Value("DATA_DIR")
	if named != "" || data == "" {
		return named
	}
	return filepath.Join(data, "data", "media")
}

// placesFrom reads where the site finds its themes and media and where it listens into held.
func placesFrom(env gonsole.Env, held *runConfig) error {
	held.themesDir = env.Value("THEMES_DIR")
	held.theme = env.Value("THEME")
	held.nodeBin = valueOr(env, "NODE_BIN", "node")
	held.mediaDir = mediaDirFrom(env)
	listening, err := listeningFrom(env, *held)
	held.listening = listening
	return err
}

// webDirFrom returns the admin folder the settings name, refusing one whose index.html it cannot read.
func webDirFrom(env gonsole.Env) (string, error) {
	dir := env.Value("WEB_DIR")
	if dir == "" {
		return "", nil
	}
	_, err := fs.ReadFile(os.DirFS(dir), "index.html")
	switch {
	case err == nil:
		return dir, nil
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("%s names %q, which holds no index.html, so unset it to serve the admin built into "+
			"this binary", env.Key("WEB_DIR"), dir)
	}
	return "", fmt.Errorf("%s names %q, whose index.html cannot be read, so fix it or unset it to serve the admin "+
		"built into this binary: %w", env.Key("WEB_DIR"), dir, err)
}

// valueOr returns the setting's value, or fallback when it is empty.
func valueOr(env gonsole.Env, name, fallback string) string {
	if value := env.Value(name); value != "" {
		return value
	}
	return fallback
}
