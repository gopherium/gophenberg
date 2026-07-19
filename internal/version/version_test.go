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
