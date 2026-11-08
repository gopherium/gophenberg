// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// switchedFrom is the build an update switched away from in the boot guard tests, to the version of this binary.
const switchedFrom = "0.18.0"

// switchedRoot returns a new install root where an update switched from 0.18.0 to this version, its state edited.
func switchedRoot(t *testing.T, edit func(*updater.State)) string {
	t.Helper()
	root := t.TempDir()
	if err := switchedInstall(root, switchedFrom, version.Version()); err != nil {
		t.Fatalf("making the install root: %v", err)
	}
	if err := editState(root, edit); err != nil {
		t.Fatalf("editing the update state: %v", err)
	}
	return root
}

// triedThreeTimes records that the build the update switched to already tried to start three times.
func triedThreeTimes(state *updater.State) {
	state.Attempts = 3
}

// runServe runs serve through the command line over env and returns what it answered, with the code main exits with.
func runServe(t *testing.T, env map[string]string) testkit.Result {
	t.Helper()
	exits := &exitCodes{}
	result := testkit.Run(t, commandLine(testkit.Getenv(env), noPlugins, exits, version.Stamp()), "", "serve")
	result.Code = exits.of(result.Code)
	return result
}

func TestServeGoesBackBeforeTheDatabaseOpensOnceTheBuildTriedToStartTooOften(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, triedThreeTimes)

	result := runServe(t, map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, installRootSetting: root,
	})

	want := "gophenberg: updater: Gophenberg " + version.Version() + " failed its start check after the " +
		"update from 0.18.0: it tried to start 4 times and GOPHENBERG_UPDATE_ATTEMPTS allows 3\n"
	if result.Code != exitFailedCheck || !strings.HasPrefix(result.Stderr, want) {
		t.Errorf("serve = %d with stderr %q, want %d and the failed check first", result.Code, result.Stderr,
			exitFailedCheck)
	}
	held, err := os.Readlink(filepath.Join(root, updater.Current))
	if held != filepath.Join("versions", switchedFrom) {
		t.Errorf("the current link names %q (%v), want the build before the update", held, err)
	}
}

func TestServeRefusesAnUpdateStateItCannotRead(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, triedThreeTimes)
	if err := os.WriteFile(updater.Layout{Root: root}.StateFile(), []byte("step: switched"), 0o644); err != nil {
		t.Fatalf("writing the state file: %v", err)
	}

	result := runServe(t, map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, installRootSetting: root,
	})

	if result.Code != gonsole.ExitFailed || !strings.Contains(result.Stderr, "updater: read the update state") {
		t.Errorf("serve = %d with stderr %q, want 1 and the state file named", result.Code, result.Stderr)
	}
}

func TestServeRefusesAnUpdateSettingOutOfRange(t *testing.T) {
	t.Parallel()

	result := runServe(t, map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_UPDATE_ATTEMPTS": "0",
	})

	if result.Code != gonsole.ExitFailed || !strings.Contains(result.Stderr, "GOPHENBERG_UPDATE_ATTEMPTS:") {
		t.Errorf("serve = %d with stderr %q, want 1 and the setting named", result.Code, result.Stderr)
	}
}

func TestServeKeepsAnUpdateWhoseFirstStartAnswersItsCheck(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, func(*updater.State) {})
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptyDatabaseURL(t), "GOPHENBERG_ADDR": "127.0.0.1:0", installRootSetting: root,
	}
	ctx, cancel := context.WithTimeout(t.Context(), siteStartLimit)
	defer cancel()
	stderr := &keptWatch{cancel: cancel}

	err := run(ctx, testkit.Getenv(env), stderr, noPlugins)

	if err != nil || ctx.Err() == nil {
		t.Fatalf("run() = %v with stderr %q, want the update kept and then a clean stop", err, stderr)
	}
	state, err := updater.Layout{Root: root}.ReadState()
	want := updater.State{Step: updater.StepCommitted, From: switchedFrom, To: version.Version(), Attempts: 1}
	if err != nil || state.Step != want.Step || state.Attempts != want.Attempts || state.To != want.To {
		t.Errorf("the update state is %+v (%v), want %+v", state, err, want)
	}
}

func TestServeMarksThatTheNewBuildReachedTheDatabaseBeforeItAsksAboutTheRole(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	root := switchedRoot(t, func(*updater.State) {})

	result := runServe(t, map[string]string{
		"GOPHENBERG_DATABASE_URL": testdb.AsServer(t, databaseURL), "GOPHENBERG_SUPERUSER_POLICY": "refuse",
		installRootSetting: root,
	})

	if result.Code != gonsole.ExitFailed || !strings.Contains(result.Stderr, errReachesServer.Error()) {
		t.Errorf("serve = %d with stderr %q, want 1 and the role refused", result.Code, result.Stderr)
	}
	state, err := updater.Layout{Root: root}.ReadState()
	if err != nil {
		t.Fatalf("reading the update state: %v", err)
	}
	if state.Attempts != 1 || !state.DatabaseReached {
		t.Errorf("the update state is %+v, want the start counted and the database marked reached", state)
	}
	if got := storedMinimumVersionAt(t, databaseURL); got != "" {
		t.Errorf("stored minimum version = %q once serve refused the role, want nothing migrated", got)
	}
}

