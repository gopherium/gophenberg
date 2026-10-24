// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"crypto/ecdsa"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// updateCheckJob names the core job that checks the update site for a newer Gophenberg.
const updateCheckJob = "core/update-check"

// channelPattern is what a channel name holds: a lowercase letter, then lowercase letters, digits and hyphens.
var channelPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// updateSettings carries where and how often a site checks for a newer Gophenberg, and what the Updates screen reads.
type updateSettings struct {
	address   *url.URL
	channel   string
	roots     []*ecdsa.PublicKey
	checks    bool
	every     time.Duration
	screenAge time.Duration
	timeout   time.Duration
	screen    server.UpdatesScreen
}

// siteUpdates runs the site's checks for a newer Gophenberg, answers what they stored and starts an update.
type siteUpdates struct {
	settings updateSettings
	store    update.Settings
	major    func(context.Context) (int, error)
	install  updater.Settings
	stamp    string
	slots    cron.Store
	admin    *adminUpdates
}

// updateSettingsFrom reads the update settings, refusing plain HTTP off loopback and roots beside the official site.
func updateSettingsFrom(env gonsole.Env) (updateSettings, error) {
	held := updateSettings{channel: valueOr(env, "UPDATE_CHANNEL", "stable")}
	if !channelPattern.MatchString(held.channel) {
		return updateSettings{}, fmt.Errorf("%s: must be lowercase letters, digits and hyphens, got %q",
			env.Key("UPDATE_CHANNEL"), held.channel)
	}
	address, err := gonsole.Parse(env, "UPDATE_URL", officialAddress(), update.ParseAddress)
	if err != nil {
		return updateSettings{}, err
	}
	held.address = address
	if held.roots, err = rootsFrom(env, address); err != nil {
		return updateSettings{}, err
	}
	return held, updateTimingsFrom(env, &held)
}

// officialAddress returns the address of the official update site.
func officialAddress() *url.URL {
	address, _ := update.ParseAddress(update.DefaultAddress)
	return address
}

// rootsFrom returns the built-in roots and the ones GOPHENBERG_TRUSTED_KEYS adds beside another update site.
func rootsFrom(env gonsole.Env, address *url.URL) ([]*ecdsa.PublicKey, error) {
	trusted, err := update.ParseRoots(env.Value("TRUSTED_KEYS"))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", env.Key("TRUSTED_KEYS"), err)
	}
	if len(trusted) > 0 && address.Hostname() == officialAddress().Hostname() {
		return nil, fmt.Errorf("%s: counts only beside a %s other than the official update site",
			env.Key("TRUSTED_KEYS"), env.Key("UPDATE_URL"))
	}
	return append(update.Roots(), trusted...), nil
}

// updateTimingsFrom reads the interval, the screen timings, the timeout and whether the site checks on its own.
func updateTimingsFrom(env gonsole.Env, held *updateSettings) error {
	for _, asked := range []struct {
		name     string
		fallback time.Duration
		into     *time.Duration
		bounds   []gonsole.Bound
	}{
		{"UPDATE_CHECK_INTERVAL", 12 * time.Hour, &held.every, nil},
		{"UPDATES_SCREEN_CHECK_AGE", time.Minute, &held.screenAge, nil},
		{"UPDATE_TIMEOUT", 30 * time.Second, &held.timeout, nil},
		{"UPDATES_SCREEN_PROGRESS_INTERVAL", 2 * time.Second, &held.screen.ProgressInterval, browserTimer},
		{"UPDATES_SCREEN_RESTART_WAIT", 5 * time.Minute, &held.screen.RestartWait, browserTimer},
	} {
		stood, err := env.Duration(asked.name, asked.fallback, asked.bounds...)
		if err != nil {
			return err
		}
		*asked.into = stood
	}
	checks, err := env.Flag("UPDATE_CHECKS", true)
	held.checks = checks
	return err
}

// config returns the check of the update site for this binary on a PostgreSQL server of major, 0 on SQLite.
func (s updateSettings) config(major int) update.Config {
	return update.Config{
		Address: s.address, Channel: s.channel, Roots: s.roots, Client: update.NewClient(s.timeout),
		Site: update.Site{Running: version.Version(), ServerMajor: major}, Now: time.Now,
	}
}

// Result returns what the checks stored.
func (u siteUpdates) Result(ctx context.Context) (update.Result, error) {
	return update.ResultOf(ctx, u.store)
}

// Check runs a check now and stores what it found.
func (u siteUpdates) Check(ctx context.Context) (update.Result, error) {
	major, err := u.major(ctx)
	if err != nil {
		return update.Result{}, err
	}
	return update.Run(ctx, u.settings.config(major), u.store)
}

// Start runs the checks of the update an administrator asked for, then the rest of it on its own.
func (u siteUpdates) Start(ctx context.Context, asked server.UpdateStart) error {
	return u.admin.start(ctx, asked)
}

