// SPDX-License-Identifier: Apache-2.0

package main

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/server"
)

func TestLoadRunConfigTakesTheLargestUploadCapItCanCarry(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":        unreachableDatabaseURL,
		"GOPHENBERG_MEDIA_UPLOAD_CAP_MB": strconv.FormatInt(int64(math.MaxInt64>>20), 10),
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want the largest carriable cap taken", err)
	}
	if want := int64(math.MaxInt64>>20) << 20; settings.mediaUploadCap != want {
		t.Errorf("the upload cap = %d, want %d", settings.mediaUploadCap, want)
	}
	if settings.mediaUploadCap <= 0 {
		t.Errorf("the upload cap = %d, want it above zero so the library keeps it", settings.mediaUploadCap)
	}
}

func TestLoadRunConfigTakesTheMostStartAttemptsAllowed(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":         unreachableDatabaseURL,
		"GOPHENBERG_THEME_START_ATTEMPTS": "1000",
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want the most attempts allowed taken", err)
	}
	if settings.themeStartAttempts != 1000 {
		t.Errorf("the start attempts = %d, want 1000", settings.themeStartAttempts)
	}
}

func TestLoadRunConfigDefaultsTheTimingsToTodaysValues(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	for name, held := range map[string]struct {
		stood time.Duration
		want  time.Duration
	}{
		"the ready timeout": {settings.themeReadyTimeout, 30 * time.Second},
		"the backoff":       {settings.themeBackoff, 500 * time.Millisecond},
		"the max backoff":   {settings.themeMaxBackoff, 30 * time.Second},
		"the stop grace":    {settings.themeStopGrace, 3 * time.Second},
		"the proxy timeout": {settings.themeProxyTimeout, 10 * time.Second},
	} {
		if held.stood != held.want {
			t.Errorf("%s = %v, want %v", name, held.stood, held.want)
		}
	}
	if settings.themeStartAttempts != 5 {
		t.Errorf("the start attempts = %d, want 5", settings.themeStartAttempts)
	}
	if settings.mediaUploadCap != 128<<20 {
		t.Errorf("the upload cap = %d, want %d", settings.mediaUploadCap, 128<<20)
	}
	if settings.definitionsImportCap != server.DefaultDefinitionsImportCap {
		t.Errorf("the definitions cap = %d, want %d",
			settings.definitionsImportCap, server.DefaultDefinitionsImportCap)
	}
}

func TestLoadRunConfigTakesTheLargestDefinitionsCapTheBodyReaderAllows(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":              unreachableDatabaseURL,
		"GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB": strconv.FormatInt(server.MaxDefinitionsImportCap>>10, 10),
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want the largest allowed cap taken", err)
	}
	if settings.definitionsImportCap != server.MaxDefinitionsImportCap {
		t.Errorf("the definitions cap = %d, want %d", settings.definitionsImportCap, server.MaxDefinitionsImportCap)
	}
}

func TestLoadRunConfigReadsTheTimingsFromTheEnvironment(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":         unreachableDatabaseURL,
		"GOPHENBERG_THEME_READY_TIMEOUT":  "45s",
		"GOPHENBERG_THEME_BACKOFF":        "250ms",
		"GOPHENBERG_THEME_MAX_BACKOFF":    "1m",
		"GOPHENBERG_THEME_STOP_GRACE":     "5s",
		"GOPHENBERG_THEME_PROXY_TIMEOUT":  "20s",
		"GOPHENBERG_THEME_START_ATTEMPTS": "9",
		"GOPHENBERG_MEDIA_UPLOAD_CAP_MB":  "64",

		"GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB": "512",
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	for name, held := range map[string]struct {
		stood time.Duration
		want  time.Duration
	}{
		"the ready timeout": {settings.themeReadyTimeout, 45 * time.Second},
		"the backoff":       {settings.themeBackoff, 250 * time.Millisecond},
		"the max backoff":   {settings.themeMaxBackoff, time.Minute},
		"the stop grace":    {settings.themeStopGrace, 5 * time.Second},
		"the proxy timeout": {settings.themeProxyTimeout, 20 * time.Second},
	} {
		if held.stood != held.want {
			t.Errorf("%s = %v, want %v", name, held.stood, held.want)
		}
	}
	if settings.themeStartAttempts != 9 {
		t.Errorf("the start attempts = %d, want 9", settings.themeStartAttempts)
	}
	if settings.mediaUploadCap != 64<<20 {
		t.Errorf("the upload cap = %d, want %d", settings.mediaUploadCap, 64<<20)
	}
	if settings.definitionsImportCap != 512<<10 {
		t.Errorf("the definitions cap = %d, want %d", settings.definitionsImportCap, 512<<10)
	}
}

