// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"testing/synctest"

	"github.com/cucumber/godog"
	"github.com/joho/godotenv"

	"github.com/gopherium/gophenberg/internal/version"
)

// serverInstallFeature is the feature server:install answers to on a fake server.
const serverInstallFeature = "../../test/features/features/server-install.feature"

// earlierRecord is the record of an earlier run for example.com that ended every step, its Caddyfile's sum left open.
const earlierRecord = `{"installer": "0.0.1", "domain": "example.com", "caddyfile": "%s",
"installed": ["postgresql", "caddy", "gophenberg.service", "/opt/gophenberg"],
"steps": {"server-env": "ended", "packages": "ended", "user": "ended", "install-root": "ended", "database": "ended",
"build": "ended", "installer-copy": "ended", "site-env": "ended", "units": "ended", "command": "ended",
"caddy": "ended", "start": "ended", "admin": "ended", "https": "ended", "summary": "ended"}}
`

// thisPlatform is the platform of this server as a release.json names it.
const thisPlatform = "linux/" + runtime.GOARCH

// probes are the programs the checks run, none of which changes a server.
var probes = []string{"systemctl --version", sandboxLine, "systemd-detect-virt", clockLine}

// systemReleases are the os-release files of the systems a scenario's server runs, by the name the scenario gives.
var systemReleases = map[string]string{
	"Ubuntu 22.04": "PRETTY_NAME=\"Ubuntu 22.04.5 LTS\"\nVERSION_ID=\"22.04\"\nID=ubuntu\n",
	"Ubuntu 24.04": ubuntuRelease,
	"Ubuntu 26.04": "PRETTY_NAME=\"Ubuntu 26.04 LTS\"\nNAME=\"Ubuntu\"\nVERSION_ID=\"26.04\"\n" +
		"VERSION=\"26.04 LTS (Resolute Raccoon)\"\nVERSION_CODENAME=resolute\nID=ubuntu\nID_LIKE=debian\n",
	"Ubuntu 26.10": "PRETTY_NAME=\"Ubuntu 26.10\"\nNAME=\"Ubuntu\"\nVERSION_ID=\"26.10\"\nID=ubuntu\nID_LIKE=debian\n",
	"Debian 12":    debianRelease(12, "bookworm"),
	"Debian 14":    debianRelease(14, "forky"),
	"Fedora 41":    "PRETTY_NAME=\"Fedora Linux 41 (Server Edition)\"\nVERSION_ID=41\nID=fedora\n",
}

// printedFile is one file a dry run prints: the line that names it, and its text.
type printedFile struct {
	header, text string
}

// hiddenValue ends each line of the site's .env that a dry run prints with the person's value hidden.
const hiddenValue = "=(kept as it is)"

// installScenario is what one server:install scenario keeps between its steps.
type installScenario struct {
	t       *testing.T
	server  *fakeServer
	env     map[string]string
	planted map[string]string
	before  map[string]string
	stdout  string
	stderr  string
	err     error
	checked map[string]bool
	planned string
	archive string
	copied  fs.FileInfo
}

// initializeServerInstall returns the binding of the server:install steps, t holding every fake server.
func initializeServerInstall(t *testing.T) func(*godog.ScenarioContext) {
	return func(sc *godog.ScenarioContext) {
		s := &installScenario{t: t, env: map[string]string{}, planted: map[string]string{}, checked: map[string]bool{}}
		sc.Given(`^a fresh Ubuntu 24.04 server run by systemd 255$`, s.freshServer)
		sc.Given(`^the operator is not root$`, s.notRoot)
		sc.Given(`^the server runs (Ubuntu 22.04|Ubuntu 24.04|Ubuntu 26.04|Ubuntu 26.10|Debian 14|Fedora 41)$`,
			s.runSystem)
		sc.Given(`^the server runs systemd (\d+)$`, s.runSystemd)
		sc.Given(`^the server runs (Ubuntu 24.04|Debian 12) with systemd (\d+)$`, s.runSystemWithSystemd)
		sc.Given(`^systemd cannot set up a sandbox on the server, a container systemd-detect-virt reports as lxc$`,
			s.failSandbox)
		sc.Given(`^the server has (\d+) MB of memory$`, s.holdMemory)
		sc.Given(`^something listens on TCP port 80$`, s.takePort80)
		sc.Given(`^example.com has no A or AAAA record$`, s.resolveNothing)
		sc.Given(`^example.com points at 198.51.100.7, where another server answers every page with 404$`,
			s.pointElsewhere)
		sc.Given(`^example.com resolves to 127.0.1.1, as a line in /etc/hosts makes it$`, s.pointAtHostsLine)
		sc.Given(`^(?:www\.)?example.com points at 198.51.100.7, a public address a NAT in front of the server forwards`+
			` to it$`, s.pointThroughNAT)
		sc.Given(`^PostgreSQL is installed on the server$`, s.installPostgreSQL)
		sc.Given(`^Caddy is installed on the server$`, s.installCaddy)
		sc.Given(`^another user can write (/opt|/var/lib/gophenberg-installer)$`, s.openFolder)
		sc.Given(`^a plain file sits at /var/lib/gophenberg-installer$`, s.fileAtInstallerFolder)
		sc.Given(`^the install record names the installer "([^"]*)"$`, s.recordInstaller)
		sc.Given(`^an earlier run installed Gophenberg for example.com on the server$`, s.installEarlier)
		sc.Given(`^another server:install is running on the server$`, s.runAnother)
		sc.Given(`^only root may read the server settings and the install record$`, s.lockRootFiles)
		sc.Given(`^the web server line of the server settings was changed by hand to none$`, s.keepOwnWebServer)
		sc.Given(`^the site's .env holds the line "([^"]*)"$`, s.addSiteEnvLine)
		sc.Given(`^the site's .env holds the line "([^"]*)", <ESC> standing for the escape character$`,
			s.addEscapedSiteEnvLine)
		sc.Given(`^unattended-upgrades is installed on the server$`, s.installUnattended)
		sc.Given(`^the environment sets (GOPHENBERG_[A-Z_]+) to (\S+)$`, s.setEnv)
		initializeServerInstallAnswers(sc, s)
		initializeServerInstallCutOffs(sc, s)
		sc.When(`^the operator runs "([^"]*)"$`, s.runLine)
		initializeServerInstallOutcomes(sc, s)
		initializeServerInstallChanges(sc, s)
		initializeServerInstallApplies(sc, s)
		initializeServerUninstall(sc, s)
	}
}

