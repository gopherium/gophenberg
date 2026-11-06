// SPDX-License-Identifier: Apache-2.0

package installer

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/version"
)

// mistypedSite is a site a person adds to the Caddyfile, its directive respond mistyped as respnd.
const mistypedSite = "\nexample.org {\n\trespnd \"Maria Perez's own site\"\n}\n"

// initializeServerInstallApplies binds the steps that set up and check a run that changes the server.
func initializeServerInstallApplies(sc *godog.ScenarioContext, s *installScenario) {
	sc.Given(`^an earlier run for example.com was cut off after account:create-admin made the first administrator `+
		`(\S+)$`, s.cutOffAfterCreateAdmin)
	sc.Given(`^the server settings set (GOPHENBERG_[A-Z_]+) to (\S+)$`, s.setServerLine)
	sc.Given(`^the hosting provider's firewall keeps TCP port 443 closed$`, s.closePort443)
	sc.When(`^the hosting provider's firewall lets TCP port 443 in$`, s.openPort443)
	sc.Given(`^the site never answers on 127.0.0.1:8081$`, s.takeSiteDown)
	sc.Given(`^the operator runs the installer's own copy$`, s.runOwnCopy)
	sc.Given(`^the person added a site to /etc/caddy/Caddyfile whose directive respnd caddy validate does not know$`,
		s.addMistypedSite)
	sc.When(`^the person fixes the typo in /etc/caddy/Caddyfile$`, s.fixTypo)
	sc.Given(`^Caddy fails to restart on the server$`, s.failCaddyRestart)
	sc.When(`^Caddy restarts again on the server$`, s.letCaddyRestart)
	sc.When(`^the person adds the site example.org to /etc/caddy/Caddyfile$`, s.addPersonSite)
	sc.Given(`^an account a person made by hand holds (\S+) under the role (\S+)$`, s.addHandMadeAccount)
	sc.Then(`^the run stops with "(.*)", followed by what the program printed$`, s.stopsWithPrinted)
	sc.Then(`^the run ran these programs, the checks' own left out:$`, s.ranPrograms)
	sc.Then(`^the run ran "([^"]*)"$`, s.ranLine)
	sc.Then(`^the run never ran "([^"]*)"$`, s.neverRan)
	sc.Then(`^the run ran "([^"]*)" in (/\S+)$`, s.ranIn)
	sc.Then(`^the run ran "([^"]*)" in (/\S+), the line "([^"]*)" on its standard input$`, s.ranInWithInput)
	sc.Then(`^the run ran "([^"]*)" with (\S+) in its environment$`, s.ranWithSetting)
	sc.Then(`^the run created no administrator$`, s.createdNoAdministrator)
	sc.Then(`^the run checked (https://\S+) trusting the system's root certificates$`, s.checkedWithSystemRoots)
	sc.Then(`^the run checked (https://\S+) trusting Caddy's own root certificate$`, s.checkedWithCaddyRoot)
	sc.Then(`^the run checked no page over HTTPS$`, s.checkedNoHTTPS)
	sc.Then(`^the run ends with:$`, s.endsWith)
	sc.Then(`^the run prints:$`, s.printsLines)
	sc.Then(`^the run ends with the lines for the person's web server that the dry run printed$`, s.endsWithWebServer)
	initializeServerInstallHeld(sc, s)
}

