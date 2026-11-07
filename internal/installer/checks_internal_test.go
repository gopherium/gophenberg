// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"cmp"
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole"
)

// ubuntuRelease is the os-release of Ubuntu 24.04.
const ubuntuRelease = `PRETTY_NAME="Ubuntu 24.04.3 LTS"
NAME="Ubuntu"
VERSION_ID="24.04"
VERSION="24.04.3 LTS (Noble Numbat)"
VERSION_CODENAME=noble
ID=ubuntu
ID_LIKE=debian
`

// sandboxLine is the systemd-run line the sandbox check runs, as the fake server records it.
const sandboxLine = "systemd-run --wait --collect -p ProtectSystem=strict -p PrivateTmp=yes -p NoNewPrivileges=yes " +
	"/bin/true"

// clockLine is the timedatectl line the clock check runs, as the fake server records it.
const clockLine = "timedatectl show -p NTPSynchronized --value"

// checkedSettings are the settings every machine check passes with on a fake server, for localhost.
var checkedSettings = Settings{
	Root: "/opt/gophenberg", Domain: "localhost", Addr: netip.MustParseAddrPort("127.0.0.1:8081"), WebServer: "caddy",
	Checks: Checks{MemoryFloorMB: 900, DiskFloorMB: 1024, DomainCheckTimeout: 10 * time.Second},
}

// meminfoOf returns a /proc/meminfo whose MemTotal is kB.
func meminfoOf(kB int) string {
	return fmt.Sprintf("MemTotal:        %d kB\nMemFree:          812345 kB\nMemAvailable:    1523456 kB\n", kB)
}

// answer is what a fake program prints and how it ends.
type answer struct {
	printed string
	err     error
}

// ownedInfo describes a file as owned by uid, whatever user the test runs as.
type ownedInfo struct {
	fs.FileInfo
	uid uint32
}

// Sys returns the system description of the file with uid as its owner.
func (i ownedInfo) Sys() any {
	stat := *i.FileInfo.Sys().(*syscall.Stat_t)
	stat.Uid = i.uid
	return &stat
}

// fakeServer is a server every machine check passes on, which records what the checks ask of it.
type fakeServer struct {
	t            *testing.T
	root         string
	euid         int
	answers      map[string]answer
	taken        map[string]error
	freeMB       map[string]uint64
	owners       map[string]uint32
	broken       map[string]error
	described    map[string]fs.FileInfo
	unmeasured   map[string]error
	addresses    []netip.Addr
	unresolved   error
	own          []net.Addr
	unlisted     error
	answerFetch  func(request *http.Request) (*http.Response, error)
	answerHTTPS  func(request *http.Request) (*http.Response, error)
	prompt       *fakePrompt
	users        map[string]*user.User
	dpkg         map[string]string
	effects      map[string]func(Program) answer
	running      map[string]bool
	siteDown     bool
	unloads      bool
	accounts     []listedAccount
	ranFrom      string
	notes        strings.Builder
	runs         []Program
	ran          []string
	listened     []string
	listeners    []*pipeListener
	measured     []string
	resolved     []string
	fetched      []string
	fetchedHTTPS []string
	httpsRoots   []*x509.CertPool
	terminals    int
}

