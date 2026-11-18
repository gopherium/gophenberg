// SPDX-License-Identifier: Apache-2.0

package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/themehost"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// officialInstall returns a scenario of an official build on an install root over a migrated database, offered 99.0.0.
func officialInstall(t *testing.T) *operatorScenario {
	t.Helper()
	s := &operatorScenario{t: t, engine: postgresOperators(), env: map[string]string{}, stamp: version.Stamp()}
	s.pointAtAnEmptyDatabase()
	if err := errors.Join(s.ranLine("migrate"), s.runOfficialBuild()); err != nil {
		t.Fatalf("readying the install: %v", err)
	}
	s.offerUpdateKeepingMinimum("99.0.0")
	return s
}

// wantAnswer fails t unless the result exited with code and its error stream carries each of texts.
func wantAnswer(t *testing.T, result testkit.Result, code int, texts ...string) {
	t.Helper()
	if result.Code != code {
		t.Errorf("the command exited with %d and stderr %q, want %d", result.Code, result.Stderr, code)
	}
	for _, text := range texts {
		if !strings.Contains(result.Stderr, text) {
			t.Errorf("stderr %q does not carry %q", result.Stderr, text)
		}
	}
}

// storeTheme stores name as the theme the site at address chose.
func storeTheme(t *testing.T, address, name string) {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), address)
	if err != nil {
		t.Fatalf("opening the database: %v", err)
	}
	defer pool.Close()
	if err := postgres.NewSettingStore(pool).Save(t.Context(), map[string]string{themehost.ActiveKey: name}); err != nil {
		t.Fatalf("storing the theme %s: %v", name, err)
	}
}

func TestUpdateApplyWantsTheVersionToRetryInTo(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:apply", "-retry")

	wantAnswer(t, got, gonsole.ExitMisused, "gophenberg: update:apply -retry takes the version to try again in -to")
}

func TestUpdateApplyNamesEverySettingItCannotUse(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_UPDATE_ATTEMPTS": "0", "GOPHENBERG_UPDATE_CHANNEL": "../beta"}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:apply")

	wantAnswer(t, got, gonsole.ExitFailed, "GOPHENBERG_UPDATE_ATTEMPTS:", "GOPHENBERG_UPDATE_CHANNEL:",
		"GOPHENBERG_DATABASE_URL")
}

func TestUpdateApplyRefusesAnAddressThatSizesThePool(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL + "&pool_max_conns=4"}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:apply")

	wantAnswer(t, got, gonsole.ExitFailed, "GOPHENBERG_DATABASE_URL: remove pool_max_conns from the address")
}

func TestUpdateApplyRefusesASQLiteSite(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_URL": emptySQLiteAddress(t)}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:apply")

	wantAnswer(t, got, gonsole.ExitFailed, "gophenberg: update:apply: this command runs only on a PostgreSQL database")
}

func TestUpdateApplyReportsADatabaseItCannotReadTheThemeFrom(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t)}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:apply")

	wantAnswer(t, got, gonsole.ExitFailed, `postgres: reading setting "theme.active"`)
}

func TestUpdateApplyRefusesAPinnedThemeThatDoesNotLoad(t *testing.T) {
	t.Parallel()

	s := officialInstall(t)
	s.env["GOPHENBERG_THEMES_DIR"], s.env["GOPHENBERG_THEME"] = t.TempDir(), "missing"

	s.runLine("update:apply")

	wantAnswer(t, s.result, gonsole.ExitFailed, "updater: the active theme does not load, so the start check of "+
		"Gophenberg 99.0.0 could not tell it from a theme the update broke")
}

func TestUpdateApplyRefusesAStoredThemeOnAKitTheReleaseDoesNotServe(t *testing.T) {
	t.Parallel()

	s := officialInstall(t)
	s.env["GOPHENBERG_THEMES_DIR"] = writeThemeDir(t, "starter")
	storeTheme(t, s.env["GOPHENBERG_DATABASE_URL"], "starter")

	s.runLine("update:apply")

	wantAnswer(t, s.result, gonsole.ExitFailed, "updater: the active theme is built on theme kit "+
		themehost.NewestKit()+", which Gophenberg 99.0.0 does not serve")
}

