// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/sdk"
)

// errInputRead is what standard input answers every command that reads it.
var errInputRead = errors.New("standard input was read")

// migratedSQLiteFile returns the path of a SQLite file migrate built every owner's schema in.
func migratedSQLiteFile(t *testing.T) string {
	t.Helper()
	path := emptySQLiteFile(t)
	if got := testkit.Run(t, Program(migratingSQLite(path), noPlugins), "", "migrate"); got.Code != gonsole.ExitDone {
		t.Fatalf("migrate = %d with stderr %q, want 0", got.Code, got.Stderr)
	}
	return path
}

// runUnread runs args through program over a standard input that fails every read.
func runUnread(t *testing.T, program gonsole.Program, args ...string) testkit.Result {
	t.Helper()
	var stdout, stderr strings.Builder
	code := program.Run(t.Context(), args, iotest.ErrReader(errInputRead), &stdout, &stderr)
	return testkit.Result{Code: code, Stdout: stdout.String(), Stderr: stderr.String()}
}

func TestEveryCommandNeedingTheWholeSiteNamesTheSQLiteError(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args  []string
		named string
	}{
		"serve":     {[]string{"serve"}, "serve"},
		"seed -yes": {[]string{"seed", "-yes"}, "seed"},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			program := Program(migratingSQLite(migratedSQLiteFile(t)), noPlugins)

			got := runUnread(t, program, tc.args...)

			want := "gophenberg: " + tc.named + ": content and types do not run on SQLite yet, " +
				"so this command needs GOPHENBERG_DATABASE_URL to name a PostgreSQL database\n"
			if got.Code != gonsole.ExitFailed || got.Stdout != "" || !strings.HasSuffix(got.Stderr, want) {
				t.Errorf("%q = %d, stdout %q, stderr %q, want 1 and %q last",
					tc.args, got.Code, got.Stdout, got.Stderr, want)
			}
		})
	}
}

func TestAccountCommandsNameWhatStopsThemOnASQLiteFile(t *testing.T) {
	t.Parallel()

	asAdmin := []string{"account:role", "author@example.com", "editor", "-yes", "-as", "admin@example.com"}
	unmigrated := saying("the command records are missing, run migrate first")
	tests := map[string]struct {
		args     []string
		file     func(*testing.T) string
		settings map[string]string
		says     func(path string) string
	}{
		"account:role on a missing file": {asAdmin, missingSQLiteFile, nil, namingTheMissingFile},
		"account:disable on a missing file": {
			[]string{"account:disable", "author@example.com", "-yes", "-as", "admin@example.com"},
			missingSQLiteFile, nil, namingTheMissingFile,
		},
		"account:grant-role on a missing file under a bad SQLite setting": {
			[]string{"account:grant-role", "-role", "editor", "-yes", "-as", "admin@example.com"},
			missingSQLiteFile, map[string]string{"GOPHENBERG_SQLITE_SYNCHRONOUS": "off"},
			saying("GOPHENBERG_SQLITE_SYNCHRONOUS"),
		},
		"account:records on a file never migrated": {[]string{"account:records"}, emptySQLiteFile, nil, unmigrated},
		"account:role on a file never migrated":    {asAdmin, emptySQLiteFile, nil, unmigrated},
		"account:list on a file never migrated":    {[]string{"account:list"}, emptySQLiteFile, nil, saying("")},
	}
	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			path := tc.file(t)
			_, before := os.Stat(path)
			settings := map[string]string{"GOPHENBERG_DATABASE_URL": sqlitePrefix + path}
			maps.Copy(settings, tc.settings)

			got := runUnread(t, Program(testkit.Getenv(settings), noPlugins), tc.args...)

			want := tc.says(path)
			if got.Code != gonsole.ExitFailed || got.Stdout != "" || !strings.Contains(got.Stderr, want) ||
				strings.Contains(got.Stderr, errPostgresOnly.Error()) {
				t.Errorf("%q = %d, stdout %q, stderr %q, want 1, %q and never %q",
					tc.args, got.Code, got.Stdout, got.Stderr, want, errPostgresOnly.Error())
			}
			if _, after := os.Stat(path); errors.Is(after, fs.ErrNotExist) != errors.Is(before, fs.ErrNotExist) {
				t.Errorf("the file after %q: %v, want it as it was before the command (%v)", tc.args, after, before)
			}
		})
	}
}

// missingSQLiteFile returns the path of a SQLite file that does not exist, in a folder of its own.
func missingSQLiteFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "site.db")
}

// namingTheMissingFile returns the line naming the missing SQLite file at path.
func namingTheMissingFile(path string) string {
	return sqlite.ErrNoFile.Error() + ": " + path
}

// saying returns the answer holding text whatever the file's path.
func saying(text string) func(string) string {
	return func(string) string { return text }
}

func TestAPaddedSQLiteAddressPicksSQLiteForEveryCommand(t *testing.T) {
	t.Parallel()

	padded := func(path string) func(string) string {
		return testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": "  " + sqlitePrefix + path + "  "})
	}
	missing := missingSQLiteFile(t)
	onFile, onMissing := padded(emptySQLiteFile(t)), padded(missing)

	migrated := testkit.Run(t, Program(onFile, noPlugins), "", "migrate")

	if migrated.Code != gonsole.ExitDone || migrated.Stdout != migratedLines {
		t.Errorf("migrate = %d, stdout %q, stderr %q, want 0 and %q",
			migrated.Code, migrated.Stdout, migrated.Stderr, migratedLines)
	}
	for _, tc := range []struct {
		getenv func(string) string
		args   []string
		code   int
		says   string
	}{
		{onFile, []string{"serve"}, gonsole.ExitFailed, "serve: " + errPostgresOnly.Error()},
		{onFile, []string{"account:list"}, gonsole.ExitDone, ""},
		{onMissing, []string{"account:role", "author@example.com", "editor", "-yes", "-as", "admin@example.com"},
			gonsole.ExitFailed, namingTheMissingFile(missing)},
	} {
		got := runUnread(t, Program(tc.getenv, noPlugins), tc.args...)
		if got.Code != tc.code || !strings.Contains(got.Stderr, tc.says) {
			t.Errorf("%q = %d with stderr %q, want %d and %q", tc.args, got.Code, got.Stderr, tc.code, tc.says)
		}
	}
}

