// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// listedNames returns the names of the listed themes in the order they are listed.
func listedNames(listed []themehost.Installed) []string {
	names := make([]string, 0, len(listed))
	for _, theme := range listed {
		names = append(names, theme.Name)
	}
	return names
}

func TestTheManagerListsTheActiveThemeFirstAndTheRestInNaturalNameOrder(t *testing.T) {
	t.Parallel()

	settings := newSettings()
	settings.values[themehost.ActiveKey] = "theme10"
	manager := managerOver(t, settings, "", "alba", "Álvaro", "Zoe", "theme2", "theme10")

	listed, err := manager.List(t.Context())

	if err != nil {
		t.Fatalf("List() = %v, want the themes listed", err)
	}
	want := []string{"theme10", "alba", "theme2", "Zoe", "Álvaro"}
	if got := listedNames(listed); !slices.Equal(got, want) {
		t.Errorf("List() names = %v, want %v", got, want)
	}
	if len(listed) == 0 || !listed[0].Active {
		t.Errorf("List() = %v, want the first theme listed as the active one", listed)
	}
}

func TestTheManagerListsThemesInNaturalNameOrderWhenNoneIsActive(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "theme10", "Zoe", "theme2", "Álvaro", "alba")

	listed, err := manager.List(t.Context())

	if err != nil {
		t.Fatalf("List() = %v, want the themes listed", err)
	}
	want := []string{"alba", "theme2", "theme10", "Zoe", "Álvaro"}
	if got := listedNames(listed); !slices.Equal(got, want) {
		t.Errorf("List() names = %v, want %v", got, want)
	}
}

func TestTheManagerListsNamesThatDifferOnlyInLetterFormsByTheirNumbers(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "ς10", "σ2")

	listed, err := manager.List(t.Context())

	if err != nil {
		t.Fatalf("List() = %v, want the themes listed", err)
	}
	want := []string{"σ2", "ς10"}
	if got := listedNames(listed); !slices.Equal(got, want) {
		t.Errorf("List() names = %v, want %v", got, want)
	}
}
