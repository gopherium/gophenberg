// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/version"
)

// listeningFeature is the feature the site answers to on the listener its settings name.
const listeningFeature = "../../test/features/features/listening.feature"

// listeningBound is how long a scenario waits for the site to answer or to end.
const listeningBound = 30 * time.Second

// listeningAnswer is the path a scenario asks for to learn the site answers.
const listeningAnswer = "/api/content/v1/items"

// listeningScenario is the one site a scenario serves and what it answered.
type listeningScenario struct {
	t       *testing.T
	env     map[string]string
	socket  string
	header  string
	address string
	port    string
	stop    context.CancelFunc
	done    chan int
	written answered
	result  testkit.Result
}

// socketWrite is what an older browser sends over the socket for a scenario.
type socketWrite struct {
	origin        string
	scheme        string
	forwardedHost string
}

// initializeListening returns the binding of the listener steps, t holding their databases and folders.
func initializeListening(t *testing.T) func(*godog.ScenarioContext) {
	return func(sc *godog.ScenarioContext) {
		s := &listeningScenario{t: t, env: map[string]string{}}
		sc.Given(`^the site listens on a socket with the visitor's address in "([^"]*)"$`, s.nameSocket)
		sc.Given(`^the settings name a free address$`, s.nameAddress)
		sc.Given(`^the host names a free PORT$`, s.namePort)
		sc.Given(`^the setting "([^"]*)" holds "([^"]*)"$`, s.hold)
		sc.Given(`^the settings hold "([^"]*)"$`, s.holdAll)
		sc.When(`^the site serves$`, s.serve)
		sc.When(`^the site starts$`, s.start)
		sc.When(`^an older browser on "([^"]*)" signs in over the socket, naming the scheme "([^"]*)"$`, s.signInWithScheme)
		sc.When(`^an older browser on "([^"]*)" signs in over the socket, naming no scheme$`, s.signInWithNoScheme)
		sc.When(`^an older browser on "([^"]*)" signs in over the socket, naming the forwarded host "([^"]*)"$`,
			s.signInWithForwardedHost)
		sc.When(`^the operator runs "([^"]*)"$`, s.runCommand)
		sc.Then(`^only the site's own user can open the socket$`, s.socketIsPrivate)
		sc.Then(`^a request over the socket from the visitor "([^"]*)" is answered$`, s.answersVisitor)
		sc.Then(`^a request over the socket that names no visitor answers 400$`, s.refusesNoVisitor)
		sc.Then(`^the site answers on the address$`, func() error { return s.answersOn(s.address) })
		sc.Then(`^the site answers on the PORT$`, func() error { return s.answersOn(s.portAddress()) })
		sc.Then(`^nothing answers on the address$`, func() error { return nothingOn(s.address) })
		sc.Then(`^nothing answers on the PORT$`, func() error { return nothingOn(s.portAddress()) })
		sc.Then(`^the write is not refused as cross-origin$`, s.notRefused)
		sc.Then(`^the write is refused as cross-origin$`, s.refused)
		sc.Then(`^the site exits with code (\d+) by itself$`, s.exitsWith)
		sc.Then(`^the site is still serving$`, s.stillServing)
		sc.Then(`^the command fails, saying "([^"]*)"$`, s.failsSaying)
		sc.After(s.stopServing)
	}
}

// nameSocket names a new socket and the header holding the visitor's address on it.
func (s *listeningScenario) nameSocket(header string) {
	s.socket, s.header = filepath.Join(s.t.TempDir(), "site.sock"), header
	s.env["GOPHENBERG_SOCKET"], s.env["GOPHENBERG_CLIENT_ADDR_HEADER"] = s.socket, header
}

// nameAddress names a free address in the settings.
func (s *listeningScenario) nameAddress() {
	s.address = testkit.FreeAddr(s.t)
	s.env["GOPHENBERG_ADDR"] = s.address
}

// namePort names a free port the way a host does, in the bare PORT.
func (s *listeningScenario) namePort() error {
	_, port, err := net.SplitHostPort(testkit.FreeAddr(s.t))
	s.port, s.env["PORT"] = port, port
	return err
}

// hold names the value of one setting.
func (s *listeningScenario) hold(name, value string) {
	s.env[name] = value
}

// holdAll names each setting of a list of NAME=value pairs split by commas.
func (s *listeningScenario) holdAll(settings string) {
	for _, setting := range strings.Split(settings, ", ") {
		name, value, _ := strings.Cut(setting, "=")
		s.hold(name, value)
	}
}