// newFakeServer returns a fake server every machine check passes on.
func newFakeServer(t *testing.T) *fakeServer {
	t.Helper()
	server := &fakeServer{
		t: t, root: t.TempDir(),
		answers: map[string]answer{
			"systemctl":        {printed: "systemd 255 (255.4-1ubuntu8.6)\n+PAM +AUDIT +SELINUX +APPARMOR\n"},
			"systemd-run":      {printed: "Running as unit: run-u7.service\nFinished with result: success\n"},
			"timedatectl":      {printed: "yes\n"},
			"createuser":       {},
			"createdb":         {},
			"gophenberg check": {printed: "every setting checks out\n"},
		},
		taken: map[string]error{}, freeMB: map[string]uint64{}, owners: map[string]uint32{}, broken: map[string]error{},
		described: map[string]fs.FileInfo{}, unmeasured: map[string]error{}, users: map[string]*user.User{},
		dpkg: map[string]string{}, running: map[string]bool{}, addresses: addresses("::ffff:203.0.113.7"),
		own: []net.Addr{cardAddress("127.0.0.1/8"), cardAddress("203.0.113.7/24"), cardAddress("::1/128")},
	}
	server.effects = map[string]func(Program) answer{
		"apt-get": server.aptGet, "dpkg-query": server.dpkgQuery, "dpkg": server.configurePackages,
		"useradd": server.useradd, "systemctl": server.systemctl, "caddy": server.caddyValidate,
		"gophenberg account:create-admin": server.createAdmin, "gophenberg account:list": server.listAccounts,
	}
	server.write("/etc/os-release", ubuntuRelease)
	server.write("/proc/meminfo", meminfoOf(2048000))
	server.folder("/run/systemd/system", "/opt", "/var/lib")
	server.program("/usr/bin/systemctl")
	server.program("/usr/sbin/runuser")
	server.program(fakeInstaller)
	return server
}

// fakeInstaller is where the installer a fake server runs sits, named from the top of the server.
const fakeInstaller = "/root/installer/gophenberg"

// packageCaddyfile is the Caddyfile the caddy package puts on a server, which serves a page of its own on port 80.
const packageCaddyfile = ":80 {\n\troot * /usr/share/caddy\n\tfile_server\n}\n"

// installPackage puts the package called name on the server as apt-get does, dpkg then showing it installed.
func (s *fakeServer) installPackage(name string) {
	s.t.Helper()
	switch name {
	case "postgresql":
		s.folder("/etc/postgresql/16/main", "/usr/lib/postgresql/16/bin", "/var/lib/postgresql/16/main")
	case "caddy":
		s.write(caddyfilePath, packageCaddyfile)
		s.program("/usr/bin/caddy")
		s.addUser("caddy", "/var/lib/caddy")
	}
	s.dpkg[name] = "ii"
}

// addUser gives the server the OS user called name with home, in the test's uid and siteGroup, so root's ids differ.
func (s *fakeServer) addUser(name, home string) {
	s.users[name] = &user.User{
		Username: name, Uid: strconv.Itoa(os.Getuid()), Gid: strconv.Itoa(siteGroup(s.t)), HomeDir: home,
	}
}

// siteGroup returns a group of the test's user other than its own where it has one, else its own.
func siteGroup(t *testing.T) int {
	t.Helper()
	groups, err := os.Getgroups()
	if err != nil {
		t.Fatalf("listing the groups of the test's user: %v", err)
	}
	for _, group := range groups {
		if group != os.Getgid() {
			return group
		}
	}
	return os.Getgid()
}

// program puts a program anybody may run at path, named from the top of the server.
func (s *fakeServer) program(path string) {
	s.t.Helper()
	s.write(path, "#!/bin/sh\n")
	s.chmod(path, 0o755)
}

// at returns the path on the test's own disk of path, named from the top of the server.
func (s *fakeServer) at(path string) string {
	return filepath.Join(s.root, path)
}

// write puts a file holding body at path, named from the top of the server.
func (s *fakeServer) write(path, body string) {
	s.t.Helper()
	if err := os.MkdirAll(filepath.Dir(s.at(path)), 0o755); err != nil {
		s.t.Fatalf("making the folder of %s: %v", path, err)
	}
	if err := os.WriteFile(s.at(path), []byte(body), 0o644); err != nil {
		s.t.Fatalf("writing %s: %v", path, err)
	}
}

// folder makes a folder at each of paths, named from the top of the server.
func (s *fakeServer) folder(paths ...string) {
	s.t.Helper()
	for _, path := range paths {
		if err := os.MkdirAll(s.at(path), 0o755); err != nil {
			s.t.Fatalf("making the folder %s: %v", path, err)
		}
	}
}

