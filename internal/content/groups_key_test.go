// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestGroupKeyFromShapesATitleLikeAKey(t *testing.T) {
	t.Parallel()

	for title, want := range map[string]string{
		"Event details":   "event-details",
		"Details":         "details",
		"Maria's picks":   "marias-picks",
		"  Spaced   out ": "spaced-out",
		"2024 plans":      "group-2024-plans",
		"!!!":             "untitled",
		"":                "untitled",
		"Été d'été":       "t-d-t",
	} {
		if got := content.GroupKeyFrom(title); got != want {
			t.Errorf("GroupKeyFrom(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestGroupKeyFromBoundsALongTitle(t *testing.T) {
	t.Parallel()

	long := strings.Repeat("a", 199) + " " + strings.Repeat("b", 50)

	got := content.GroupKeyFrom(long)

	if len(got) != 199 || strings.HasSuffix(got, "-") {
		t.Errorf("GroupKeyFrom(long) = %q with length %d, want 199 characters and no trailing dash", got, len(got))
	}
}
