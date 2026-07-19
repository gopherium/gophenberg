// SPDX-License-Identifier: Apache-2.0

package testdb_test

import (
	"encoding/hex"
	"testing"

	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/testdb"
)

func TestMigratorCreatesAuthAndCoreSchemas(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}

	db := pgtestdb.New(t, testdb.Config(), testdb.Migrator())

	for _, schema := range []string{"auth", "core"} {
		var found bool
		err := db.QueryRow(
			"SELECT EXISTS (SELECT 1 FROM information_schema.schemata WHERE schema_name = $1)", schema,
		).Scan(&found)
		if err != nil {
			t.Fatalf("querying schemata for %q: %v", schema, err)
		}
		if !found {
			t.Errorf("schema %q not found after migrating", schema)
		}
	}

	var usersTable *string
	if err := db.QueryRow("SELECT to_regclass('auth.users')::text").Scan(&usersTable); err != nil {
		t.Fatalf("querying auth.users: %v", err)
	}
	if usersTable == nil {
		t.Fatal("auth.users not found: authkit migrations did not run before core ones")
	}
}

func TestMigratorHashIsStableHex(t *testing.T) {
	t.Parallel()

	first, err := testdb.Migrator().Hash()
	if err != nil {
		t.Fatalf("Hash() error = %v, want nil", err)
	}
	second, err := testdb.Migrator().Hash()
	if err != nil {
		t.Fatalf("second Hash() error = %v, want nil", err)
	}

	if first != second {
		t.Errorf("Hash() = %q then %q, want a stable fingerprint", first, second)
	}
	if _, err := hex.DecodeString(first); err != nil || first == "" {
		t.Errorf("Hash() = %q, want non-empty hex", first)
	}
}