// link puts a link at path that points at target, both named from the top of the server.
func (s *fakeServer) link(path, target string) {
	s.t.Helper()
	if err := os.MkdirAll(filepath.Dir(s.at(path)), 0o755); err != nil {
		s.t.Fatalf("making the folder of %s: %v", path, err)
	}
	if err := os.Symlink(s.at(target), s.at(path)); err != nil {
		s.t.Fatalf("linking %s to %s: %v", path, target, err)
	}
}

// earlierBuild is the build an earlier run left the current link naming.
const earlierBuild = "0.19.0"

// pointCurrent puts the earlier build with its program in the install root and points current at it, as Point does.
func (s *fakeServer) pointCurrent() {
	s.t.Helper()
	s.program("/opt/gophenberg/versions/" + earlierBuild + "/gophenberg")
	if err := os.Symlink("versions/"+earlierBuild, s.at("/opt/gophenberg/current")); err != nil {
		s.t.Fatalf("pointing current at %s: %v", earlierBuild, err)
	}
}

// remove takes the file or folder at path away, named from the top of the server.
func (s *fakeServer) remove(path string) {
	s.t.Helper()
	if err := os.RemoveAll(s.at(path)); err != nil {
		s.t.Fatalf("removing %s: %v", path, err)
	}
}

// chmod gives the file at path, named from the top of the server, mode.
func (s *fakeServer) chmod(path string, mode fs.FileMode) {
	s.t.Helper()
	if err := os.Chmod(s.at(path), mode); err != nil {
		s.t.Fatalf("changing the mode of %s: %v", path, err)
	}
}

// serverPath returns the path from the top of the server of path on the test's own disk.
func (s *fakeServer) serverPath(path string) string {
	return "/" + strings.TrimPrefix(strings.TrimPrefix(path, s.root), "/")
}

// machine returns the machine the checks reach this fake server through.
func (s *fakeServer) machine() Machine {
	return Machine{
		Run: s.run, Root: s.root, Lstat: s.lstat, Free: s.free, Listen: s.listen, ListenPacket: s.listenPacket,
		Fetch: s.fetch, Resolve: s.resolve, Addresses: s.ownAddresses, Terminal: s.terminal,
		Executable: s.executable, LookupUser: s.lookupUser, FetchHTTPS: s.fetchHTTPS,
		Euid: func() int { return s.euid },
	}
}

// terminal returns the fake terminal a test gave the server, or the error of a run with no terminal at all.
func (s *fakeServer) terminal() (Prompt, error) {
	s.terminals++
	if s.prompt == nil {
		return nil, &fs.PathError{Op: "open", Path: "/dev/tty", Err: syscall.ENXIO}
	}
	return s.prompt, nil
}

// executable returns the installer the fake server runs, on the test's own disk, the one a test ran it from if any.
func (s *fakeServer) executable() (string, error) {
	return s.at(cmp.Or(s.ranFrom, fakeInstaller)), nil
}

// lookupUser returns the OS user called name that a test or a fake program gave the server.
func (s *fakeServer) lookupUser(name string) (*user.User, error) {
	if found, held := s.users[name]; held {
		return found, nil
	}
	return nil, user.UnknownUserError(name)
}

// fetchHTTPS records the page asked for and the roots that came with it, then answers as the test says, else as Caddy.
func (s *fakeServer) fetchHTTPS(request *http.Request, roots *x509.CertPool) (*http.Response, error) {
	s.fetchedHTTPS = append(s.fetchedHTTPS, fmt.Sprintf("%s with roots %t", request.URL, roots != nil))
	s.httpsRoots = append(s.httpsRoots, roots)
	if s.answerHTTPS == nil {
		return s.caddyPage(request)
	}
	return s.answerHTTPS(request)
}

