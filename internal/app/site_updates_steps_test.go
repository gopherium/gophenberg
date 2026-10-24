// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cucumber/godog"
	"github.com/gopherium/framework/gonsole"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// cronSecret is the cron secret of the site a scenario serves, so that no visit runs a cron job.
const cronSecret = "a cron secret that holds more than thirty-two characters"

// adminEmail is the address of the administrator who asks the site for its updates.
const adminEmail = "admin@example.com"

// siteAnswer is what the site answers about its updates, read by the names the admin reads it by.
type siteAnswer struct {
	Running      string          `json:"running"`
	CanUpdate    bool            `json:"can_update"`
	CannotUpdate string          `json:"cannot_update"`
	Refused      []string        `json:"refused"`
	Job          *answeredUpdate `json:"job"`
	Backups      answeredBackups `json:"backups"`
	NextCheck    *time.Time      `json:"next_check"`
	body         string
}

// answeredUpdate is the last update the site names: its step, the versions it went from and to, and its failure.
type answeredUpdate struct {
	Step    string           `json:"step"`
	From    string           `json:"from"`
	To      string           `json:"to"`
	Failure *answeredFailure `json:"failure"`
}

// answeredFailure is how an update failed: when, from and to which versions, at which step and why.
type answeredFailure struct {
	At    time.Time `json:"at"`
	From  string    `json:"from"`
	To    string    `json:"to"`
	Step  string    `json:"step"`
	Error string    `json:"error"`
}

// wantedFailure is the failure a scenario expects: the version the update went to, the step and a part of the error.
type wantedFailure struct {
	to   string
	step string
	text string
}

// answeredBackups is how many backups the site keeps, how many it may keep and when the newest was taken.
type answeredBackups struct {
	Kept   int        `json:"kept"`
	Limit  int        `json:"limit"`
	Newest *time.Time `json:"newest"`
}

// initializeSiteUpdates binds the steps of the scenarios where an administrator asks the site for its updates to s.
func initializeSiteUpdates(sc *godog.ScenarioContext, s *operatorScenario) {
	sc.Given(`^an earlier update failed at the step "([^"]*)"$`, s.failEarlierUpdate)
	sc.Given(`^an earlier update from "([^"]*)" to this version failed at the step "([^"]*)"$`, s.failUpdateToThis)
	sc.Given(`^the last check ran on the build "([^"]*)" and found this version$`, s.checkedOnBuild)
	sc.Given(`^the operator ran "([^"]*)" and it failed$`, s.ranLineFailing)
	sc.Given(`^the install root keeps (\d+) database backups, the newest taken at "([^"]*)"$`, s.keepBackups)
	sc.Given(`^the settings keep (\d+) backups$`, s.keepBackupLimit)
	sc.Given(`^the check for a newer version ran an hour ago$`, s.checkedAnHourAgo)
	sc.Given(`^the settings turn the checks for a newer version off$`, s.turnChecksOff)
	sc.When(`^an administrator asks the site for its updates$`, s.askForUpdates)
	sc.Then(`^the update status names when the update from this version to "([^"]*)" failed, at the step "([^"]*)", `+
		`and why: "([^"]*)"$`, s.statusNamesFailure)
	sc.Then(`^the update status names no failure$`, s.statusNamesNoFailure)
	sc.Then(`^the site names this version as the one that runs$`, s.namesRunning)
	sc.Then(`^the site says this install can update itself$`, s.saysItCanUpdate)
	sc.Then(`^the site says this install cannot update itself, since "([^"]*)"$`, s.saysItCannotUpdate)
	sc.Then(`^the site names no refused version and no update$`, s.namesNothingRefused)
	sc.Then(`^the site names no update on offer$`, s.namesNoOffer)
	sc.Then(`^the site names no failure$`, s.namesNoFailure)
	sc.Then(`^the site names "([^"]*)" as refused$`, s.namesRefused)
	sc.Then(`^the site names the update from this version to "([^"]*)" at the step "([^"]*)"$`, s.namesUpdate)
	sc.Then(`^the site names when the update from this version to "([^"]*)" failed, at the step "([^"]*)", `+
		`and why: "([^"]*)"$`, s.namesFailure)
	sc.Then(`^the site counts (\d+) backups kept out of (\d+)(?:, the newest taken at "([^"]*)")?$`, s.countsBackups)
	sc.Then(`^the site says the next check is due now$`, s.nextCheckDueNow)
	sc.Then(`^the site says the next check runs in (\d+) hours$`, s.nextCheckIn)
	sc.Then(`^the site names no next check$`, s.namesNoNextCheck)
	initializeAdminUpdates(sc, s)
}