func TestServeOnSQLiteAnswersBeforeTheSiteOrTheFileIsTouched(t *testing.T) {
	t.Parallel()

	path := emptySQLiteFile(t)
	registered := false
	plugins := listedAs("probe", func(sdk.Deps) (sdk.Plugin, error) {
		registered = true
		return idle("probe"), nil
	})

	got := testkit.Run(t, Program(migratingSQLite(path), plugins), "", "serve")

	if want := "serve: " + errPostgresOnly.Error(); got.Code != gonsole.ExitFailed ||
		!strings.Contains(got.Stderr, want) {
		t.Errorf("serve = %d with stderr %q, want 1 and %q", got.Code, got.Stderr, want)
	}
	if registered {
		t.Error("the plugins registered, want serve to answer before it builds the site")
	}
	if names := folderNames(t, filepath.Dir(path)); !slices.Equal(names, []string{"site.db"}) {
		t.Errorf("the folder holds %v after serve, want only site.db with no lock file", names)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("reading the file: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("the file holds %d bytes after serve, want 0 with nothing migrated", info.Size())
	}
}

func TestSeedOnSQLiteMigratesTheFileThenNamesTheSQLiteError(t *testing.T) {
	t.Parallel()

	path := emptySQLiteFile(t)
	k := &keeper{}

	got := testkit.Run(t, Program(migratingSQLite(path), keeping(k)), "", "seed", "-yes")

	if want := "seed: " + errPostgresOnly.Error(); got.Code != gonsole.ExitFailed ||
		!strings.Contains(got.Stderr, want) {
		t.Errorf("seed -yes = %d with stderr %q, want 1 and %q", got.Code, got.Stderr, want)
	}
	if want := []string{"migrate", "stop"}; !slices.Equal(k.steps(), want) {
		t.Errorf("the plugin was asked %v, want %v with no demo data stored", k.steps(), want)
	}
	for table, versions := range appliedVersions(t, openSQLiteFile(t, path)) {
		if !appliedOne(versions) {
			t.Errorf("%s holds the versions %v after seed -yes, want the file migrated first", table, versions)
		}
	}
}

func TestAccountReadsOnAnAddressPgxCannotParseHoldNoPartOfTheAddress(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": "postgres://postgres:secretword@localhost:notaport/postgres",
	})
	want := "gophenberg: postgres: open the database: pgx cannot parse the address or one of its settings\n"

	for _, command := range []string{"account:list", "account:records"} {
		t.Run(command, func(t *testing.T) {
			t.Parallel()

			got := testkit.Run(t, Program(env, noPlugins), "", command)

			if got.Code != gonsole.ExitFailed || got.Stdout != "" || got.Stderr != want {
				t.Errorf("%s = %d, stdout %q, stderr %q, want 1 and only %q",
					command, got.Code, got.Stdout, got.Stderr, want)
			}
		})
	}
}

func TestAccountRecordsOnAPostgreSQLDatabaseWithNoRecordsTableSaysToMigrate(t *testing.T) {
	t.Parallel()

	getenv := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t)})

	got := testkit.Run(t, Program(getenv, noPlugins), "", "account:records")

	want := "gophenberg: the command records are missing, run migrate first\n"
	if got.Code != gonsole.ExitFailed || got.Stderr != want {
		t.Errorf("account:records = %d, stderr %q, want 1 and %q", got.Code, got.Stderr, want)
	}
}

func TestAPostgreSQLAccountChangeStillReadsTheRolesOfTheRegisteredPlugins(t *testing.T) {
	t.Parallel()

	env := testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL})
	failing := listedAs("teams", func(sdk.Deps) (sdk.Plugin, error) { return nil, errors.New("a setting is missing") })

	got := testkit.Run(t, Program(env, failing), "",
		"account:role", "author@example.com", "editor", "-yes", "-as", "admin@example.com")

	if want := "register plugins: pluginkit: teams register: a setting is missing"; got.Code != gonsole.ExitFailed ||
		!strings.Contains(got.Stderr, want) {
		t.Errorf("account:role on PostgreSQL = %d with stderr %q, want 1 and %q", got.Code, got.Stderr, want)
	}
}

func TestTheSQLiteAccountStoresRunOnTheHandleOfTheRun(t *testing.T) {
	t.Parallel()

	runs := openSQLiteFile(t, migratedSQLiteFile(t))
	elsewhere := testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": sqlitePrefix + filepath.Join(t.TempDir(), "site.db"),
	})

	stores, release, err := accountStores(elsewhere)(t.Context(), gonsole.Call{}.WithDB(runs))

	if err != nil || release != nil {
		t.Fatalf("accountStores() = a release %v and %v, want the run's handle and nothing to release", release != nil, err)
	}
	if held, err := stores.Records.Held(t.Context()); err != nil || !held {
		t.Errorf("the records are held %v (%v), want the stores over the run's migrated handle", held, err)
	}
}