// initializeServerInstallHeld binds the steps that check what a run that changes the server leaves on it.
func initializeServerInstallHeld(sc *godog.ScenarioContext, s *installScenario) {
	sc.Then(`^the server holds /etc/caddy/Caddyfile.before-gophenberg as the caddy package wrote it$`,
		s.keptPackageCaddyfile)
	sc.Then(`^the server holds /etc/caddy/Caddyfile with the certificate email (\S+)$`, s.caddyfileEmails)
	sc.Then(`^the install record holds the SHA-256 of the Caddyfile the run wrote$`, s.recordsCaddyfileSum)
	sc.Then(`^the install record marks every step it took ended$`, s.recordEndsEveryStep)
	sc.Then(`^the server holds nothing at (/\S+)$`, s.holdsNothingAt)
	sc.Then(`^the server holds "([^"]*)", owned by (root|gophenberg) with mode (0[0-7]{3}):$`, s.holdsText)
	sc.Then(`^the site's unit on the server holds the line "([^"]*)"$`, s.writtenUnitHolds)
	sc.Then(`^the current link still names the earlier build (\S+)$`, s.stillNames)
	sc.Then(`^/etc/caddy/Caddyfile is as the earlier run left it$`, s.caddyfileLeftAlone)
	sc.Then(`^/etc/caddy/Caddyfile still holds the person's site$`, s.caddyfileKeepsSite)
	sc.Then(`^the installer's own copy is the file the run ran, left as it was$`, s.copyLeftAlone)
}

// cutOffAfterCreateAdmin leaves the server as a later step of a first run left it, cut off once email was an admin.
func (s *installScenario) cutOffAfterCreateAdmin(email string) error {
	s.installEarlier()
	record, err := readRecord(s.server.machine())
	if err != nil {
		return err
	}
	record.Steps[stepAdmin] = stepStarted
	delete(record.Steps, "https")
	delete(record.Steps, "summary")
	s.server.saveRecord(record)
	s.server.accounts = append(s.server.accounts, listedAccount{Email: email, Name: "Maria", Role: "admin"})
	return nil
}

// setServerLine sets the line key of server.env to value, as a hand edit does.
func (s *installScenario) setServerLine(key, value string) error {
	held, err := os.ReadFile(s.server.at(serverEnvPath))
	if err != nil {
		return err
	}
	var lines []string
	for line := range strings.Lines(string(held)) {
		if strings.HasPrefix(line, key+"=") {
			line = key + "=" + value + "\n"
		}
		lines = append(lines, line)
	}
	s.server.write(serverEnvPath, strings.Join(lines, ""))
	return nil
}

// closePort443 has every HTTPS fetch time out, as a firewall that drops TCP port 443 makes it.
func (s *installScenario) closePort443() {
	s.server.answerHTTPS = func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial tcp 203.0.113.7:443: i/o timeout")
	}
}

// openPort443 lets every HTTPS fetch through to Caddy again.
func (s *installScenario) openPort443() {
	s.server.answerHTTPS = nil
}

// takeSiteDown has the site refuse every connection, as a site that never starts listening does.
func (s *installScenario) takeSiteDown() {
	s.server.siteDown = true
}

// runOwnCopy has the run run the installer's own copy, noting the file it is.
func (s *installScenario) runOwnCopy() error {
	running, err := os.ReadFile(s.server.at(fakeInstaller))
	if err != nil {
		return err
	}
	s.server.write(installerCopy, string(running))
	s.server.ranFrom = installerCopy
	s.copied, err = os.Stat(s.server.at(installerCopy))
	return err
}

// addMistypedSite adds a site to the Caddyfile whose directive caddy validate does not know.
func (s *installScenario) addMistypedSite() error {
	held, err := os.ReadFile(s.server.at(caddyfilePath))
	if err != nil {
		return err
	}
	s.server.write(caddyfilePath, string(held)+mistypedSite)
	return nil
}

// fixTypo puts the directive respond in place of the mistyped respnd in the Caddyfile.
func (s *installScenario) fixTypo() error {
	held, err := os.ReadFile(s.server.at(caddyfilePath))
	if err != nil {
		return err
	}
	s.server.write(caddyfilePath, strings.Replace(string(held), "respnd", "respond", 1))
	return nil
}

// failCaddyRestart has every restart of caddy.service fail, as a Caddy that cannot start does.
func (s *installScenario) failCaddyRestart() {
	s.server.effects["systemctl"] = func(program Program) answer {
		if lineOf(program) == "systemctl restart caddy.service" {
			return failed("systemctl", "Job for caddy.service failed.\n")
		}
		return s.server.systemctl(program)
	}
}