// initializeServerInstallCutOffs binds the steps that leave the server as a first run cut off part way left it.
func initializeServerInstallCutOffs(sc *godog.ScenarioContext, s *installScenario) {
	sc.Given(`^a first run for example.com was cut off while apt-get installed PostgreSQL$`, s.cutOffInPackages)
	sc.Given(`^a first run for example.com was cut off after createuser made the database role gophenberg$`,
		s.cutOffAfterCreateuser)
	sc.Given(`^a first run for example.com was cut off after it unpacked part of the build of this installer$`,
		s.cutOffInBuild)
}

// initializeServerInstallChanges binds the steps that check what a server:install run changed on the server.
func initializeServerInstallChanges(sc *godog.ScenarioContext, s *installScenario) {
	sc.Then(`^the server holds every file the dry run printed, with its owner and mode$`, s.holdsPrintedFiles)
	sc.Then(`^the server holds these folders:$`, s.holdsFolders)
	sc.Then(`^the server holds the install record, owned by (root|gophenberg) with mode (0[0-7]{3})$`, s.holdsRecord)
	sc.Then(`^the server holds the installer's own copy, owned by (root|gophenberg) with mode (0[0-7]{3})$`,
		s.holdsInstallerCopy)
	sc.Then(`^the build folder of this installer holds only the files of its archive, owned by gophenberg$`,
		s.holdsOwnBuild)
	sc.Then(`^the current link names the build of this installer, owned by gophenberg$`, s.pointsAtOwnBuild)
	sc.Then(`^the run ran "([^"]*)" (before|after) "([^"]*)"$`, s.ranInOrder)
	sc.Then(`^the install record marks the step (\S+) (started|ended)$`, s.recordMarks)
}

// initializeServerInstallAnswers binds the steps that give a run its archive, its answers and the files it checks.
func initializeServerInstallAnswers(sc *godog.ScenarioContext, s *installScenario) {
	sc.Given(`^the operator's terminal answers:$`, s.answerOnTerminal)
	sc.Given(`^the release archive of this installer is at (/\S+)$`, s.putOwnArchive)
	sc.Given(`^a release archive of Gophenberg ([0-9.]+) is at (/\S+)$`, s.putArchiveOfVersion)
	sc.Given(`^a release archive of this installer built for (\S+) is at (/\S+)$`, s.putArchiveForPlatform)
	sc.Given(`^the file (/\S+) holds the line "([^"]*)"$`, s.writeLine)
	sc.Given(`^the file (/\S+) holds the line "([^"]*)" ending in a carriage return$`, s.writeLineWithReturn)
	sc.Given(`^another package put a gophenberg command at /usr/local/bin/gophenberg$`, s.putOtherCommand)
	sc.Given(`^the line "([^"]*)" was taken out of /etc/caddy/Caddyfile$`, s.takeOutCaddyfileLine)
	sc.Given(`^the current link of the install root names /opt/gophenberg/versions/0.19.0 by its full path$`,
		s.pointCurrentByFullPath)
	sc.Given(`^the current link of the install root names versions/0.19.0, a build folder deleted by hand$`,
		s.deleteCurrentBuild)
	sc.Given(`^the current link of the install root names versions/0.19.0, whose program nobody may run$`,
		s.stopCurrentProgram)
}

// initializeServerInstallOutcomes binds the steps that check what a server:install run did to s.
func initializeServerInstallOutcomes(sc *godog.ScenarioContext, s *installScenario) {
	sc.Then(`^the run stops with "(.*)"$`, s.stopsWith)
	sc.Then(`^the run stops because this installer is older than the ([0-9.]+) installer$`, s.stopsAsOlder)
	sc.Then(`^the run stops because the archive (\S+) holds Gophenberg ([0-9.]+)$`, s.stopsAsOtherVersion)
	sc.Then(`^the run stops because the archive (\S+) is built for (\S+)$`, s.stopsAsOtherPlatform)
	sc.Then(`^the run asked nothing on the terminal$`, s.askedNothing)
	sc.Then(`^the terminal showed:$`, s.terminalShowed)
	sc.Then(`^the run succeeds$`, s.succeeds)
	sc.Then(`^the run notes "([^"]*)"$`, s.notes)
	sc.Then(`^the run notes nothing$`, s.notesNothing)
	sc.Then(`^the run prints nothing$`, s.printsNothing)
	sc.Then(`^nothing the run printed holds "([^"]*)"$`, s.printedNothingOf)
	sc.Then(`^nothing on the server changed$`, s.changedNothing)
	sc.Then(`^the checks listened on no port$`, s.listenedNowhere)
	sc.Then(`^the checks fetched no page$`, s.fetchedNothing)
	sc.Then(`^the checks listened only on 127.0.0.1:8081 and TCP port 5432$`, s.listenedOnSiteAndDatabase)
	sc.Then(`^the checks looked up no domain$`, s.resolvedNothing)
	sc.Then(`^the dry run writes only these files:$`, s.writesOnly)
	sc.Then(`^the dry run prints for the person's web server:$`, s.printsForWebServer)
	sc.Then(`^the dry run lists these steps:$`, s.listsSteps)
	sc.Then(`^the dry run writes "([^"]*)", owned by (root|gophenberg) with mode (0[0-7]{3}):$`, s.writes)
	sc.Then(`^the dry run writes "([^"]*)", owned by root with mode 0644, holding Caddy's repository key$`,
		s.writesCaddyKey)
	sc.Then(`^the dry run writes no other file$`, s.writesNoOtherFile)
	sc.Then(`^the dry run writes every other file as it does for example.com$`, s.writesAsForExample)
	sc.Then(`^the site's unit (holds|lacks) the line "([^"]*)"$`, s.unitHolds)
}