// Overview returns what the admin shows: what the checks stored, this binary, its install and the next check.
func (u siteUpdates) Overview(ctx context.Context) (server.UpdatesOverview, error) {
	result, resultErr := u.Result(ctx)
	install, installErr := updater.ReadOverview(u.install)
	next, nextErr := u.nextCheck(ctx)
	if err := errors.Join(resultErr, installErr, nextErr); err != nil {
		return server.UpdatesOverview{}, err
	}
	reason := updater.CannotUpdate(u.install, u.stamp)
	running := version.Version()
	return server.UpdatesOverview{
		Result: withoutReachedOffer(result, running), Overview: withoutReachedFailure(install, running),
		Running: running, CanUpdate: reason == "", CannotUpdate: reason, NextCheck: next,
	}, nil
}

// withoutReachedOffer returns result without its offer once running is that version or a newer one, as after an update.
func withoutReachedOffer(result update.Result, running string) update.Result {
	if result.Offer != nil && !version.Before(running, result.Offer.Version) {
		result.Offer = nil
	}
	return result
}

// withoutReachedFailure returns install without the failure of an update to running or an older version.
func withoutReachedFailure(install updater.Overview, running string) updater.Overview {
	if !failedOnReachedVersion(install.Job, running) {
		return install
	}
	cleared := *install.Job
	cleared.Failure = nil
	install.Job = &cleared
	if cleared.Step == "" {
		install.Job = nil
	}
	return install
}

// failedOnReachedVersion reports whether job holds the failure of an update to running or an older version.
func failedOnReachedVersion(job *updater.LastUpdate, running string) bool {
	if job == nil || job.Failure == nil {
		return false
	}
	return !version.Before(running, job.Failure.To)
}

// nextCheck returns when the check job runs next, now while it is due, and none while the checks are off.
func (u siteUpdates) nextCheck(ctx context.Context) (*time.Time, error) {
	if !u.settings.checks {
		return nil, nil
	}
	next, err := cron.NextRun(ctx, u.slots, updateCheckJob)
	if now := time.Now(); next.Before(now) {
		next = now
	}
	next = next.UTC()
	return &next, err
}

// siteUpdatesOf returns the checks and the install of the site compose built, run by a binary of stamp.
func siteUpdatesOf(settings runConfig, built site, stamp string) siteUpdates {
	return siteUpdates{
		settings: settings.updates, store: built.settings, major: built.major, install: settings.install,
		stamp: stamp, slots: built.cronStore,
	}
}

// updateJobs returns the job that checks for a newer Gophenberg every interval, none while the checks are off.
func updateJobs(settings updateSettings, updates siteUpdates) []cron.Job {
	if !settings.checks {
		return nil
	}
	return []cron.Job{{Name: updateCheckJob, Every: settings.every, Run: func(ctx context.Context, _ time.Time) error {
		_, err := updates.Check(ctx)
		return err
	}}}
}

// updateCheckCommand returns the command that checks the update site for a newer Gophenberg now.
func updateCheckCommand() gonsole.Command {
	return gonsole.Command{
		Name: "update:check", Summary: "check the update site for a newer Gophenberg and keep what it found",
		JSON: true, Migrates: true, Run: runUpdateCheck,
	}
}

// runUpdateCheck checks the update site now and prints what it found, one line for people or one document.
func runUpdateCheck(ctx context.Context, call gonsole.Call) error {
	settings, err := updateSettingsFrom(call.Env)
	if err != nil {
		return err
	}
	updates, release := commandUpdates(ctx, call, settings)
	defer release()
	result, err := updates.Check(ctx)
	if err != nil {
		return err
	}
	if call.JSON {
		return call.Encode(result)
	}
	_, err = fmt.Fprintln(call.Stdout, updateSummary(result))
	return err
}

// commandUpdates returns the checks over the database the command migrated, and what releases its handle.
func commandUpdates(ctx context.Context, call gonsole.Call, settings updateSettings) (siteUpdates, func()) {
	address, _ := call.DatabaseURL()
	if onSQLite(address) {
		db, _ := call.DB(ctx)
		return siteUpdates{settings: settings, store: sqlite.NewSettingStore(db), major: noMajor}, func() {}
	}
	handle, _ := openPostgres(call.Env, address)
	updates := siteUpdates{settings: settings, store: postgres.NewSettingStore(handle.Pool), major: majorOf(handle.DB)}
	return updates, func() { _ = handle.Close() }
}

// majorOf returns the read of the PostgreSQL major the server db connects to runs.
func majorOf(db *sql.DB) func(context.Context) (int, error) {
	return func(ctx context.Context) (int, error) {
		return postgres.ServerMajor(ctx, db)
	}
}

// noMajor answers the PostgreSQL major of a site on SQLite, which runs none.
func noMajor(context.Context) (int, error) {
	return 0, nil
}

// updateSummary returns the line update:check prints for people.
func updateSummary(result update.Result) string {
	if result.Offer == nil {
		return "Gophenberg " + version.Version() + " is the newest version the update site offers"
	}
	line := "Gophenberg " + result.Offer.Version + " is available, this is " + version.Version()
	if result.Offer.NeedsPostgreSQL {
		line += fmt.Sprintf(", it needs PostgreSQL %d or newer", result.Offer.PostgreSQL)
	}
	if result.Offer.LongJump {
		line += ", and going back to this version after it needs the backup"
	}
	return line
}
