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

// RunAsRoot makes the backups see the process as root until t ends.
func RunAsRoot(t *testing.T) {
	ran := geteuid
	geteuid = func() int { return 0 }
	t.Cleanup(func() { geteuid = ran })
}

// FailModes makes every mode a backup sets answer err until t ends.
func FailModes(t *testing.T, err error) {
	set := chmod
	chmod = func(string, os.FileMode) error { return err }
	t.Cleanup(func() { chmod = set })
}
