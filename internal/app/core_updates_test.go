// SPDX-License-Identifier: Apache-2.0

package app

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// coreUpdatesFeature is the feature the updates of the core from the command line answer to.
const coreUpdatesFeature = "../../test/features/features/core-updates.feature"

// installRootSetting is the setting naming the install root a server install keeps its builds in.
const installRootSetting = "GOPHENBERG_INSTALL_ROOT"

// siteStartLimit is how long a scenario lets the site serve before it stops it, whatever the boot guard did.
const siteStartLimit = time.Minute

// switchBackup is the backup an update that raised the minimum version took before its switch from 0.18.0.
const switchBackup = "backups/0.18.0-20261020T120000Z.dump"

// officialStamp is the build stamp of a binary the release workflow built, the only one that updates itself.
const officialStamp = "official"

// initializeCoreUpdates binds the steps of the core update scenarios to s.
func initializeCoreUpdates(sc *godog.ScenarioContext, s *operatorScenario) {
	sc.Given(`^an install root running this version as a custom build$`, s.runCustomBuild)
	sc.Given(`^an install root running this version as an official build$`, s.runOfficialBuild)
	sc.Given(`^an official build with no install root, like the Docker image$`, s.runOfficialWithoutRoot)
	sc.Given(`^an update site offering version "([^"]*)" that keeps the minimum version of this binary$`,
		s.offerUpdateKeepingMinimum)
	sc.Given(`^the archive changed after it was released$`, s.changeArchive)
	sc.Given(`^only a stranger signed the archive$`, s.signArchiveAsStranger)
	sc.Given(`^version "([^"]*)" failed its start check after an update and went back$`, s.refuseVersion)
	sc.Given(`^an install root where an update switched from (?:this version|"([^"]*)") to `+
		`(?:this version|"([^"]*)")$`, s.switchBetween)
	sc.Given(`^(?:this version|the new build) already tried to start (\d+) times$`, s.triedToStart)
	sc.Given(`^the update raised the minimum version, with a backup taken before the switch$`, s.raiseMinimum)
	sc.Given(`^the new build passed its check$`, s.passCheck)
	sc.Given(`^the new build reached the database$`, s.markDatabaseReached)
	sc.Given(`^the site failed to start (\d+) times$`, s.failStarts)
	sc.Given(`^the start check of this version runs out of time$`, s.outlastCheck)
	sc.Given(`^a data folder outside the install root$`, s.nameDataFolder)
	sc.Given(`^a data folder keeps the backup and the state of the update$`, s.moveToDataFolder)
	sc.When(`^the site starts$`, s.startSite)
	sc.When(`^the service manager runs "([^"]*)" with (MONITOR_[A-Z_]+) set to "([^"]*)"$`, s.runAfterFailure)
	sc.Then(`^the command asks for a restart to start "([^"]*)"$`, s.asksForRestart)
	sc.Then(`^the (current|previous) build is (?:still )?(?:this version|"([^"]*)")$`, s.linksTo)
	sc.Then(`^the install root holds no download and no build of "([^"]*)"$`, s.holdsNoBuildOf)
	sc.Then(`^the install root keeps a database backup named after this version$`, s.keepsBackup)
	sc.Then(`^the data folder keeps a database backup named after this version$`, s.dataFolderKeepsBackup)
	sc.Then(`^the data folder keeps the state of the update$`, s.dataFolderKeepsState)
	sc.Then(`^the install root keeps no backup and no state$`, s.rootKeepsNothing)
	sc.Then(`^only the site's own user can open the data folder$`, s.dataFolderIsPrivate)
	sc.Then(`^the output names the backup in the data folder$`, s.outputNamesDataBackup)
	sc.Then(`^the error names the backup in the data folder$`, s.errorNamesDataBackup)
	sc.Then(`^the update status lists the backup in the data folder$`, s.statusListsDataBackup)
	sc.Then(`^the error names the backup taken before the switch$`, s.namesBackup)
	sc.Then(`^the update status says the update to this version is kept$`, s.statusKeepsUpdate)
	sc.Then(`^the update status lists (?:this version|version "([^"]*)") as refused$`, s.statusRefuses)
	sc.Then(`^the update status lists no refused version$`, s.statusRefusesNone)
	sc.Then(`^the update status names the backup taken before the switch$`, s.statusNamesBackup)
	initializeSiteUpdates(sc, s)
}

// runOfficialBuild runs this version as an official build from a new install root, backing up with a stand-in pg_dump.
func (s *operatorScenario) runOfficialBuild() error {
	s.stamp = officialStamp
	s.env["GOPHENBERG_PG_DUMP_BIN"] = standInDump(s.t)
	return s.runFromInstallRoot()
}

// runOfficialWithoutRoot runs this version as an official build with no install root.
func (s *operatorScenario) runOfficialWithoutRoot() {
	s.stamp = officialStamp
}

