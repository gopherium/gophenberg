// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"maps"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/cron/crontest"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// errMajor is the failure a stubbed read of the server's major answers with.
var errMajor = errors.New("major failure")

// heldSettings is a site setting store in memory.
type heldSettings struct {
	mu     sync.Mutex
	values map[string]string
}

// Lookup returns the value stored under key and whether the key is set at all.
func (h *heldSettings) Lookup(_ context.Context, key string) (string, bool, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	value, found := h.values[key]
	return value, found, nil
}

// Save stores every given value.
func (h *heldSettings) Save(_ context.Context, values map[string]string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	maps.Copy(h.values, values)
	return nil
}

// Update stores what change makes of the values held under keys, as one write.
func (h *heldSettings) Update(
	_ context.Context, keys []string, change func(held map[string]string) (map[string]string, error),
) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	held := map[string]string{}
	for _, key := range keys {
		if value, found := h.values[key]; found {
			held[key] = value
		}
	}
	values, err := change(held)
	maps.Copy(h.values, values)
	return err
}

// updatesAt returns the update settings for the site, and the store a check keeps its state in.
func updatesAt(t *testing.T, site *updateSite) (updateSettings, *heldSettings) {
	t.Helper()
	settings, err := updateSettingsFrom(settingsEnv(testkit.Getenv(map[string]string{
		"GOPHENBERG_UPDATE_URL": site.address, "GOPHENBERG_TRUSTED_KEYS": site.trustedRoots(),
	})))
	if err != nil {
		t.Fatalf("updateSettingsFrom() error = %v, want nil", err)
	}
	return settings, &heldSettings{values: map[string]string{}}
}

// offeringSite returns an update site whose stable index offers 99.0.0, signed by both roots.
func offeringSite(t *testing.T) *updateSite {
	t.Helper()
	return newOfferingSite(t, "99.0.0", "99.0.0")
}

// major returns a read of the server's major that answers major and err.
func major(major int, err error) func(context.Context) (int, error) {
	return func(context.Context) (int, error) {
		return major, err
	}
}

func TestUpdateSettingsFallBackToTheOfficialSite(t *testing.T) {
	t.Parallel()

	settings, err := updateSettingsFrom(settingsEnv(testkit.Getenv(nil)))

	if err != nil || settings.address.String() != update.DefaultAddress || settings.channel != "stable" ||
		len(settings.roots) != 2 || !settings.checks {
		t.Fatalf("updateSettingsFrom() = %+v, %v, want the official site, stable, its two roots and checks on",
			settings, err)
	}
	if settings.every != 12*time.Hour || settings.screenAge != time.Minute || settings.timeout != 30*time.Second {
		t.Errorf("updateSettingsFrom() times = %v, %v and %v, want 12h, 1m and 30s", settings.every,
			settings.screenAge, settings.timeout)
	}
}

func TestUpdateSettingsTrustMoreRootsOnlyBesideAnotherSite(t *testing.T) {
	t.Parallel()

	site := newUpdateSite(t)
	settings, err := updateSettingsFrom(settingsEnv(testkit.Getenv(map[string]string{
		"GOPHENBERG_UPDATE_URL": "https://mirror.example.com", "GOPHENBERG_TRUSTED_KEYS": site.trustedRoots(),
		"GOPHENBERG_UPDATE_CHANNEL": "beta", "GOPHENBERG_UPDATE_CHECKS": "false",
	})))

	if err != nil || len(settings.roots) != 4 || settings.channel != "beta" || settings.checks {
		t.Errorf("updateSettingsFrom() = %+v, %v, want the built-in roots and the two trusted ones, beta, checks off",
			settings, err)
	}
}