func TestUpdateApplyPrintsTheStateItSwitchedToAsOneDocument(t *testing.T) {
	t.Parallel()

	s := officialInstall(t)

	s.runLine("update:apply -json -skip-backup")

	wantAnswer(t, s.result, exitRestart, "gophenberg: restart Gophenberg to start 99.0.0")
	var state updater.State
	if err := json.Unmarshal([]byte(s.result.Stdout), &state); err != nil {
		t.Fatalf("update:apply -json answered %q, no JSON document: %v", s.result.Stdout, err)
	}
	want := updater.State{Step: updater.StepSwitched, From: version.Version(), To: "99.0.0"}
	if state.Step != want.Step || state.From != want.From || state.To != want.To || state.Backup != "" {
		t.Errorf("update:apply -json -skip-backup answered %+v, want %+v with no backup", state, want)
	}
	if !strings.Contains(s.result.Stdout, `"refused": []`) {
		t.Errorf("update:apply -json answered %q, want an empty list of refused versions", s.result.Stdout)
	}
}

func TestSwitchSummaryNamesTheBackupOrSaysThereIsNone(t *testing.T) {
	t.Parallel()

	switched := updater.State{From: "0.20.0", To: "0.21.0"}
	backedUp := switched
	backedUp.Backup = "backups/0.20.0-20261020T120000Z.dump"

	for state, want := range map[*updater.State]string{
		&switched: "switched from Gophenberg 0.20.0 to 0.21.0, with no database backup",
		&backedUp: "switched from Gophenberg 0.20.0 to 0.21.0, and the database backup is " +
			"/opt/gophenberg/backups/0.20.0-20261020T120000Z.dump",
	} {
		if got := switchSummary("/opt/gophenberg", *state); got != want {
			t.Errorf("switchSummary(%+v) = %q, want %q", *state, got, want)
		}
	}
}

func TestUpdateRollbackByHandAsksForARestartToStartTheBuildBeforeTheUpdate(t *testing.T) {
	t.Parallel()

	s := &operatorScenario{t: t, env: map[string]string{}, stamp: version.Stamp()}
	if err := s.switchBetween("", "99.0.0"); err != nil {
		t.Fatalf("making the install root: %v", err)
	}

	s.runLine("update:rollback")

	wantAnswer(t, s.result, exitRestart, "gophenberg: restart Gophenberg to start "+version.Version())
	want := "took back the update from Gophenberg " + version.Version() + " to 99.0.0, so current names " +
		version.Version() + " again\n"
	if s.result.Stdout != want {
		t.Errorf("update:rollback printed %q, want %q", s.result.Stdout, want)
	}
	if err := s.linksTo(updater.Current, ""); err != nil {
		t.Error(err)
	}
}

func TestUpdateRollbackAndStatusRefuseASettingOutOfRange(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"update:rollback", "update:status"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			env := map[string]string{"GOPHENBERG_UPDATE_VERSIONS_KEEP": "none"}

			got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", name)

			wantAnswer(t, got, gonsole.ExitFailed, "GOPHENBERG_UPDATE_VERSIONS_KEEP:")
		})
	}
}

func TestUpdateStatusReportsAStateFileItCannotRead(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, func(*updater.State) {})
	if err := os.WriteFile(updater.Layout{Root: root}.StateFile(), []byte("step: switched"), 0o644); err != nil {
		t.Fatalf("writing the state file: %v", err)
	}

	got := testkit.Run(t, Program(testkit.Getenv(map[string]string{installRootSetting: root}), noPlugins), "",
		"update:status")

	wantAnswer(t, got, gonsole.ExitFailed, "updater: read the update state")
}

func TestUpdateStatusOfAnInstallWithNoRootSaysItUpdatesByHand(t *testing.T) {
	t.Parallel()

	got := testkit.Run(t, Program(testkit.Getenv(nil), noPlugins), "", "update:status")

	want := "GOPHENBERG_INSTALL_ROOT is empty, so this install updates by hand, as " +
		"https://docs.gophenberg.org/self-hosting/updates-and-backups/ shows\n"
	if got.Code != gonsole.ExitDone || got.Stdout != want {
		t.Errorf("update:status = %d, %q, %q, want 0 and %q", got.Code, got.Stdout, got.Stderr, want)
	}
}

