// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"os"
	"os/user"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/version"
)

// wantedRemoves is what server:uninstall lists as removed on a server a first run for example.com set up.
const wantedRemoves = `server:uninstall removes:
  gophenberg.service, stopped and disabled
  /etc/systemd/system/gophenberg.service
  /etc/systemd/system/gophenberg-rollback.service
  /etc/caddy/Caddyfile, putting back /etc/caddy/Caddyfile.before-gophenberg, the one the caddy package wrote
  /etc/systemd/system/caddy.service.d/gophenberg.conf
  /etc/caddy/gophenberg.caddy
  /usr/local/share/gophenberg
  /usr/local/bin/gophenberg
  /usr/local/lib/gophenberg
  /opt/gophenberg/versions
  /opt/gophenberg/downloads
  /opt/gophenberg/current
`

// wantedRestart is the line server:uninstall lists for the restart of Caddy that a run cut off part way left undone.
const wantedRestart = "  Gophenberg's settings in Caddy, restarting Caddy on /etc/caddy/Caddyfile, the one the caddy " +
	"package wrote\n"

// wantedPackagesKept is what server:uninstall lists as kept once the install record and the site's data are listed.
const wantedPackagesKept = "  the packages server:install installed, which sudo apt-get purge postgresql caddy " +
	"removes\n  /etc/apt/keyrings/caddy.asc, /etc/apt/sources.list.d/caddy.sources and " +
	"/etc/apt/preferences.d/gophenberg-caddy, part of the setup of Caddy's own apt repository\n"

// wantedKeeps is what server:uninstall lists as kept on a server a first run for example.com set up.
const wantedKeeps = `server:uninstall keeps:
  the database gophenberg and its role gophenberg
  the install root /opt/gophenberg, with the site's .env, media, themes and backups
  /etc/gophenberg/server.env
  the system user gophenberg
  /var/lib/gophenberg-installer/install.json, so a later install picks up what is kept
` + wantedPackagesKept

// uninstalledPaths are the paths server:uninstall removes on a server a first run for example.com set up.
var uninstalledPaths = []string{
	"/etc/systemd/system/gophenberg.service", "/etc/systemd/system/gophenberg-rollback.service", caddyDropIn,
	keptCaddyfilePath, caddySitePath, maintenanceFolder, commandPath, installerCopy, "/opt/gophenberg/versions",
	"/opt/gophenberg/downloads", "/opt/gophenberg/current",
}

// keptPaths are the paths server:uninstall keeps on a server a first run for example.com set up.
var keptPaths = []string{
	siteEnvAt, "/opt/gophenberg/media", "/opt/gophenberg/themes", "/opt/gophenberg/backups", "/opt/gophenberg/state",
	serverEnvPath, recordPath, caddyKeyPath, "/etc/apt/sources.list.d/caddy.sources",
	"/etc/apt/preferences.d/gophenberg-caddy",
}

// removedPrograms are the programs server:uninstall runs on a server a first run for example.com set up.
var removedPrograms = []string{
	"systemctl disable --now gophenberg.service", "systemctl reset-failed gophenberg.service gophenberg-rollback.service",
	"systemctl daemon-reload", "systemctl daemon-reload", "systemctl restart caddy.service",
}

// droppedPrograms are the programs server:uninstall runs after removedPrograms with -delete-data.
var droppedPrograms = []string{
	"runuser -u postgres -- dropdb gophenberg", "runuser -u postgres -- dropuser gophenberg", "userdel gophenberg",
}

// uninstallOn runs server:uninstall on server under env, applying its changes or not, deleting the data or not.
func uninstallOn(server *fakeServer, env map[string]string, apply, deleteData bool) (stdout, stderr string, err error) {
	return uninstallWith(server.t, server.machine(), gonsole.Call{Env: envOf(env), Apply: apply}, deleteData)
}

// uninstallWith runs server:uninstall on machine as call asks, catching its answer and notes, as uninstallOn returns.
func uninstallWith(
	t *testing.T, machine Machine, call gonsole.Call, deleteData bool,
) (stdout, stderr string, err error) {
	var answer, notes strings.Builder
	call.Stdout, call.Stderr = &answer, &notes
	err = Uninstall(t.Context(), machine, call, deleteData)
	return answer.String(), notes.String(), err
}

// firstInstalled returns a fake server a first run for example.com installed Gophenberg on, its removals answered.
func firstInstalled(t *testing.T) *fakeServer {
	t.Helper()
	server := newFakeServer(t)
	if _, _, err := runInstall(server, flaggedRunOn(t, server)); err != nil {
		t.Fatalf("Install() = %v, want Gophenberg installed", err)
	}
	server.answerRemovals()
	server.forgetRun()
	return server
}

// recordOf returns the install record on server.
func recordOf(t *testing.T, server *fakeServer) Record {
	t.Helper()
	record, err := readRecord(server.machine())
	if err != nil {
		t.Fatalf("readRecord() = %v, want the install record", err)
	}
	return record
}

// cutOffInCaddyOn leaves the record of server as a first run cut off in the caddy step once it wrote the Caddyfile.
func cutOffInCaddyOn(t *testing.T, server *fakeServer) {
	t.Helper()
	record := recordOf(t, server)
	record.Caddyfile, record.Steps[stepCaddy] = "", stepStarted
	for _, id := range stepIDs[slices.Index(stepIDs, stepCaddy)+1:] {
		delete(record.Steps, id)
	}
	server.saveRecord(record)
}

// wantNothingAt fails t when server holds anything at one of paths.
func wantNothingAt(t *testing.T, server *fakeServer, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if server.machine().there(path) {
			t.Errorf("the server holds %s, want it removed", path)
		}
	}
}

// wantThere fails t unless server holds something at each of paths.
func wantThere(t *testing.T, server *fakeServer, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if !server.machine().there(path) {
			t.Errorf("the server holds nothing at %s, want it kept", path)
		}
	}
}

