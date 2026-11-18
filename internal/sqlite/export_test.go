// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"database/sql"
	"testing"

	"github.com/gopherium/gophenberg/internal/content/contentstore"
	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// RaiseFrom is raiseFrom, for the tests of a raise another binary got to first.
var RaiseFrom = raiseFrom

// GiveFolder is giveFolder, for the tests of a folder another user swapped for a link.
var GiveFolder = giveFolder

// SettingsLookup is settingsLookup, for the tests of a claim whose look for the settings table fails.
const SettingsLookup = settingsLookup

// BuildsLookup is buildsLookup, for the tests of a claim whose read of the builds fails.
const BuildsLookup = buildsLookup

// FieldDefinitions returns the field definition reads the content store sends on the handle.
func FieldDefinitions(handle *sql.DB) contentstore.DefinitionQueries {
	return typeQueries{queries: db.New(handle)}
}

// TypeStatements returns the type store's calls as the adapter sends them on the handle.
func TypeStatements(handle *sql.DB) typestore.Queries {
	return typeQueries{queries: db.New(handle)}
}

// TypeRules is typeRulesBroken, for the tests of the rule each SQLite failure of a type write names.
var TypeRules = typeRulesBroken

// ReadError is readError, for the tests of what the rules read from a SQLite failure.
var ReadError = readError

// RunAsRoot makes the backups see the process as root until t ends.
func RunAsRoot(t *testing.T) {
	ran := geteuid
	geteuid = func() int { return 0 }
	t.Cleanup(func() { geteuid = ran })
}