// failEarlierUpdate records that an earlier update failed an hour ago at the step given.
func (s *operatorScenario) failEarlierUpdate(step string) error {
	return editState(s.env[installRootSetting], func(state *updater.State) {
		state.Failure = &updater.JobFailure{
			At: time.Now().Add(-time.Hour).UTC(), Step: updater.Step(step), Error: "an earlier update failed",
		}
	})
}

// failUpdateToThis records that an earlier update from the build from to this version failed an hour ago at step.
func (s *operatorScenario) failUpdateToThis(from, step string) error {
	return editState(s.env[installRootSetting], func(state *updater.State) {
		state.Failure = &updater.JobFailure{
			At: time.Now().Add(-time.Hour).UTC(), From: from, To: version.Version(), Step: updater.Step(step),
			Error: "an earlier update failed",
		}
	})
}

// checkedOnBuild stores what a check by the build running found on an update site offering this version.
func (s *operatorScenario) checkedOnBuild(ctx context.Context, running string) error {
	s.offerUpdate(version.Version())
	address, addressErr := update.ParseAddress(s.site.address)
	roots, rootsErr := update.ParseRoots(s.site.trustedRoots())
	pool, poolErr := pgxpool.New(ctx, s.env["GOPHENBERG_DATABASE_URL"])
	if err := errors.Join(addressErr, rootsErr, poolErr); err != nil {
		return err
	}
	defer pool.Close()
	check := update.Config{
		Address: address, Channel: "stable", Roots: roots, Client: update.NewClient(siteStartLimit),
		Site: update.Site{Running: running}, Now: time.Now,
	}
	result, err := update.Run(ctx, check, postgres.NewSettingStore(pool))
	if err != nil {
		return err
	}
	if result.Offer == nil || result.Offer.Version != version.Version() {
		return fmt.Errorf("the check by %s found %+v, want %s on offer", running, result.Offer, version.Version())
	}
	return nil
}

// ranLineFailing runs the command line and fails unless the command failed.
func (s *operatorScenario) ranLineFailing(line string) error {
	s.runLine(line)
	return s.exitsWith(gonsole.ExitFailed)
}