// wantUnchanged fails t when server holds other files than before, or ran a program.
func wantUnchanged(t *testing.T, server *fakeServer, before map[string]string) {
	t.Helper()
	if !maps.Equal(server.snapshot(), before) {
		t.Errorf("the server changed, want it as it was")
	}
	if len(server.ran) != 0 {
		t.Errorf("the run ran %q, want nothing run", server.ran)
	}
}

func TestUninstallRefusesARunThatIsNotRootsAndChangesNothing(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.euid = 1000
	before := server.snapshot()

	stdout, _, err := uninstallOn(server, nil, true, true)

	wantRefused(t, err, "server:uninstall must run as root, such as with sudo, so nothing was changed")
	if stdout != "" {
		t.Errorf("Uninstall() printed %q as a plain user, want nothing", stdout)
	}
	wantUnchanged(t, server, before)
}

func TestUninstallStopsOnAServerWithNoInstallRecord(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	before := server.snapshot()

	_, _, err := uninstallOn(server, nil, true, false)

	wantRefused(t, err, "this server has no install record at /var/lib/gophenberg-installer/install.json, so "+
		"server:uninstall has nothing to remove")
	wantUnchanged(t, server, before)
}

func TestUninstallStopsAtAnInstallerFolderAnotherUserOwnsBeforeItReadsTheRecord(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.owners[installerFolder] = 1000
	server.write(recordPath, "{")
	before := server.snapshot()

	_, _, err := uninstallOn(server, nil, true, false)

	wantRefused(t, err, installerFolderOpen)
	wantUnchanged(t, server, before)
}

func TestUninstallRefusesServerSettingsAndARecordItCannotRead(t *testing.T) {
	t.Parallel()

	unreadable := firstInstalled(t)
	unreadable.remove(serverEnvPath)
	unreadable.folder(serverEnvPath)
	relative := firstInstalled(t)
	relative.write(serverEnvPath, "GOPHENBERG_INSTALL_ROOT=opt/gophenberg\nGOPHENBERG_PUBLIC_URL=http://example.com\n")
	broken := firstInstalled(t)
	broken.write(recordPath, "{")

	for server, want := range map[*fakeServer]string{
		unreadable: "installer: read " + unreadable.at(serverEnvPath) + ": is a directory",
		relative: `GOPHENBERG_INSTALL_ROOT: must be an absolute path, got "opt/gophenberg"` + "\n" +
			`GOPHENBERG_PUBLIC_URL: must be https:// followed by ` + wantedHost + `, got "http://example.com"`,
		broken: "installer: /var/lib/gophenberg-installer/install.json is not an install record: unexpected end " +
			"of JSON input",
	} {
		_, _, err := uninstallOn(server, nil, true, true)

		wantRefused(t, err, want)
		if len(server.ran) != 0 {
			t.Errorf("the run ran %q, want nothing run", server.ran)
		}
	}
}

func TestUninstallWithoutApplyListsWhatItRemovesAndKeepsAndChangesNothing(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	before := server.snapshot()

	stdout, stderr, err := uninstallOn(server, nil, false, false)

	if err != nil || stdout != wantedRemoves+wantedKeeps {
		t.Errorf("Uninstall() = %v, printing\n%s\nwant\n%s", err, stdout, wantedRemoves+wantedKeeps)
	}
	if stderr != "" {
		t.Errorf("Uninstall() noted %q, want nothing", stderr)
	}
	wantUnchanged(t, server, before)
}

func TestUninstallTakesItsSettingsFromServerEnvAloneNeverTheEnvironment(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	env := map[string]string{"GOPHENBERG_INSTALL_ROOT": "/srv/planted", "GOPHENBERG_INSTALL_WEB_SERVER": "none"}

	stdout, _, err := uninstallOn(server, env, false, false)

	if err != nil || stdout != wantedRemoves+wantedKeeps {
		t.Errorf("Uninstall() = %v, printing\n%s\nwant the install root and the web server of server.env\n%s", err,
			stdout, wantedRemoves+wantedKeeps)
	}
}

func TestUninstallTakesTheDefaultsOfTheSettingsServerEnvLeavesOut(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write(serverEnvPath, "# Written by gophenberg server:install.\n")

	stdout, _, err := uninstallOn(server, nil, false, false)

	if err != nil || stdout != wantedRemoves+wantedKeeps {
		t.Errorf("Uninstall() = %v, printing\n%s\nwant the default install root and web server", err, stdout)
	}
}

func TestUninstallWithApplyRemovesWhatServerInstallSetUpInOrderAndKeepsTheData(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)

	stdout, stderr, err := uninstallOn(server, nil, true, false)

	if err != nil || stdout != wantedRemoves+wantedKeeps+removedLine {
		t.Fatalf("Uninstall() = %v, printing\n%s\nwant the lists and the last line", err, stdout)
	}
	if stderr != "" {
		t.Errorf("Uninstall() noted %q, want nothing", stderr)
	}
	if !slices.Equal(server.ran, removedPrograms) {
		t.Errorf("the run ran %q, want %q", server.ran, removedPrograms)
	}
	wantNothingAt(t, server, uninstalledPaths...)
	wantThere(t, server, keptPaths...)
	wantText(t, server.at(caddyfilePath), packageCaddyfile)
	if _, held := server.users[siteUser]; !held || server.running[pieceService] {
		t.Errorf("the server has the user gophenberg %t and runs the site %t, want the user kept and the site "+
			"stopped", held, server.running[pieceService])
	}
}

func TestUninstallLeavesARecordALaterInstallPicksUpWhatIsKeptFrom(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)

	_, _, err := uninstallOn(server, nil, true, false)

	want := Record{
		Installer: version.Version(), Domain: "example.com",
		Installed: []string{piecePostgreSQL, pieceCaddy, "/opt/gophenberg"}, Steps: map[string]string{
			"server-env": stepEnded, "packages": stepEnded, "user": stepEnded, "database": stepEnded, stepAdmin: stepEnded,
		},
	}
	got := recordOf(t, server)
	if err != nil || !recordsEqual(got, want) {
		t.Errorf("Uninstall() = %v, leaving the record %+v, want %+v", err, got, want)
	}
	wantEntry(t, server.at(recordPath), 0o600, nil)
}