// run records program, then answers it through its effect on the server or as the answers say, else fails the test.
func (s *fakeServer) run(_ context.Context, program Program) (string, error) {
	s.runs = append(s.runs, program)
	s.ran = append(s.ran, lineOf(program))
	key := programKey(program)
	if effect, found := s.effects[key]; found {
		reply := effect(program)
		return reply.printed, reply.err
	}
	reply, found := s.answers[key]
	if !found {
		s.t.Errorf("the run ran %s, which the fake server does not answer", key)
	}
	return reply.printed, reply.err
}

// programKey names program as the fake server answers it, a command of the site by gophenberg and its first argument.
func programKey(program Program) string {
	if filepath.Base(program.Name) != siteUser || len(program.Args) == 0 {
		return program.Name
	}
	return siteUser + " " + program.Args[0]
}

// lineOf returns the command line of program as the runner starts it, with runuser in front for another OS user.
func lineOf(program Program) string {
	line := strings.Join(append([]string{program.Name}, program.Args...), " ")
	if program.User == "" {
		return line
	}
	return "runuser -u " + program.User + " -- " + line
}

// aptGet installs each package an apt-get install line names, as apt-get does, and changes nothing on an update.
func (s *fakeServer) aptGet(program Program) answer {
	if !slices.Contains(program.Args, "install") {
		return answer{printed: "Reading package lists... Done\n"}
	}
	for _, name := range []string{piecePostgreSQL, pieceCaddy} {
		if slices.Contains(program.Args, name) {
			s.installPackage(name)
		}
	}
	return answer{printed: "Setting up postgresql (16+257build1.1) ...\n"}
}

// dpkgQuery prints the status of each package its line names, failing as dpkg-query does on one it cannot find.
func (s *fakeServer) dpkgQuery(program Program) answer {
	var reply answer
	for _, name := range program.Args[3:] {
		status, found := s.dpkg[name]
		if !found {
			reply.printed += "dpkg-query: no packages found matching " + name + "\n"
			reply.err = errors.New("installer: run dpkg-query: exit status 1")
			continue
		}
		reply.printed += fmt.Sprintf("%-3s\n", status)
	}
	return reply
}

// configurePackages sets up every package dpkg left half set up, as dpkg --configure -a does.
func (s *fakeServer) configurePackages(Program) answer {
	for name := range s.dpkg {
		s.dpkg[name] = "ii"
	}
	return answer{}
}

// useradd adds the OS user its line names, at the home folder its line names, as useradd does.
func (s *fakeServer) useradd(program Program) answer {
	s.addUser(program.Args[len(program.Args)-1], program.Args[slices.Index(program.Args, "--home-dir")+1])
	return answer{}
}

