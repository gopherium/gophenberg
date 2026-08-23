// SPDX-License-Identifier: Apache-2.0

package themehost

import "testing"

func TestParseKitReadsAPlainVersion(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		declared string
		want     kitVersion
	}{
		{declared: "0.9.0", want: kitVersion{0, 9, 0}},
		{declared: "1.0.0", want: kitVersion{1, 0, 0}},
		{declared: "12.34.56", want: kitVersion{12, 34, 56}},
	} {
		t.Run(tc.declared, func(t *testing.T) {
			t.Parallel()

			got, ok := parseKit(tc.declared)

			if !ok {
				t.Fatalf("parseKit(%q) refused a plain version", tc.declared)
			}
			if got != tc.want {
				t.Errorf("parseKit(%q) = %+v, want %+v", tc.declared, got, tc.want)
			}
		})
	}
}

func TestParseKitRefusesAnythingButAPlainVersion(t *testing.T) {
	t.Parallel()

	refused := []string{
		"", "^0.1.0", "~0.9.0", "0.9", "0.9.0.1",
		"v0.9.0", "latest", "0.9.x", "a.b.c", "0.9.0-beta",
	}
	for _, declared := range refused {
		t.Run(declared, func(t *testing.T) {
			t.Parallel()

			if _, ok := parseKit(declared); ok {
				t.Errorf("parseKit(%q) accepted a version that is not three plain numbers", declared)
			}
		})
	}
}

func TestServesFollowsTheCompatibilityRule(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		served   kitVersion
		declared kitVersion
		want     bool
	}{
		{name: "the same version", served: kitVersion{0, 9, 0}, declared: kitVersion{0, 9, 0}, want: true},
		{name: "an earlier minor while at 0.x", served: kitVersion{0, 9, 0}, declared: kitVersion{0, 8, 0}, want: false},
		{name: "a later minor while at 0.x", served: kitVersion{0, 9, 0}, declared: kitVersion{0, 10, 0}, want: false},
		{name: "an earlier patch while at 0.x", served: kitVersion{0, 9, 2}, declared: kitVersion{0, 9, 1}, want: true},
		{name: "a later patch while at 0.x", served: kitVersion{0, 9, 0}, declared: kitVersion{0, 9, 1}, want: false},
		{name: "an earlier minor once past 1.0", served: kitVersion{1, 4, 0}, declared: kitVersion{1, 2, 0}, want: true},
		{name: "a later minor once past 1.0", served: kitVersion{1, 2, 0}, declared: kitVersion{1, 4, 0}, want: false},
		{name: "an earlier patch once past 1.0", served: kitVersion{1, 2, 3}, declared: kitVersion{1, 2, 1}, want: true},
		{name: "a different major", served: kitVersion{1, 0, 0}, declared: kitVersion{2, 0, 0}, want: false},
		{name: "an earlier major", served: kitVersion{2, 0, 0}, declared: kitVersion{1, 0, 0}, want: false},
		{name: "zero against one", served: kitVersion{1, 0, 0}, declared: kitVersion{0, 9, 0}, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := serves(tc.served, tc.declared); got != tc.want {
				t.Errorf("serves(%+v, %+v) = %t, want %t", tc.served, tc.declared, got, tc.want)
			}
		})
	}
}