func TestUpdateSettingsRefuseWhatTheyCannotUse(t *testing.T) {
	t.Parallel()

	site := newUpdateSite(t)
	for name, tc := range map[string]struct {
		env  map[string]string
		want string
	}{
		"plain HTTP to another host": {map[string]string{"GOPHENBERG_UPDATE_URL": "http://mirror.example.com"},
			"GOPHENBERG_UPDATE_URL"},
		"trusted roots beside the official site": {map[string]string{"GOPHENBERG_TRUSTED_KEYS": site.trustedRoots()},
			"GOPHENBERG_TRUSTED_KEYS"},
		"a trusted root that is no P-256 key": {map[string]string{
			"GOPHENBERG_UPDATE_URL": "https://mirror.example.com", "GOPHENBERG_TRUSTED_KEYS": "z6Mk"},
			"GOPHENBERG_TRUSTED_KEYS"},
		"a channel that climbs out": {map[string]string{"GOPHENBERG_UPDATE_CHANNEL": "../beta"},
			"GOPHENBERG_UPDATE_CHANNEL"},
		"a channel in capitals":     {map[string]string{"GOPHENBERG_UPDATE_CHANNEL": "Stable"}, "GOPHENBERG_UPDATE_CHANNEL"},
		"an interval of zero":       {map[string]string{"GOPHENBERG_UPDATE_CHECK_INTERVAL": "0s"}, "UPDATE_CHECK_INTERVAL"},
		"a screen age in words":     {map[string]string{"GOPHENBERG_UPDATES_SCREEN_CHECK_AGE": "soon"}, "SCREEN_CHECK_AGE"},
		"a negative timeout":        {map[string]string{"GOPHENBERG_UPDATE_TIMEOUT": "-1s"}, "GOPHENBERG_UPDATE_TIMEOUT"},
		"checks neither on nor off": {map[string]string{"GOPHENBERG_UPDATE_CHECKS": "maybe"}, "GOPHENBERG_UPDATE_CHECKS"},
		"a progress interval of zero": {map[string]string{"GOPHENBERG_UPDATES_SCREEN_PROGRESS_INTERVAL": "0s"},
			"GOPHENBERG_UPDATES_SCREEN_PROGRESS_INTERVAL"},
		"a progress interval under a millisecond": {
			map[string]string{"GOPHENBERG_UPDATES_SCREEN_PROGRESS_INTERVAL": "1500us"}, "whole number of milliseconds"},
		"a restart wait no browser timer holds": {
			map[string]string{"GOPHENBERG_UPDATES_SCREEN_RESTART_WAIT": "2147483648ms"},
			"GOPHENBERG_UPDATES_SCREEN_RESTART_WAIT"},
	} {
		_, err := updateSettingsFrom(settingsEnv(testkit.Getenv(tc.env)))
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("updateSettingsFrom() with %s = %v, want a refusal naming %s", name, err, tc.want)
		}
	}
}

func TestUpdateSettingsTellTheUpdatesScreenHowOftenToAskAndHowLongToWait(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		env  map[string]string
		want server.UpdatesScreen
	}{
		"by default": {nil, server.UpdatesScreen{ProgressInterval: 2 * time.Second, RestartWait: 5 * time.Minute}},
		"once set": {map[string]string{
			"GOPHENBERG_UPDATES_SCREEN_PROGRESS_INTERVAL": "500ms", "GOPHENBERG_UPDATES_SCREEN_RESTART_WAIT": "90s",
		}, server.UpdatesScreen{ProgressInterval: 500 * time.Millisecond, RestartWait: 90 * time.Second}},
	} {
		settings, err := updateSettingsFrom(settingsEnv(testkit.Getenv(tc.env)))

		if err != nil || settings.screen != tc.want {
			t.Errorf("updateSettingsFrom() %s tells the Updates screen %+v, %v, want %+v", name, settings.screen, err,
				tc.want)
		}
	}
}

func TestLoadRunConfigRefusesUpdateSettingsItCannotUse(t *testing.T) {
	t.Parallel()

	_, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_UPDATE_CHANNEL": "../beta",
	}))

	if err == nil || !strings.Contains(err.Error(), "GOPHENBERG_UPDATE_CHANNEL") {
		t.Errorf("loadRunConfig() = %v, want the update channel refused", err)
	}
}

func TestUpdateJobsHoldTheCheckOnlyWhileChecksAreOn(t *testing.T) {
	t.Parallel()

	updates := siteUpdates{store: &heldSettings{values: map[string]string{}}, major: major(18, nil)}
	on := updateJobs(updateSettings{checks: true, every: time.Hour}, updates)
	off := updateJobs(updateSettings{every: time.Hour}, updates)

	if len(on) != 1 || on[0].Name != updateCheckJob || on[0].Every != time.Hour || len(off) != 0 {
		t.Errorf("updateJobs() = %+v on and %+v off, want the hourly check alone, then nothing", on, off)
	}
}