// standInDump writes a pg_dump stand-in that names a PostgreSQL newer than any server and dumps a few bytes.
func standInDump(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "pg_dump")
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then\n  echo 'pg_dump (PostgreSQL) 99.0'\n  exit 0\nfi\n" +
		"printf 'a dump of the site database'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("writing the pg_dump stand-in: %v", err)
	}
	return bin
}

// refuseVersion records that offered failed its start check after an update from this version, which took it back.
func (s *operatorScenario) refuseVersion(offered string) error {
	return editState(s.env[installRootSetting], func(state *updater.State) {
		*state = updater.State{
			Step: updater.StepRolledBack, From: version.Version(), To: offered, Attempts: 1, Refused: []string{offered},
		}
	})
}

// passCheck records that the build the update switched to passed its start check.
func (s *operatorScenario) passCheck() error {
	return editState(s.env[installRootSetting], func(state *updater.State) { state.Step = updater.StepCommitted })
}

// markDatabaseReached records that the build the update switched to reached the database at an earlier start.
func (s *operatorScenario) markDatabaseReached() error {
	return editState(s.env[installRootSetting], func(state *updater.State) { state.DatabaseReached = true })
}

// asksForRestart fails unless the command asked for a restart to start the version given.
func (s *operatorScenario) asksForRestart(release string) error {
	return s.errorSays("restart Gophenberg to start " + release)
}

// updateStatus returns the document update:status -json answers, leaving the result of the step before it alone.
func (s *operatorScenario) updateStatus() (updater.Status, error) {
	answered := s.answer("", "update:status", "-json")
	var status updater.Status
	if err := json.Unmarshal([]byte(answered.Stdout), &status); err != nil {
		return status, fmt.Errorf("update:status -json answered %q and %q, no JSON document: %w", answered.Stdout,
			answered.Stderr, err)
	}
	return status, nil
}

// statusKeepsUpdate fails unless update:status says the update to this version passed its start check.
func (s *operatorScenario) statusKeepsUpdate() error {
	status, err := s.updateStatus()
	if err != nil {
		return err
	}
	if status.Update.Step != updater.StepCommitted || status.Update.To != version.Version() {
		return fmt.Errorf("update:status says %+v, want the update to %s kept", status.Update, version.Version())
	}
	return nil
}

// statusRefuses fails unless update:status lists the version given, or this version for none, as refused.
func (s *operatorScenario) statusRefuses(release string) error {
	release = cmp.Or(release, version.Version())
	status, err := s.updateStatus()
	if err != nil {
		return err
	}
	if !slices.Contains(status.Update.Refused, release) {
		return fmt.Errorf("update:status refuses %q, want %s among them", status.Update.Refused, release)
	}
	return nil
}

// statusRefusesNone fails unless update:status lists no refused version.
func (s *operatorScenario) statusRefusesNone() error {
	status, err := s.updateStatus()
	if err != nil {
		return err
	}
	if len(status.Update.Refused) != 0 {
		return fmt.Errorf("update:status refuses %q, want none", status.Update.Refused)
	}
	return nil
}

// statusNamesBackup fails unless what update:status prints for people shows how to restore the backup by its full path.
func (s *operatorScenario) statusNamesBackup() error {
	want := "to bring the site back: restore the backup taken before the update, " +
		filepath.Join(s.env[installRootSetting], switchBackup)
	answered := s.answer("", "update:status")
	if !strings.Contains(answered.Stdout, want) {
		return fmt.Errorf("update:status printed %q and %q, want it to carry %q", answered.Stdout, answered.Stderr,
			want)
	}
	return nil
}

// switchedInstall makes root an install root where an update switched from the build from to the build to.
func switchedInstall(root, from, to string) error {
	for _, name := range []string{from, to} {
		if err := placeBuild(root, name); err != nil {
			return err
		}
	}
	layout := updater.Layout{Root: root}
	return errors.Join(layout.Point(updater.Previous, from), layout.Point(updater.Current, to),
		layout.WriteState(updater.State{Step: updater.StepSwitched, From: from, To: to}))
}

// editState rewrites the update state the install root at root holds through edit.
func editState(root string, edit func(*updater.State)) error {
	layout := updater.Layout{Root: root}
	state, err := layout.ReadState()
	if err != nil {
		return err
	}
	edit(&state)
	return layout.WriteState(state)
}

// switchBetween points the settings at a new install root switched from from to to, this version for none.
func (s *operatorScenario) switchBetween(from, to string) error {
	root := s.t.TempDir()
	s.env[installRootSetting] = root
	return switchedInstall(root, cmp.Or(from, version.Version()), cmp.Or(to, version.Version()))
}

