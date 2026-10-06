// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"

	"github.com/gopherium/framework/gonsole"
	accounts "github.com/gopherium/framework/gonsole/auth"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/sdk"
)

// composeConfig carries the values compose reads.
type composeConfig struct {
	databaseURL string
	fieldDepth  int
	mediaDir    string
	getenv      func(string) string
}

// site is what compose builds over one pool.
type site struct {
	pool       *pgxpool.Pool
	users      *authkitpg.UserStore
	content    *postgres.ContentStore
	types      *postgres.TypeStore
	registry   *content.Registry
	media      *postgres.MediaStore
	library    media.Store
	settings   *postgres.SettingStore
	readers    *postgres.UserSettingStore
	registered []sdk.Plugin
	failed     error
}

// composeOf returns the values compose reads out of the server settings.
func composeOf(settings runConfig, getenv func(string) string) composeConfig {
	return composeConfig{
		databaseURL: settings.databaseURL, fieldDepth: settings.fieldDepth, mediaDir: settings.mediaDir, getenv: getenv,
	}
}

// openStores builds the pool, the stores and the registry, migrating nothing.
func openStores(ctx context.Context, cfg composeConfig) (site, error) {
	pool, err := pgxpool.New(ctx, cfg.databaseURL)
	if err != nil {
		return site{}, fmt.Errorf("parse database url: %w", err)
	}
	built := site{
		pool:     pool,
		users:    authkitpg.NewUserStore(pool),
		content:  postgres.NewContentStore(pool),
		types:    postgres.NewTypeStore(pool),
		media:    postgres.NewMediaStore(pool),
		settings: postgres.NewSettingStore(pool),
		readers:  postgres.NewUserSettingStore(pool),
	}
	built.registry = registryFrom(cfg.fieldDepth, built.types)
	if cfg.mediaDir != "" {
		built.library = built.media
	}
	return built, nil
}

// migrations returns the schema steps every database takes, in the order they apply.
func migrations() []gonsole.Step {
	return []gonsole.Step{accounts.Migration(), accounts.RecordMigration(), {Name: "core", Run: postgres.Migrate}}
}

// migrate applies every schema step to the database at databaseURL.
func migrate(ctx context.Context, databaseURL string) error {
	for _, step := range migrations() {
		if err := step.Run(ctx, databaseURL); err != nil {
			return err
		}
	}
	return nil
}