func TestTheUpdateJobStoresWhatTheSiteOffers(t *testing.T) {
	t.Parallel()

	settings, store := updatesAt(t, offeringSite(t))
	settings.checks, settings.every = true, time.Hour
	job := updateJobs(settings, siteUpdates{settings: settings, store: store, major: major(18, nil)})[0]

	if err := job.Run(t.Context(), time.Now()); err != nil {
		t.Fatalf("the update job = %v, want nil", err)
	}
	result, err := update.ResultOf(t.Context(), store)
	if err != nil || result.Offer == nil || result.Offer.Version != "99.0.0" {
		t.Errorf("the stored result = %+v, %v, want 99.0.0 offered", result, err)
	}
}

func TestSiteUpdatesRunAndAnswerTheChecks(t *testing.T) {
	t.Parallel()

	settings, store := updatesAt(t, offeringSite(t))
	updates := siteUpdates{settings: settings, store: store, major: major(14, nil)}

	checked, err := updates.Check(t.Context())
	if err != nil || checked.Offer == nil || !checked.Offer.NeedsPostgreSQL {
		t.Fatalf("Check() = %+v, %v, want 99.0.0 offered, needing a newer PostgreSQL than 14", checked, err)
	}
	stored, err := updates.Result(t.Context())
	if err != nil || stored.Offer == nil || stored.Offer.Version != "99.0.0" {
		t.Errorf("Result() = %+v, %v, want the result Check stored", stored, err)
	}
}

func TestSiteUpdatesReportAMajorTheyCannotRead(t *testing.T) {
	t.Parallel()

	settings, store := updatesAt(t, offeringSite(t))
	updates := siteUpdates{settings: settings, store: store, major: major(0, errMajor)}

	if result, err := updates.Check(t.Context()); !errors.Is(err, errMajor) || result.FailedAt != nil {
		t.Errorf("Check() = %+v, %v, want the failure to read the major and nothing stored", result, err)
	}
}

// errSettings is the failure a site setting store that cannot be read answers with.
var errSettings = errors.New("setting store failure")

// unreadSettings is a site setting store that cannot be read.
type unreadSettings struct {
	heldSettings
}

// Lookup answers errSettings.
func (*unreadSettings) Lookup(context.Context, string) (string, bool, error) {
	return "", false, errSettings
}

// unreadSlots is a cron store whose slots cannot be read.
type unreadSlots struct {
	cron.Store
}

// Slots answers errSettings.
func (unreadSlots) Slots(context.Context) ([]cron.Slot, error) {
	return nil, errSettings
}

// installAt returns the settings of a server install whose install root is root, empty for none.
func installAt(t *testing.T, root string) updater.Settings {
	t.Helper()
	settings, err := updater.SettingsFrom(settingsEnv(testkit.Getenv(map[string]string{installRootSetting: root})))
	if err != nil {
		t.Fatalf("updater.SettingsFrom() = %v, want nil", err)
	}
	return settings
}

// slotsAt returns a cron store holding the check job, due next at next.
func slotsAt(t *testing.T, next time.Time) cron.Store {
	t.Helper()
	store := crontest.NewMemoryStore()
	ran := next.Add(-time.Hour)
	lease, taken, err := store.Take(t.Context(), cron.Lease{
		Name: updateCheckJob, Token: "the last check", Taken: ran, Until: ran.Add(time.Minute),
	})
	if err != nil || !taken {
		t.Fatalf("Take() = %v, %v, want the check job taken", taken, err)
	}
	if err := store.Settle(t.Context(), lease, cron.Outcome{NextRun: next, Finished: ran}); err != nil {
		t.Fatalf("Settle() = %v, want nil", err)
	}
	return store
}

// officialUpdates returns the updates of an official build on a new install root, checking every 12 hours.
func officialUpdates(t *testing.T) siteUpdates {
	t.Helper()
	settings, store := updatesAt(t, offeringSite(t))
	settings.checks, settings.every = true, 12*time.Hour
	return siteUpdates{
		settings: settings, store: store, major: major(18, nil), install: installAt(t, t.TempDir()),
		stamp: officialStamp, slots: crontest.NewMemoryStore(),
	}
}