// triedToStart records that the build the update switched to already tried to start the times given.
func (s *operatorScenario) triedToStart(times int) error {
	return editState(s.env[installRootSetting], func(state *updater.State) { state.Attempts = times })
}

// raiseMinimum records that the update raised the minimum version, with the backup it took before the switch.
func (s *operatorScenario) raiseMinimum() error {
	root := s.env[installRootSetting]
	backup := filepath.Join(root, switchBackup)
	if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(backup, []byte("a dump of the site database"), 0o600); err != nil {
		return err
	}
	return editState(root, func(state *updater.State) { state.LongJump, state.Backup = true, switchBackup })
}

// startSite serves the site over the scenario's settings until the boot guard keeps the update or the start ends.
func (s *operatorScenario) startSite() {
	s.env["GOPHENBERG_ADDR"] = "127.0.0.1:0"
	ctx, cancel := context.WithTimeout(s.t.Context(), siteStartLimit)
	defer cancel()
	exits := &exitCodes{}
	stderr := &keptWatch{cancel: cancel}
	var stdout strings.Builder
	code := commandLine(testkit.Getenv(s.env), s.loaded, exits, s.stamp).Run(
		ctx, []string{"serve"}, strings.NewReader(""), &stdout, stderr)
	s.result = testkit.Result{Code: exits.of(code), Stdout: stdout.String(), Stderr: stderr.String()}
}

// failStarts starts the site the times given, failing unless each start stops with exit code 1.
func (s *operatorScenario) failStarts(times int) error {
	for range times {
		s.startSite()
		if s.result.Code != gonsole.ExitFailed {
			return fmt.Errorf("serve exited with code %d and stderr %q, want %d", s.result.Code, s.result.Stderr,
				gonsole.ExitFailed)
		}
	}
	return nil
}

// outlastCheck leaves the start check of the build that starts no time, so it runs out before any page answers.
func (s *operatorScenario) outlastCheck() {
	s.env["GOPHENBERG_UPDATE_VERIFY_TIMEOUT"] = "1ns"
}

// keptWatch records what a run writes to its error stream and calls cancel once the boot guard kept the update.
type keptWatch struct {
	mu     sync.Mutex
	text   strings.Builder
	cancel context.CancelFunc
}

// Write records p and calls cancel when p says the boot guard kept the update.
func (w *keptWatch) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if strings.Contains(string(p), "kept the update after its start check") {
		w.cancel()
	}
	return w.text.Write(p)
}

// String returns everything the run wrote to its error stream.
func (w *keptWatch) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.text.String()
}

// namesBackup fails unless the error names the full path of the backup the update state names.
func (s *operatorScenario) namesBackup() error {
	root := s.env[installRootSetting]
	state, err := updater.Layout{Root: root}.ReadState()
	if err != nil || state.Backup == "" {
		return fmt.Errorf("the update state names the backup %q (%v), want one", state.Backup, err)
	}
	return s.errorSays(filepath.Join(root, state.Backup))
}

// runCustomBuild runs this version from a new install root, failing unless this binary carries no build stamp.
func (s *operatorScenario) runCustomBuild() error {
	if version.Stamp() != "" {
		return fmt.Errorf("this binary carries the build stamp %q, want none for a custom build", version.Stamp())
	}
	return s.runFromInstallRoot()
}

// runFromInstallRoot points the settings at a new install root whose current link names a build of this version.
func (s *operatorScenario) runFromInstallRoot() error {
	root := s.t.TempDir()
	if err := placeBuild(root, version.Version()); err != nil {
		return err
	}
	s.env[installRootSetting] = root
	return os.Symlink(filepath.Join("versions", version.Version()), filepath.Join(root, "current"))
}

// placeBuild writes a stand-in gophenberg of the build called name into the versions folder of root.
func placeBuild(root, name string) error {
	folder := filepath.Join(root, "versions", name)
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(folder, "gophenberg"), []byte("#!/bin/sh\n"), 0o755)
}

// offerUpdateKeepingMinimum starts an update site offering version offered under the minimum version of this binary.
func (s *operatorScenario) offerUpdateKeepingMinimum(offered string) {
	s.offerRelease(offered, version.Minimum())
}

// runAfterFailure runs the command line as the rollback unit of the service manager does, with variable set to value.
func (s *operatorScenario) runAfterFailure(line, variable, value string) {
	s.env[variable] = value
	s.runLine(line)
}

// linksTo fails unless the install root's link called link names the build called name, or this version for none.
func (s *operatorScenario) linksTo(link, name string) error {
	if name == "" {
		name = version.Version()
	}
	want := filepath.Join("versions", name)
	held, err := os.Readlink(filepath.Join(s.env[installRootSetting], link))
	if err != nil || held != want {
		return fmt.Errorf("the %s link names %q (%v), want %q", link, held, err, want)
	}
	return nil
}

