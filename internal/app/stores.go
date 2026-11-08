// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole"
	accountpg "github.com/gopherium/framework/gonsole/auth/postgres"
	accountsqlite "github.com/gopherium/framework/gonsole/auth/sqlite"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/gouncer/authkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	authkitsqlite "github.com/gopherium/gouncer/authkit/sqlite"
	"github.com/gopherium/gouncer/avatar"

	"github.com/gopherium/gophenberg/internal/accountbridge"
	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/sdk"
)

// composeConfig carries the values compose reads.
type composeConfig struct {
	databaseURL string
	handle      *sql.DB
	describe    bool
	fieldDepth  int
	mediaDir    string
	getenv      func(string) string
	roles       *goncierge.Registry
}

// userStore is the account store the site serves, signs in with and sweeps expired sessions and tokens from.
type userStore interface {
	server.UserStore
	authkit.SessionReaper
	authkit.TokenReaper
}

// site is what compose builds over one PostgreSQL handle, one SQLite handle or, while describing, no database.
type site struct {
	pgHandle    *dbkitpg.Handle
	users       userStore
	content     *postgres.ContentStore
	types       *postgres.TypeStore
	registry    *content.Registry
	saved       mediahost.Saved
	library     media.Store
	settings    update.Settings
	readers     content.UserSettingStore
	release     func()
	registered  []sdk.Plugin
	failed      error
	lane        *pluginLane
	roles       *goncierge.Registry
	defaultRole string
	avatars     avatar.Options
	cronStore   cron.Store
	major       func(context.Context) (int, error)

	pluginRecords pluginkit.PluginStore
	laneSettings  laneSettings
	accounts      *accountbridge.Reader
	deps          sdk.Deps
}

// composeOf returns the values compose reads out of the server settings.
func composeOf(settings runConfig, getenv func(string) string) composeConfig {
	return composeConfig{
		databaseURL: settings.databaseURL, fieldDepth: settings.fieldDepth, mediaDir: settings.mediaDir, getenv: getenv,
	}
}

// openStores builds the stores and the registry of the engine the address picks, only the registry while describing.
func openStores(cfg composeConfig) (site, error) {
	switch {
	case onSQLite(cfg.databaseURL):
		return sqliteStores(cfg), nil
	case cfg.describe:
		return site{registry: registryFrom(cfg.fieldDepth, nil), release: func() {}}, nil
	}
	return postgresStores(cfg)
}

// postgresStores opens the capped pool, builds every store over it and the registry, closing the pool on release.
func postgresStores(cfg composeConfig) (site, error) {
	handle, err := openPostgres(settingsEnv(cfg.getenv), cfg.databaseURL)
	if err != nil {
		return site{}, err
	}
	pool := handle.Pool
	store := postgres.NewMediaStore(pool)
	built := site{
		pgHandle:  handle,
		users:     authkitpg.NewUserStore(pool),
		content:   postgres.NewContentStore(pool),
		types:     postgres.NewTypeStore(pool),
		saved:     store.Saved,
		library:   libraryOf(store, cfg.mediaDir),
		settings:  postgres.NewSettingStore(pool),
		readers:   postgres.NewUserSettingStore(pool),
		release:   func() { _ = handle.Close() },
		cronStore: postgres.NewCronStore(pool),
		major:     majorOf(handle.DB),

		pluginRecords: postgres.NewPluginStore(pool),
	}
	built.registry = registryFrom(cfg.fieldDepth, built.types)
	return built, nil
}

// sqliteStores builds the user, media, setting and cron stores over the SQLite handle, leaving content and types nil.
func sqliteStores(cfg composeConfig) site {
	store := sqlite.NewMediaStore(cfg.handle)
	return site{
		users:     authkitsqlite.NewUserStore(cfg.handle),
		registry:  registryFrom(cfg.fieldDepth, nil),
		saved:     store.Saved,
		library:   libraryOf(store, cfg.mediaDir),
		settings:  sqlite.NewSettingStore(cfg.handle),
		readers:   sqlite.NewUserSettingStore(cfg.handle),
		release:   func() {},
		cronStore: sqlite.NewCronStore(cfg.handle),
		major:     noMajor,

		pluginRecords: sqlite.NewPluginStore(cfg.handle),
	}
}

// libraryOf returns the media store the site lends while a media folder is set, nil without one.
func libraryOf(store media.Store, mediaDir string) media.Store {
	if mediaDir == "" {
		return nil
	}
	return store
}

// handleOpener returns the opener of a run's one SQLite handle under the SQLite settings, nil on any other address.
func handleOpener(getenv func(string) string) func(context.Context, string) (*sql.DB, error) {
	env := settingsEnv(getenv)
	if !onSQLite(env.Value("DATABASE_URL")) {
		return nil
	}
	return func(_ context.Context, address string) (*sql.DB, error) {
		if err := checkSQLitePath(env, address); err != nil {
			return nil, err
		}
		settings, err := sqlite.SettingsFrom(env)
		if err != nil {
			return nil, err
		}
		return sqlite.Open(address, settings)
	}
}

// migrations returns the schema steps of the engine the database address getenv names picks, in the order they apply.
func migrations(getenv func(string) string) []gonsole.Step {
	env := settingsEnv(getenv)
	if onSQLite(env.Value("DATABASE_URL")) {
		return sqliteMigrations(env)
	}
	steps := postgresMigrations(postgresCore(env))
	steps[0] = checkedFirst(env, steps[0])
	return steps
}

// postgresMigrations returns the steps a PostgreSQL database takes in the order they apply, the core step last.
func postgresMigrations(core gonsole.Step) []gonsole.Step {
	return []gonsole.Step{accountpg.Migration(), accountpg.RecordMigration(), core}
}

// sqliteMigrations returns the steps a SQLite database takes on the run's one handle, in the order they apply.
func sqliteMigrations(env gonsole.Env) []gonsole.Step {
	return []gonsole.Step{
		sqliteStep("accounts", env, guardedSQLite(accountStep(accountsqlite.Migration))),
		sqliteStep("records", env, accountStep(accountsqlite.RecordMigration)),
		sqliteStep("core", env, migrateSQLiteCore),
	}
}

// sqliteStep returns the step named name that reads the migration lock settings from env, then applies on the handle.
func sqliteStep(
	name string, env gonsole.Env, apply func(context.Context, *sql.DB, sqlite.MigrateOptions) error,
) gonsole.Step {
	return gonsole.Step{Name: name, RunOn: func(ctx context.Context, db *sql.DB) error {
		opts, err := sqlite.MigrateOptionsFrom(env)
		if err != nil {
			return err
		}
		return apply(ctx, db, opts)
	}}
}

// accountStep returns the run on a handle of the account schema step build makes under the lock settings it is handed.
func accountStep(
	build func(accountsqlite.MigrateOptions) gonsole.Step,
) func(context.Context, *sql.DB, sqlite.MigrateOptions) error {
	return func(ctx context.Context, db *sql.DB, opts sqlite.MigrateOptions) error {
		return build(accountsqlite.MigrateOptions(opts)).RunOn(ctx, db)
	}
}

// migrate applies every PostgreSQL schema step once db lets this binary on, the core one on db, the rest at the URL.
func migrate(ctx context.Context, databaseURL string, db *sql.DB) error {
	if err := guardPostgres(ctx, db); err != nil {
		return err
	}
	for _, step := range postgresMigrations(gonsole.Step{Name: "core", RunOn: migratePostgresCore}) {
		if err := step.Apply(ctx, databaseURL, db); err != nil {
			return err
		}
	}
	return nil
}