func TestSiteUpdatesOverviewNamesWhatTheChecksStoredBesideThisBinaryAndItsInstall(t *testing.T) {
	t.Parallel()

	updates := officialUpdates(t)
	next := time.Now().Add(11 * time.Hour).UTC().Truncate(time.Second)
	updates.slots = slotsAt(t, next)
	if _, err := updates.Check(t.Context()); err != nil {
		t.Fatalf("Check() = %v, want nil", err)
	}

	overview, err := updates.Overview(t.Context())

	if err != nil || overview.Offer == nil || overview.Offer.Version != "99.0.0" {
		t.Fatalf("Overview() = %+v, %v, want 99.0.0 offered", overview, err)
	}
	want := updater.Overview{Refused: []string{}, Backups: updater.BackupCount{Limit: 3}}
	if overview.Running != version.Version() || !overview.CanUpdate || overview.CannotUpdate != "" ||
		!reflect.DeepEqual(overview.Overview, want) {
		t.Errorf("Overview() = %+v, want %s running, able to update itself and %+v", overview, version.Version(), want)
	}
	if overview.NextCheck == nil || !overview.NextCheck.Equal(next) {
		t.Errorf("Overview() names the next check %v, want %v", overview.NextCheck, next)
	}
}

func TestSiteUpdatesOverviewSaysWhyAnInstallCannotUpdateItself(t *testing.T) {
	t.Parallel()

	for want, change := range map[updater.Reason]func(*siteUpdates){
		updater.CustomBuild:   func(updates *siteUpdates) { updates.stamp = "" },
		updater.NoInstallRoot: func(updates *siteUpdates) { updates.install = installAt(t, "") },
	} {
		updates := officialUpdates(t)
		change(&updates)

		overview, err := updates.Overview(t.Context())

		if err != nil || overview.CanUpdate || overview.CannotUpdate != want {
			t.Errorf("Overview() = %+v, %v, want it unable to update itself, since %s", overview, err, want)
		}
	}
}

func TestSiteUpdatesOverviewNamesTheNextCheckDueNowOrNoneWhileTheChecksAreOff(t *testing.T) {
	t.Parallel()

	past := slotsAt(t, time.Now().Add(-time.Hour))
	for name, tc := range map[string]struct {
		change func(*siteUpdates)
		due    bool
	}{
		"a check job that never ran": {change: func(*siteUpdates) {}, due: true},
		"a check job past its time":  {change: func(u *siteUpdates) { u.slots = past }, due: true},
		"checks turned off":          {change: func(u *siteUpdates) { u.settings.checks = false }},
		"checks off, slots unread":   {change: func(u *siteUpdates) { u.settings.checks, u.slots = false, unreadSlots{} }},
	} {
		updates := officialUpdates(t)
		tc.change(&updates)
		before := time.Now()

		overview, err := updates.Overview(t.Context())

		if err != nil || !dueNowOrNone(overview.NextCheck, tc.due, before) {
			t.Errorf("Overview() with %s names the next check %v, %v, want it due now %v", name, overview.NextCheck,
				err, tc.due)
		}
	}
}

// dueNowOrNone reports whether next names a time from before until now when due, and no time at all otherwise.
func dueNowOrNone(next *time.Time, due bool, before time.Time) bool {
	if !due {
		return next == nil
	}
	if next == nil || next.Before(before) {
		return false
	}
	return !next.After(time.Now())
}

func TestSiteUpdatesOverviewReportsWhatItCannotRead(t *testing.T) {
	t.Parallel()

	for name, change := range map[string]func(*testing.T, *siteUpdates){
		"a setting store it cannot read": func(_ *testing.T, u *siteUpdates) { u.store = &unreadSettings{} },
		"a cron store it cannot read":    func(_ *testing.T, u *siteUpdates) { u.slots = unreadSlots{} },
		"a state file it cannot read": func(t *testing.T, u *siteUpdates) {
			layout := updater.Layout{Root: u.install.Root}
			if err := errors.Join(layout.WriteState(updater.State{}), os.WriteFile(layout.StateFile(), []byte("{"),
				0o644)); err != nil {
				t.Fatalf("writing a state file it cannot read: %v", err)
			}
		},
	} {
		updates := officialUpdates(t)
		change(t, &updates)

		if _, err := updates.Overview(t.Context()); err == nil {
			t.Errorf("Overview() over %s = nil, want an error", name)
		}
	}
}