// addHandMadeAccount gives the site's database an account that holds email under role, as one made by hand does.
func (s *installScenario) addHandMadeAccount(email, role string) {
	s.server.accounts = append(s.server.accounts, listedAccount{Email: email, Name: "Maria Perez", Role: role})
}

// letCaddyRestart has caddy.service restart again.
func (s *installScenario) letCaddyRestart() {
	s.server.effects["systemctl"] = s.server.systemctl
}

// stopsWithPrinted fails unless the run stopped with want followed by what the program that failed printed.
func (s *installScenario) stopsWithPrinted(want string) error {
	if s.err == nil || !strings.HasPrefix(s.err.Error(), want+", after it printed:\n") {
		return fmt.Errorf("the run ended with %v, want %q and what the program printed", s.err, want)
	}
	return nil
}

// ranPrograms fails unless the run ran the program lines want lists, in order, besides the checks' own.
func (s *installScenario) ranPrograms(want *godog.DocString) error {
	taken := slices.DeleteFunc(slices.Clone(s.server.ran), func(ran string) bool { return slices.Contains(probes, ran) })
	if got := strings.Join(taken, "\n"); got != want.Content {
		return fmt.Errorf("the run ran\n%s\nwant\n%s", got, want.Content)
	}
	return nil
}

// ranProgram returns the program the run ran as line, failing when it ran none.
func (s *installScenario) ranProgram(line string) (Program, error) {
	at := slices.IndexFunc(s.server.runs, func(program Program) bool { return lineOf(program) == line })
	if at < 0 {
		return Program{}, fmt.Errorf("the run ran\n%s\nwith no %q", strings.Join(s.server.ran, "\n"), line)
	}
	return s.server.runs[at], nil
}

// ranLine fails unless the run ran the program line.
func (s *installScenario) ranLine(line string) error {
	_, err := s.ranProgram(line)
	return err
}

// neverRan fails when the run ran the program line.
func (s *installScenario) neverRan(line string) error {
	if slices.Contains(s.server.ran, line) {
		return fmt.Errorf("the run ran\n%s\nwant no %q", strings.Join(s.server.ran, "\n"), line)
	}
	return nil
}

// ranIn fails unless the run ran the program line in the folder dir.
func (s *installScenario) ranIn(line, dir string) error {
	program, err := s.ranProgram(line)
	if err == nil && program.Dir != dir {
		err = fmt.Errorf("the run ran %q in %q, want %s", line, program.Dir, dir)
	}
	return err
}

// ranInWithInput fails unless the run ran the program line in dir with input and its newline on its standard input.
func (s *installScenario) ranInWithInput(line, dir, input string) error {
	program, err := s.ranProgram(line)
	if err != nil {
		return err
	}
	if got := [2]string{program.Dir, program.Stdin}; got != [2]string{dir, input + "\n"} {
		return fmt.Errorf("the run ran %q in %q with %q on its standard input, want %s and %q", line, got[0], got[1],
			dir, input+"\n")
	}
	return nil
}

// ranWithSetting fails unless the run ran the program line with setting among its extra settings.
func (s *installScenario) ranWithSetting(line, setting string) error {
	program, err := s.ranProgram(line)
	if err == nil && !slices.Contains(program.Env, setting) {
		err = fmt.Errorf("the run ran %q with the settings %q, want %s among them", line, program.Env, setting)
	}
	return err
}

// createdNoAdministrator fails when the run ran account:create-admin at all.
func (s *installScenario) createdNoAdministrator() error {
	if s.server.createdAdmin() {
		return fmt.Errorf("the run ran\n%s\nwant no account:create-admin", strings.Join(s.server.ran, "\n"))
	}
	return nil
}

