// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// askPause is the pause between two asks of a scenario that waits for the site to name what it expects.
const askPause = 50 * time.Millisecond

// servedSite is a site a scenario serves in the background, with a client signed in as the administrator.
type servedSite struct {
	base   string
	client *http.Client
	cancel context.CancelFunc
	done   chan int
	stderr *keptWatch
}

// answered is what the site answered one request of a scenario: its status and its body.
type answered struct {
	status int
	body   []byte
}

// answeredError is the code and the message of a request the site refused.
type answeredError struct {
	Code    string `json:"code"`
	Message string `json:"error"`
}

// initializeAdminUpdates binds the steps of the scenarios where an administrator starts an update from the admin to s.
func initializeAdminUpdates(sc *godog.ScenarioContext, s *operatorScenario) {
	sc.After(s.leaveSite)
	sc.Given(`^the database backup waits until the scenario lets it end$`, s.holdBackup)
	sc.Given(`^update:apply holds the update lock$`, s.holdUpdateLock)
	sc.Given(`^the Updates screen settings (?:are left out|are "([^"]*)" and "([^"]*)")$`, s.nameScreenSettings)
	sc.When(`^an administrator (starts|retries) the update to "([^"]*)" from the admin$`, s.startFromAdmin)
	sc.When(`^the database backup ends$`, s.endBackup)
	sc.When(`^an administrator asks the site for its settings$`, s.askForSettings)
	sc.Then(`^the site answers that the update started$`, s.answersStarted)
	sc.Then(`^the site stops with code (\d+) and asks for a restart to start "([^"]*)"$`, s.stopsForRestart)
	sc.Then(`^the update status says the update to "([^"]*)" waits for its first start$`, s.statusWaitsForStart)
	sc.Then(`^a second start of the update to "([^"]*)" is refused with the code "([^"]*)"$`, s.refusesSecondStart)
	sc.Then(`^the site refuses the start with the code "([^"]*)", saying "([^"]*)"$`, s.refusesStart)
	sc.Then(`^the site keeps serving the build that runs$`, s.keepsServing)
	sc.Then(`^the site tells the Updates screen to ask every (\d+) milliseconds and to wait (\d+) milliseconds `+
		`for the restart$`, s.tellsScreen)
}

// serveSigned serves the site in the background, no visit running a cron job, and signs the administrator in.
func (s *operatorScenario) serveSigned() error {
	if err := s.holdAccount("administrator", adminEmail); err != nil {
		return err
	}
	address := testkit.FreeAddr(s.t)
	s.env["GOPHENBERG_ADDR"] = address
	s.env["GOPHENBERG_CRON_ON_VISITS"], s.env["GOPHENBERG_CRON_SECRET"] = "false", cronSecret
	serving, cancel := context.WithCancel(s.t.Context())
	site := &servedSite{
		base: "http://" + address, cancel: cancel, done: make(chan int, 1), stderr: &keptWatch{cancel: cancel},
	}
	exits := &exitCodes{}
	program := commandLine(testkit.Getenv(s.env), s.loaded(), exits, s.stamp)
	go func() {
		site.done <- exits.of(program.Run(serving, []string{"serve"}, strings.NewReader(""), io.Discard, site.stderr))
	}()
	s.served = site
	awaitServer(s.t, site.base)
	client, err := signedIn(site.base)
	site.client = client
	return err
}

// signedIn returns a client of the site at base holding the session of the administrator.
func signedIn(base string) (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Jar: jar, Timeout: siteStartLimit}
	login, _ := json.Marshal(map[string]string{"email": adminEmail, "password": typedPassword})
	got, err := read(client.Post(base+"/api/auth/login", "application/json", bytes.NewReader(login)))
	if err == nil && got.status != http.StatusOK {
		err = fmt.Errorf("signing in answered %d and %s, want 200", got.status, got.body)
	}
	return client, err
}