// recordsEqual reports whether got and want hold the same installer, steps, pieces, domain and Caddyfile sum.
func recordsEqual(got, want Record) bool {
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	return bytes.Equal(gotJSON, wantJSON)
}

func TestUninstallKeepsACaddyfileThePersonChangedAndTakesOutOnlyTheImportLine(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write(caddyfilePath, wantedCaddyfile+personSite)

	stdout, _, err := uninstallOn(server, nil, true, false)

	if err != nil {
		t.Fatalf("Uninstall() = %v, want the import line taken out", err)
	}
	wantText(t, server.at(caddyfilePath), strings.Replace(wantedCaddyfile, caddyImport+"\n", "", 1)+personSite)
	wantEntry(t, server.at(caddyfilePath), 0o644, nil)
	wantThere(t, server, caddyDropIn, keptCaddyfilePath)
	want := slices.Concat(removedPrograms[:3], []string{"systemctl reload caddy.service"})
	if !slices.Equal(server.ran, want) {
		t.Errorf("the run ran %q, want %q", server.ran, want)
	}
	listed := "  the line import /etc/caddy/gophenberg.caddy of /etc/caddy/Caddyfile\n  /etc/caddy/gophenberg.caddy\n"
	kept := "server:uninstall keeps:\n  /etc/caddy/Caddyfile, which changed since server:install wrote it, so a " +
		"reinstall asks for that line back\n  /etc/systemd/system/caddy.service.d/gophenberg.conf, which gives Caddy " +
		"the folder of the admin socket its Caddyfile names\n  /etc/caddy/Caddyfile.before-gophenberg, the Caddyfile " +
		"the caddy package wrote\n  the database"
	if !strings.Contains(stdout, listed) || !strings.Contains(stdout, kept) {
		t.Errorf("Uninstall() printed\n%s\nwant the import line removed and the Caddyfile and drop-in kept", stdout)
	}
	wantKeptCaddy(t, server, caddyfileSum(wantedCaddyfile))
}

func TestUninstallKeepsTheModeOwnerAndGroupOfACaddyfileThePersonChanged(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write(caddyfilePath, wantedCaddyfile+personSite)
	server.chmod(caddyfilePath, 0o640)
	if err := os.Chown(server.at(caddyfilePath), -1, siteGroup(t)); err != nil {
		t.Fatalf("giving the Caddyfile a group: %v", err)
	}

	_, _, err := uninstallOn(server, nil, true, false)

	if err != nil {
		t.Fatalf("Uninstall() = %v, want the import line taken out", err)
	}
	wantText(t, server.at(caddyfilePath), strings.Replace(wantedCaddyfile, caddyImport+"\n", "", 1)+personSite)
	wantEntry(t, server.at(caddyfilePath), 0o640, &owner{uid: os.Getuid(), gid: siteGroup(t)})
}

func TestUninstallStopsWhereTheCaddyfileThePersonChangedIsGoneBeforeItsImportLineComesOut(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write(caddyfilePath, wantedCaddyfile+personSite)
	running, err := removingOn(server.machine(), gonsole.Call{Env: envOf(nil), Stdout: io.Discard, Apply: true})
	if err != nil {
		t.Fatalf("removingOn() = %v, want the run ready", err)
	}
	t.Cleanup(running.close)
	if err := running.plan(); err != nil {
		t.Fatalf("plan() = %v, want the lists made", err)
	}
	server.remove(caddyfilePath)

	err = running.apply(t.Context())

	wantRefused(t, err, "installer: openat etc/caddy/Caddyfile: no such file or directory")
	wantNothingAt(t, server, caddyfilePath)
}

// wantKeptCaddy fails t unless the record on server holds sum as its Caddyfile's and marks the caddy step ended.
func wantKeptCaddy(t *testing.T, server *fakeServer, sum string) {
	t.Helper()
	record := recordOf(t, server)
	if record.Caddyfile != sum || record.Steps[stepCaddy] != stepEnded {
		t.Errorf("the record holds the sum %q and marks caddy %q, want %q and ended", record.Caddyfile,
			record.Steps[stepCaddy], sum)
	}
}

func TestUninstallPutsBackThePackagesCaddyfileOverTheOneARunCutOffInTheCaddyStepWrote(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	cutOffInCaddyOn(t, server)

	_, _, err := uninstallOn(server, nil, true, false)

	if err != nil || !slices.Contains(server.ran, "systemctl restart caddy.service") {
		t.Errorf("Uninstall() = %v, running %q, want Caddy restarted on the package's Caddyfile", err, server.ran)
	}
	wantText(t, server.at(caddyfilePath), packageCaddyfile)
	wantNothingAt(t, server, keptCaddyfilePath, caddyDropIn)
	if record := recordOf(t, server); record.Caddyfile != "" || record.Steps[stepCaddy] != "" {
		t.Errorf("the record holds the sum %q and marks caddy %q, want neither", record.Caddyfile,
			record.Steps[stepCaddy])
	}
}

func TestUninstallTakesOutTheImportLineOfACaddyfileThePersonChangedAfterACutOffRunAndRestartsCaddy(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	cutOffInCaddyOn(t, server)
	server.write(caddyfilePath, wantedCaddyfile+personSite)

	_, _, err := uninstallOn(server, nil, true, false)

	want := slices.Concat(removedPrograms[:3], []string{"systemctl restart caddy.service"})
	if err != nil || !slices.Equal(server.ran, want) {
		t.Errorf("Uninstall() = %v, running %q, want the Caddyfile kept and Caddy restarted %q", err, server.ran, want)
	}
	wantText(t, server.at(caddyfilePath), strings.Replace(wantedCaddyfile, caddyImport+"\n", "", 1)+personSite)
	wantKeptCaddy(t, server, caddyfileSum(wantedCaddyfile))
}

