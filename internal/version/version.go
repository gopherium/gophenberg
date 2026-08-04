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

// product names the application wherever it identifies itself.
const product = "Gophenberg"

// Generator returns how the application names itself and its feature version to a reader.
func Generator(v string) string {
	return product + " " + MajorMinor(v)
}

// MajorMinor returns the major and minor parts of a version.
func MajorMinor(v string) string {
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
}