// lstat describes the file at path on the test's own disk as owned by root, or by the owner a test gave it.
func (s *fakeServer) lstat(path string) (fs.FileInfo, error) {
	server := s.serverPath(path)
	if err := s.broken[server]; err != nil {
		return nil, err
	}
	if info, found := s.described[server]; found {
		return info, nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	return ownedInfo{FileInfo: info, uid: s.owners[server]}, nil
}

// free returns 10 GB for a folder on the test's own disk that exists, or the megabytes a test gave it.
func (s *fakeServer) free(path string) (uint64, error) {
	server := s.serverPath(path)
	s.measured = append(s.measured, server)
	if err := s.unmeasured[server]; err != nil {
		return 0, err
	}
	if _, err := os.Stat(path); err != nil {
		return 0, err
	}
	if megabytes, found := s.freeMB[server]; found {
		return megabytes << 20, nil
	}
	return 10240 << 20, nil
}

// listen listens in memory on a TCP port, failing with the error a test gave the port.
func (s *fakeServer) listen(network, address string) (net.Listener, error) {
	s.listened = append(s.listened, network+" "+address)
	if err := s.taken[network+" "+address]; err != nil {
		return nil, err
	}
	listener := newPipeListener()
	s.listeners = append(s.listeners, listener)
	return listener, nil
}

// listenPacket holds a UDP port for nothing, failing with the error a test gave the port.
func (s *fakeServer) listenPacket(network, address string) (net.PacketConn, error) {
	s.listened = append(s.listened, network+" "+address)
	if err := s.taken[network+" "+address]; err != nil {
		return nil, err
	}
	return packetPort{}, nil
}

// resolve records the network and the domain asked for and returns the addresses a test gave every domain.
func (s *fakeServer) resolve(_ context.Context, network, host string) ([]netip.Addr, error) {
	s.resolved = append(s.resolved, network+" "+host)
	return s.addresses, s.unresolved
}

// ownAddresses returns the addresses a test gave the network cards of the server.
func (s *fakeServer) ownAddresses() ([]net.Addr, error) {
	return s.own, s.unlisted
}

// fetch records the page asked for, then answers as the test says, a token through the newest listener, else a page.
func (s *fakeServer) fetch(request *http.Request) (*http.Response, error) {
	s.fetched = append(s.fetched, request.Host+" "+request.URL.String())
	if s.answerFetch != nil {
		return s.answerFetch(request)
	}
	if strings.HasPrefix(request.URL.Path, tokenFolder) {
		return s.listeners[len(s.listeners)-1].fetch(request)
	}
	return s.sitePage(request)
}

// check returns the machine check of this fake server with checkedSettings, no install record and no environment.
func (s *fakeServer) check() *machineCheck {
	return &machineCheck{machine: s.machine(), settings: checkedSettings, env: envOf(nil), notes: &s.notes}
}

// ownWebServerCheck returns the machine check of this fake server for example.com, its web server the person's own.
func (s *fakeServer) ownWebServerCheck() *machineCheck {
	check := s.check()
	check.settings.Domain, check.settings.WebServer = "example.com", "none"
	return check
}

// wantPassed fails t unless the checks passed with notes printed.
func (s *fakeServer) wantPassed(err error, notes string) {
	s.t.Helper()
	if err != nil || s.notes.String() != notes {
		s.t.Errorf("the checks = %v, noting %q, want them passed noting %q", err, s.notes.String(), notes)
	}
}

func TestTheMachineChecksPassAFreshUbuntuServerAndNoteNothing(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	check := server.check()

	err := check.run(t.Context())

	server.wantPassed(err, "")
	if check.found.systemd != 255 {
		t.Errorf("the checks found systemd %d, want 255", check.found.systemd)
	}
	if want := []string{"systemctl --version", sandboxLine, clockLine}; !slices.Equal(server.ran, want) {
		t.Errorf("the checks ran %q, want %q", server.ran, want)
	}
	wantListened := []string{"tcp :80", "tcp :443", "udp :443", "tcp 127.0.0.1:8081", "tcp :5432"}
	if !slices.Equal(server.listened, wantListened) {
		t.Errorf("the checks listened on %q, want %q", server.listened, wantListened)
	}
	if want := []string{"/opt/gophenberg", "/opt", "/var/lib"}; !slices.Equal(server.measured, want) {
		t.Errorf("the checks measured the disk under %q, want %q", server.measured, want)
	}
}

func TestInstallStopsARunThatIsNotRootsBeforeItReadsTheRecordOrRunsAnything(t *testing.T) {
	t.Parallel()

	for _, dryRun := range []bool{false, true} {
		server := newFakeServer(t)
		server.euid = 1000
		server.write(recordPath, "{")

		_, _, err := runInstall(server, Request{Domain: "example.com", DryRun: dryRun})

		wantRefused(t, err, "server:install must run as root, such as with sudo, so nothing was changed")
		if len(server.ran) != 0 {
			t.Errorf("the run ran %q as a plain user, want nothing run", server.ran)
		}
	}
}

func TestNotRootNamesTheCommandThatMustRunAsRoot(t *testing.T) {
	t.Parallel()

	wantRefused(t, notRoot("server:uninstall"), "server:uninstall must run as root, such as with sudo, so nothing "+
		"was changed")
}

// otherCommand is the stop of a run that finds a gophenberg command on the path server:install did not write.
const otherCommand = "this server already has a /usr/local/bin/gophenberg that server:install did not write, and " +
	"server:install puts the gophenberg command there, so nothing was changed"

func TestTheMachineChecksPassAGophenbergCommandServerInstallWrote(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.write(commandPath, wantedCommand)

	err := server.check().run(t.Context())

	server.wantPassed(err, "")
}

func TestTheMachineChecksStopAtAGophenbergCommandServerInstallDidNotWrite(t *testing.T) {
	t.Parallel()

	for what, put := range map[string]func(*fakeServer){
		"another script": func(s *fakeServer) { s.write(commandPath, "#!/bin/sh\nexec /opt/other/gophenberg \"$@\"\n") },
		"a shorter file": func(s *fakeServer) { s.write(commandPath, "#!/bin/sh\n") },
		"a folder":       func(s *fakeServer) { s.folder(commandPath) },
		"a link to the command server:install writes": func(s *fakeServer) {
			s.write("/usr/local/share/gophenberg/command", wantedCommand)
			s.link(commandPath, "/usr/local/share/gophenberg/command")
		},
	} {
		server := newFakeServer(t)
		put(server)

		err := server.check().run(t.Context())

		if err == nil || err.Error() != otherCommand {
			t.Errorf("the checks of %s on the path = %v, want %q", what, err, otherCommand)
		}
	}
}

func TestTheMachineChecksRefuseAGophenbergCommandTheyCannotReadOrDescribe(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root reads a file of mode 0000")
	}
	locked := newFakeServer(t)
	locked.write(commandPath, wantedCommand)
	locked.chmod(commandPath, 0)
	broken := newFakeServer(t)
	broken.broken[commandPath] = syscall.EIO

	for server, want := range map[*fakeServer]string{
		locked: "installer: open " + locked.at(commandPath) + ": permission denied",
		broken: "installer: input/output error",
	} {
		err := server.check().run(t.Context())

		wantRefused(t, err, want)
	}
}