func TestServeMarksNothingWhenTheDatabaseNeverAnswers(t *testing.T) {
	t.Parallel()

	root := switchedRoot(t, func(*updater.State) {})

	result := runServe(t, map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_DATABASE_WAIT": "0", installRootSetting: root,
	})

	if result.Code != gonsole.ExitFailed || !strings.Contains(result.Stderr, "the database did not answer within 0s") {
		t.Errorf("serve = %d with stderr %q, want 1 and the wait run out", result.Code, result.Stderr)
	}
	state, err := updater.Layout{Root: root}.ReadState()
	if err != nil {
		t.Fatalf("reading the update state: %v", err)
	}
	if state.Attempts != 1 || state.DatabaseReached {
		t.Errorf("the update state is %+v, want the start counted and the database not marked", state)
	}
}

// guardedBoot returns a start on the database at databaseURL whose boot guard counted a start on the install root.
func guardedBoot(t *testing.T, databaseURL, root string) boot {
	t.Helper()
	handle, err := openPostgres(settingsEnv(testkit.Getenv(nil)), databaseURL)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	settings := timedConfig()
	settings.serving, settings.databaseURL = servingDefaults, databaseURL
	settings.install, err = updater.SettingsFrom(settingsEnv(testkit.Getenv(map[string]string{
		installRootSetting: root,
	})))
	if err != nil {
		t.Fatalf("reading the install settings: %v", err)
	}
	guard, err := updater.Start(settings.install, version.Version(), testLogger(io.Discard))
	if err != nil {
		t.Fatalf("Start() = %v, want the start counted", err)
	}
	return boot{settings: settings, built: site{pgHandle: handle}, host: pluginkit.NewHost(),
		logger: testLogger(io.Discard), guard: guard}
}

func TestReadyStopsBeforeItMigratesWhenItCannotMarkTheDatabaseReached(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which writes to a folder whatever its mode says")
	}
	databaseURL := emptyDatabaseURL(t)
	start := guardedBoot(t, databaseURL, switchedRoot(t, func(*updater.State) {}))
	folder := filepath.Dir(updater.Layout{Root: start.settings.install.Root}.StateFile())
	if err := os.Chmod(folder, 0o500); err != nil {
		t.Fatalf("locking %q: %v", folder, err)
	}
	t.Cleanup(func() { _ = os.Chmod(folder, 0o755) })

	err := start.reachDatabase(t.Context())

	if err == nil || !strings.Contains(err.Error(), "updater: write the update state") {
		t.Errorf("reachDatabase() = %v, want the mark it could not write", err)
	}
	if got := storedMinimumVersionAt(t, databaseURL); got != "" {
		t.Errorf("stored minimum version = %q after the mark failed, want nothing migrated", got)
	}
}

func TestReadyStopsBeforeItMigratesALongJumpTakenBackWhileItWaitedForTheDatabase(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	start := guardedBoot(t, databaseURL, switchedRoot(t, func(state *updater.State) { state.LongJump = true }))
	if _, err := updater.Rollback(start.settings.install); err != nil {
		t.Fatalf("Rollback() = %v, want the long jump taken back before its build reached the database", err)
	}

	err := start.reachDatabase(t.Context())

	want := "updater: another command took back the update to Gophenberg " + version.Version() +
		" while this start waited for the database, so it stops before it migrates"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("reachDatabase() = %v, want %q", err, want)
	}
	if got := storedMinimumVersionAt(t, databaseURL); got != "" {
		t.Errorf("stored minimum version = %q after the update was taken back, want nothing migrated", got)
	}
}

func TestExitCodesAnswerTheCodeAnErrorAskedForOnlyInPlaceOfAFailure(t *testing.T) {
	t.Parallel()

	asked := &exitCodes{code: exitFailedCheck}
	for given, want := range map[int]int{
		gonsole.ExitDone:    gonsole.ExitDone,
		gonsole.ExitFailed:  exitFailedCheck,
		gonsole.ExitMisused: gonsole.ExitMisused,
	} {
		if got := asked.of(given); got != want {
			t.Errorf("of(%d) with %d kept = %d, want %d", given, exitFailedCheck, got, want)
		}
	}
	if got := (&exitCodes{}).of(gonsole.ExitFailed); got != gonsole.ExitFailed {
		t.Errorf("of(1) with no code kept = %d, want 1", got)
	}
}

func TestMainAnswersTheExitCodeTheBootGuardAsksFor(t *testing.T) {
	args := os.Args
	t.Cleanup(func() { os.Args = args })
	os.Args = []string{"gophenberg", "serve"}
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, installRootSetting: switchedRoot(t, triedThreeTimes),
	}

	if code := Main(testkit.Getenv(env), noPlugins); code != exitFailedCheck {
		t.Errorf("Main() = %d, want %d once the boot guard went back", code, exitFailedCheck)
	}
}