// createdAdmin reports whether a run on the server ran the site's account:create-admin.
func (s *fakeServer) createdAdmin() bool {
	return slices.ContainsFunc(s.runs, func(program Program) bool {
		return programKey(program) == "gophenberg account:create-admin"
	})
}

// checkedWithSystemRoots fails unless every HTTPS fetch of the run asked for address trusting the system's roots.
func (s *installScenario) checkedWithSystemRoots(address string) error {
	return s.checkedHTTPS(address, func(roots *x509.CertPool) bool { return roots == nil })
}

// checkedWithCaddyRoot fails unless every HTTPS fetch of the run asked for address trusting Caddy's local root alone.
func (s *installScenario) checkedWithCaddyRoot(address string) error {
	return s.checkedHTTPS(address, func(roots *x509.CertPool) bool { return roots.Equal(fakeCaddyRoots()) })
}

// checkedHTTPS fails unless the run fetched address over HTTPS, each time with roots trusted takes.
func (s *installScenario) checkedHTTPS(address string, trusted func(*x509.CertPool) bool) error {
	fetched := s.server.fetchedHTTPS
	if len(fetched) == 0 {
		return fmt.Errorf("the run fetched no page over HTTPS, want %s", address)
	}
	for n, page := range fetched {
		if !strings.HasPrefix(page, address+" ") || !trusted(s.server.httpsRoots[n]) {
			return fmt.Errorf("the run fetched %q over HTTPS, want %s alone with the roots the scenario names", fetched,
				address)
		}
	}
	return nil
}

// checkedNoHTTPS fails when the run fetched any page over HTTPS.
func (s *installScenario) checkedNoHTTPS() error {
	if len(s.server.fetchedHTTPS) != 0 {
		return fmt.Errorf("the run fetched %q over HTTPS, want no page", s.server.fetchedHTTPS)
	}
	return nil
}

// endsWith fails unless what the run printed ends with want and its newline.
func (s *installScenario) endsWith(want *godog.DocString) error {
	text := withThisVersion(want.Content)
	if !strings.HasSuffix(s.stdout, "\n"+text+"\n") {
		return fmt.Errorf("the run printed\n%s\nwant it to end with\n%s", s.stdout, text)
	}
	return nil
}

// printsLines fails unless what the run printed holds the lines of want, whole and in order.
func (s *installScenario) printsLines(want *godog.DocString) error {
	text := withThisVersion(want.Content)
	if !strings.Contains("\n"+s.stdout, "\n"+text+"\n") {
		return fmt.Errorf("the run printed\n%s\nwant it to hold\n%s", s.stdout, text)
	}
	return nil
}

// withThisVersion returns text with each %VERSION% replaced by the version of this installer.
func withThisVersion(text string) string {
	return strings.ReplaceAll(text, "%VERSION%", version.Version())
}

// endsWithWebServer fails unless what the run printed ends with the text the dry run prints for the web server.
func (s *installScenario) endsWithWebServer() error {
	text, found := printedText(s.planned)
	if !found || !strings.HasSuffix(s.stdout, "\n\n"+text) {
		return fmt.Errorf("the run printed\n%s\nwant it to end with the text the dry run printed\n%s", s.stdout, text)
	}
	return nil
}

// keptPackageCaddyfile fails unless the server keeps the Caddyfile of the caddy package as Caddyfile.before-gophenberg.
func (s *installScenario) keptPackageCaddyfile() error {
	return s.holdsFile(keptCaddyfilePath, "root", "0644", packageCaddyfile)
}

// caddyfileEmails fails unless the Caddyfile on the server names email as the email of Caddy's certificate account.
func (s *installScenario) caddyfileEmails(email string) error {
	held, err := os.ReadFile(s.server.at(caddyfilePath))
	if err == nil && !slices.Contains(trimmedLines(string(held)), "email "+email) {
		err = fmt.Errorf("the server holds the Caddyfile\n%s\nwant the line email %s", held, email)
	}
	return err
}

