// SPDX-License-Identifier: Apache-2.0

package themehost

import "slices"

// servedKits names the theme kit versions this release serves, oldest first.
var servedKits = []string{"0.9.0"}

// ServedKits returns the theme kit versions this release serves.
func ServedKits() []string {
	return slices.Clone(servedKits)
}

// NewestKit returns the newest theme kit version this release serves.
func NewestKit() string {
	return servedKits[len(servedKits)-1]
}
