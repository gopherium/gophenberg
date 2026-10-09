// SPDX-License-Identifier: Apache-2.0

package app

import (
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/sdk"
)

// schemaHeld reports whether the database at databaseURL holds the schema named name.
func schemaHeld(t *testing.T, databaseURL, name string) bool {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("opening %s: %v", databaseURL, err)
	}
	defer pool.Close()
	var held bool
	if err := pool.QueryRow(t.Context(), "SELECT to_regnamespace($1) IS NOT NULL", name).Scan(&held); err != nil {
		t.Fatalf("looking for the %s schema: %v", name, err)
	}
	return held
}

// idsOf returns the identifier of every plugin, in order.
func idsOf(plugins []sdk.Plugin) []string {
	ids := make([]string, 0, len(plugins))
	for _, plugin := range plugins {
		ids = append(ids, plugin.ID())
	}
	return ids
}

func TestComposeRegistersThePluginsWithoutMigrating(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	getenv := testkit.Getenv(map[string]string{"GOPHENBERG_FEED_ITEMS": "7"})
	var handed sdk.Deps

	built, err := compose(t.Context(), composeConfig{databaseURL: databaseURL, fieldDepth: 4, getenv: getenv},
		func(deps sdk.Deps) ([]sdk.Plugin, error) {
			handed = deps
			return keeping(&keeper{})(deps)
		})

	if err != nil {
		t.Fatalf("compose() error = %v, want nil", err)
	}
	defer built.close()
	if !slices.Contains(idsOf(built.registered), "keeper") {
		t.Errorf("registered = %v, want the keeper plugin among them", idsOf(built.registered))
	}
	if handed.DatabaseURL != databaseURL || handed.Content == nil || handed.Env.Within("FEED_").Value("ITEMS") != "7" {
		t.Errorf("plugins were handed %q, %v and FEED_ITEMS %q, want the address, a content reader and the settings",
			handed.DatabaseURL, handed.Content, handed.Env.Within("FEED_").Value("ITEMS"))
	}
	for _, schema := range []string{"auth", "core"} {
		if schemaHeld(t, databaseURL, schema) {
			t.Errorf("the %s schema exists after compose, want nothing migrated", schema)
		}
	}
}

func TestComposeInDescribeModeNeedsNoDatabaseSetting(t *testing.T) {
	t.Parallel()

	var handed sdk.Deps

	built, err := compose(t.Context(), composeConfig{fieldDepth: 4}, func(deps sdk.Deps) ([]sdk.Plugin, error) {
		handed = deps
		return keeping(&keeper{})(deps)
	})

	if err != nil {
		t.Fatalf("compose() error = %v, want the plugins registered without a database setting", err)
	}
	defer built.close()
	if !slices.Contains(idsOf(built.registered), "keeper") {
		t.Errorf("registered = %v, want the keeper plugin among them", idsOf(built.registered))
	}
	if handed.Getenv == nil || handed.Env.Getenv == nil {
		t.Fatal("plugins were handed no settings reader, want one that is never nil")
	}
	if held := handed.Env.Value("FEED_ITEMS"); held != "" {
		t.Errorf("Env reads FEED_ITEMS as %q with no environment, want nothing", held)
	}
}

func TestServerConfigCarriesEveryComposedValue(t *testing.T) {
	t.Parallel()

	getenv := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":              unreachableDatabaseURL,
		"GOPHENBERG_SITE_TITLE":                "Field Notes",
		"GOPHENBERG_THEME_PROXY_TIMEOUT":       "7s",
		"GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB": "64",
		"GOPHENBERG_UPLOAD_TIMEOUT":            "9m",
		"GOPHENBERG_CACHE_ASSET_MAX_AGE":       "2h",
	})
	settings, err := loadRunConfig(getenv)
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	built, err := compose(t.Context(), composeOf(settings, getenv), noPlugins)
	if err != nil {
		t.Fatalf("compose() error = %v, want nil", err)
	}
	defer built.close()

	cfg := serverConfig(settings, built, pluginkit.NewHost(), definitions.Walked{}, slog.New(slog.DiscardHandler))

	if cfg.Registry != built.registry || cfg.Users != built.users || cfg.Content != built.content ||
		cfg.Types != built.types || cfg.Settings != built.settings || cfg.Readers != built.readers {
		t.Error("serverConfig() serves other stores or another registry than the ones compose built")
	}
	if cfg.Cache != cachePolicyFrom(settings) || cfg.Cache.AssetMaxAge != 2*time.Hour {
		t.Errorf("Cache = %+v, want the windows the settings name", cfg.Cache)
	}
	if cfg.ThemeTimeout != 7*time.Second || cfg.DefinitionsImportCap != 64<<10 ||
		cfg.UploadTimeout != 9*time.Minute || cfg.SiteTitle != "Field Notes" {
		t.Errorf("ThemeTimeout %v, DefinitionsImportCap %d, UploadTimeout %v, SiteTitle %q, want 7s, %d, 9m0s and %q",
			cfg.ThemeTimeout, cfg.DefinitionsImportCap, cfg.UploadTimeout, cfg.SiteTitle, 64<<10, "Field Notes")
	}
}