func TestLoadRunConfigRefusesATimingItCannotStand(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		key   string
		value string
	}{
		"a ready timeout that is not a duration": {"GOPHENBERG_THEME_READY_TIMEOUT", "soon"},
		"a ready timeout standing at zero":       {"GOPHENBERG_THEME_READY_TIMEOUT", "0s"},
		"a ready timeout below zero":             {"GOPHENBERG_THEME_READY_TIMEOUT", "-1s"},
		"a backoff that is not a duration":       {"GOPHENBERG_THEME_BACKOFF", "quick"},
		"a backoff below zero":                   {"GOPHENBERG_THEME_BACKOFF", "-1ms"},
		"a max backoff below zero":               {"GOPHENBERG_THEME_MAX_BACKOFF", "-1s"},
		"a stop grace standing at zero":          {"GOPHENBERG_THEME_STOP_GRACE", "0s"},
		"a proxy timeout below zero":             {"GOPHENBERG_THEME_PROXY_TIMEOUT", "-5s"},
		"start attempts that are not a number":   {"GOPHENBERG_THEME_START_ATTEMPTS", "many"},
		"start attempts standing at zero":        {"GOPHENBERG_THEME_START_ATTEMPTS", "0"},
		"start attempts below zero":              {"GOPHENBERG_THEME_START_ATTEMPTS", "-2"},
		"an upload cap that is not a number":     {"GOPHENBERG_MEDIA_UPLOAD_CAP_MB", "big"},
		"an upload cap standing at zero":         {"GOPHENBERG_MEDIA_UPLOAD_CAP_MB", "0"},
		"an upload cap below zero":               {"GOPHENBERG_MEDIA_UPLOAD_CAP_MB", "-8"},
		"a definitions cap that is not a number": {"GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB", "roomy"},
		"a definitions cap standing at zero":     {"GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB", "0"},
		"a definitions cap below zero":           {"GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB", "-4"},
		"a definitions cap past what the body reader allows": {
			"GOPHENBERG_DEFINITIONS_IMPORT_CAP_KB", strconv.FormatInt(server.MaxDefinitionsImportCap>>10+1, 10),
		},
		"an upload cap of more bytes than there are": {
			"GOPHENBERG_MEDIA_UPLOAD_CAP_MB", strconv.FormatInt(int64(math.MaxInt64>>20)+1, 10),
		},
		"an upload cap so large it wraps to nothing": {
			"GOPHENBERG_MEDIA_UPLOAD_CAP_MB", "17592186044416",
		},
		"start attempts of more than anyone waits for": {
			"GOPHENBERG_THEME_START_ATTEMPTS", strconv.Itoa(math.MaxInt32),
		},
		"start attempts one past the most allowed": {
			"GOPHENBERG_THEME_START_ATTEMPTS", "1001",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := loadRunConfig(testGetenv(map[string]string{
				"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
				asked.key:                 asked.value,
			}))

			if err == nil {
				t.Fatalf("loadRunConfig() error = nil, want %s refused", asked.key)
			}
			if !strings.Contains(err.Error(), asked.key) {
				t.Errorf("error = %v, want it to name %s", err, asked.key)
			}
		})
	}
}

func TestLoadRunConfigRefusesAMaxBackoffUnderTheBackoff(t *testing.T) {
	t.Parallel()

	_, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":      unreachableDatabaseURL,
		"GOPHENBERG_THEME_BACKOFF":     "10s",
		"GOPHENBERG_THEME_MAX_BACKOFF": "1s",
	}))

	if err == nil {
		t.Fatalf("loadRunConfig() error = nil, want the max backoff refused")
	}
	if !strings.Contains(err.Error(), "GOPHENBERG_THEME_MAX_BACKOFF") {
		t.Errorf("error = %v, want it to name GOPHENBERG_THEME_MAX_BACKOFF", err)
	}
}