// recordsCaddyfileSum fails unless the install record holds the SHA-256 of the Caddyfile on the server.
func (s *installScenario) recordsCaddyfileSum() error {
	record, err := readRecord(s.server.machine())
	if err != nil {
		return err
	}
	held, err := os.ReadFile(s.server.at(caddyfilePath))
	if err == nil && record.Caddyfile != caddyfileSum(string(held)) {
		err = fmt.Errorf("the install record holds the Caddyfile sum %q, want that of\n%s", record.Caddyfile, held)
	}
	return err
}

// recordEndsEveryStep fails unless the install record marks each step of a first run ended, and no other.
func (s *installScenario) recordEndsEveryStep() error {
	record, err := readRecord(s.server.machine())
	if err != nil {
		return err
	}
	want := map[string]string{}
	for _, id := range stepIDs {
		want[id] = stepEnded
	}
	if !maps.Equal(record.Steps, want) {
		return fmt.Errorf("the install record marks %v, want %v", record.Steps, want)
	}
	return nil
}

// holdsNothingAt fails when the server holds anything at path.
func (s *installScenario) holdsNothingAt(path string) error {
	if _, err := os.Lstat(s.server.at(path)); !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("the server holds %s, want nothing there: %v", path, err)
	}
	return nil
}

// holdsText fails unless the server holds the file at path, owned by owner with mode, holding want line for line.
func (s *installScenario) holdsText(path, owner, mode string, want *godog.DocString) error {
	return s.holdsFile(path, owner, mode, want.Content+"\n")
}

// writtenUnitHolds fails unless the site's unit on the server holds the line.
func (s *installScenario) writtenUnitHolds(line string) error {
	held, err := os.ReadFile(s.server.at("/etc/systemd/system/gophenberg.service"))
	if err == nil && !slices.Contains(strings.Split(string(held), "\n"), line) {
		err = fmt.Errorf("the server holds the site's unit\n%s\nwant the line %q", held, line)
	}
	return err
}

// stillNames fails unless the current link names the build called build, as Point writes it.
func (s *installScenario) stillNames(build string) error {
	target, err := os.Readlink(s.server.at("/opt/gophenberg/current"))
	if err == nil && target != filepath.Join("versions", build) {
		err = fmt.Errorf("the current link names %s, want versions/%s", target, build)
	}
	return err
}

// caddyfileLeftAlone fails unless the Caddyfile on the server is the one the earlier run wrote.
func (s *installScenario) caddyfileLeftAlone() error {
	return s.holdsFile(caddyfilePath, "root", "0644", wantedCaddyfile)
}

// caddyfileKeepsSite fails unless the Caddyfile on the server still holds the site the person added to it.
func (s *installScenario) caddyfileKeepsSite() error {
	held, err := os.ReadFile(s.server.at(caddyfilePath))
	if err == nil && !strings.Contains(string(held), "example.org {\n") {
		err = fmt.Errorf("the server holds the Caddyfile\n%s\nwant the person's site example.org in it", held)
	}
	return err
}

// copyLeftAlone fails unless the installer's own copy is still the file the run ran, never written again.
func (s *installScenario) copyLeftAlone() error {
	held, err := os.Stat(s.server.at(installerCopy))
	if err != nil {
		return err
	}
	if !os.SameFile(held, s.copied) || !held.ModTime().Equal(s.copied.ModTime()) {
		return fmt.Errorf("the run wrote the installer's own copy %s again, want it left alone", installerCopy)
	}
	return nil
}

// listedAccount is one account as account:list -json lists it.
type listedAccount struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Disabled bool   `json:"disabled"`
}