// portAddress returns the loopback address of the port the host named.
func (s *listeningScenario) portAddress() string {
	return net.JoinHostPort("127.0.0.1", s.port)
}

// start runs serve over a fresh database, in the background and under the scenario's settings.
func (s *listeningScenario) start() {
	s.env["GOPHENBERG_DATABASE_URL"] = emptyDatabaseURL(s.t)
	ctx, cancel := context.WithCancel(s.t.Context())
	s.stop, s.done = cancel, make(chan int, 1)
	exits := &exitCodes{}
	program := commandLine(testkit.Getenv(s.env), noPlugins, exits, version.Stamp())
	go func() {
		s.done <- exits.of(program.Run(ctx, []string{"serve"}, strings.NewReader(""), io.Discard, io.Discard))
	}()
}

// serve starts the site and waits until it answers where its settings name.
func (s *listeningScenario) serve() error {
	s.start()
	deadline := time.Now().Add(listeningBound)
	var err error
	for time.Now().Before(deadline) {
		if err = s.answered(); err == nil {
			return nil
		}
		select {
		case code := <-s.done:
			s.stop()
			s.stop = nil
			return fmt.Errorf("the site exited with code %d before it answered: %w", code, err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	return fmt.Errorf("the site never answered: %w", err)
}

// answered reports whether the site answers at the place the scenario named first of socket, address and port.
func (s *listeningScenario) answered() error {
	if s.socket != "" {
		_, err := s.overSocket(s.request("203.0.113.7"))
		return err
	}
	if s.address != "" {
		return s.answersOn(s.address)
	}
	return s.answersOn(s.portAddress())
}

// request returns a read of the site over the socket, naming the visitor when one is given.
func (s *listeningScenario) request(visitor string) *http.Request {
	request, err := http.NewRequestWithContext(s.t.Context(), http.MethodGet, "http://site"+listeningAnswer, nil)
	if err != nil {
		s.t.Fatalf("building the request: %v", err)
	}
	if visitor != "" {
		request.Header.Set(s.header, visitor)
	}
	return request
}

// overSocket sends request over the socket on a connection of its own and returns what the site answered.
func (s *listeningScenario) overSocket(request *http.Request) (answered, error) {
	dial := func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", s.socket)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: dial, DisableKeepAlives: true}, Timeout: listeningBound}
	return read(client.Do(request))
}

// signInWithScheme signs in from origin the way an older browser does, the program in front naming the scheme.
func (s *listeningScenario) signInWithScheme(origin, scheme string) error {
	return s.signIn(socketWrite{origin: origin, scheme: scheme})
}

// signInWithNoScheme signs in from origin the way an older browser does, nothing naming the scheme.
func (s *listeningScenario) signInWithNoScheme(origin string) error {
	return s.signIn(socketWrite{origin: origin})
}

// signInWithForwardedHost signs in from origin to the site's own host, a forwarded header naming another one.
func (s *listeningScenario) signInWithForwardedHost(origin, host string) error {
	return s.signIn(socketWrite{origin: origin, scheme: "https", forwardedHost: host})
}

// signIn posts a sign in over the socket with only the older Origin signal, keeping what the site answered.
func (s *listeningScenario) signIn(write socketWrite) error {
	login := strings.NewReader(fmt.Sprintf(`{"email":%q,"password":%q}`, adminEmail, typedPassword))
	request, err := http.NewRequestWithContext(s.t.Context(), http.MethodPost, "http://site/api/auth/login", login)
	if err != nil {
		return err
	}
	parsed, err := url.Parse(write.origin)
	if err != nil {
		return err
	}
	request.Host = parsed.Host
	if write.forwardedHost != "" {
		request.Host = "example.com"
		request.Header.Set("X-Forwarded-Host", write.forwardedHost)
	}
	if write.scheme != "" {
		request.Header.Set("X-Forwarded-Proto", write.scheme)
	}
	request.Header.Set("Origin", write.origin)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(s.header, "203.0.113.7")
	s.written, err = s.overSocket(request)
	return err
}

// runCommand runs the command line over the scenario's settings and a database nothing answers at.
func (s *listeningScenario) runCommand(line string) {
	s.env["GOPHENBERG_DATABASE_URL"] = unreachableDatabaseURL
	s.result = testkit.Run(s.t, Program(testkit.Getenv(s.env), noPlugins), "", strings.Fields(line)...)
}

// socketIsPrivate fails unless the socket file answers to its owner alone.
func (s *listeningScenario) socketIsPrivate() error {
	info, err := os.Stat(s.socket)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o600 {
		return fmt.Errorf("the socket has the mode %o, want 600", info.Mode().Perm())
	}
	return nil
}

// answersVisitor fails unless a request naming the visitor is answered with 200.
func (s *listeningScenario) answersVisitor(visitor string) error {
	got, err := s.overSocket(s.request(visitor))
	if err != nil {
		return err
	}
	if got.status != http.StatusOK {
		return fmt.Errorf("the site answered %d and %s, want 200", got.status, got.body)
	}
	return nil
}

// refusesNoVisitor fails unless a request naming no visitor is answered with 400.
func (s *listeningScenario) refusesNoVisitor() error {
	got, err := s.overSocket(s.request(""))
	if err != nil {
		return err
	}
	if got.status != http.StatusBadRequest {
		return fmt.Errorf("the site answered %d, want 400", got.status)
	}
	return nil
}

// answersOn fails unless the site answers on address with 200, on a connection that closes behind the request.
func (s *listeningScenario) answersOn(address string) error {
	request, err := http.NewRequestWithContext(s.t.Context(), http.MethodGet, "http://"+address+listeningAnswer, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Transport: &http.Transport{DisableKeepAlives: true}, Timeout: listeningBound}
	got, err := read(client.Do(request))
	if err == nil && got.status != http.StatusOK {
		err = fmt.Errorf("the site answered %d on %s, want 200", got.status, address)
	}
	return err
}

// nothingOn fails when something accepts a connection on address.
func nothingOn(address string) error {
	conn, err := net.DialTimeout("tcp", address, time.Second)
	if err != nil {
		return nil
	}
	_ = conn.Close()
	return fmt.Errorf("something listens on %s, want nothing", address)
}

// crossOriginRefused reports whether the write was refused for its origin.
func (s *listeningScenario) crossOriginRefused() bool {
	return s.written.status == http.StatusForbidden && strings.Contains(string(s.written.body), "request_cross_origin")
}

// notRefused fails when the write was refused for its origin.
func (s *listeningScenario) notRefused() error {
	if s.crossOriginRefused() {
		return fmt.Errorf("the site answered %d and %s, want the write past the origin check",
			s.written.status, s.written.body)
	}
	return nil
}

// refused fails unless the write was refused for its origin.
func (s *listeningScenario) refused() error {
	if !s.crossOriginRefused() {
		return fmt.Errorf("the site answered %d and %s, want the write refused for its origin",
			s.written.status, s.written.body)
	}
	return nil
}

// exitsWith fails unless the site ends by itself with code.
func (s *listeningScenario) exitsWith(code int) error {
	select {
	case got := <-s.done:
		s.stop()
		s.stop = nil
		if got != code {
			return fmt.Errorf("the site exited with code %d, want %d", got, code)
		}
		return nil
	case <-time.After(listeningBound):
		return fmt.Errorf("the site still served %v after it started, want it to end by itself", listeningBound)
	}
}

// stillServing fails when the run of the site has ended.
func (s *listeningScenario) stillServing() error {
	select {
	case got := <-s.done:
		s.stop = nil
		return fmt.Errorf("the site exited with code %d, want it serving", got)
	default:
		return nil
	}
}

// failsSaying fails unless the command failed with the text in its stderr.
func (s *listeningScenario) failsSaying(text string) error {
	if s.result.Code == 0 || !strings.Contains(s.result.Stderr, text) {
		return fmt.Errorf("the command exited with %d and stderr %q, want a failure saying %q",
			s.result.Code, s.result.Stderr, text)
	}
	return nil
}

// stopServing stops the site a scenario left running and waits for it.
func (s *listeningScenario) stopServing(ctx context.Context, _ *godog.Scenario, err error) (context.Context, error) {
	if s.stop == nil {
		return ctx, err
	}
	s.stop()
	code := <-s.done
	if code != 0 {
		return ctx, errors.Join(err, fmt.Errorf("the site exited with code %d, want 0", code))
	}
	return ctx, err
}

func TestListening(t *testing.T) {
	t.Parallel()

	suite := godog.TestSuite{
		ScenarioInitializer: initializeListening(t),
		Options: &godog.Options{
			Format: "pretty", Paths: []string{listeningFeature}, Strict: true, TestingT: t,
		},
	}
	if suite.Run() != 0 {
		t.Error("the feature did not pass")
	}
}