// read returns the status and the body of response, or err when the request got no answer.
func read(response *http.Response, err error) (answered, error) {
	if err != nil {
		return answered{}, err
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	return answered{status: response.StatusCode, body: body}, err
}

// get asks the site for path as the administrator.
func (site *servedSite) get(path string) (answered, error) {
	return read(site.client.Get(site.base + path))
}

// post sends body to path as the administrator.
func (site *servedSite) post(path, body string) (answered, error) {
	return read(site.client.Post(site.base+path, "application/json", strings.NewReader(body)))
}

// ended waits for the run of the site to end on its own within the start limit, then returns the code it answered.
func (site *servedSite) ended() (int, error) {
	select {
	case code := <-site.done:
		return code, nil
	case <-time.After(siteStartLimit):
		site.cancel()
		return <-site.done, fmt.Errorf("the site still served %v after the start, want it stopped on its own", siteStartLimit)
	}
}

// stopServing stops the site the scenario serves, failing unless its run ended cleanly.
func (s *operatorScenario) stopServing() error {
	site := s.served
	if site == nil {
		return errors.New("the scenario serves no site, want one serving")
	}
	s.served = nil
	site.cancel()
	if code := <-site.done; code != gonsole.ExitDone {
		return fmt.Errorf("the site stopped with %d and stderr %q, want 0", code, site.stderr.String())
	}
	return nil
}

// leaveSite stops the site a scenario still serves and lets go of the update lock it holds, once the scenario ends.
func (s *operatorScenario) leaveSite(ctx context.Context, _ *godog.Scenario, _ error) (context.Context, error) {
	if s.served != nil {
		s.served.cancel()
		<-s.served.done
		s.served = nil
	}
	if s.release != nil {
		s.release()
	}
	return ctx, nil
}

// readUpdates asks the site the scenario serves what it answers about its updates, keeping it in s.updates.
func (s *operatorScenario) readUpdates() error {
	got, err := s.served.get("/api/updates")
	if err != nil {
		return err
	}
	if got.status != http.StatusOK {
		return fmt.Errorf("GET /api/updates answered %d and %s, want 200", got.status, got.body)
	}
	s.updates = siteAnswer{body: string(got.body)}
	if err := json.Unmarshal(got.body, &s.updates); err != nil {
		return fmt.Errorf("the site answered %q, no JSON document: %w", got.body, err)
	}
	return nil
}

// siteNames runs check over what the site answered about its updates, asking again while it serves until check passes.
func (s *operatorScenario) siteNames(check func() error) error {
	if s.served == nil {
		return check()
	}
	deadline := time.Now().Add(siteStartLimit)
	for {
		if err := s.readUpdates(); err != nil {
			return err
		}
		err := check()
		if err == nil || time.Now().After(deadline) {
			return err
		}
		time.Sleep(askPause)
	}
}

// holdBackup backs up with a pg_dump stand-in whose dump waits until the scenario writes its gate file.
func (s *operatorScenario) holdBackup() error {
	folder := s.t.TempDir()
	s.gate = filepath.Join(folder, "gate")
	bin := filepath.Join(folder, "pg_dump")
	script := "#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then\n  echo 'pg_dump (PostgreSQL) 99.0'\n  exit 0\nfi\n" +
		"while [ ! -e '" + s.gate + "' ]; do sleep 0.05; done\nprintf 'a dump of the site database'\n"
	s.env["GOPHENBERG_PG_DUMP_BIN"] = bin
	return os.WriteFile(bin, []byte(script), 0o755)
}

// endBackup writes the gate file the pg_dump stand-in waits for, so the backup ends.
func (s *operatorScenario) endBackup() error {
	return os.WriteFile(s.gate, []byte("go on"), 0o600)
}

// holdUpdateLock takes the update lock as a running update:apply holds it, until the scenario ends.
func (s *operatorScenario) holdUpdateLock(ctx context.Context) error {
	release, err := updater.Lock(ctx, testdb.Open(s.t, s.env["GOPHENBERG_DATABASE_URL"]))
	s.release = release
	return err
}

// nameScreenSettings names how often the Updates screen asks for progress and how long it waits, empty for neither.
func (s *operatorScenario) nameScreenSettings(progress, wait string) {
	s.env["GOPHENBERG_UPDATES_SCREEN_PROGRESS_INTERVAL"] = progress
	s.env["GOPHENBERG_UPDATES_SCREEN_RESTART_WAIT"] = wait
}

// startFromAdmin serves the site and asks it, as the administrator, to start the update to release, retrying it or not.
func (s *operatorScenario) startFromAdmin(asked, release string) error {
	if err := s.serveSigned(); err != nil {
		return err
	}
	return s.askStart(release, asked == "retries")
}

// askStart asks the site the scenario serves to start the update to release, keeping its answer in s.started.
func (s *operatorScenario) askStart(release string, retry bool) error {
	body, _ := json.Marshal(map[string]any{"version": release, "retry": retry})
	started, err := s.served.post("/api/updates/apply", string(body))
	s.started = started
	return err
}

// answersStarted fails unless the site answered 202 and the overview of its updates, naming this version running.
func (s *operatorScenario) answersStarted() error {
	var overview siteAnswer
	err := json.Unmarshal(s.started.body, &overview)
	if err != nil || s.started.status != http.StatusAccepted || overview.Running != version.Version() {
		return fmt.Errorf("the start answered %d and %s (%v), want 202 and the overview naming %s running",
			s.started.status, s.started.body, err, version.Version())
	}
	return nil
}

// stopsForRestart waits for the site to stop on its own, failing unless it exits with code and asks to start release.
func (s *operatorScenario) stopsForRestart(code int, release string) error {
	site := s.served
	if site == nil {
		return errors.New("the scenario serves no site, want one serving")
	}
	ended, err := site.ended()
	s.served = nil
	s.result = testkit.Result{Code: ended, Stderr: site.stderr.String()}
	return errors.Join(err, s.exitsWith(code), s.asksForRestart(release))
}

// statusWaitsForStart fails unless update:status says the update from this version to release waits for a start.
func (s *operatorScenario) statusWaitsForStart(release string) error {
	status, err := s.updateStatus()
	if err != nil {
		return err
	}
	held := status.Update
	if held.Step != updater.StepSwitched || held.From != version.Version() || held.To != release {
		return fmt.Errorf("update:status says %+v, want the update from %s to %s switched", held, version.Version(),
			release)
	}
	return nil
}

// refusesSecondStart asks the site to start the update to release again, failing unless it refuses with 409 and code.
func (s *operatorScenario) refusesSecondStart(release, code string) error {
	if err := s.askStart(release, false); err != nil {
		return err
	}
	if s.started.status != http.StatusConflict {
		return fmt.Errorf("the second start answered %d and %s, want 409", s.started.status, s.started.body)
	}
	return s.refusesStart(code, "")
}

// refusesStart fails unless the site refused the start with code and a message carrying text.
func (s *operatorScenario) refusesStart(code, text string) error {
	var refused answeredError
	err := json.Unmarshal(s.started.body, &refused)
	if err != nil || s.started.status < http.StatusBadRequest || refused.Code != code ||
		!strings.Contains(refused.Message, text) {
		return fmt.Errorf("the start answered %d and %s (%v), want it refused with %s saying %q", s.started.status,
			s.started.body, err, code, text)
	}
	return nil
}

// keepsServing fails unless the site still serves this version, then stops it and fails unless it stopped cleanly.
func (s *operatorScenario) keepsServing() error {
	if s.served == nil {
		return errors.New("the scenario serves no site, want one serving")
	}
	got, err := s.served.get("/api/version")
	if err == nil && (got.status != http.StatusOK || !strings.Contains(string(got.body), version.Version())) {
		err = fmt.Errorf("GET /api/version answered %d and %s, want 200 and %s", got.status, got.body, version.Version())
	}
	return errors.Join(err, s.stopServing())
}

// askForSettings serves the site, asks it as the administrator for its settings and keeps what it answers.
func (s *operatorScenario) askForSettings() error {
	if err := s.serveSigned(); err != nil {
		return err
	}
	told, err := s.served.get("/api/settings")
	s.told = told
	return errors.Join(err, s.stopServing())
}

// tellsScreen fails unless the settings tell the Updates screen to ask every ask milliseconds and to wait wait.
func (s *operatorScenario) tellsScreen(ask, wait int64) error {
	var told struct {
		Progress *int64 `json:"updates_progress_milliseconds"`
		Wait     *int64 `json:"updates_restart_wait_milliseconds"`
	}
	err := json.Unmarshal(s.told.body, &told)
	if err != nil || told.Progress == nil || told.Wait == nil || *told.Progress != ask || *told.Wait != wait {
		return fmt.Errorf("the settings answered %d and %s (%v), want %d and %d milliseconds", s.told.status,
			s.told.body, err, ask, wait)
	}
	return nil
}
