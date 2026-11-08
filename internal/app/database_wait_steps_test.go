// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole/testkit"
)

// databaseWaitFeature is the feature serve answers to while its database starts late.
const databaseWaitFeature = "../../test/features/features/database-wait.feature"

// waitedLines are the lines serve logs while it waits for its database, by the words a scenario names them with.
var waitedLines = map[string]string{
	"it waits for the database": waitingLine,
	"the database answered":     answeredLine,
}

// waitScenario is what one database wait scenario keeps beside the operator scenario: the stand-in for its database.
type waitScenario struct {
	*operatorScenario
	mu       sync.Mutex
	timer    *time.Timer
	listener net.Listener
	ended    bool
}

// initializeDatabaseWait binds the steps of the scenarios where serve waits for a database that starts late to s.
func initializeDatabaseWait(sc *godog.ScenarioContext, s *operatorScenario) {
	w := &waitScenario{operatorScenario: s}
	sc.After(w.closeStandIn)
	sc.Given(`^serve tries the database again every "([^"]*)"$`, w.tryEvery)
	sc.Given(`^serve waits "([^"]*)" for the database$`, w.waitFor)
	sc.Given(`^the database refuses every connection$`, w.refuseAll)
	sc.Given(`^the database refuses every connection for (\d+) seconds$`, w.refuseFor)
	sc.Given(`^the settings name a wrong password for the database$`, w.nameWrongPassword)
	sc.When(`^the site starts and listens$`, w.startAndListen)
	sc.Then(`^the site logged once that (it waits for the database|the database answered)$`, w.loggedOnce)
	sc.Then(`^the site never waited for the database$`, w.neverWaited)
}

// tryEvery names the pause serve takes between two tries of the database.
func (w *waitScenario) tryEvery(interval string) {
	w.env["GOPHENBERG_DATABASE_WAIT_INTERVAL"] = interval
}

// waitFor names how long serve waits for a database that is not up yet.
func (w *waitScenario) waitFor(wait string) {
	w.env["GOPHENBERG_DATABASE_WAIT"] = wait
}

// refuseAll points the settings at a loopback address nothing listens on, in place of the scenario's database.
func (w *waitScenario) refuseAll() error {
	_, _, err := w.moveDatabase()
	return err
}

// refuseFor points the settings at a stand-in that refuses every connection for the seconds given, then forwards each.
func (w *waitScenario) refuseFor(seconds int) error {
	address, upstream, err := w.moveDatabase()
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.timer = time.AfterFunc(time.Duration(seconds)*time.Second, func() { w.openStandIn(address, upstream) })
	return nil
}

// moveDatabase points the settings at a loopback address nothing listens on, answering it and the database's address.
func (w *waitScenario) moveDatabase() (string, string, error) {
	held, err := url.Parse(w.env["GOPHENBERG_DATABASE_URL"])
	if err != nil {
		return "", "", err
	}
	upstream := held.Host
	held.Host = testkit.FreeAddr(w.t)
	w.env["GOPHENBERG_DATABASE_URL"] = held.String()
	return held.Host, upstream, nil
}

// openStandIn listens at address and forwards every connection it accepts to upstream, unless the scenario ended.
func (w *waitScenario) openStandIn(address, upstream string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.ended {
		return
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		w.t.Errorf("opening the database stand-in at %s: %v", address, err)
		return
	}
	w.listener = listener
	go forwardAll(listener, upstream)
}

// closeStandIn keeps the stand-in from opening and closes it, once the scenario ends.
func (w *waitScenario) closeStandIn(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.ended = true
	if w.timer != nil {
		w.timer.Stop()
	}
	if w.listener == nil {
		return ctx, nil
	}
	return ctx, w.listener.Close()
}

// forwardAll forwards every connection listener accepts to upstream, until listener closes.
func forwardAll(listener net.Listener, upstream string) {
	for {
		accepted, err := listener.Accept()
		if err != nil {
			return
		}
		go forward(accepted, upstream)
	}
}

// forward copies bytes both ways between accepted and a new connection to upstream, until either side ends.
func forward(accepted net.Conn, upstream string) {
	defer func() { _ = accepted.Close() }()
	dialed, err := net.Dial("tcp", upstream)
	if err != nil {
		return
	}
	defer func() { _ = dialed.Close() }()
	ended := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(dialed, accepted); ended <- struct{}{} }()
	go func() { _, _ = io.Copy(accepted, dialed); ended <- struct{}{} }()
	<-ended
}

// nameWrongPassword names the scenario's database with a password its role does not have.
func (w *waitScenario) nameWrongPassword() error {
	held, err := url.Parse(w.env["GOPHENBERG_DATABASE_URL"])
	if err != nil {
		return err
	}
	held.User = url.UserPassword(held.User.Username(), "wrongpassword")
	w.env["GOPHENBERG_DATABASE_URL"] = held.String()
	return nil
}

// startAndListen serves the site over the scenario's settings until it listens, failing when it never does.
func (w *waitScenario) startAndListen() error {
	w.env["GOPHENBERG_ADDR"] = "127.0.0.1:0"
	ctx, cancel := context.WithTimeout(w.t.Context(), siteStartLimit)
	defer cancel()
	exits := &exitCodes{}
	logged := &strings.Builder{}
	code := commandLine(testkit.Getenv(w.env), w.loaded(), exits, w.stamp).Run(
		ctx, []string{"serve"}, strings.NewReader(""), io.Discard, listenRecorder{cancel: cancel, logged: logged})
	w.result = testkit.Result{Code: exits.of(code), Stderr: logged.String()}
	if !strings.Contains(w.result.Stderr, "msg=listening") {
		return fmt.Errorf("the site never listened, its stderr says %q", w.result.Stderr)
	}
	return nil
}

// loggedOnce fails unless the site logged the line a scenario names by what exactly once.
func (w *waitScenario) loggedOnce(what string) error {
	line := waitedLines[what]
	if held := strings.Count(w.result.Stderr, line); held != 1 {
		return fmt.Errorf("the site logged %q %d times, want once, its stderr says %q", line, held, w.result.Stderr)
	}
	return nil
}

// neverWaited fails when the site logged that it waits for the database.
func (w *waitScenario) neverWaited() error {
	if strings.Contains(w.result.Stderr, waitingLine) {
		return fmt.Errorf("the site logged %q, want it stopped without waiting, its stderr says %q", waitingLine,
			w.result.Stderr)
	}
	return nil
}

func TestDatabaseWait(t *testing.T) {
	t.Parallel()

	runOperatorFeature(t, databaseWaitFeature, postgresOperators())
}
