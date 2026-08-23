// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"slices"
	"strconv"
	"strings"
)

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

// ServesKit reports whether this release serves a theme built on the declared kit version.
func ServesKit(declared string) bool {
	built, ok := parseKit(declared)
	if !ok {
		return false
	}
	for _, candidate := range servedKits {
		if served, ok := parseKit(candidate); ok && serves(served, built) {
			return true
		}
	}
	return false
}

// kitVersion is a theme kit version broken into the numbers it carries.
type kitVersion struct {
	major int
	minor int
	patch int
}

// parseKit returns the version a manifest declares, refusing anything but three plain numbers.
func parseKit(declared string) (kitVersion, bool) {
	parts := strings.Split(declared, ".")
	if len(parts) != 3 {
		return kitVersion{}, false
	}
	numbers := make([]int, 0, len(parts))
	for _, part := range parts {
		if !onlyDigits(part) {
			return kitVersion{}, false
		}
		number, err := strconv.Atoi(part)
		if err != nil {
			return kitVersion{}, false
		}
		numbers = append(numbers, number)
	}
	return kitVersion{major: numbers[0], minor: numbers[1], patch: numbers[2]}, true
}

// onlyDigits reports whether a version part is written as digits alone, with no sign or spacing.
func onlyDigits(part string) bool {
	if part == "" {
		return false
	}
	for _, character := range part {
		if character < '0' || character > '9' {
			return false
		}
	}
	return true
}

// serves reports whether a served kit answers everything a theme built on the declared one asks for.
func serves(served, declared kitVersion) bool {
	if served.major != declared.major {
		return false
	}
	if served.major == 0 && served.minor != declared.minor {
		return false
	}
	return compareKits(served, declared) >= 0
}

// compareKits returns a negative number when the first version is older, and a positive one when it is newer.
func compareKits(a, b kitVersion) int {
	if a.major != b.major {
		return a.major - b.major
	}
	if a.minor != b.minor {
		return a.minor - b.minor
	}
	return a.patch - b.patch
}