func TestUninstallPutsBackThePackagesCaddyfileWhereTheCaddyfileIsGone(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.remove(caddyfilePath)

	_, _, err := uninstallOn(server, nil, true, false)

	if err != nil || !slices.Equal(server.ran, removedPrograms) {
		t.Errorf("Uninstall() = %v, running %q, want %q", err, server.ran, removedPrograms)
	}
	wantText(t, server.at(caddyfilePath), packageCaddyfile)
}

// wantedNoCaddyfile is the stop of a run that owes Caddy a change while both the Caddyfile and the package's are gone.
const wantedNoCaddyfile = "server:uninstall needs /etc/caddy/Caddyfile to take Gophenberg's site out of Caddy, and " +
	"it is not there, so nothing was changed"

func TestUninstallStopsBeforeAnyChangeWhereTheCaddyfileAndThePackagesOneAreGone(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.remove(caddyfilePath)
	server.remove(keptCaddyfilePath)
	before := server.snapshot()

	stdout, _, err := uninstallOn(server, nil, true, false)

	wantRefused(t, err, wantedNoCaddyfile)
	if stdout != "" {
		t.Errorf("Uninstall() printed %q, want nothing", stdout)
	}
	wantUnchanged(t, server, before)
}

func TestUninstallStopsBeforeAnyChangeWhereARestartIsOwedAndBothCaddyfilesAreGone(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.remove(caddySitePath)
	server.remove(caddyfilePath)
	server.remove(keptCaddyfilePath)
	record := recordOf(t, server)
	record.Caddyfile, record.Steps[stepCaddy] = "", stepStarted
	server.saveRecord(record)
	before := server.snapshot()

	_, _, err := uninstallOn(server, nil, true, false)

	wantRefused(t, err, wantedNoCaddyfile)
	wantUnchanged(t, server, before)
}

// wantedKeptOwnCaddyfile is the line server:uninstall keeps a Caddyfile the person changed under with no import line.
const wantedKeptOwnCaddyfile = "\n  /etc/caddy/Caddyfile, which changed since server:install wrote it, so a " +
	"reinstall asks for the line import /etc/caddy/gophenberg.caddy in it\n"

func TestUninstallKeepsACaddyfileWithoutTheImportLineAndTheSumOfTheRecordAsTheyAre(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write(caddyfilePath, ownCaddyfile)

	stdout, _, err := uninstallOn(server, nil, true, false)

	want := slices.Concat(removedPrograms[:3], []string{"systemctl reload caddy.service"})
	if err != nil || !slices.Equal(server.ran, want) {
		t.Errorf("Uninstall() = %v, running %q, want Caddy reloaded while gophenberg.caddy is there", err, server.ran)
	}
	if !strings.Contains(stdout, "server:uninstall keeps:"+wantedKeptOwnCaddyfile) {
		t.Errorf("Uninstall() printed\n%s\nwant the Caddyfile kept first with the line%s", stdout,
			wantedKeptOwnCaddyfile)
	}
	wantText(t, server.at(caddyfilePath), ownCaddyfile)
	wantKeptCaddy(t, server, caddyfileSum(wantedCaddyfile))
}

// ownWebServerOn installs Gophenberg on server for example.com beside a Caddy the person set up, with -web-server none.
func ownWebServerOn(t *testing.T, server *fakeServer) {
	t.Helper()
	server.folder("/etc/caddy")
	server.program("/usr/bin/caddy")
	server.write(caddyfilePath, ownCaddyfile)
	request := flaggedRunOn(t, server)
	request.WebServer = webServerNone
	if _, _, err := runInstall(server, request); err != nil {
		t.Fatalf("Install() = %v, want Gophenberg installed beside the person's own web server", err)
	}
	server.answerRemovals()
	server.forgetRun()
}

func TestUninstallLeavesCaddyAloneOnAServerThatKeepsItsOwnWebServer(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	ownWebServerOn(t, server)

	stdout, _, err := uninstallOn(server, nil, true, false)

	if err != nil || !slices.Equal(server.ran, removedPrograms[:3]) {
		t.Errorf("Uninstall() = %v, running %q, want Caddy left alone", err, server.ran)
	}
	wantText(t, server.at(caddyfilePath), ownCaddyfile)
	want := "  the packages server:install installed, which sudo apt-get purge postgresql removes\n" + removedLine
	if !strings.HasSuffix(stdout, "\n"+want) {
		t.Errorf("Uninstall() printed\n%s\nwant it to end with\n%s", stdout, want)
	}
}

func TestUninstallTakesGophenbergsSiteOutOfCaddyWhileItIsThereWhateverServerEnvSays(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	(&installScenario{server: server}).keepOwnWebServerOrFail(t)

	_, _, err := uninstallOn(server, nil, true, false)

	if err != nil || !slices.Equal(server.ran, removedPrograms) {
		t.Errorf("Uninstall() = %v, running %q, want %q", err, server.ran, removedPrograms)
	}
	wantText(t, server.at(caddyfilePath), packageCaddyfile)
	wantNothingAt(t, server, caddySitePath)
}

// keepOwnWebServerOrFail changes the web server line of server.env to none, failing t when it cannot.
func (s *installScenario) keepOwnWebServerOrFail(t *testing.T) {
	t.Helper()
	if err := s.keepOwnWebServer(); err != nil {
		t.Fatalf("changing the web server line: %v", err)
	}
}

func TestUninstallKeepsAGophenbergCommandServerInstallDidNotWrite(t *testing.T) {
	t.Parallel()

	for what, put := range map[string]func(*fakeServer){
		"another script": func(s *fakeServer) { s.write(commandPath, "#!/bin/sh\nexec /opt/other/gophenberg \"$@\"\n") },
		"a folder": func(s *fakeServer) {
			s.remove(commandPath)
			s.folder(commandPath)
		},
		"a link to the command server:install writes": func(s *fakeServer) {
			s.write("/usr/local/share/command", wantedCommand)
			s.remove(commandPath)
			s.link(commandPath, "/usr/local/share/command")
		},
	} {
		server := firstInstalled(t)
		put(server)

		stdout, _, err := uninstallOn(server, nil, true, false)

		kept := "\n  /usr/local/bin/gophenberg, which server:install did not write\n"
		if err != nil || !strings.Contains(stdout, kept) {
			t.Errorf("Uninstall() over %s = %v, printing\n%s\nwant the command kept", what, err, stdout)
		}
		wantThere(t, server, commandPath)
	}
}

