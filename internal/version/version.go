// SPDX-License-Identifier: Apache-2.0

// Package version exposes application-wide metadata.
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var raw string

// Version returns the application version.
func Version() string {
	return strings.TrimSpace(raw)
}

// MajorMinor returns the major and minor parts of a version.
func MajorMinor(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
}
