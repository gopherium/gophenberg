// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"context"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// WordPress keeps the active theme on top and sorts the rest with case ignored
// and numbers in natural order. Sorting by raw bytes put a capitalised or
// accented name out of place and read theme10 before theme2 (#509).
func TestManagerListPutsTheActiveThemeFirstAndSortsTheRestNaturally(t *testing.T) {
	t.Parallel()

	settings := newSettings()
	settings.values[themehost.ActiveKey] = "theme2"
	manager := managerOver(t, settings, "", "Zoe", "theme10", "alba", "Álvaro", "theme2")

	installed, err := manager.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(installed))
	for i, theme := range installed {
		got[i] = theme.Name
	}
	want := []string{"theme2", "alba", "theme10", "Zoe", "Álvaro"}
	if len(got) != len(want) {
		t.Fatalf("listed %d themes, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("theme order = %v, want %v", got, want)
		}
	}
	if !installed[0].Active {
		t.Fatalf("the first theme is %q, not the active one", installed[0].Name)
	}
}