func TestUninstallFindsNothingLeftToRemoveOnASecondRunAndLeavesTheRecordUnwritten(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	if _, _, err := uninstallOn(server, nil, true, false); err != nil {
		t.Fatalf("Uninstall() = %v, want the first run through", err)
	}
	server.forgetRun()
	before := server.snapshot()
	written, err := os.Stat(server.at(recordPath))
	if err != nil {
		t.Fatalf("describing the record: %v", err)
	}

	stdout, _, err := uninstallOn(server, nil, true, false)

	if want := "server:uninstall finds nothing left to remove.\n" + wantedKeeps + removedLine; err != nil ||
		stdout != want {
		t.Errorf("Uninstall() = %v, printing\n%s\nwant\n%s", err, stdout, want)
	}
	wantUnchanged(t, server, before)
	if held, err := os.Stat(server.at(recordPath)); err != nil || !os.SameFile(held, written) {
		t.Errorf("the second run wrote the record again (%v), want it left as it was", err)
	}
}

func TestUninstallWithDeleteDataDropsTheDatabaseAndRemovesTheRootServerEnvAndTheUser(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)

	stdout, _, err := uninstallOn(server, nil, true, true)

	if err != nil {
		t.Fatalf("Uninstall() = %v, want the data removed too", err)
	}
	if want := slices.Concat(removedPrograms, droppedPrograms); !slices.Equal(server.ran, want) {
		t.Errorf("the run ran %q, want %q", server.ran, want)
	}
	wantNothingAt(t, server, "/opt/gophenberg", serverEnvPath)
	wantThere(t, server, "/opt", recordPath)
	if _, held := server.users[siteUser]; held {
		t.Errorf("the server still has the user gophenberg, want it removed")
	}
	want := Record{Installer: version.Version(), Domain: "example.com", Installed: []string{piecePostgreSQL, pieceCaddy}}
	if got := recordOf(t, server); !recordsEqual(got, want) || len(got.Steps) != 0 {
		t.Errorf("Uninstall() left the record %+v, want %+v", got, want)
	}
	listed := "  /opt/gophenberg/current\n  the database gophenberg and its role gophenberg\n  the install root " +
		"/opt/gophenberg and everything in it\n  /etc/gophenberg\n  the system user gophenberg\n" +
		"server:uninstall keeps:\n  /var/lib/gophenberg-installer/install.json, holding the packages server:install " +
		"installed, so a later install takes them as its own\n" + wantedPackagesKept + removedLine
	if !strings.HasSuffix(stdout, listed) {
		t.Errorf("Uninstall() printed\n%s\nwant it to end with\n%s", stdout, listed)
	}
}

func TestUninstallWithDeleteDataTakesADatabaseAndARoleThatDoNotExistAsDropped(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.answers["dropdb"] = failed("dropdb", "dropdb: error: database removal failed: ERROR:  database "+
		"\"gophenberg\" does not exist\n")
	server.answers["dropuser"] = failed("dropuser", "dropuser: error: removal of role \"gophenberg\" failed: ERROR:  "+
		"role \"gophenberg\" does not exist\n")

	_, _, err := uninstallOn(server, nil, true, true)

	if err != nil || !slices.Contains(server.ran, "userdel gophenberg") {
		t.Errorf("Uninstall() = %v, running %q, want the missing database taken as dropped", err, server.ran)
	}
}

func TestUninstallWithDeleteDataSkipsADatabaseTheRecordNeverMade(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	record := recordOf(t, server)
	delete(record.Steps, "database")
	server.saveRecord(record)

	stdout, _, err := uninstallOn(server, nil, true, true)

	if err != nil || slices.Contains(server.ran, "runuser -u postgres -- dropdb gophenberg") {
		t.Errorf("Uninstall() = %v, running %q, want no database dropped", err, server.ran)
	}
	if strings.Contains(stdout, "the database gophenberg") {
		t.Errorf("Uninstall() printed\n%s\nwant no line for a database the record never made", stdout)
	}
}

func TestUninstallStopsWithWhatAFailedProgramPrintedAndLeavesTheRecordItSavedBeforeTheCaddyfileWentBack(t *testing.T) {
	t.Parallel()

	noPostgres := "runuser: user postgres does not exist or the user entry does not contain all the required fields\n"
	for _, row := range []struct{ program, printed string }{
		{"dropdb", "dropdb: error: database removal failed: ERROR:  database \"gophenberg\" is being accessed\n"},
		{"dropdb", noPostgres},
		{"dropuser", noPostgres},
		{"userdel", "userdel: user gophenberg is currently used by process 4242\n"},
	} {
		program, printed := row.program, row.printed
		server := firstInstalled(t)
		want := recordOf(t, server)
		want.Caddyfile, want.Steps[stepCaddy] = "", stepStarted
		server.answers[program] = failed(program, printed)
		delete(server.effects, program)

		_, _, err := uninstallOn(server, nil, true, true)

		wantRefused(t, err, "installer: run "+program+": exit status 1, after it printed:\n"+
			strings.TrimSuffix(printed, "\n"))
		if got := recordOf(t, server); !recordsEqual(got, want) {
			t.Errorf("a failed %s left the record %+v, want the one saved before the Caddyfile went back %+v",
				program, got, want)
		}
	}
}