// laterCheck returns the machine check of server for a later run, whose record owns Caddy and the Caddyfile it wrote.
func laterCheck(server *fakeServer) *machineCheck {
	check := server.check()
	check.record.Installed = []string{pieceCaddy}
	check.record.Caddyfile = caddyfileSum(wantedCaddyfile)
	return check
}

func TestTheMachineChecksStopAtALostCaddyfileLineOnceTheRecordHoldsItsSumWhateverTheCaddyMark(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.write(caddyfilePath, strings.Replace(wantedCaddyfile, "import "+caddySitePath+"\n", "", 1))
	check := laterCheck(server)
	check.record.Steps = map[string]string{stepCaddy: stepStarted}

	err := check.run(t.Context())

	wantRefused(t, err, `/etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy" that an earlier `+
		"run wrote, which has Caddy serve the site, and it must go back in before server:install runs again, so "+
		"nothing was changed")
}

func TestTheMachineChecksPassALaterRunWhoseCaddyfileHoldsTheLinesTheFirstRunWrote(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.write(caddyfilePath, wantedCaddyfile+"\nexample.org {\n\trespond \"Maria Perez's own site\"\n}\n")

	err := laterCheck(server).run(t.Context())

	server.wantPassed(err, "")
}

func TestTheMachineChecksStopALaterRunWhoseCaddyfileLostALineTheFirstRunWrote(t *testing.T) {
	t.Parallel()

	for _, lost := range caddyfileLines {
		server := newFakeServer(t)
		server.write(caddyfilePath, strings.Replace(wantedCaddyfile, lost.line+"\n", "", 1))

		err := laterCheck(server).run(t.Context())

		wantRefused(t, err, `/etc/caddy/Caddyfile lacks the line "`+lost.line+`" that an earlier run wrote, `+lost.why+
			", and it must go back in before server:install runs again, so nothing was changed")
	}
}

