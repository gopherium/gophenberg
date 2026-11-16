// SPDX-License-Identifier: Apache-2.0

package pluginsettings

import (
	"slices"
	"strings"
	"time"

	"github.com/gopherium/framework/pluginkit/settings"

	"github.com/gopherium/gophenberg/sdk"
)

var _ sdk.Settings = (*reader)(nil)

// corePrefix starts the variable of each core setting a plugin may read.
const corePrefix = "GOPHENBERG_"

// coreNames lists the core settings a plugin may read.
var coreNames = []string{"TRUSTED_PROXIES"}

const (
	// undeclared is the log message of a read of a key the manifest does not declare.
	undeclared = "plugin read an undeclared setting"
	// otherType is the log message of a read with the reader of another type.
	otherType = "plugin read a setting as another type"
	// offList is the log message of a read of a core setting off the list.
	offList = "plugin read a core setting off the list"
)

// misread is one way one plugin misread one key or core name, logged once.
type misread struct {
	plugin  string
	message string
	key     string
}

// reader reads one plugin's resolved settings, logging each misread once.
type reader struct {
	sdk.HostOnly
	set    *Set
	id     string
	values settings.Values
}

// coreOf returns each core setting on the list, read once from getenv and trimmed.
func coreOf(getenv func(string) string) map[string]string {
	core := make(map[string]string, len(coreNames))
	for _, name := range coreNames {
		core[name] = strings.TrimSpace(getenv(corePrefix + name))
	}
	return core
}

// Text returns a text or choice setting.
func (r *reader) Text(key string) string {
	return r.lookup(key, settings.TypeText).Text
}

// Count returns a count setting.
func (r *reader) Count(key string) int {
	return r.lookup(key, settings.TypeCount).Count
}

// Flag returns a flag setting.
func (r *reader) Flag(key string) bool {
	return r.lookup(key, settings.TypeFlag).Flag
}

// Duration returns a duration setting.
func (r *reader) Duration(key string) time.Duration {
	return r.lookup(key, settings.TypeDuration).Duration
}

// Lines returns a lines setting as a new slice on every call.
func (r *reader) Lines(key string) []string {
	return slices.Clone(r.lookup(key, settings.TypeLines).Lines)
}

// Secret returns a secret setting.
func (r *reader) Secret(key string) string {
	return r.lookup(key, settings.TypeSecret).Secret.Reveal()
}

// Core returns a core setting from the list the site publishes, such as TRUSTED_PROXIES.
func (r *reader) Core(name string) string {
	value, listed := r.set.core[name]
	if !listed {
		r.warn(offList, name, "name", name)
	}
	return value
}

// lookup returns the value of key when the reader read takes its type, else the zero value with the misread logged.
func (r *reader) lookup(key, read string) settings.Value {
	value, declared := r.values[key]
	if !declared {
		r.warn(undeclared, key, "key", key)
		return settings.Value{}
	}
	if readerOf(value.Type) != read {
		r.warn(otherType, key, "key", key, "type", value.Type, "read", read)
		return settings.Value{}
	}
	return value
}

// warn logs message with the plugin's id and attrs at error level the first time the plugin misreads key that way.
func (r *reader) warn(message, key string, attrs ...any) {
	if _, logged := r.set.logged.LoadOrStore(misread{plugin: r.id, message: message, key: key}, true); logged {
		return
	}
	r.set.logger.Error(message, append([]any{"plugin", r.id}, attrs...)...)
}

// readerOf returns the reader of a setting of type kind, the text reader for a choice.
func readerOf(kind string) string {
	if kind == settings.TypeChoice {
		return settings.TypeText
	}
	return kind
}