func TestUpdateStatusAnswersEmptyListsForNoRefusedVersionAndNoBackup(t *testing.T) {
	t.Parallel()

	for name, env := range map[string]map[string]string{
		"an install with no install root": nil,
		"an empty install root":           {installRootSetting: t.TempDir()},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:status", "-json")

			for _, want := range []string{`"refused": []`, `"backups": []`} {
				if got.Code != gonsole.ExitDone || !strings.Contains(got.Stdout, want) {
					t.Errorf("update:status -json = %d, %q, %q, want 0 and %s", got.Code, got.Stdout, got.Stderr, want)
				}
			}
		})
	}
}

func TestUpdateStatusPrintsEachPartOfTheStatusForPeople(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, func(state *updater.State) { state.Refused = []string{"0.17.5"} })
	backup := filepath.Join(root, switchBackup)
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		t.Fatalf("making the backups folder: %v", err)
	}
	if err := os.WriteFile(backup, []byte("a dump"), 0o600); err != nil {
		t.Fatalf("writing the backup: %v", err)
	}

	got := testkit.Run(t, Program(testkit.Getenv(map[string]string{installRootSetting: root}), noPlugins), "",
		"update:status")

	want := strings.Join([]string{
		"install root: " + root,
		"current build: " + version.Version(),
		"previous build: 0.18.0",
		"last update: from 0.18.0 to " + version.Version() + ", waits for the new build to start and pass its " +
			"start check",
		"refused versions: 0.17.5",
		"backups kept: " + backup,
		"",
	}, "\n")
	if got.Code != gonsole.ExitDone || got.Stdout != want {
		t.Errorf("update:status = %d, %q, %q, want 0 and\n%s", got.Code, got.Stdout, got.Stderr, want)
	}
}

func TestStatusTextNamesNoBuildNoRefusedVersionAndNoBackupAsNone(t *testing.T) {
	t.Parallel()

	got := statusText(settingsEnv(testkit.Getenv(nil)), updater.Status{Root: "/opt/gophenberg"})

	want := "install root: /opt/gophenberg\ncurrent build: none\nprevious build: none\nlast update: none yet\n" +
		"refused versions: none\nbackups kept: none\n"
	if got != want {
		t.Errorf("statusText() of an empty install root = %q, want %q", got, want)
	}
}

func TestStatusTextNamesWhenWhereAndWhyTheLastUpdateFailed(t *testing.T) {
	t.Parallel()

	failure := &updater.JobFailure{
		At: time.Date(2026, 10, 22, 9, 12, 0, 0, time.UTC), From: "0.20.0", To: "0.21.0", Step: updater.StepDownload,
		Error: "updater: refused it",
	}

	got := statusText(settingsEnv(testkit.Getenv(nil)), updater.Status{
		Root: "/opt/gophenberg", Update: updater.State{Failure: failure},
	})

	want := "install root: /opt/gophenberg\ncurrent build: none\nprevious build: none\nlast update: none yet\n" +
		"last failure: from 0.20.0 to 0.21.0, at the step download on 2026-10-22T09:12:00Z: updater: refused it\n" +
		"refused versions: none\nbackups kept: none\n"
	if got != want {
		t.Errorf("statusText() after a failed update = %q, want %q", got, want)
	}
}

func TestLastUpdateTextSaysWhereTheLastUpdateStands(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		state updater.State
		want  string
	}{
		{updater.State{Step: updater.StepSwitched, From: "0.20.0", To: "0.21.0", Refused: []string{"0.21.0"}},
			"from 0.20.0 to 0.21.0, the new build failed its start check, so current names 0.20.0 again"},
		{updater.State{Step: updater.StepCommitted, From: "0.20.0", To: "0.21.0"},
			"from 0.20.0 to 0.21.0, kept, since the new build passed its start check"},
		{updater.State{Step: updater.StepRolledBack, From: "0.20.0", To: "0.21.0"},
			"from 0.20.0 to 0.21.0, taken back, so current names the build before it again"},
		{updater.State{Step: "verified", From: "0.20.0", To: "0.21.0"}, "from 0.20.0 to 0.21.0, at the step verified"},
	} {
		if got := lastUpdateText(tc.state); got != tc.want {
			t.Errorf("lastUpdateText(%+v) = %q, want %q", tc.state, got, tc.want)
		}
	}
}
