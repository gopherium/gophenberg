// SPDX-License-Identifier: Apache-2.0

package version_test

import (
	"regexp"
	"testing"

	"github.com/gopherium/gophenberg/internal/version"
)

func TestVersionIsCleanSemver(t *testing.T) {
	t.Parallel()

	got := version.Version()

	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(got) {
		t.Errorf("Version() = %q, want a trimmed semver like 0.0.0", got)
	}
}

func TestGeneratorNamesTheProductAndItsFeatureVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{in: "1.2.3", want: "Gophenberg 1.2"},
		{in: "0.0.0", want: "Gophenberg 0.0"},
		{in: "10.20.30", want: "Gophenberg 10.20"},
	}
	for _, tc := range cases {
		if got := version.Generator(tc.in); got != tc.want {
			t.Errorf("Generator(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMajorMinorDropsThePatch(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{in: "1.2.3", want: "1.2"},
		{in: "0.0.0", want: "0.0"},
		{in: "10.20.30", want: "10.20"},
		{in: "1.2", want: "1.2"},
		{in: "1.2.3-beta.4", want: "1.2"},
		{in: "1", want: "1"},
		{in: "", want: ""},
	}
	for _, tc := range cases {
		if got := version.MajorMinor(tc.in); got != tc.want {
			t.Errorf("MajorMinor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