// freshServer gives the scenario a fresh Ubuntu 24.04 server run by systemd 255 that every check passes on.
func (s *installScenario) freshServer() {
	s.server = newFakeServer(s.t)
}

// notRoot has the run start as a plain user.
func (s *installScenario) notRoot() {
	s.server.euid = 1000
}

// runSystem has the server run system, by the os-release systemReleases holds for it.
func (s *installScenario) runSystem(system string) {
	s.server.write(osReleasePath, systemReleases[system])
}

// runSystemd has the server run systemd major.
func (s *installScenario) runSystemd(major int) {
	s.server.answers["systemctl"] = answer{printed: fmt.Sprintf("systemd %d (%d.4-1)\n+PAM +AUDIT\n", major, major)}
}

// runSystemWithSystemd has the server run system, Ubuntu 24.04 or Debian 12, with systemd major.
func (s *installScenario) runSystemWithSystemd(system string, major int) {
	s.runSystem(system)
	s.runSystemd(major)
}

// failSandbox has systemd fail to set up the sandbox of the site's service in a container reported as lxc.
func (s *installScenario) failSandbox() {
	s.server.answers["systemd-run"] = answer{
		printed: "Job for run-u7.service failed.\n", err: fmt.Errorf("installer: run systemd-run: exit status 1"),
	}
	s.server.answers["systemd-detect-virt"] = answer{printed: "lxc\n"}
}

// holdMemory gives the server megabytes of memory.
func (s *installScenario) holdMemory(megabytes int) {
	s.server.write(meminfoPath, meminfoOf(megabytes*1024))
}

// takePort80 has something else listen on TCP port 80.
func (s *installScenario) takePort80() {
	s.server.taken["tcp :80"] = inUse("tcp", ":80")
}

// resolveNothing has every domain resolve to no address.
func (s *installScenario) resolveNothing() {
	s.server.unresolved = &net.DNSError{Err: "no such host", Name: "example.com", IsNotFound: true}
}

// pointElsewhere has every domain point at 198.51.100.7, where another server answers every page with 404.
func (s *installScenario) pointElsewhere() {
	s.server.addresses = addresses("198.51.100.7")
	s.server.answerFetch = answered(http.StatusNotFound, "404 page not found\n")
}

// pointAtHostsLine has every domain resolve to 127.0.1.1, as Go reads a line of /etc/hosts.
func (s *installScenario) pointAtHostsLine() {
	s.server.addresses = addresses("::ffff:127.0.1.1")
}

// pointThroughNAT has every domain point at 198.51.100.7, a public address that none of the server's cards holds.
func (s *installScenario) pointThroughNAT() {
	s.server.addresses = addresses("198.51.100.7")
}

// installPostgreSQL puts PostgreSQL's settings on the server, as its package does.
func (s *installScenario) installPostgreSQL() {
	s.server.folder("/etc/postgresql/16/main")
}

// installCaddy puts Caddy's settings and program on the server, as its package does.
func (s *installScenario) installCaddy() {
	s.server.folder("/etc/caddy")
	s.server.program("/usr/bin/caddy")
}

// openFolder makes the folder at path where it is missing and lets every user write it.
func (s *installScenario) openFolder(path string) {
	s.server.folder(path)
	s.server.chmod(path, 0o777)
}

// fileAtInstallerFolder puts a plain file where the installer folder belongs.
func (s *installScenario) fileAtInstallerFolder() {
	s.server.write(installerFolder, "")
}

// recordInstaller has the install record name installer as the newest installer that ran on the server.
func (s *installScenario) recordInstaller(installer string) {
	s.server.write(recordPath, `{"installer": "`+installer+`"}`)
}

// installEarlier sets the server up as an earlier run for example.com left it, every port held by what it installed.
func (s *installScenario) installEarlier() {
	server := s.server
	server.write(serverEnvPath, firstServerEnv)
	server.write(recordPath, fmt.Sprintf(earlierRecord, caddyfileSum(wantedCaddyfile)))
	server.installPackage(piecePostgreSQL)
	server.installPackage(pieceCaddy)
	server.addUser(siteUser, "/opt/gophenberg")
	server.running["caddy.service"], server.running["gophenberg.service"] = true, true
	server.write(keptCaddyfilePath, packageCaddyfile)
	server.write("/etc/caddy/Caddyfile", wantedCaddyfile)
	server.write(caddySitePath, wantedCaddySite)
	server.write("/etc/systemd/system/gophenberg.service", wantedSiteUnit)
	server.write(commandPath, wantedCommand)
	server.write(siteEnvAt, wantedSiteEnv)
	server.pointCurrent()
	for _, held := range portListens {
		network, address, _ := strings.Cut(held, " ")
		server.taken[held] = inUse(network, address)
	}
}