func TestLoadRunConfigDefaultsTheCacheWindows(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	for name, held := range map[string]struct {
		stood time.Duration
		want  time.Duration
	}{
		"the asset window":  {settings.cacheAssetMaxAge, time.Hour},
		"the media window":  {settings.cacheMediaMaxAge, time.Hour},
		"the shared window": {settings.cacheContentSharedMaxAge, time.Minute},
		"the stale window":  {settings.cacheContentStaleWhileRevalidate, 5 * time.Minute},
	} {
		if held.stood != held.want {
			t.Errorf("%s = %v, want %v", name, held.stood, held.want)
		}
	}
}

func TestLoadRunConfigReadsTheCacheWindowsFromTheEnvironment(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":                         unreachableDatabaseURL,
		"GOPHENBERG_CACHE_ASSET_MAX_AGE":                  "2h",
		"GOPHENBERG_CACHE_MEDIA_MAX_AGE":                  "90s",
		"GOPHENBERG_CACHE_CONTENT_SHARED_MAX_AGE":         "30s",
		"GOPHENBERG_CACHE_CONTENT_STALE_WHILE_REVALIDATE": "10m",
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	for name, held := range map[string]struct {
		stood time.Duration
		want  time.Duration
	}{
		"the asset window":  {settings.cacheAssetMaxAge, 2 * time.Hour},
		"the media window":  {settings.cacheMediaMaxAge, 90 * time.Second},
		"the shared window": {settings.cacheContentSharedMaxAge, 30 * time.Second},
		"the stale window":  {settings.cacheContentStaleWhileRevalidate, 10 * time.Minute},
	} {
		if held.stood != held.want {
			t.Errorf("%s = %v, want %v", name, held.stood, held.want)
		}
	}
}

func TestLoadRunConfigRefusesACacheWindowItCannotServe(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		key   string
		value string
	}{
		"a window that is not a duration": {"GOPHENBERG_CACHE_ASSET_MAX_AGE", "soon"},
		"a window standing at zero":       {"GOPHENBERG_CACHE_ASSET_MAX_AGE", "0s"},
		"a window below zero":             {"GOPHENBERG_CACHE_MEDIA_MAX_AGE", "-1s"},
		"a window under a second":         {"GOPHENBERG_CACHE_MEDIA_MAX_AGE", "500ms"},
		"a window of part seconds":        {"GOPHENBERG_CACHE_CONTENT_SHARED_MAX_AGE", "1500ms"},
		"a stale window under a second":   {"GOPHENBERG_CACHE_CONTENT_STALE_WHILE_REVALIDATE", "10ms"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := loadRunConfig(testGetenv(map[string]string{
				"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
				asked.key:                 asked.value,
			}))

			if err == nil {
				t.Fatalf("loadRunConfig() error = nil, want %s refused", asked.key)
			}
			if !strings.Contains(err.Error(), asked.key) {
				t.Errorf("error = %v, want it to name %s", err, asked.key)
			}
		})
	}
}

func TestCachePolicyCarriesEveryWindowTheEnvironmentNamed(t *testing.T) {
	t.Parallel()

	settings := timedConfig()

	held := cachePolicyFrom(settings)

	for name, asked := range map[string]struct {
		stood time.Duration
		want  time.Duration
	}{
		"AssetMaxAge":                 {held.AssetMaxAge, settings.cacheAssetMaxAge},
		"MediaMaxAge":                 {held.MediaMaxAge, settings.cacheMediaMaxAge},
		"ContentSharedMaxAge":         {held.ContentSharedMaxAge, settings.cacheContentSharedMaxAge},
		"ContentStaleWhileRevalidate": {held.ContentStaleWhileRevalidate, settings.cacheContentStaleWhileRevalidate},
	} {
		if asked.stood != asked.want {
			t.Errorf("%s = %v, want %v", name, asked.stood, asked.want)
		}
	}
}
