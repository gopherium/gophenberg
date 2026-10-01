// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/jackc/pgx/v5/pgxpool"
)

// accountsSite returns the settings of a fresh database the command line migrated, holding an admin, an editor and
// an author.
func accountsSite(t *testing.T) func(string) string {
	t.Helper()
	getenv := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t)})
	if got := testkit.Run(t, program(getenv, noPlugins), "", "migrate"); got.Code != gonsole.ExitDone {
		t.Fatalf("migrate = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	for _, role := range []string{"admin", "editor", "author"} {
		got := testkit.Run(t, program(getenv, noPlugins), typedPassword+"\n",
			"account:create-admin", "-email", role+"@example.com", "-name", "Holder", "-role", role)
		if got.Code != gonsole.ExitDone {
			t.Fatalf("account:create-admin %s = %d with stderr %q, want 0", role, got.Code, got.Stderr)
		}
	}
	return getenv
}

// authorRole returns the role the author account holds on the database getenv names.
func authorRole(t *testing.T, getenv func(string) string) string {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), getenv("GOPHENBERG_DATABASE_URL"))
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	defer pool.Close()
	var held string
	err = pool.QueryRow(t.Context(), "SELECT role FROM auth.users WHERE email = 'author@example.com'").Scan(&held)
	if err != nil {
		t.Fatalf("reading the role of the author account: %v", err)
	}
	return held
}

// recordsOf returns the lines account:records lists on the database getenv names.
func recordsOf(t *testing.T, getenv func(string) string) []string {
	t.Helper()
	got := testkit.Run(t, program(getenv, noPlugins), "", "account:records")
	if got.Code != gonsole.ExitDone {
		t.Fatalf("account:records = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	return strings.FieldsFunc(got.Stdout, func(r rune) bool { return r == '\n' })
}

func TestAccountChangesWantTheActingAccount(t *testing.T) {
	t.Parallel()

	getenv := accountsSite(t)

	got := testkit.Run(t, program(getenv, noPlugins), "", "account:role", "author@example.com", "editor", "-yes")

	if got.Code != gonsole.ExitMisused || !strings.Contains(got.Stderr, "-as") {
		t.Errorf("account:role without -as = %d with stderr %q, want 2 and -as asked for", got.Code, got.Stderr)
	}
	if held := authorRole(t, getenv); held != "author" {
		t.Errorf("author@example.com holds %q, want its role unchanged", held)
	}
}

func TestAnEditorCannotChangeTheRoleOfAnAccount(t *testing.T) {
	t.Parallel()

	getenv := accountsSite(t)

	got := testkit.Run(t, program(getenv, noPlugins), "",
		"account:role", "author@example.com", "editor", "-yes", "-as", "editor@example.com")

	if got.Code != gonsole.ExitFailed || !strings.Contains(got.Stderr, "which lacks manage_users") {
		t.Errorf("account:role as an editor = %d with stderr %q, want 1 and the capability named", got.Code, got.Stderr)
	}
	if held := authorRole(t, getenv); held != "author" {
		t.Errorf("author@example.com holds %q, want its role unchanged", held)
	}
	if held := recordsOf(t, getenv); len(held) != 0 {
		t.Errorf("records = %q, want none", held)
	}
}

func TestAnAdminsRoleChangeIsRecorded(t *testing.T) {
	t.Parallel()

	getenv := accountsSite(t)

	got := testkit.Run(t, program(getenv, noPlugins), "",
		"account:role", "author@example.com", "editor", "-yes", "-as", "admin@example.com")

	if got.Code != gonsole.ExitDone {
		t.Fatalf("account:role as an admin = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	if held := authorRole(t, getenv); held != "editor" {
		t.Errorf("author@example.com holds %q, want editor", held)
	}
	held := recordsOf(t, getenv)
	if len(held) != 1 || !strings.Contains(held[0], "admin@example.com  account:role") {
		t.Errorf("records = %q, want the one change admin@example.com applied", held)
	}
}

func TestAPreviewOfAnAccountChangeRecordsNothing(t *testing.T) {
	t.Parallel()

	getenv := accountsSite(t)

	got := testkit.Run(t, program(getenv, noPlugins), "",
		"account:role", "author@example.com", "editor", "-as", "admin@example.com")

	if got.Code != gonsole.ExitDone || !strings.Contains(got.Stderr, "dry run, nothing changed") {
		t.Errorf("account:role preview = %d with stderr %q, want 0 and a dry run", got.Code, got.Stderr)
	}
	if held := authorRole(t, getenv); held != "author" {
		t.Errorf("author@example.com holds %q, want its role unchanged", held)
	}
	if held := recordsOf(t, getenv); len(held) != 0 {
		t.Errorf("records = %q, want none", held)
	}
}
