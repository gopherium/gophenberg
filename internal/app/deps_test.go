// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/gouncer/avatar"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/role"
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

// handedKeeper returns the registration of a keeper plugin whose Register keeps what it is handed in handed.
func handedKeeper(handed *sdk.Deps) registrations {
	return listedAs("keeper", func(deps sdk.Deps) (sdk.Plugin, error) {
		*handed = deps
		return &keeper{deps: deps}, nil
	})
}

func TestComposeRegistersThePluginsWithoutMigrating(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	getenv := testkit.Getenv(map[string]string{"GOPHENBERG_FEED_ITEMS": "7"})
	var handed sdk.Deps

	built, err := compose(composeConfig{databaseURL: databaseURL, fieldDepth: 4, getenv: getenv},
		handedKeeper(&handed))

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
	if handed.Accounts == nil {
		t.Error("plugins were handed no account reader, want one")
	}
	for _, schema := range []string{"auth", "core"} {
		if schemaHeld(t, databaseURL, schema) {
			t.Errorf("the %s schema exists after compose, want nothing migrated", schema)
		}
	}
}

func TestComposeReadsTheAvatarSettingsWithTheirDefaults(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		env  map[string]string
		want avatar.Options
	}{
		"nothing set": {
			env:  map[string]string{},
			want: avatar.Options{Mode: avatar.ModeInitials, Default: avatar.DefaultMystery, Rating: avatar.RatingG},
		},
		"each set": {
			env: map[string]string{
				"GOPHENBERG_AVATARS": "gravatar", "GOPHENBERG_AVATAR_DEFAULT": "identicon", "GOPHENBERG_AVATAR_RATING": "PG",
			},
			want: avatar.Options{Mode: avatar.ModeGravatar, Default: avatar.DefaultIdenticon, Rating: avatar.RatingPG},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			built, err := compose(composeConfig{
				databaseURL: unreachableDatabaseURL, fieldDepth: 4, getenv: testkit.Getenv(asked.env),
			}, noPlugins)
			if err != nil {
				t.Fatalf("compose() error = %v, want nil", err)
			}
			defer built.close()

			if built.avatars != asked.want {
				t.Errorf("avatars = %+v, want %+v", built.avatars, asked.want)
			}
		})
	}
}

func TestComposeRefusesEachBadAvatarSetting(t *testing.T) {
	t.Parallel()

	for _, bad := range []struct{ name, value string }{
		{"GOPHENBERG_AVATARS", "photos"},
		{"GOPHENBERG_AVATAR_DEFAULT", "smiley"},
		{"GOPHENBERG_AVATAR_DEFAULT", "initials"},
		{"GOPHENBERG_AVATAR_RATING", "NC17"},
	} {
		t.Run(bad.name+"="+bad.value, func(t *testing.T) {
			t.Parallel()

			_, err := compose(composeConfig{
				databaseURL: unreachableDatabaseURL, fieldDepth: 4,
				getenv: testkit.Getenv(map[string]string{bad.name: bad.value}),
			}, noPlugins)

			if message := fmt.Sprint(err); !strings.Contains(message, bad.name) || !strings.Contains(message, bad.value) {
				t.Errorf("compose() with %s=%q = %v, want an error naming the setting and its value",
					bad.name, bad.value, err)
			}
		})
	}
}

func TestComposeInDescribeModeNeedsNoDatabaseSetting(t *testing.T) {
	t.Parallel()

	var handed sdk.Deps

	built, err := compose(composeConfig{describe: true, fieldDepth: 4}, handedKeeper(&handed))

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

func TestPluginsAreLentNoDatabaseAddressOnSQLite(t *testing.T) {
	t.Parallel()

	deps := pluginDeps(composeConfig{databaseURL: sqliteAddress}, site{lane: &pluginLane{sqlite: true}}, nil)

	if deps.DatabaseURL != "" {
		t.Errorf("DatabaseURL = %q, want no address lent on a SQLite database", deps.DatabaseURL)
	}
}

func TestReadyOpensNoPluginShareOnSQLite(t *testing.T) {
	t.Parallel()

	built := site{
		lane: &pluginLane{sqlite: true}, roles: goncierge.New(), defaultRole: role.Member,
		laneSettings: laneSettings{conns: 4, reserve: 2, queryTimeout: time.Second, txTimeout: time.Second},
	}

	err := built.ready()

	if err != nil || built.lane.share.Load() != nil {
		t.Errorf("ready() = %v with share %v, want nil and the share left closed", err, built.lane.share.Load())
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
		"GOPHENBERG_AVATARS":                   "off",
		"GOPHENBERG_UPDATES_SCREEN_CHECK_AGE":  "90s",
	})
	settings, err := loadRunConfig(getenv)
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	built, err := compose(composeOf(settings, getenv), noPlugins)
	if err != nil {
		t.Fatalf("compose() error = %v, want nil", err)
	}
	defer built.close()

	cfg := configOver(settings, built)

	if cfg.Registry != built.registry || cfg.Users != built.users || cfg.Content != built.content ||
		cfg.Types != built.types || cfg.Settings != built.settings || cfg.Readers != built.readers {
		t.Error("serverConfig() serves other stores or another registry than the ones compose built")
	}
	if cfg.Roles != built.roles {
		t.Error("serverConfig() serves other roles than the ones compose declared")
	}
	if cfg.Avatars != built.avatars || cfg.Avatars.Mode != avatar.ModeOff {
		t.Errorf("Avatars = %+v, want the %+v compose read from the settings", cfg.Avatars, built.avatars)
	}
	if cfg.Cache != cachePolicyFrom(settings) || cfg.Cache.AssetMaxAge != 2*time.Hour {
		t.Errorf("Cache = %+v, want the windows the settings name", cfg.Cache)
	}
	if cfg.Updates == nil || cfg.UpdatesCheckAge != 90*time.Second {
		t.Errorf("Updates = %v and UpdatesCheckAge = %v, want the site's checks and 1m30s", cfg.Updates,
			cfg.UpdatesCheckAge)
	}
	if cfg.ThemeTimeout != 7*time.Second || cfg.DefinitionsImportCap != 64<<10 ||
		cfg.UploadTimeout != 9*time.Minute || cfg.SiteTitle != "Field Notes" {
		t.Errorf("ThemeTimeout %v, DefinitionsImportCap %d, UploadTimeout %v, SiteTitle %q, want 7s, %d, 9m0s and %q",
			cfg.ThemeTimeout, cfg.DefinitionsImportCap, cfg.UploadTimeout, cfg.SiteTitle, 64<<10, "Field Notes")
	}
}
