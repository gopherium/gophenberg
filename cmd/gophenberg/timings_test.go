// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
	"time"
)

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
