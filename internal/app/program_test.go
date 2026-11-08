// SPDX-License-Identifier: Apache-2.0

package app

import (
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/version"
)

// listedCommands are the names the listing of the command line holds, in the order it prints them.
var listedCommands = []string{
	"check", "help", "list", "migrate", "seed", "serve", "version",
	"account:create-admin", "account:disable", "account:enable", "account:grant-role", "account:list",
	"account:records", "account:role", "database:backup", "database:create-owner", "server:install", "server:uninstall",
	"update:apply", "update:check", "update:rollback", "update:status",
}

func TestProgramListsEveryCommandWhenNoneIsNamed(t *testing.T) {
	t.Parallel()

	got := testkit.Run(t, Program(testkit.Getenv(nil), noPlugins), "")

	want := strings.Join([]string{
		"Gophenberg Version " + version.Version(),
		"",
		"Usage:",
		"  gophenberg <command> [flags] [arguments]",
		"",
		"Every command answers -h. A command that offers -json answers one JSON document. " +
			"A command that offers -yes is a dry run until -yes.",
		"",
		"Available commands:",
		"  about                  print the version, the build stamp, the commit and the theme kits of this binary",
		"  check                  check every setting, every plugin and every command name",
		"  help                   print the help of one command",
		"  list                   list every command",
		"  migrate                apply every schema step",
		"  seed                   store the demo data",
		"  serve                  run the server",
		"  version                print the version",
		" account",
		"  account:create-admin   create an account under a role",
		"  account:disable        disable one account",
		"  account:enable         enable one disabled account",
		"  account:grant-role     give a role to every account holding none",
		"  account:list           list every account with its role",
		"  account:records        list who applied which change, the newest first",
		"  account:role           set one account's role",
		" database",
		"  database:backup        back up the SQLite file into the backups folder",
		"  database:create-owner  create the role the site logs in as and give it the database, as a superuser",
		" server",
		"  server:install         install Gophenberg on this Linux server with PostgreSQL, Caddy and a systemd service",
		"  server:uninstall       remove what server:install set up on this server, keeping the site's data unless " +
			"-delete-data",
		" update",
		"  update:apply           update this server install to a newer Gophenberg, then ask for a restart",
		"  update:check           check the update site for a newer Gophenberg and keep what it found",
		"  update:rollback        take back the last update, pointing current at the build before it",
		"  update:status          show the builds of this server install, its last update and its backups",
		"",
		"Every command is described at https://docs.gophenberg.org/self-hosting/commands/",
		"",
	}, "\n")
	if got.Code != gonsole.ExitDone || got.Stdout != want || got.Stderr != "" {
		t.Errorf("bare run = %d, stdout %q, stderr %q, want 0 and the listing\n%s", got.Code, got.Stdout, got.Stderr, want)
	}
}

func TestProgramRefusesTheCommandNamesFromBeforeTheCommandLine(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"createadmin", "grantrole"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := testkit.Run(t, Program(testkit.Getenv(nil), noPlugins), "", name)

			if got.Code != gonsole.ExitMisused || !strings.Contains(got.Stderr, `unknown command "`+name+`"`) {
				t.Errorf("%s = %d with stderr %q, want 2 and the unknown command named", name, got.Code, got.Stderr)
			}
		})
	}
}

func TestProgramAnswersEveryHelpPageWithoutADatabase(t *testing.T) {
	t.Parallel()

	for _, name := range listedCommands {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := testkit.Run(t, Program(testkit.Getenv(nil), noPlugins), "", "help", name)

			if got.Code != gonsole.ExitDone || !strings.Contains(got.Stdout, "gophenberg "+name) || got.Stderr != "" {
				t.Errorf("help %s = %d, stdout %q, stderr %q, want 0 and its page", name, got.Code, got.Stdout, got.Stderr)
			}
		})
	}
}

func TestCheckNamesAMalformedSetting(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":         unreachableDatabaseURL,
		"GOPHENBERG_THEME_START_ATTEMPTS": "many",
	})

	got := testkit.Run(t, Program(env, noPlugins), "", "check")

	if got.Code != gonsole.ExitFailed || !strings.Contains(got.Stderr, "GOPHENBERG_THEME_START_ATTEMPTS") {
		t.Errorf("check = %d with stderr %q, want 1 and the setting named", got.Code, got.Stderr)
	}
}

func TestCheckNamesAMalformedRecordSetting(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL":           unreachableDatabaseURL,
		"GOPHENBERG_COMMAND_RECORD_TIMEOUT": "soon",
	})

	got := testkit.Run(t, Program(env, noPlugins), "", "check")

	if got.Code != gonsole.ExitFailed || !strings.Contains(got.Stderr, "GOPHENBERG_COMMAND_RECORD_TIMEOUT") {
		t.Errorf("check = %d with stderr %q, want 1 and the record setting named", got.Code, got.Stderr)
	}
}

func TestCheckPassesAValidSiteWithoutReachingTheDatabase(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL})

	got := testkit.Run(t, Program(env, noPlugins), "", "check")

	if got.Code != gonsole.ExitDone || got.Stdout != "settings, plugins and command names are valid\n" {
		t.Errorf("check = %d, stdout %q, stderr %q, want 0 and every setting valid", got.Code, got.Stdout, got.Stderr)
	}
}

func TestProgramSeedOnlyPreviewsUntilYes(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"seed"}, {"seed", "--"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			t.Parallel()

			databaseURL := emptyDatabaseURL(t)
			env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL})

			got := testkit.Run(t, Program(env, noPlugins), "", args...)

			if got.Code != gonsole.ExitDone || got.Stdout != "would store the demo data\n" ||
				!strings.Contains(got.Stderr, "dry run, nothing changed, pass -yes to apply") {
				t.Errorf("%q = %d, stdout %q, stderr %q, want 0 and a preview", args, got.Code, got.Stdout, got.Stderr)
			}
			for _, schema := range []string{"auth", "core"} {
				if schemaHeld(t, databaseURL, schema) {
					t.Errorf("the %s schema exists after a preview, want nothing written", schema)
				}
			}
		})
	}
}
