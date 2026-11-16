// SPDX-License-Identifier: Apache-2.0

// Package pluginsettings checks each plugin's declared settings, works out their values and lends each plugin a reader.
package pluginsettings

import (
	"cmp"
	"log/slog"
	"sync"

	"github.com/gopherium/framework/pluginkit/settings"
	"github.com/gopherium/framework/pluginkit/wire"

	"github.com/gopherium/gophenberg/sdk"
)

// rules are the variable prefix and the reserved key starts every plugin's settings follow.
var rules = settings.Rules{Prefix: "GOPHENBERG_PLUGIN_", Reserved: []string{"job_", "db_"}}

// Set holds every plugin's resolved settings, each plugin's rejection, and the core values plugins may read.
type Set struct {
	values   map[string]settings.Values
	rejected map[string]error
	core     map[string]string
	logger   *slog.Logger
	logged   sync.Map
}

// New checks and resolves each plugin's declared settings over getenv, logging misreads to logger.
func New(declared map[string][]wire.Setting, getenv func(string) string, logger *slog.Logger) *Set {
	set := &Set{
		values:   make(map[string]settings.Values, len(declared)),
		rejected: make(map[string]error),
		core:     coreOf(getenv),
		logger:   cmp.Or(logger, slog.New(slog.DiscardHandler)),
	}
	for id, list := range declared {
		values, err := resolve(id, list, getenv)
		if err != nil {
			set.rejected[id] = err
			continue
		}
		set.values[id] = values
	}
	return set
}

// For returns plugin id's reader, one with no keys for a plugin that declares none.
func (s *Set) For(id string) sdk.Settings {
	return &reader{set: s, id: id, values: s.values[id]}
}

// Rejection returns why plugin id's settings were rejected, nil when they were not.
func (s *Set) Rejection(id string) error {
	return s.rejected[id]
}

// resolve checks the settings plugin id declares and works out each value from getenv, then from its default.
func resolve(id string, declared []wire.Setting, getenv func(string) string) (settings.Values, error) {
	schema, err := settings.Check(id, declared, rules)
	if err != nil {
		return nil, err
	}
	found := make(map[string]string, len(schema.Fields))
	for _, field := range schema.Fields {
		found[field.Env] = getenv(field.Env)
	}
	return schema.Resolve(found)
}