func TestUninstallFinishesWhatARunCutOffOnceTheCaddyfileWentBackLeftUndone(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.cutSystemctl("systemctl daemon-reload", 2)
	if _, _, err := uninstallOn(server, nil, true, false); err == nil {
		t.Fatalf("Uninstall() went through, want it cut off at its second daemon-reload")
	}
	if cut := recordOf(t, server); cut.Caddyfile != "" || cut.Steps[stepCaddy] != stepStarted {
		t.Errorf("the cut-off run left the sum %q and marks caddy %q, want no sum and caddy started", cut.Caddyfile,
			cut.Steps[stepCaddy])
	}
	server.answerRemovals()
	server.forgetRun()

	stdout, _, err := uninstallOn(server, nil, true, false)

	if want := []string{"systemctl daemon-reload", "systemctl restart caddy.service"}; err != nil ||
		!slices.Equal(server.ran, want) {
		t.Errorf("Uninstall() = %v, running %q, want %q", err, server.ran, want)
	}
	listed := "server:uninstall removes:\n" + wantedRestart + "  /etc/caddy/gophenberg.caddy\n"
	if !strings.HasPrefix(stdout, listed) || strings.Contains(stdout, caddyDropIn) {
		t.Errorf("Uninstall() printed\n%s\nwant it to begin with\n%s", stdout, listed)
	}
	wantNothingAt(t, server, caddyDropIn, keptCaddyfilePath, caddySitePath)
	wantText(t, server.at(caddyfilePath), packageCaddyfile)
	if record := recordOf(t, server); record.Caddyfile != "" || record.Steps[stepCaddy] != "" {
		t.Errorf("the record holds the sum %q and marks caddy %q, want neither", record.Caddyfile,
			record.Steps[stepCaddy])
	}
}

func TestUninstallRemovesTheDropInAFirstRunCutOffBeforeItsCaddyStepLeftAndRestartsCaddy(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.remove(caddySitePath)
	server.write(caddyfilePath, packageCaddyfile)
	server.remove(keptCaddyfilePath)
	record := recordOf(t, server)
	record.Caddyfile = ""
	for _, id := range stepIDs[slices.Index(stepIDs, stepCaddy):] {
		delete(record.Steps, id)
	}
	server.saveRecord(record)

	stdout, _, err := uninstallOn(server, nil, true, false)

	if err != nil || !slices.Equal(server.ran, removedPrograms) {
		t.Errorf("Uninstall() = %v, running %q, want %q", err, server.ran, removedPrograms)
	}
	listed := "\n" + wantedRestart + "  " + caddyDropIn + "\n"
	if !strings.Contains(stdout, listed) {
		t.Errorf("Uninstall() printed\n%s\nwant the drop-in removed and Caddy restarted", stdout)
	}
	wantNothingAt(t, server, caddyDropIn)
	if record := recordOf(t, server); record.Caddyfile != "" || record.Steps[stepCaddy] != "" {
		t.Errorf("the record holds the sum %q and marks caddy %q, want neither", record.Caddyfile,
			record.Steps[stepCaddy])
	}
}

func TestUninstallLeavesNoCaddyStepOnceItRestartedCaddyForADropInBackAfterAnUninstall(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	if _, _, err := uninstallOn(server, nil, true, false); err != nil {
		t.Fatalf("Uninstall() = %v, want the first run through", err)
	}
	server.write(caddyDropIn, "[Service]\nRuntimeDirectory=caddy\n")
	found := recordOf(t, server)
	server.forgetRun()

	_, _, err := uninstallOn(server, nil, true, false)

	if want := []string{"systemctl daemon-reload", "systemctl restart caddy.service"}; err != nil ||
		!slices.Equal(server.ran, want) {
		t.Errorf("Uninstall() = %v, running %q, want %q", err, server.ran, want)
	}
	if got := recordOf(t, server); !recordsEqual(got, found) {
		t.Errorf("Uninstall() left the record %+v, want it as found %+v", got, found)
	}
}

func TestUninstallRestartsNoCaddyOverACaddyfileThePersonWroteWhileThePackagesOneIsKept(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write(caddyfilePath, ownCaddyfile)
	record := recordOf(t, server)
	record.Caddyfile = ""
	server.saveRecord(record)

	stdout, _, err := uninstallOn(server, nil, true, false)

	want := slices.Concat(removedPrograms[:3], []string{"systemctl reload caddy.service"})
	if err != nil || !slices.Equal(server.ran, want) {
		t.Errorf("Uninstall() = %v, running %q, want %q", err, server.ran, want)
	}
	if strings.Contains(stdout, "restarting Caddy") {
		t.Errorf("Uninstall() printed\n%s\nwant no restart of Caddy on the Caddyfile the person wrote", stdout)
	}
	wantText(t, server.at(caddyfilePath), ownCaddyfile)
	wantThere(t, server, keptCaddyfilePath, caddyDropIn)
}

func TestUninstallReloadsCaddyOnTheRunAfterOneCutOffBeforeItsReload(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write(caddyfilePath, wantedCaddyfile+personSite)
	server.cutSystemctl("systemctl reload caddy.service", 1)
	if _, _, err := uninstallOn(server, nil, true, false); err == nil {
		t.Fatalf("Uninstall() went through, want it cut off at the reload of Caddy")
	}
	wantThere(t, server, caddySitePath)
	server.answerRemovals()
	server.forgetRun()

	_, _, err := uninstallOn(server, nil, true, false)

	if want := []string{"systemctl reload caddy.service"}; err != nil || !slices.Equal(server.ran, want) {
		t.Errorf("Uninstall() = %v, running %q, want %q", err, server.ran, want)
	}
	wantNothingAt(t, server, caddySitePath, maintenanceFolder)
	wantText(t, server.at(caddyfilePath), strings.Replace(wantedCaddyfile, caddyImport+"\n", "", 1)+personSite)
	wantKeptCaddy(t, server, caddyfileSum(wantedCaddyfile))
}

func TestUninstallWithDeleteDataRemovesOnlyAnInstallRootTheRecordOwns(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	record := recordOf(t, server)
	record.Installed = slices.DeleteFunc(record.Installed, func(piece string) bool { return piece == "/opt/gophenberg" })
	server.saveRecord(record)

	stdout, _, err := uninstallOn(server, nil, true, true)

	kept := "\n  /opt/gophenberg, since the install record does not show that server:install made it\n"
	if err != nil || !strings.Contains(stdout, kept) {
		t.Errorf("Uninstall() = %v, printing\n%s\nwant the install root kept", err, stdout)
	}
	if strings.Contains(stdout, "/opt/gophenberg/versions") {
		t.Errorf("Uninstall() printed\n%s\nwant nothing of an install root the record does not own", stdout)
	}
	wantThere(t, server, siteEnvAt, "/opt/gophenberg/versions", "/opt/gophenberg/current")
}

