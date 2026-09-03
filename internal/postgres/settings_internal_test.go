// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"slices"
	"testing"
)

func TestLockedKeysOrdersEveryWriteTheSameWay(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"theme.previous":     "starter",
		"content.per_page":   "5",
		"media.jpeg_quality": "30",
		"theme.active":       "driftwood",
	}
	want := []string{"content.per_page", "media.jpeg_quality", "theme.active", "theme.previous"}

	for range 20 {
		if held := lockedKeys(values); !slices.Equal(held, want) {
			t.Fatalf("lockedKeys() = %v, want %v every time", held, want)
		}
	}
}

func TestLockedKeysHoldsNothingForAnEmptyWrite(t *testing.T) {
	t.Parallel()

	if held := lockedKeys(map[string]string{}); len(held) != 0 {
		t.Errorf("lockedKeys() = %v, want nothing to lock", held)
	}
}
