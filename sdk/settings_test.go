// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"reflect"
	"slices"
	"testing"
	"time"
)

// siteSettings is a settings reader the way a site builds one over the values it resolved, sealed by HostOnly.
type siteSettings struct {
	HostOnly
	values map[string]any
}

// Text returns the text resolved for key.
func (s siteSettings) Text(key string) string {
	return held[string](s.values, key)
}

// Count returns the count resolved for key.
func (s siteSettings) Count(key string) int {
	return held[int](s.values, key)
}

// Flag returns the flag resolved for key.
func (s siteSettings) Flag(key string) bool {
	return held[bool](s.values, key)
}

// Duration returns the duration resolved for key.
func (s siteSettings) Duration(key string) time.Duration {
	return held[time.Duration](s.values, key)
}

// Lines returns a copy of the lines resolved for key.
func (s siteSettings) Lines(key string) []string {
	return slices.Clone(held[[]string](s.values, key))
}

// Secret returns the secret resolved for key.
func (s siteSettings) Secret(key string) string {
	return held[string](s.values, key)
}

// Core returns the core setting resolved under name.
func (s siteSettings) Core(name string) string {
	return held[string](s.values, name)
}

// held returns the value of type T stored under key, the zero value when none is.
func held[T any](values map[string]any, key string) T {
	value, _ := values[key].(T)
	return value
}

func TestASiteLendsSettingsByEmbeddingHostOnly(t *testing.T) {
	t.Parallel()

	deps := Deps{Settings: siteSettings{values: map[string]any{
		"title": "Field Notes", "per_page": 25, "open": true, "keep_for": 48 * time.Hour,
		"tags": []string{"news", "events"}, "api_key": "key-for-example", "TRUSTED_PROXIES": "192.0.2.0/24",
	}}}
	deps.Settings.hostOnly()

	read := []any{
		deps.Settings.Text("title"), deps.Settings.Count("per_page"), deps.Settings.Flag("open"),
		deps.Settings.Duration("keep_for"), deps.Settings.Lines("tags"), deps.Settings.Secret("api_key"),
		deps.Settings.Core("TRUSTED_PROXIES"),
	}
	want := []any{"Field Notes", 25, true, 48 * time.Hour, []string{"news", "events"}, "key-for-example", "192.0.2.0/24"}
	if !reflect.DeepEqual(read, want) {
		t.Errorf("the lent reader reads %v, want %v, each value the site resolved", read, want)
	}
}

func TestDepsLendSettingsAndNoReaderOfTheEnvironment(t *testing.T) {
	t.Parallel()

	var names []string
	for _, field := range reflect.VisibleFields(reflect.TypeFor[Deps]()) {
		names = append(names, field.Name)
	}
	if want := []string{"DatabaseURL", "DB", "Content", "Accounts", "Settings"}; !slices.Equal(names, want) {
		t.Errorf("Deps holds %v, want %v, so a plugin reads its settings and never the environment", names, want)
	}
}