func TestUninstallListsAsKeptAnInstallRootTheRecordOwnsThatServerEnvNoLongerNames(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	env := map[string]string{"GOPHENBERG_INSTALL_ROOT": "/srv/gophenberg"}
	if _, _, err := runInstallUnder(server, env, flaggedRunOn(t, server)); err != nil {
		t.Fatalf("Install() = %v, want Gophenberg installed in /srv/gophenberg", err)
	}
	server.answerRemovals()
	server.forgetRun()
	server.remove(serverEnvPath)

	stdout, _, err := uninstallOn(server, nil, true, true)

	kept := "server:uninstall keeps:\n  /srv/gophenberg, an install root server:install made, kept since " +
		"/etc/gophenberg/server.env does not name it\n"
	if err != nil || !strings.Contains(stdout, kept) {
		t.Errorf("Uninstall() = %v, printing\n%s\nwant it to hold\n%s", err, stdout, kept)
	}
	wantThere(t, server, "/srv/gophenberg/.env")
	want := []string{piecePostgreSQL, pieceCaddy}
	if got := recordOf(t, server).Installed; !slices.Equal(got, want) {
		t.Errorf("Uninstall() left the record owning %q, want %q", got, want)
	}
}

func TestUninstallListsNoInstallRootServerEnvNoLongerNamesThatIsGoneAndLeavesItOutOfTheRecord(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	if err := (&installScenario{server: server}).setServerLine("GOPHENBERG_INSTALL_ROOT", "/srv/gophenberg"); err != nil {
		t.Fatalf("changing the install root line: %v", err)
	}
	server.remove("/opt/gophenberg")

	stdout, _, err := uninstallOn(server, nil, true, true)

	if err != nil || strings.Contains(stdout, "/opt/gophenberg") {
		t.Errorf("Uninstall() = %v, printing\n%s\nwant nothing of an install root that is gone", err, stdout)
	}
	want := []string{piecePostgreSQL, pieceCaddy}
	if got := recordOf(t, server).Installed; !slices.Equal(got, want) {
		t.Errorf("Uninstall() left the record owning %q, want %q", got, want)
	}
}

func TestUninstallWithDeleteDataKeepsTheSumOfACaddyfileThePersonChanged(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write(caddyfilePath, wantedCaddyfile+personSite)

	_, _, err := uninstallOn(server, nil, true, true)

	if err != nil {
		t.Fatalf("Uninstall() = %v, want the data removed and the Caddyfile kept", err)
	}
	want := Record{
		Installer: version.Version(), Domain: "example.com", Installed: []string{piecePostgreSQL, pieceCaddy},
		Caddyfile: caddyfileSum(wantedCaddyfile), Steps: map[string]string{stepCaddy: stepEnded},
	}
	if got := recordOf(t, server); !recordsEqual(got, want) {
		t.Errorf("Uninstall() left the record %+v, want %+v", got, want)
	}
}

func TestUninstallStopsAtAnInstallRootThatIsNotARealFolder(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.remove("/opt/gophenberg")
	server.folder("/srv/elsewhere")
	if err := os.Symlink(server.at("/srv/elsewhere"), server.at("/opt/gophenberg")); err != nil {
		t.Fatalf("linking the install root: %v", err)
	}

	_, _, err := uninstallOn(server, nil, true, true)

	wantRefused(t, err, "installer: open the install root /opt/gophenberg: it is not a real folder")
	if len(server.ran) != 0 {
		t.Errorf("the run ran %q, want nothing run", server.ran)
	}
}

func TestUninstallRemovesThroughTheInstallTreeFollowingNoLink(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write("/srv/outside/archive.tar.gz", "kept\n")
	server.remove("/opt/gophenberg/downloads")
	if err := os.Symlink("../../srv/outside", server.at("/opt/gophenberg/downloads")); err != nil {
		t.Fatalf("linking the downloads folder: %v", err)
	}
	server.link("/opt/gophenberg/media/outside", "/srv/outside")

	_, _, err := uninstallOn(server, nil, true, true)

	if err != nil {
		t.Fatalf("Uninstall() = %v, want the links removed", err)
	}
	wantText(t, server.at("/srv/outside/archive.tar.gz"), "kept\n")
	wantNothingAt(t, server, "/opt/gophenberg")
}

func TestUninstallRefusesAnUpdateStateItCannotReachInsideTheInstallRoot(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.write("/srv/outside/update.json", "{}\n")
	server.remove("/opt/gophenberg/state")
	server.link("/opt/gophenberg/state", "/srv/outside")

	_, _, err := uninstallOn(server, nil, true, false)

	want := "installer: cannot tell whether /opt/gophenberg/state/update.json is there: "
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("Uninstall() = %v, want the state link that leads out refused", err)
	}
	wantThere(t, server, "/srv/outside/update.json", unitPaths[0])
}

func TestUninstallRefusesACaddyfileItCannotRead(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.remove(caddyfilePath)
	server.folder(caddyfilePath)

	_, _, err := uninstallOn(server, nil, true, false)

	wantRefused(t, err, "installer: read "+server.at(caddyfilePath)+": is a directory")
}

func TestUninstallStopsWhenItCannotTellWhetherTheSiteUserIsThere(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	machine := server.machine()
	machine.LookupUser = func(string) (*user.User, error) { return nil, errors.New("cannot read /etc/passwd") }

	_, _, err := uninstallWith(t, machine, gonsole.Call{Env: envOf(nil), Apply: true}, false)

	wantRefused(t, err, "installer: cannot read /etc/passwd")
	if len(server.ran) != 0 {
		t.Errorf("the run ran %q, want nothing run", server.ran)
	}
}

