// SPDX-License-Identifier: Apache-2.0

package sdk

import "time"

// Settings reads the settings a plugin.json declares, with no error and no query, which only the site implements.
type Settings interface {
	// Text returns a text or choice setting.
	Text(key string) string
	// Count returns a count setting.
	Count(key string) int
	// Flag returns a flag setting.
	Flag(key string) bool
	// Duration returns a duration setting.
	Duration(key string) time.Duration
	// Lines returns a lines setting as a new slice on every call.
	Lines(key string) []string
	// Secret returns a secret setting.
	Secret(key string) string
	// Core returns a core setting from the list the site publishes, such as TRUSTED_PROXIES.
	Core(name string) string
	hostOnly()
}