// keepBackups writes count database backups into the install root, the newest taken at newest, each other a day older.
func (s *operatorScenario) keepBackups(count int, newest string) error {
	at, err := time.Parse(time.RFC3339, newest)
	if err != nil {
		return err
	}
	folder := filepath.Join(s.env[installRootSetting], "backups")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		return err
	}
	for day := range count {
		name := version.Version() + "-" + at.AddDate(0, 0, -day).Format("20060102T150405Z") + ".dump"
		if err := os.WriteFile(filepath.Join(folder, name), []byte("a dump of the site database"), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// keepBackupLimit sets how many backups the install root keeps.
func (s *operatorScenario) keepBackupLimit(limit int) {
	s.env["GOPHENBERG_UPDATE_BACKUPS_KEEP"] = strconv.Itoa(limit)
}

// turnChecksOff stops the checks for a newer version the site makes on its own.
func (s *operatorScenario) turnChecksOff() {
	s.env["GOPHENBERG_UPDATE_CHECKS"] = "false"
}

// checkedAnHourAgo records in the cron store of the site that the check for a newer version ran an hour ago.
func (s *operatorScenario) checkedAnHourAgo(ctx context.Context) error {
	pool, err := pgxpool.New(ctx, s.env["GOPHENBERG_DATABASE_URL"])
	if err != nil {
		return err
	}
	defer pool.Close()
	store := postgres.NewCronStore(pool)
	ran := time.Now().Add(-time.Hour)
	lease, taken, err := store.Take(ctx, cron.Lease{
		Name: updateCheckJob, Token: "the check an hour ago", Taken: ran, Until: ran.Add(time.Minute),
	})
	if err != nil || !taken {
		return fmt.Errorf("taking the check job = %v, %v, want it taken", taken, err)
	}
	return store.Settle(ctx, lease, cron.Outcome{NextRun: ran.Add(12 * time.Hour), Finished: ran})
}

// askForUpdates serves the site, signs an administrator in and keeps what the site answers about its updates.
func (s *operatorScenario) askForUpdates() error {
	if err := s.serveSigned(); err != nil {
		return err
	}
	return errors.Join(s.readUpdates(), s.stopServing())
}

// statusFailure returns the failure the document update:status -json answers names, nil for none.
func (s *operatorScenario) statusFailure() (*answeredFailure, error) {
	answered := s.answer("", "update:status", "-json")
	var status struct {
		Update struct {
			Failure *answeredFailure `json:"failure"`
		} `json:"update"`
	}
	if err := json.Unmarshal([]byte(answered.Stdout), &status); err != nil {
		return nil, fmt.Errorf("update:status -json answered %q and %q, no JSON document: %w", answered.Stdout,
			answered.Stderr, err)
	}
	return status.Update.Failure, nil
}

// failedRecently fails unless failure names the update from this version that want names, within the last minute.
func failedRecently(failure *answeredFailure, want wantedFailure) error {
	if failure == nil {
		return fmt.Errorf("names no failure, want one at the step %s", want.step)
	}
	named := answeredFailure{From: version.Version(), To: want.to, Step: want.step}
	if !sameUpdateAndStep(*failure, named) || !strings.Contains(failure.Error, want.text) {
		return fmt.Errorf("names the failure %+v, want the update from %s to %s at the step %s saying %q", failure,
			named.From, want.to, want.step, want.text)
	}
	if age := time.Since(failure.At); age < 0 || age > siteStartLimit {
		return fmt.Errorf("names a failure at %v, want one within the last minute", failure.At)
	}
	return nil
}

// sameUpdateAndStep reports whether a and b name the same versions and the same step, whatever their time and error.
func sameUpdateAndStep(a, b answeredFailure) bool {
	a.At, a.Error, b.At, b.Error = time.Time{}, "", time.Time{}, ""
	return a == b
}

// statusNamesFailure fails unless update:status names, as one document and for people, which update failed and how.
func (s *operatorScenario) statusNamesFailure(release, step, text string) error {
	failure, err := s.statusFailure()
	if err != nil {
		return err
	}
	if err := failedRecently(failure, wantedFailure{to: release, step: step, text: text}); err != nil {
		return fmt.Errorf("update:status -json %w", err)
	}
	printed := s.answer("", "update:status").Stdout
	want := "last failure: from " + version.Version() + " to " + release + ", at the step " + step + " on " +
		failure.At.UTC().Format(time.RFC3339) + ": "
	if !strings.Contains(printed, want) || !strings.Contains(printed, text) {
		return fmt.Errorf("update:status printed %q, want it to carry %q and %q", printed, want, text)
	}
	return nil
}

// statusNamesNoFailure fails when update:status names a failure, as one document or for people.
func (s *operatorScenario) statusNamesNoFailure() error {
	failure, err := s.statusFailure()
	if err != nil {
		return err
	}
	if printed := s.answer("", "update:status").Stdout; failure != nil || strings.Contains(printed, "last failure") {
		return fmt.Errorf("update:status names the failure %+v and prints %q, want none", failure, printed)
	}
	return nil
}

// namesRunning fails unless the site names this version as the one that runs.
func (s *operatorScenario) namesRunning() error {
	if s.updates.Running != version.Version() {
		return fmt.Errorf("the site answered %s, want %s running", s.updates.body, version.Version())
	}
	return nil
}

// saysItCanUpdate fails unless the site says this install updates itself and names no reason it cannot.
func (s *operatorScenario) saysItCanUpdate() error {
	if !s.updates.CanUpdate || strings.Contains(s.updates.body, `"cannot_update"`) {
		return fmt.Errorf("the site answered %s, want it to update itself with no cannot_update", s.updates.body)
	}
	return nil
}

// saysItCannotUpdate fails unless the site says this install cannot update itself, naming reason.
func (s *operatorScenario) saysItCannotUpdate(reason string) error {
	if s.updates.CanUpdate || s.updates.CannotUpdate != reason {
		return fmt.Errorf("the site answered %s, want it unable to update itself, since %s", s.updates.body, reason)
	}
	return nil
}

// namesNothingRefused fails unless the site names an empty list of refused versions and no update.
func (s *operatorScenario) namesNothingRefused() error {
	listed := s.updates.Refused != nil && len(s.updates.Refused) == 0
	if !listed || !strings.Contains(s.updates.body, `"job":null`) {
		return fmt.Errorf("the site answered %s, want refused [] and job null", s.updates.body)
	}
	return nil
}

// namesNoOffer fails unless the site names no update on offer.
func (s *operatorScenario) namesNoOffer() error {
	if !strings.Contains(s.updates.body, `"offer":null`) {
		return fmt.Errorf("the site answered %s, want offer null", s.updates.body)
	}
	return nil
}

// namesNoFailure fails unless the site names no failure of an update.
func (s *operatorScenario) namesNoFailure() error {
	if s.updates.Job != nil && s.updates.Job.Failure != nil {
		return fmt.Errorf("the site answered %s, want no failure", s.updates.body)
	}
	return nil
}

// namesRefused fails unless the site names release among the refused versions.
func (s *operatorScenario) namesRefused(release string) error {
	if !slices.Contains(s.updates.Refused, release) {
		return fmt.Errorf("the site answered %s, want %s refused", s.updates.body, release)
	}
	return nil
}

// namesUpdate fails unless the site names the update from this version to release at step.
func (s *operatorScenario) namesUpdate(release, step string) error {
	want := answeredUpdate{Step: step, From: version.Version(), To: release}
	return s.siteNames(func() error {
		if s.updates.Job == nil || *s.updates.Job != want {
			return fmt.Errorf("the site answered %s, want the job %+v", s.updates.body, want)
		}
		return nil
	})
}

// namesFailure fails unless the site names the update to release that failed last, at step and saying text.
func (s *operatorScenario) namesFailure(release, step, text string) error {
	return s.siteNames(func() error {
		if s.updates.Job == nil {
			return fmt.Errorf("the site answered %s, want a job that failed", s.updates.body)
		}
		if err := failedRecently(s.updates.Job.Failure, wantedFailure{to: release, step: step, text: text}); err != nil {
			return fmt.Errorf("the site %w", err)
		}
		return nil
	})
}

// countsBackups fails unless the site counts kept backups out of limit, the newest taken at newest, none for "".
func (s *operatorScenario) countsBackups(kept, limit int, newest string) error {
	backups := s.updates.Backups
	if backups.Kept != kept || backups.Limit != limit || newestText(backups.Newest) != newest {
		return fmt.Errorf("the site answered %s, want %d backups kept out of %d, the newest at %q", s.updates.body,
			kept, limit, newest)
	}
	return nil
}

// newestText returns at written as RFC 3339 in UTC, empty for none.
func newestText(at *time.Time) string {
	if at == nil {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

// nextCheckIn fails unless the site says the next check runs the hours given from now, within a minute.
func (s *operatorScenario) nextCheckIn(hours int) error {
	return s.nextCheckAt(time.Now().Add(time.Duration(hours) * time.Hour))
}

// nextCheckDueNow fails unless the site says the next check runs now, within a minute.
func (s *operatorScenario) nextCheckDueNow() error {
	return s.nextCheckAt(time.Now())
}

// nextCheckAt fails unless the site names a next check within a minute of want.
func (s *operatorScenario) nextCheckAt(want time.Time) error {
	next := s.updates.NextCheck
	if next == nil || next.Sub(want).Abs() > siteStartLimit {
		return fmt.Errorf("the site answered %s, want the next check near %v", s.updates.body, want)
	}
	return nil
}

// namesNoNextCheck fails unless the site names no next check.
func (s *operatorScenario) namesNoNextCheck() error {
	if !strings.Contains(s.updates.body, `"next_check":null`) {
		return fmt.Errorf("the site answered %s, want next_check null", s.updates.body)
	}
	return nil
}