// fakeCaddyRoot is the local root certificate the fake Caddy makes once it starts, in PEM.
var fakeCaddyRoot = sync.OnceValue(func() string {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Caddy Local Authority - ECC Root"},
		NotBefore: time.Unix(0, 0), NotAfter: time.Unix(1<<32, 0), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, key.Public(), key)
	if err != nil {
		panic(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
})

// fakeCaddyRoots returns the roots that trust the fake Caddy's local root certificate alone.
func fakeCaddyRoots() *x509.CertPool {
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM([]byte(fakeCaddyRoot()))
	return roots
}

// failed is the answer of a program called name that printed printed and exited 1, as the runner reports it.
func failed(name, printed string) answer {
	return answer{printed: printed, err: errors.New("installer: run " + name + ": exit status 1")}
}

// systemctl answers systemctl as systemd does: its version, a unit started or restarted, a reload of a running unit.
func (s *fakeServer) systemctl(program Program) answer {
	verb, unit := program.Args[0], program.Args[len(program.Args)-1]
	switch verb {
	case "--version":
		return s.answers["systemctl"]
	case "start", "restart":
		s.startUnit(unit)
	case "reload":
		if !s.running[unit] {
			return failed("systemctl", unit+" is not active, cannot reload.\n")
		}
	}
	return answer{}
}

// startUnit runs unit, Caddy making its local root certificate as it starts, as it does for localhost.
func (s *fakeServer) startUnit(unit string) {
	s.running[unit] = true
	if caddy, found := s.users[pieceCaddy]; found && unit == caddyUnit {
		s.write(filepath.Join(caddy.HomeDir, caddyLocalRoot), fakeCaddyRoot())
	}
}

// caddyValidate answers caddy validate, refusing a Caddyfile that holds the mistyped directive respnd.
func (s *fakeServer) caddyValidate(Program) answer {
	held, _ := os.ReadFile(s.at(caddyfilePath))
	if strings.Contains(string(held), "respnd") {
		return failed("caddy", "Error: adapting config using caddyfile: /etc/caddy/Caddyfile:10: unrecognized "+
			"directive: respnd\n")
	}
	return answer{printed: "Valid configuration\n"}
}

// createAdmin creates the account account:create-admin names, refusing an email an account already holds.
func (s *fakeServer) createAdmin(program Program) answer {
	email := flagValue(program.Args, "-email")
	if slices.ContainsFunc(s.accounts, func(held listedAccount) bool { return held.Email == email }) {
		return failed(program.Name, "Password: gouncer: email already taken\n")
	}
	s.accounts = append(s.accounts, listedAccount{
		Email: email, Name: flagValue(program.Args, "-name"), Role: flagValue(program.Args, "-role"),
	})
	return answer{printed: "Password: created user " + email + "\n"}
}

// flagValue returns the value that follows flag in args.
func flagValue(args []string, flag string) string {
	return args[slices.Index(args, flag)+1]
}

// listAccounts lists every account as account:list -json does, indented as gonsole prints it.
func (s *fakeServer) listAccounts(Program) answer {
	listed, _ := json.MarshalIndent(map[string][]listedAccount{"accounts": slices.Clone(s.accounts)}, "", "  ")
	return answer{printed: string(listed) + "\n"}
}

// sitePage answers a page of the site with 200 while gophenberg.service runs, refusing the connection otherwise.
func (s *fakeServer) sitePage(request *http.Request) (*http.Response, error) {
	if !s.running[pieceService] || s.siteDown {
		return nil, errors.New("dial tcp " + request.URL.Host + ": connect: connection refused")
	}
	return answered(http.StatusOK, "")(request)
}

// caddyPage answers an HTTPS page with 200 while caddy.service runs, refusing the connection otherwise.
func (s *fakeServer) caddyPage(request *http.Request) (*http.Response, error) {
	if !s.running[caddyUnit] {
		return nil, errors.New("dial tcp " + request.URL.Host + ":443: connect: connection refused")
	}
	return answered(http.StatusOK, "")(request)
}

// saveRecord puts record on the server as its install record.
func (s *fakeServer) saveRecord(record Record) {
	s.t.Helper()
	body, err := json.Marshal(record)
	if err != nil {
		s.t.Fatalf("encoding the install record: %v", err)
	}
	s.write(recordPath, string(body))
}