// runAnother holds the lock of the installer folder as another run does, until the test ends.
func (s *installScenario) runAnother() error {
	folder, err := os.Open(s.server.at(installerFolder))
	if err != nil {
		return err
	}
	s.t.Cleanup(func() { _ = folder.Close() })
	return syscall.Flock(int(folder.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

// lockRootFiles takes away every right on server.env and the install record, as a plain user meets them.
func (s *installScenario) lockRootFiles() {
	s.server.chmod(serverEnvPath, 0)
	s.server.chmod(recordPath, 0)
}

// keepOwnWebServer changes the web server line of server.env from caddy to none, as a hand edit does.
func (s *installScenario) keepOwnWebServer() error {
	held, err := os.ReadFile(s.server.at(serverEnvPath))
	if err != nil {
		return err
	}
	s.server.write(serverEnvPath, strings.Replace(string(held), "WEB_SERVER=caddy", "WEB_SERVER=none", 1))
	return nil
}

// addSiteEnvLine adds line to the site's .env, keeping the value it sets by its key.
func (s *installScenario) addSiteEnvLine(line string) error {
	held, err := os.ReadFile(s.server.at(siteEnvAt))
	if err != nil {
		return err
	}
	set, err := godotenv.Unmarshal(line)
	if err != nil {
		return err
	}
	maps.Copy(s.planted, set)
	s.server.write(siteEnvAt, string(held)+line+"\n")
	return nil
}

// addEscapedSiteEnvLine adds line to the site's .env with the escape character in place of each <ESC>.
func (s *installScenario) addEscapedSiteEnvLine(line string) error {
	return s.addSiteEnvLine(strings.ReplaceAll(line, "<ESC>", "\x1b"))
}

// installUnattended puts the program of unattended-upgrades on the server.
func (s *installScenario) installUnattended() {
	s.server.program(unattendedUpgrade)
}

// setEnv sets the setting called key to value in the environment of the run.
func (s *installScenario) setEnv(key, value string) {
	s.env[key] = value
}

// answerOnTerminal gives the run a terminal that answers each line of answers in turn, an empty line by Enter alone.
func (s *installScenario) answerOnTerminal(answers *godog.DocString) {
	s.server.prompt = &fakePrompt{answers: strings.Split(answers.Content, "\n")}
}

// putOwnArchive puts the release archive of this installer for this server at path.
func (s *installScenario) putOwnArchive(path string) {
	s.server.write(path, releaseArchiveOf(s.t, version.Version(), thisPlatform))
	s.archive = path
}

// putArchiveOfVersion puts a release archive of Gophenberg release for this server at path.
func (s *installScenario) putArchiveOfVersion(release, path string) {
	s.server.write(path, releaseArchiveOf(s.t, release, thisPlatform))
}

// putArchiveForPlatform puts a release archive of this installer built for platform at path.
func (s *installScenario) putArchiveForPlatform(platform, path string) {
	s.server.write(path, releaseArchiveOf(s.t, version.Version(), platform))
}

// writeLine puts a file at path that holds line and its newline.
func (s *installScenario) writeLine(path, line string) {
	s.server.write(path, line+"\n")
}

// writeLineWithReturn puts a file at path that holds line ended by a carriage return and a newline.
func (s *installScenario) writeLineWithReturn(path, line string) {
	s.server.write(path, line+"\r\n")
}

// putOtherCommand puts a gophenberg command on the path that another package wrote.
func (s *installScenario) putOtherCommand() {
	s.server.write(commandPath, "#!/bin/sh\nexec /opt/other/bin/gophenberg \"$@\"\n")
	s.server.chmod(commandPath, 0o755)
}

// takeOutCaddyfileLine takes line out of the Caddyfile an earlier run wrote, as a hand edit does.
func (s *installScenario) takeOutCaddyfileLine(line string) error {
	held, err := os.ReadFile(s.server.at("/etc/caddy/Caddyfile"))
	if err != nil {
		return err
	}
	kept := strings.Replace(string(held), line+"\n", "", 1)
	if kept == string(held) {
		return fmt.Errorf("the Caddyfile\n%s\nholds no line %q", held, line)
	}
	s.server.write("/etc/caddy/Caddyfile", kept)
	return nil
}

// pointCurrentByFullPath points the current link at its build by the full path, which updater.Point never writes.
func (s *installScenario) pointCurrentByFullPath() error {
	s.server.remove("/opt/gophenberg/current")
	return os.Symlink("/opt/gophenberg/versions/0.19.0", s.server.at("/opt/gophenberg/current"))
}

// deleteCurrentBuild deletes the build folder the current link names and leaves the link, as a hand deletion does.
func (s *installScenario) deleteCurrentBuild() {
	s.server.remove("/opt/gophenberg/versions/0.19.0")
}

// stopCurrentProgram takes every execute bit off the program of the build the current link names, as a chmod does.
func (s *installScenario) stopCurrentProgram() {
	s.server.chmod(currentProgram, 0o644)
}

// runLine runs a server:uninstall line, or a server:install line in fake time, so that no wait takes real time.
func (s *installScenario) runLine(line string) error {
	if strings.HasPrefix(line, uninstallCommand) {
		return s.runUninstall(line)
	}
	request, err := requestOf(line)
	if err != nil {
		return err
	}
	s.before = s.server.snapshot()
	s.server.forgetRun()
	synctest.Test(s.t, func(t *testing.T) {
		s.stdout, s.stderr, s.err = installUnder(t.Context(), s.server, s.env, request)
	})
	if request.DryRun {
		s.planned = s.stdout
	}
	return nil
}

// forgetRun forgets the programs, the HTTPS pages and the terminals an earlier run of the scenario asked for.
func (s *fakeServer) forgetRun() {
	s.runs, s.ran, s.fetchedHTTPS, s.httpsRoots, s.terminals = nil, nil, nil, nil, 0
}

// requestOf returns what the server:install line asks for.
func requestOf(line string) (Request, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 || fields[0] != "server:install" {
		return Request{}, fmt.Errorf("the line %q runs no server:install", line)
	}
	flags := flag.NewFlagSet("server:install", flag.ContinueOnError)
	var request Request
	flags.StringVar(&request.Domain, "domain", "", "")
	flags.StringVar(&request.Archive, "archive", "", "")
	flags.StringVar(&request.AdminEmail, "admin-email", "", "")
	flags.StringVar(&request.AdminName, "admin-name", "", "")
	flags.StringVar(&request.AdminPasswordFile, "admin-password-file", "", "")
	flags.StringVar(&request.WebServer, "web-server", "", "")
	flags.BoolVar(&request.SkipDomainCheck, "skip-domain-check", false, "")
	flags.BoolVar(&request.DryRun, "dry-run", false, "")
	return request, flags.Parse(fields[1:])
}

// snapshot returns every file, folder and link on the fake server by its path, with its mode and what it holds.
func (s *fakeServer) snapshot() map[string]string {
	held := map[string]string{}
	err := filepath.WalkDir(s.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		held[s.serverPath(path)] = info.Mode().String() + " " + contentOf(path, info)
		return nil
	})
	if err != nil {
		s.t.Errorf("reading the fake server: %v", err)
	}
	return held
}

// contentOf returns the text of the file at path, the path a link leads to, or nothing for a folder.
func contentOf(path string, info fs.FileInfo) string {
	if info.Mode()&fs.ModeSymlink != 0 {
		target, _ := os.Readlink(path)
		return target
	}
	if info.IsDir() {
		return ""
	}
	held, _ := os.ReadFile(path)
	return string(held)
}

// stopsWith fails unless the run stopped with want.
func (s *installScenario) stopsWith(want string) error {
	if s.err == nil || s.err.Error() != want {
		return fmt.Errorf("the run ended with %v, want %q", s.err, want)
	}
	return nil
}

// stopsAsOlder fails unless the run stopped because this installer is older than the one the record names.
func (s *installScenario) stopsAsOlder(recorded string) error {
	return s.stopsWith("this installer is " + version.Version() + ", and this server was set up by the " + recorded +
		" installer, so run the install.sh line again")
}

// stopsAsOtherVersion fails unless the run stopped because the archive at path holds Gophenberg release.
func (s *installScenario) stopsAsOtherVersion(path, release string) error {
	return s.stopsWith("the archive " + path + " holds Gophenberg " + release + ", not this installer's " +
		version.Version() + ", and server:install installs its own release only, so nothing was changed")
}

// stopsAsOtherPlatform fails unless the run stopped because the archive at path is built for platform.
func (s *installScenario) stopsAsOtherPlatform(path, platform string) error {
	return s.stopsWith("the archive " + path + " is built for " + platform + ", not for this server's " +
		thisPlatform + ", so nothing was changed")
}

// askedNothing fails when the run opened the terminal at all.
func (s *installScenario) askedNothing() error {
	if s.server.terminals != 0 {
		return fmt.Errorf("the run opened the terminal %d times, want it never opened", s.server.terminals)
	}
	return nil
}

// terminalShowed fails unless the terminal showed the questions and lines want lists, one a line, in order.
func (s *installScenario) terminalShowed(want *godog.DocString) error {
	if s.server.prompt == nil {
		return errors.New("the scenario gave the run no terminal")
	}
	if got := s.server.prompt.shown(); got != want.Content {
		return fmt.Errorf("the terminal showed\n%s\nwant\n%s", got, want.Content)
	}
	if !s.server.prompt.closed {
		return errors.New("the run left the terminal open")
	}
	return nil
}

// succeeds fails unless the run passed every check.
func (s *installScenario) succeeds() error {
	if s.err != nil {
		return fmt.Errorf("the run stopped with %q, want every check passed", s.err)
	}
	return nil
}

// notes fails unless the run noted the line on its error stream.
func (s *installScenario) notes(line string) error {
	if !slices.Contains(strings.Split(s.stderr, "\n"), line) {
		return fmt.Errorf("the run noted %q, want the line %q", s.stderr, line)
	}
	return nil
}

// notesNothing fails when the run noted anything on its error stream.
func (s *installScenario) notesNothing() error {
	if s.stderr != "" {
		return fmt.Errorf("the run noted %q, want nothing", s.stderr)
	}
	return nil
}

// printsNothing fails when the run printed anything on its answer stream.
func (s *installScenario) printsNothing() error {
	if s.stdout != "" {
		return fmt.Errorf("the run printed %q, want nothing", s.stdout)
	}
	return nil
}

// printedNothingOf fails when the run printed text on its answer stream, its error stream or in its error.
func (s *installScenario) printedNothingOf(text string) error {
	if printed := s.stdout + s.stderr + fmt.Sprint(s.err); strings.Contains(printed, text) {
		return fmt.Errorf("the run printed\n%s\nwant nothing that holds %q", printed, text)
	}
	return nil
}

// changedNothing fails when a file, folder or link on the server changed, or a program ran that is not a check's.
func (s *installScenario) changedNothing() error {
	if after := s.server.snapshot(); !maps.Equal(after, s.before) {
		return fmt.Errorf("the server held\n%v\nbefore the run and\n%v\nafter it", s.before, after)
	}
	for _, ran := range s.server.ran {
		if !slices.Contains(probes, ran) {
			return fmt.Errorf("the run ran %q, which is no check", ran)
		}
	}
	return nil
}

// listenedNowhere fails when the checks listened on any port.
func (s *installScenario) listenedNowhere() error {
	if len(s.server.listened) != 0 {
		return fmt.Errorf("the checks listened on %q, want no port", s.server.listened)
	}
	return nil
}

// fetchedNothing fails when the checks fetched any page.
func (s *installScenario) fetchedNothing() error {
	if len(s.server.fetched) != 0 {
		return fmt.Errorf("the checks fetched %q, want no page", s.server.fetched)
	}
	return nil
}

// listenedOnSiteAndDatabase fails unless the checks listened on the site's own port and PostgreSQL's alone, in order.
func (s *installScenario) listenedOnSiteAndDatabase() error {
	if want := []string{"tcp 127.0.0.1:8081", "tcp :5432"}; !slices.Equal(s.server.listened, want) {
		return fmt.Errorf("the checks listened on %q, want %q", s.server.listened, want)
	}
	return nil
}

// resolvedNothing fails when the checks looked up any domain.
func (s *installScenario) resolvedNothing() error {
	if len(s.server.resolved) != 0 {
		return fmt.Errorf("the checks looked up %q, want no domain", s.server.resolved)
	}
	return nil
}

// writesOnly fails unless the files the dry run prints are those want names, one path a line, in order.
func (s *installScenario) writesOnly(want *godog.DocString) error {
	var paths []string
	for line := range strings.Lines(s.stdout) {
		if named, isHeader := strings.CutPrefix(line, "  writes "); isHeader {
			path, _, _ := strings.Cut(named, ", owned by ")
			paths = append(paths, path)
		}
	}
	if got := strings.Join(paths, "\n"); got != want.Content {
		return fmt.Errorf("the dry run writes\n%s\nwant\n%s", got, want.Content)
	}
	return nil
}

// printsForWebServer fails unless the text a step of the dry run prints is want, line for line.
func (s *installScenario) printsForWebServer(want *godog.DocString) error {
	got, found := printedText(s.stdout)
	if !found {
		return fmt.Errorf("the dry run printed\n%s\nwith no step that prints a text", s.stdout)
	}
	if got != want.Content+"\n" {
		return fmt.Errorf("the dry run prints\n%s\nwant\n%s", got, want.Content)
	}
	return nil
}

// printedText returns the text the step of a dry run that prints a text prints, as its stdout shows it indented.
func printedText(stdout string) (string, bool) {
	_, after, found := strings.Cut(stdout, "\n  prints:\n")
	block, _, _ := strings.Cut(after, "\nStep ")
	var printed strings.Builder
	for line := range strings.Lines(block) {
		printed.WriteString(strings.TrimPrefix(line, "    "))
	}
	return printed.String(), found
}

// listsSteps fails unless the steps the dry run lists are want, line for line.
func (s *installScenario) listsSteps(want *godog.DocString) error {
	var listed []string
	for line := range strings.Lines(s.stdout) {
		if strings.HasPrefix(line, "Step ") {
			listed = append(listed, strings.TrimSuffix(line, "\n"))
		}
	}
	if got := strings.Join(listed, "\n"); got != want.Content {
		return fmt.Errorf("the dry run lists\n%s\nwant\n%s", got, want.Content)
	}
	return nil
}

// printedFiles returns every file the dry run prints, by its path.
func (s *installScenario) printedFiles() map[string]printedFile {
	printed := map[string]printedFile{}
	var path string
	var text strings.Builder
	keep := func() {
		if path != "" {
			held := printed[path]
			held.text = strings.TrimRight(text.String(), "\n") + "\n"
			printed[path] = held
		}
	}
	for line := range strings.Lines(s.stdout) {
		named, isHeader := strings.CutPrefix(line, "  writes ")
		switch {
		case isHeader:
			keep()
			path, _, _ = strings.Cut(named, ", owned by ")
			printed[path] = printedFile{header: strings.TrimSuffix(line, "\n")}
			text.Reset()
		case strings.HasPrefix(line, "Step "):
			keep()
			path = ""
		case path != "":
			text.WriteString(strings.TrimPrefix(line, "    "))
		}
	}
	keep()
	return printed
}

// writes fails unless the dry run prints the file at path, owned by owner with mode, holding want line for line.
func (s *installScenario) writes(path, owner, mode string, want *godog.DocString) error {
	return s.writesText(path, "owned by "+owner+" with mode "+mode, want.Content+"\n")
}

// writesText fails unless the dry run prints the file at path with the owner and mode named, holding want.
func (s *installScenario) writesText(path, owned, want string) error {
	s.checked[path] = true
	printed, found := s.printedFiles()[path]
	if !found {
		return fmt.Errorf("the dry run printed\n%s\nwith no file %s", s.stdout, path)
	}
	if header := "  writes " + path + ", " + owned + ":"; printed.header != header {
		return fmt.Errorf("the dry run names the file with %q, want %q", printed.header, header)
	}
	if printed.text != want {
		return fmt.Errorf("the dry run writes %s holding\n%s\nwant\n%s", path, printed.text, want)
	}
	return nil
}

// writesCaddyKey fails unless the dry run prints the key of Caddy's own repository, as the binary carries it.
func (s *installScenario) writesCaddyKey() error {
	return s.writesText("/etc/apt/keyrings/caddy.asc", "owned by root with mode 0644", caddyKey)
}

// writesNoOtherFile fails when the dry run prints a file no step of the scenario checked.
func (s *installScenario) writesNoOtherFile() error {
	for path := range s.printedFiles() {
		if !s.checked[path] {
			return fmt.Errorf("the dry run writes %s, which the scenario does not check", path)
		}
	}
	return nil
}

// writesAsForExample fails unless every file the scenario did not check is as a dry run for example.com prints it.
func (s *installScenario) writesAsForExample() error {
	example := newFakeServer(s.t)
	example.program(unattendedUpgrade)
	stdout, _, err := runInstall(example, Request{Domain: "example.com", AdminEmail: "admin@example.com", DryRun: true})
	if err != nil {
		return fmt.Errorf("the dry run for example.com stopped with %w", err)
	}
	wanted := (&installScenario{stdout: stdout}).printedFiles()
	got := s.printedFiles()
	if gotPaths, wantedPaths := slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(wanted)); !slices.Equal(
		gotPaths, wantedPaths) {
		return fmt.Errorf("the dry run writes %v, want %v", gotPaths, wantedPaths)
	}
	for path, file := range wanted {
		if !s.checked[path] && got[path] != file {
			return fmt.Errorf("the dry run writes %s as\n%s\nwant it as for example.com\n%s", path, got[path].text,
				file.text)
		}
	}
	return nil
}

// unitHolds fails unless the site's unit the dry run prints holds the line, or lacks it when holds says so.
func (s *installScenario) unitHolds(holds, line string) error {
	unit := s.printedFiles()["/etc/systemd/system/gophenberg.service"].text
	held := slices.Contains(strings.Split(unit, "\n"), line)
	if held != (holds == "holds") {
		return fmt.Errorf("the site's unit holding the line %q is %s, want the unit that %s it:\n%s", line,
			strconv.FormatBool(held), holds, unit)
	}
	return nil
}

// ownBuild is the build folder of this installer in the install root, named from the top of the server.
var ownBuild = "/opt/gophenberg/versions/" + version.Version()

// cutOffIn leaves the server as a first run for example.com left it, cut off in the step id, owning installed.
func (s *installScenario) cutOffIn(id string, installed ...string) error {
	s.server.write(serverEnvPath, firstServerEnv)
	record := Record{Installer: version.Version(), Domain: "example.com", Steps: map[string]string{}, Installed: installed}
	for _, done := range stepIDs[:slices.Index(stepIDs, id)] {
		record.Steps[done] = stepEnded
		record.Installed = append(record.Installed, s.leftBy(done)...)
	}
	record.Steps[id] = stepStarted
	body, err := json.Marshal(record)
	s.server.write(recordPath, string(body))
	return err
}

// leftBy puts on the server what the step called id leaves once it ended, returning the pieces the record then owns.
func (s *installScenario) leftBy(id string) []string {
	switch id {
	case "packages":
		s.server.installPackage(piecePostgreSQL)
		s.server.installPackage(pieceCaddy)
		return []string{piecePostgreSQL, pieceCaddy}
	case "user":
		s.server.addUser("gophenberg", "/opt/gophenberg")
	case "install-root":
		s.server.folder("/opt/gophenberg/versions", "/opt/gophenberg/downloads", "/opt/gophenberg/state",
			"/opt/gophenberg/backups", "/opt/gophenberg/media", "/opt/gophenberg/themes")
		return []string{"/opt/gophenberg"}
	}
	return nil
}

// cutOffInPackages leaves the server as a first run left it when it was cut off while dpkg set up PostgreSQL.
func (s *installScenario) cutOffInPackages() error {
	s.server.folder("/etc/postgresql/16/main")
	s.server.dpkg[piecePostgreSQL] = "iF"
	return s.cutOffIn("packages", piecePostgreSQL, pieceCaddy)
}

// cutOffAfterCreateuser leaves the server as a first run left it when it was cut off right after createuser.
func (s *installScenario) cutOffAfterCreateuser() error {
	s.server.answers["createuser"] = answer{
		printed: "createuser: error: creation of new role failed: ERROR:  role \"gophenberg\" already exists\n",
		err:     errors.New("installer: run createuser: exit status 1"),
	}
	return s.cutOffIn("database")
}

// cutOffInBuild leaves the server as a first run left it when it was cut off while it unpacked this installer's build.
func (s *installScenario) cutOffInBuild() error {
	s.server.write(ownBuild+"/gophenberg", "#!/bin/sh\n# cut off while it was written\n")
	s.server.folder(ownBuild + "/LICENSES")
	return s.cutOffIn("build")
}

// holdsPrintedFiles fails unless the server holds every file the dry run printed, with its text, owner and mode.
func (s *installScenario) holdsPrintedFiles() error {
	if s.planned == "" {
		return errors.New("the scenario ran no dry run")
	}
	for path, printed := range (&installScenario{stdout: s.planned}).printedFiles() {
		owned, _ := strings.CutPrefix(printed.header, "  writes "+path+", owned by ")
		owner, mode, _ := strings.Cut(strings.TrimSuffix(owned, ":"), " with mode ")
		err := s.holdsPrinted(path, printed.text)
		if err == nil {
			err = s.ownedAs(path, owner, mode, 0)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// holdsPrinted fails unless the file at path holds printed line for line, each hidden value as the scenario set it.
func (s *installScenario) holdsPrinted(path, printed string) error {
	held, err := os.ReadFile(s.server.at(path))
	if err != nil {
		return fmt.Errorf("the server holds no file %s: %w", path, err)
	}
	if !slices.EqualFunc(strings.Split(string(held), "\n"), strings.Split(printed, "\n"), s.heldAsPrinted) {
		return fmt.Errorf("the server holds %s as\n%s\nwant it as the dry run printed it\n%s", path, held, printed)
	}
	return nil
}

// heldAsPrinted reports whether held is the line printed, or sets the value the scenario set for the key it hides.
func (s *installScenario) heldAsPrinted(held, printed string) bool {
	key, hidden := strings.CutSuffix(printed, hiddenValue)
	if !hidden {
		return held == printed
	}
	set, _ := godotenv.Unmarshal(held)
	planted, found := s.planted[key]
	return found && maps.Equal(set, map[string]string{key: planted})
}

// holdsFile fails unless the server holds a regular file at path that holds text, owned by owner with mode.
func (s *installScenario) holdsFile(path, owner, mode, text string) error {
	held, err := os.ReadFile(s.server.at(path))
	if err != nil {
		return fmt.Errorf("the server holds no file %s: %w", path, err)
	}
	if string(held) != text {
		return fmt.Errorf("the server holds %s as\n%s\nwant\n%s", path, held, text)
	}
	return s.ownedAs(path, owner, mode, 0)
}

// holdsFolders fails unless the server holds each folder the table names, not a link, owned by its owner with its mode.
func (s *installScenario) holdsFolders(folders *godog.Table) error {
	for _, row := range folders.Rows[1:] {
		if err := s.ownedAs(row.Cells[0].Value, row.Cells[1].Value, row.Cells[2].Value, fs.ModeDir); err != nil {
			return err
		}
	}
	return nil
}

// ownedAs fails unless the server holds an entry of the type kind at path, owned by owner with mode.
func (s *installScenario) ownedAs(path, owner, mode string, kind fs.FileMode) error {
	info, err := os.Lstat(s.server.at(path))
	if err != nil {
		return fmt.Errorf("the server holds nothing at %s: %w", path, err)
	}
	if info.Mode().Type() != kind {
		return fmt.Errorf("the server holds %s as %s, want the type %s", path, info.Mode(), kind)
	}
	if got := fmt.Sprintf("%04o", info.Mode().Perm()); got != mode {
		return fmt.Errorf("the server holds %s with mode %s, want %s", path, got, mode)
	}
	return s.ownedBy(path, owner)
}

// ownedBy fails unless the entry at path, not what a link leads to, belongs to the ids the server gives owner.
func (s *installScenario) ownedBy(path, owner string) error {
	info, err := os.Lstat(s.server.at(path))
	if err != nil {
		return err
	}
	uid, gid, err := s.idsOf(owner)
	if err != nil {
		return err
	}
	stat := info.Sys().(*syscall.Stat_t)
	if int(stat.Uid) != uid || int(stat.Gid) != gid {
		return fmt.Errorf("%s belongs to %d:%d, want %s's %d:%d", path, stat.Uid, stat.Gid, owner, uid, gid)
	}
	return nil
}

// idsOf returns the ids of owner on the server, root's own the test's, since a test may give a file to no other ids.
func (s *installScenario) idsOf(owner string) (uid, gid int, err error) {
	if owner == "root" {
		return os.Getuid(), os.Getgid(), nil
	}
	found, held := s.server.users[owner]
	if !held {
		return 0, 0, fmt.Errorf("the server has no user %s", owner)
	}
	uid, uidErr := strconv.Atoi(found.Uid)
	gid, gidErr := strconv.Atoi(found.Gid)
	return uid, gid, errors.Join(uidErr, gidErr)
}

// holdsRecord fails unless the server holds an install record it can read, owned by owner with mode.
func (s *installScenario) holdsRecord(owner, mode string) error {
	if _, err := readRecord(s.server.machine()); err != nil {
		return err
	}
	return s.ownedAs(recordPath, owner, mode, 0)
}

// holdsInstallerCopy fails unless the installer's own copy holds the installer that ran, owned by owner with mode.
func (s *installScenario) holdsInstallerCopy(owner, mode string) error {
	running, err := os.ReadFile(s.server.at(fakeInstaller))
	if err != nil {
		return err
	}
	return s.holdsFile(installerCopy, owner, mode, string(running))
}

// holdsOwnBuild fails unless this installer's build folder holds the files of its archive alone, owned by gophenberg.
func (s *installScenario) holdsOwnBuild() error {
	want, err := archived(s.server.at(s.archive))
	if err != nil {
		return err
	}
	got := map[string]string{}
	err = filepath.WalkDir(s.server.at(ownBuild), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := s.ownedBy(s.server.serverPath(path), "gophenberg"); err != nil || entry.IsDir() {
			return err
		}
		held, err := os.ReadFile(path)
		got[strings.TrimPrefix(path, s.server.at(ownBuild)+"/")] = string(held)
		return err
	})
	if err != nil {
		return err
	}
	if !maps.Equal(got, want) {
		return fmt.Errorf("the build folder %s holds %v, want the files of its archive %v", ownBuild, got, want)
	}
	return nil
}

// archived returns each file of the release archive at path by its name, with what it holds.
func archived(path string) (map[string]string, error) {
	archive, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = archive.Close() }()
	unzipped, err := gzip.NewReader(archive)
	if err != nil {
		return nil, err
	}
	entries, files := tar.NewReader(unzipped), map[string]string{}
	for {
		header, err := entries.Next()
		if errors.Is(err, io.EOF) {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(entries)
		if err != nil {
			return nil, err
		}
		files[header.Name] = string(body)
	}
}

// pointsAtOwnBuild fails unless the current link names this installer's build and belongs to gophenberg.
func (s *installScenario) pointsAtOwnBuild() error {
	const current = "/opt/gophenberg/current"
	target, err := os.Readlink(s.server.at(current))
	if err != nil {
		return err
	}
	if want := "versions/" + version.Version(); target != want {
		return fmt.Errorf("the current link names %s, want %s", target, want)
	}
	return s.ownedBy(current, "gophenberg")
}

// ranInOrder fails unless the run ran the program line first, then second when order is after, or the other way.
func (s *installScenario) ranInOrder(first, order, second string) error {
	at, other := slices.Index(s.server.ran, first), slices.Index(s.server.ran, second)
	ran := strings.Join(s.server.ran, "\n")
	if at < 0 || other < 0 {
		return fmt.Errorf("the run ran\n%s\nwant it to run both %q and %q", ran, first, second)
	}
	if (at < other) != (order == "before") {
		return fmt.Errorf("the run ran\n%s\nwant %q %s %q", ran, first, order, second)
	}
	return nil
}

// recordMarks fails unless the install record on the server marks the step called id as mark.
func (s *installScenario) recordMarks(id, mark string) error {
	record, err := readRecord(s.server.machine())
	if err != nil {
		return err
	}
	if got := record.Steps[id]; got != mark {
		return fmt.Errorf("the install record marks the step %s %q, want %s", id, got, mark)
	}
	return nil
}

// runFeature runs the feature at path on fake servers through the one initializer, @wip left out unless set.
func runFeature(t *testing.T, path string) {
	t.Helper()
	tags := "~@wip"
	if os.Getenv("GOPHENBERG_BDD_WIP") != "" {
		tags = ""
	}
	suite := godog.TestSuite{
		ScenarioInitializer: initializeServerInstall(t),
		Options: &godog.Options{
			Format: "pretty", Paths: []string{path}, Tags: tags, Strict: true, TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Error("the feature did not pass")
	}
}

func TestServerInstall(t *testing.T) {
	t.Parallel()

	runFeature(t, serverInstallFeature)
}