// holdsNoBuildOf fails when the install root holds a download or a build folder of the version offered.
func (s *operatorScenario) holdsNoBuildOf(offered string) error {
	root := s.env[installRootSetting]
	for _, path := range []string{
		filepath.Join(root, "downloads", offered+".partial"), filepath.Join(root, "versions", offered),
	} {
		if _, err := os.Lstat(path); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("the install root holds %s (%v), want nothing there", path, err)
		}
	}
	return nil
}

// keepsBackup fails unless the install root keeps one database backup named after this version, holding bytes.
func (s *operatorScenario) keepsBackup() error {
	_, err := backupIn(s.env[installRootSetting])
	return err
}

// backupIn returns the one database backup folder keeps of this version, failing on none or on an empty one.
func backupIn(folder string) (string, error) {
	pattern := filepath.Join(folder, "backups", version.Version()+"-*.dump")
	held, err := filepath.Glob(pattern)
	if err != nil || len(held) != 1 {
		return "", fmt.Errorf("the folder keeps the backups %q (%v), want one matching %s", held, err, pattern)
	}
	info, err := os.Stat(held[0])
	if err != nil || info.Size() == 0 {
		return "", fmt.Errorf("the backup %s holds no bytes (%v), want the database", held[0], err)
	}
	return held[0], nil
}

// nameDataFolder names a data folder nothing has made yet, beside the install root.
func (s *operatorScenario) nameDataFolder() {
	s.env["GOPHENBERG_DATA_DIR"] = filepath.Join(s.t.TempDir(), "gophenberg-data")
}

// moveToDataFolder names a data folder and moves the backup and the state out of the install root into it.
func (s *operatorScenario) moveToDataFolder() error {
	s.nameDataFolder()
	root, data := s.env[installRootSetting], s.env["GOPHENBERG_DATA_DIR"]
	if err := os.Mkdir(data, 0o700); err != nil {
		return err
	}
	for _, folder := range []string{"backups", "state"} {
		if err := os.Rename(filepath.Join(root, folder), filepath.Join(data, folder)); err != nil {
			return err
		}
	}
	return nil
}

// errorNamesDataBackup fails unless the data folder keeps the backup taken before the switch and the error names it.
func (s *operatorScenario) errorNamesDataBackup() error {
	backup := filepath.Join(s.env["GOPHENBERG_DATA_DIR"], switchBackup)
	if _, err := os.Stat(backup); err != nil {
		return err
	}
	return s.errorSays(backup)
}

// dataFolderKeepsBackup fails unless the data folder keeps the backup of this version.
func (s *operatorScenario) dataFolderKeepsBackup() error {
	_, err := backupIn(s.env["GOPHENBERG_DATA_DIR"])
	return err
}

// dataFolderKeepsState fails unless the data folder holds the state of the update.
func (s *operatorScenario) dataFolderKeepsState() error {
	_, err := os.Stat(filepath.Join(s.env["GOPHENBERG_DATA_DIR"], "state", "update.json"))
	return err
}

// rootKeepsNothing fails when the install root holds a backup or the state of the update.
func (s *operatorScenario) rootKeepsNothing() error {
	root := s.env[installRootSetting]
	for _, held := range []string{filepath.Join(root, "state", "update.json"), filepath.Join(root, "backups")} {
		if _, err := os.Stat(held); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("the install root holds %s (%v), want it kept in the data folder", held, err)
		}
	}
	return nil
}

// dataFolderIsPrivate fails unless the data folder answers to its owner alone.
func (s *operatorScenario) dataFolderIsPrivate() error {
	info, err := os.Stat(s.env["GOPHENBERG_DATA_DIR"])
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o700 {
		return fmt.Errorf("the data folder has the mode %o, want 700", info.Mode().Perm())
	}
	return nil
}

// outputNamesDataBackup fails unless the command printed the path of the backup in the data folder.
func (s *operatorScenario) outputNamesDataBackup() error {
	backup, err := backupIn(s.env["GOPHENBERG_DATA_DIR"])
	if err != nil {
		return err
	}
	if !strings.Contains(s.result.Stdout+s.result.Stderr, backup) {
		return fmt.Errorf("the command printed %q and %q, want it to name %s", s.result.Stdout, s.result.Stderr, backup)
	}
	return nil
}

// statusListsDataBackup fails unless update:status lists the backup at its path in the data folder.
func (s *operatorScenario) statusListsDataBackup() error {
	backup, err := backupIn(s.env["GOPHENBERG_DATA_DIR"])
	if err != nil {
		return err
	}
	answered := s.answer("", "update:status")
	if !strings.Contains(answered.Stdout, "backups kept: "+backup) {
		return fmt.Errorf("update:status printed %q, want it to list %s", answered.Stdout, backup)
	}
	return nil
}

func TestCoreUpdates(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, coreUpdatesFeature, postgresOperators())
}