func TestTheMachineChecksStopALaterRunWhoseCaddyfileIsGone(t *testing.T) {
	t.Parallel()

	err := laterCheck(newFakeServer(t)).run(t.Context())

	wantRefused(t, err, `/etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy" that an earlier `+
		"run wrote, which has Caddy serve the site, and it must go back in before server:install runs again, so "+
		"nothing was changed")
}

func TestTheMachineChecksRefuseACaddyfileTheyCannotRead(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.folder(caddyfilePath)

	err := laterCheck(server).run(t.Context())

	wantRefused(t, err, "installer: read "+server.at(caddyfilePath)+": is a directory")
}

func TestTheMachineChecksLeaveTheCaddyfileAloneOnAFirstRunAndWithoutCaddy(t *testing.T) {
	t.Parallel()

	own := newFakeServer(t).ownWebServerCheck()
	own.record.Caddyfile = caddyfileSum(wantedCaddyfile)
	unwritten := newFakeServer(t)
	unwritten.write(caddyfilePath, packageCaddyfile)
	cutOff := unwritten.check()
	cutOff.record.Steps, cutOff.record.Installed = map[string]string{stepCaddy: stepStarted}, []string{pieceCaddy}

	for _, check := range []*machineCheck{newFakeServer(t).check(), own, cutOff} {
		err := check.run(t.Context())

		if err != nil {
			t.Errorf("the checks without a Caddyfile to check = %v, want them passed", err)
		}
	}
}

func TestTheLinesALaterRunNeedsAreLinesOfTheCaddyfileTheFirstRunWrites(t *testing.T) {
	t.Parallel()

	for _, domain := range []string{"example.com", "localhost"} {
		files := firstSite(t, domain, 255).caddyFiles()
		caddyfile := files[len(files)-1]

		for _, needed := range caddyfileLines {
			if caddyfile.path != caddyfilePath || !slices.Contains(trimmedLines(caddyfile.text), needed.line) {
				t.Errorf("the first run writes %s for %s as\n%s\nwant it to hold the line %q", caddyfile.path, domain,
					caddyfile.text, needed.line)
			}
		}
	}
}

func TestTheMachineChecksStopWhereSystemdCannotSetUpTheSandboxOfTheSite(t *testing.T) {
	t.Parallel()

	for printed, reported := range map[string]string{"lxc\n": "lxc", "": "nothing"} {
		server := newFakeServer(t)
		server.answers["systemd-run"] = answer{
			printed: "Job for run-u7.service failed.\n", err: errors.New("installer: run systemd-run: exit status 1"),
		}
		server.answers["systemd-detect-virt"] = answer{printed: printed}

		err := server.check().run(t.Context())

		wantRefused(t, err, "systemd cannot set up the sandbox the site's service runs in on this server, as on a "+
			"container plan without nesting, and systemd-detect-virt reports "+reported+", so nothing was changed")
	}
}

func TestTheMachineChecksTakeMemoryRightAtTheFloor(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.write("/proc/meminfo", meminfoOf(900*1024))

	err := server.check().run(t.Context())

	server.wantPassed(err, "")
}

func TestTheMachineChecksStopOnLessMemoryThanTheFloor(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.write("/proc/meminfo", meminfoOf(900*1024-1))

	err := server.check().run(t.Context())

	wantRefused(t, err, "this server has 899 MB of memory, less than the 900 MB GOPHENBERG_INSTALL_MEMORY_FLOOR_MB "+
		"asks, so nothing was changed")
}

func TestTheMachineChecksStopWhenTheyCannotTellTheMemory(t *testing.T) {
	t.Parallel()

	missing := newFakeServer(t)
	missing.remove("/proc/meminfo")
	unnamed := newFakeServer(t)
	unnamed.write("/proc/meminfo", "MemFree:          812345 kB\n")

	for _, server := range []*fakeServer{missing, unnamed} {
		err := server.check().run(t.Context())

		wantRefused(t, err, "cannot tell how much memory this server has: /proc/meminfo has no MemTotal line in kB, "+
			"so nothing was changed")
	}
}

