// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"testing"

	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"
)

func TestEveryFreshConnectionCallsTheSiteFunctions(t *testing.T) {
	t.Parallel()

	settings := testSettings()
	db := sqlitetest.Open(t, settings.Options())

	for i, conn := range fullPool(t, db, settings.MaxConns) {
		var folded, field, layout string
		err := conn.QueryRowContext(t.Context(), `SELECT casefold('ÉCOLE'), `+
			`strip_field_path('{"a":1,"b":2}', '["a"]'), strip_layout('{"rows":["a","b"]}', '["rows","a"]')`).
			Scan(&folded, &field, &layout)
		if err != nil {
			t.Fatalf("calling the site functions on connection %d error = %v, want nil", i, err)
		}
		if folded != "école" || field != `{"b":2}` || layout != `{"rows":["b"]}` {
			t.Errorf("the site functions on connection %d = %q, %q and %q, want école, {\"b\":2} and {\"rows\":[\"b\"]}",
				i, folded, field, layout)
		}
	}
}

func TestFieldsContainOnAFreshConnection(t *testing.T) {
	t.Parallel()

	settings := testSettings()
	db := sqlitetest.Open(t, settings.Options())

	for i, conn := range fullPool(t, db, settings.MaxConns) {
		var held bool
		query := `SELECT fields_contain('{"count":4,"tags":["red"]}', '{"count":4.0,"tags":["red"]}')`
		if err := conn.QueryRowContext(t.Context(), query).Scan(&held); err != nil {
			t.Fatalf("fields_contain on connection %d error = %v, want nil", i, err)
		}
		if !held {
			t.Errorf("fields_contain on connection %d = false, want true for a filter the fields hold", i)
		}
	}
}

func TestOpenRegistersTheSiteFunctions(t *testing.T) {
	t.Parallel()

	address, _ := emptyFile(t)
	db := open(t, address, testSettings())

	var folded string
	if err := db.QueryRowContext(t.Context(), "SELECT casefold('Straße')").Scan(&folded); err != nil {
		t.Fatalf("SELECT casefold() error = %v, want nil", err)
	}
	if folded != "straße" {
		t.Errorf("casefold('Straße') = %q, want straße", folded)
	}
}

func TestTheSiteFunctionsServeAnIndexExpression(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())
	mustExec(t, db, "CREATE TABLE docs (doc TEXT)")

	for name, expression := range map[string]string{
		"folded": "casefold(doc)", "fields": `strip_field_path(doc, '["a"]')`, "layout": `strip_layout(doc, '["a"]')`,
	} {
		statement := "CREATE INDEX docs_" + name + "_idx ON docs (" + expression + ")"
		if _, err := db.ExecContext(t.Context(), statement); err != nil {
			t.Errorf("%s error = %v, want a function deterministic enough to index", statement, err)
		}
	}
}

func TestCasefoldFoldsTextByUnicodeSimpleFolding(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	for text, want := range map[string]string{
		"ÉCOLE": "école", "Straße": "straße", "K": "k", "ΣΑΣ": "σασ", "already folded": "already folded",
	} {
		var got string
		if err := db.QueryRowContext(t.Context(), "SELECT casefold(?)", text).Scan(&got); err != nil {
			t.Fatalf("casefold(%q) error = %v, want nil", text, err)
		}
		if got != want {
			t.Errorf("casefold(%q) = %q, want %q", text, got, want)
		}
	}
}

func TestCasefoldKeepsAValueThatIsNotText(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	var number, kind string
	var null *string
	err := db.QueryRowContext(t.Context(), "SELECT casefold(42), typeof(casefold(42)), casefold(NULL)").
		Scan(&number, &kind, &null)
	if err != nil {
		t.Fatalf("casefold() on a number and NULL error = %v, want nil", err)
	}
	if number != "42" || kind != "integer" || null != nil {
		t.Errorf("casefold(42) = %s of type %s and casefold(NULL) = %v, want 42 of type integer and NULL",
			number, kind, null)
	}
}
