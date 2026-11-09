// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"os"
	"testing"
)

// RaiseFrom is raiseFrom, for the tests of a raise another binary got to first.
var RaiseFrom = raiseFrom

// GiveFolder is giveFolder, for the tests of a folder another user swapped for a link.
var GiveFolder = giveFolder

// SettingsLookup is settingsLookup, for the tests of a claim whose look for the settings table fails.
const SettingsLookup = settingsLookup

// BuildsLookup is buildsLookup, for the tests of a claim whose read of the builds fails.
const BuildsLookup = buildsLookup

// otherThanRoot is a user id that is not root.
const otherThanRoot = 1000

// RunAsRoot makes the backups see the process as root until t ends.
func RunAsRoot(t *testing.T) {
	runAs(t, 0)
}

// RunAsUser makes the backups see the process as a user other than root until t ends.
func RunAsUser(t *testing.T) {
	runAs(t, otherThanRoot)
}

// runAs makes the backups see the process as the user euid until t ends.
func runAs(t *testing.T, euid int) {
	ran := geteuid
	geteuid = func() int { return euid }
	t.Cleanup(func() { geteuid = ran })
}

// FailModes makes every mode a backup sets answer err until t ends.
func FailModes(t *testing.T, err error) {
	set := chmod
	chmod = func(string, os.FileMode) error { return err }
	t.Cleanup(func() { chmod = set })
}