func TestUninstallReturnsWhatStopsItsLists(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	call := gonsole.Call{Env: envOf(nil), Stdout: refusingWriter{}, Apply: true}

	err := Uninstall(t.Context(), server.machine(), call, false)

	wantRefused(t, err, "the terminal is gone")
	if len(server.ran) != 0 {
		t.Errorf("the run ran %q, want nothing run once its lists could not be printed", server.ran)
	}
}

func TestUninstallReturnsWhatStopsItsLastLineOnceItMadeEveryChange(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	running, err := removingOn(server.machine(), gonsole.Call{Env: envOf(nil), Stdout: refusingWriter{}, Apply: true})
	if err != nil {
		t.Fatalf("removingOn() = %v, want the run ready", err)
	}
	t.Cleanup(running.close)
	if err := running.plan(); err != nil {
		t.Fatalf("plan() = %v, want the lists made", err)
	}

	err = running.apply(t.Context())

	wantRefused(t, err, "the terminal is gone")
	wantNothingAt(t, server, uninstalledPaths...)
}

func TestUninstallStopsWhereItCannotPutBackThePackagesCaddyfile(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.remove(keptCaddyfilePath)
	server.folder(keptCaddyfilePath)

	_, _, err := uninstallOn(server, nil, true, false)

	if err == nil || !strings.HasPrefix(err.Error(), "installer: put "+keptCaddyfilePath+" back as "+caddyfilePath+": ") {
		t.Errorf("Uninstall() = %v, want the put back refused", err)
	}
	if slices.Contains(server.ran, "systemctl restart caddy.service") {
		t.Errorf("the run ran %q, want Caddy left running as it was", server.ran)
	}
}

func TestUninstallStopsWhereAFolderRootAloneWritesRefusesItsChange(t *testing.T) {
	t.Parallel()
	skipAsRoot(t)

	for folder, want := range map[string]string{
		"/etc/systemd/system": "installer: remove /etc/systemd/system/gophenberg.service: ",
		"/etc/caddy":          "installer: write /etc/caddy/Caddyfile: ",
		"/opt/gophenberg":     "installer: remove /opt/gophenberg/versions: ",
		"/opt":                "installer: remove the install root /opt/gophenberg: ",
	} {
		server := firstInstalled(t)
		if folder == "/etc/caddy" {
			server.write(caddyfilePath, wantedCaddyfile+personSite)
		}
		server.chmod(folder, 0o555)
		t.Cleanup(func() { server.chmod(folder, 0o755) })

		_, _, err := uninstallOn(server, nil, true, folder == "/opt")

		if err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("Uninstall() under a locked %s = %v, want %q", folder, err, want)
		}
	}
}

func TestUninstallSkipsAnInstallRootThatIsGone(t *testing.T) {
	t.Parallel()

	server := firstInstalled(t)
	server.remove("/opt/gophenberg")

	stdout, _, err := uninstallOn(server, nil, true, true)

	if err != nil || strings.Contains(stdout, "/opt/gophenberg") {
		t.Errorf("Uninstall() = %v, printing\n%s\nwant nothing of an install root that is gone", err, stdout)
	}
	wantNothingAt(t, server, serverEnvPath)
}

func TestUninstallRefusesAServerItCannotOpenAsATree(t *testing.T) {
	t.Parallel()
	skipAsRoot(t)

	server := firstInstalled(t)
	server.chmod("/", 0o311)
	t.Cleanup(func() { server.chmod("/", 0o755) })

	_, _, err := uninstallOn(server, nil, true, false)

	wantRefused(t, err, "installer: open "+server.root+": permission denied")
	if len(server.ran) != 0 {
		t.Errorf("the run ran %q, want nothing run", server.ran)
	}
}

func TestUninstallStopsWhereItCannotLeaveTheInstallRecord(t *testing.T) {
	t.Parallel()

	for caddyfile, last := range map[string]bool{"the installer's": false, "one the person changed": true} {
		server := firstInstalled(t)
		if last {
			server.write(caddyfilePath, wantedCaddyfile+personSite)
		}
		if err := os.Rename(server.at(installerFolder), server.at("/var/lib/kept-installer")); err != nil {
			t.Fatalf("moving the installer's folder: %v", err)
		}
		kept, err := os.Lstat(server.at("/var/lib/kept-installer"))
		if err != nil {
			t.Fatalf("describing the installer's folder: %v", err)
		}
		if err := os.Symlink("kept-installer", server.at(installerFolder)); err != nil {
			t.Fatalf("linking the installer's folder: %v", err)
		}
		server.described[installerFolder] = ownedInfo{FileInfo: kept}

		stdout, _, err := uninstallOn(server, nil, true, false)

		wantRefused(t, err, "installer: make the folder var/lib/gophenberg-installer: it is not a real folder")
		if strings.HasSuffix(stdout, removedLine) {
			t.Errorf("Uninstall() over %s Caddyfile printed\n%s\nwant no last line once the record could not be left",
				caddyfile, stdout)
		}
		if held := server.machine().there(caddySitePath); held == last {
			t.Errorf("Uninstall() over %s Caddyfile left gophenberg.caddy there %t, want %t, the record saved first "+
				"while the Caddyfile goes back and last otherwise", caddyfile, held, !last)
		}
	}
}

func TestTheUninstallPathsAndStepsAreTheOnesTheInstallWrites(t *testing.T) {
	t.Parallel()

	wantThere(t, firstInstalled(t), slices.Concat(uninstalledPaths, keptPaths)...)
	written := writtenPaths(firstSite(t, "example.com", 255).unitFiles())
	if want := append(slices.Clone(unitPaths), caddyDropIn); !slices.Equal(written, want) {
		t.Errorf("the units step writes %q, want %q", written, want)
	}
	if want := []string{"versions", "downloads", "current", "previous", "state/update.json"}; !slices.Equal(
		builtNames, want) {
		t.Errorf("server:uninstall removes %q in the install root, want %q", builtNames, want)
	}
	for _, id := range keptSteps {
		if !slices.Contains(stepIDs, id) {
			t.Errorf("server:uninstall keeps the step %s, which no run takes", id)
		}
	}
}