func TestTheMachineChecksRefuseAMemoryReportTheyCannotRead(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.remove("/proc/meminfo")
	server.folder("/proc/meminfo")

	err := server.check().run(t.Context())

	wantRefused(t, err, "installer: read "+server.at("/proc/meminfo")+": is a directory")
}

func TestTheMachineChecksTakeFreeDiskRightAtTheFloor(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.freeMB["/opt"], server.freeMB["/var/lib"] = 1024, 1024

	err := server.check().run(t.Context())

	server.wantPassed(err, "")
}

func TestTheMachineChecksStopOnLessFreeDiskThanTheFloor(t *testing.T) {
	t.Parallel()

	for _, folder := range []string{"/opt", "/var/lib"} {
		server := newFakeServer(t)
		server.freeMB[folder] = 1023

		err := server.check().run(t.Context())

		wantRefused(t, err, "this server has 1023 MB free under "+folder+", less than the 1024 MB "+
			"GOPHENBERG_INSTALL_DISK_FLOOR_MB asks, so nothing was changed")
	}
}

func TestTheMachineChecksMeasureTheDiskAtTheNearestFolderThatExists(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.remove("/opt")
	server.remove("/var")
	check := server.check()
	check.settings.Root = "/srv/sites/gophenberg"
	server.folder("/srv")

	err := check.run(t.Context())

	server.wantPassed(err, "")
	want := []string{"/srv/sites/gophenberg", "/srv/sites", "/srv", "/var/lib", "/var", "/"}
	if !slices.Equal(server.measured, want) {
		t.Errorf("the checks measured the disk under %q, want %q", server.measured, want)
	}
}

func TestTheMachineChecksRefuseADiskTheyCannotMeasure(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.unmeasured["/opt"] = &fs.PathError{Op: "statfs", Path: server.at("/opt"), Err: syscall.EIO}

	err := server.check().run(t.Context())

	wantRefused(t, err, "installer: statfs "+server.at("/opt")+": input/output error")
}

func TestTheMachineChecksWarnWhenTheClockIsNotSynchronized(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.answers["timedatectl"] = answer{printed: "no\n"}

	err := server.check().run(t.Context())

	server.wantPassed(err, "warning: the clock of this server is not synchronized, and HTTPS and the signed update "+
		"index both need the right time, so turn on NTP with sudo timedatectl set-ntp true\n")
}

func TestTheMachineChecksSayNothingOfAClockTheyCannotRead(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.answers["timedatectl"] = answer{err: errors.New("installer: no program timedatectl in /usr/sbin:/usr/bin")}

	err := server.check().run(t.Context())

	server.wantPassed(err, "")
}

func TestInstallRunsTheMachineChecksAndPrintsTheirNotesOnTheErrorStream(t *testing.T) {
	t.Parallel()

	server := newFakeServer(t)
	server.write("/etc/os-release", debianRelease(12, "bookworm"))
	server.answers["timedatectl"] = answer{printed: "no\n"}
	var stdout, stderr strings.Builder

	err := Install(t.Context(), server.machine(), gonsole.Call{Env: envOf(nil), Stdout: &stdout, Stderr: &stderr},
		firstRun)

	want := "Debian GNU/Linux 12 (bookworm) is not yet tested in Gophenberg's CI, which tests Ubuntu 24.04 and " +
		"Ubuntu 26.04\n" + wantedFirewallNote + "warning: the clock of this server is not synchronized, and HTTPS " +
		"and the signed update index both need the right time, so turn on NTP with sudo timedatectl set-ntp true\n"
	if err != nil || stderr.String() != want {
		t.Errorf("Install() = %v, noting %q, want nil and the notes\n%s", err, stderr.String(), want)
	}
	if !strings.HasPrefix(stdout.String(), wantedLead) {
		t.Errorf("Install() printed %q, want its answer to hold the steps alone", stdout.String())
	}
}