func TestSiteUpdatesOverviewDropsAnOfferTheRunningVersionReached(t *testing.T) {
	t.Parallel()

	for running, kept := range map[string]bool{"0.20.0": true, "0.20.1": false, "0.20.2": false} {
		got := withoutReachedOffer(update.Result{Offer: &update.Found{Version: "0.20.1"}}, running)

		if (got.Offer != nil) != kept {
			t.Errorf("withoutReachedOffer() on %s = %+v, want the offer of 0.20.1 kept %v", running, got.Offer, kept)
		}
	}
}

func TestSiteUpdatesOverviewDropsTheFailureOfAnUpdateTheRunningVersionReached(t *testing.T) {
	t.Parallel()

	failure := &updater.JobFailure{From: "0.20.0", To: "0.20.1", Step: updater.StepDownload}
	committed := &updater.LastUpdate{Step: updater.StepCommitted, From: "0.19.0", To: "0.20.0", Failure: failure}
	cleared := &updater.LastUpdate{Step: updater.StepCommitted, From: "0.19.0", To: "0.20.0"}
	for name, tc := range map[string]struct {
		job     *updater.LastUpdate
		running string
		want    *updater.LastUpdate
	}{
		"a failure of an update to a newer version":     {job: committed, running: "0.20.0", want: committed},
		"a failure of an update to the running version": {job: committed, running: "0.20.1", want: cleared},
		"a failure alone, to the running version":       {job: &updater.LastUpdate{Failure: failure}, running: "0.20.1"},
		"no update at all":                              {running: "0.20.1"},
	} {
		got := withoutReachedFailure(updater.Overview{Job: tc.job}, tc.running)

		if !reflect.DeepEqual(got.Job, tc.want) {
			t.Errorf("withoutReachedFailure() with %s = %+v, want %+v", name, got.Job, tc.want)
		}
	}
}

func TestUpdateSummaryNamesTheVersionOnOffer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		result update.Result
		want   string
	}{
		{update.Result{Offer: &update.Found{Version: "99.0.0"}},
			"Gophenberg 99.0.0 is available, this is " + version.Version()},
		{update.Result{Offer: &update.Found{Version: "99.0.0", LongJump: true, NeedsPostgreSQL: true, PostgreSQL: 17}},
			"Gophenberg 99.0.0 is available, this is " + version.Version() +
				", it needs PostgreSQL 17 or newer, and going back to this version after it needs the backup"},
		{update.Result{}, "Gophenberg " + version.Version() + " is the newest version the update site offers"},
	} {
		if got := updateSummary(tc.result); got != tc.want {
			t.Errorf("updateSummary(%+v) = %q, want %q", tc.result.Offer, got, tc.want)
		}
	}
}

func TestUpdateCheckPrintsOneLineForPeople(t *testing.T) {
	t.Parallel()

	site := offeringSite(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": emptySQLiteAddress(t), "GOPHENBERG_UPDATE_URL": site.address,
		"GOPHENBERG_TRUSTED_KEYS": site.trustedRoots(),
	}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:check")

	if got.Code != 0 || !strings.Contains(got.Stdout, "Gophenberg 99.0.0 is available") {
		t.Errorf("update:check = %d, %q, %q, want 0 and the version on offer", got.Code, got.Stdout, got.Stderr)
	}
}

func TestUpdateCheckRefusesSettingsItCannotUse(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_URL": emptySQLiteAddress(t), "GOPHENBERG_UPDATE_CHANNEL": "../x"}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:check")

	if got.Code == 0 || !strings.Contains(got.Stderr, "GOPHENBERG_UPDATE_CHANNEL") {
		t.Errorf("update:check = %d, %q, want a refusal naming the channel", got.Code, got.Stderr)
	}
}

func TestUpdateCheckReportsADatabaseItCannotReach(t *testing.T) {
	t.Parallel()

	env := map[string]string{"GOPHENBERG_DATABASE_URL": "postgres://postgres:gophenberg@localhost:9/postgres?pool_x=1"}

	got := testkit.Run(t, Program(testkit.Getenv(env), noPlugins), "", "update:check")

	if got.Code == 0 {
		t.Errorf("update:check on an address it refuses = %d, %q, want a failure", got.Code, got.Stderr)
	}
}
