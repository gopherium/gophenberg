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

// managerChoosing returns a manager over the named themes with the given theme stored as the choice.
func managerChoosing(t *testing.T, active string, names ...string) *themehost.Manager {
	t.Helper()

	settings := newSettings()
	settings.values[themehost.ActiveKey] = active
	return managerOver(t, settings, "", names...)
}

// wantListed fails the test unless the manager lists exactly the wanted names in order, returning what it listed.
func wantListed(t *testing.T, manager *themehost.Manager, want ...string) []themehost.Installed {
	t.Helper()

	listed, err := manager.List(t.Context())
	if err != nil {
		t.Fatalf("List() = %v, want the themes listed", err)
	}
	if got := listedNames(listed); !slices.Equal(got, want) {
		t.Fatalf("List() names = %v, want %v", got, want)
	}
	return listed
}

func TestTheManagerListsTheActiveThemeFirstAndTheRestInNaturalNameOrder(t *testing.T) {
	t.Parallel()

	manager := managerChoosing(t, "theme10", "alba", "Álvaro", "Zoe", "theme2", "theme10")

	listed := wantListed(t, manager, "theme10", "alba", "theme2", "Zoe", "Álvaro")

	if !listed[0].Active {
		t.Errorf("List() = %v, want the first theme listed as the active one", listed)
	}
}

func TestTheManagerListsAChoiceWhoseFilesAreGoneAheadOfTheInstalledThemes(t *testing.T) {
	t.Parallel()

	wantListed(t, managerChoosing(t, "zephyr", "alba", "theme2"), "zephyr", "alba", "theme2")
}

func TestTheManagerListsThemesInNaturalNameOrderWhenNoneIsActive(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "theme10", "Zoe", "theme2", "Álvaro", "alba")

	wantListed(t, manager, "alba", "theme2", "theme10", "Zoe", "Álvaro")
}

func TestTheManagerListsNamesThatDifferOnlyInLetterFormsByTheirNumbers(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "ς10", "σ2")

	wantListed(t, manager, "σ2", "ς10")
}

func TestTheManagerListsAPrefixFirstAndARunOpeningWithAZeroBeforeTheRest(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "theme3", "theme1", "the", "theme01", "theme", "theme2")

	wantListed(t, manager, "the", "theme", "theme01", "theme1", "theme2", "theme3")
}

func TestTheManagerListsDotlessAndDottedIApartFromI(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "ı2", "İ3", "i10")

	wantListed(t, manager, "i10", "İ3", "ı2")
}

func TestTheManagerKeepsNamesThatCompareEqualInByteOrder(t *testing.T) {
	t.Parallel()

	manager := managerOver(t, newSettings(), "", "alba", "Alba")

	wantListed(t, manager, "Alba", "alba")
}
