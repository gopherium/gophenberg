// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/gopherium/framework/gonsole"
	"github.com/gopherium/framework/pluginkit"
	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/server"
)

// sessionSweepJob names the core job that sweeps expired sessions, expired account links and the accounts they leave.
const sessionSweepJob = "core/session-sweep"

// minSecretLength is the fewest characters a cron secret may hold.
const minSecretLength = 32

// cronSettings carries the settings of the cron jobs.
type cronSettings struct {
	secret     string
	onVisits   bool
	pace       time.Duration
	timeLimit  time.Duration
	lease      time.Duration
	stopGrace  time.Duration
	sweepEvery time.Duration
}

// cronSettingsFrom reads the cron job settings, refusing a short secret, a site no trigger reaches and a short lease.
func cronSettingsFrom(env gonsole.Env) (cronSettings, error) {
	var held cronSettings
	if err := cronTimingsFrom(env, &held); err != nil {
		return cronSettings{}, err
	}
	if err := cronTriggersFrom(env, &held); err != nil {
		return cronSettings{}, err
	}
	return held, nil
}

// cronTimingsFrom reads the five cron job durations, each above zero, and refuses a lease not above the time limit.
func cronTimingsFrom(env gonsole.Env, held *cronSettings) error {
	for _, asked := range []struct {
		name     string
		fallback time.Duration
		into     *time.Duration
	}{
		{"CRON_VISIT_PACE", time.Minute, &held.pace},
		{"CRON_TIME_LIMIT", 30 * time.Second, &held.timeLimit},
		{"CRON_LEASE", time.Minute, &held.lease},
		{"CRON_STOP_GRACE", 2 * time.Second, &held.stopGrace},
		{"SESSION_SWEEP_INTERVAL", time.Hour, &held.sweepEvery},
	} {
		stood, err := env.Duration(asked.name, asked.fallback)
		if err != nil {
			return err
		}
		*asked.into = stood
	}
	if held.lease <= held.timeLimit {
		return fmt.Errorf("%s: must stand above %s, got %v",
			env.Key("CRON_LEASE"), env.Key("CRON_TIME_LIMIT"), held.lease)
	}
	return nil
}

// cronTriggersFrom reads the cron secret and the visit switch, refusing a short secret by its length alone.
func cronTriggersFrom(env gonsole.Env, held *cronSettings) error {
	held.secret = env.Value("CRON_SECRET")
	if length := utf8.RuneCountInString(held.secret); held.secret != "" && length < minSecretLength {
		return fmt.Errorf("%s: must be at least %d characters, got %d",
			env.Key("CRON_SECRET"), minSecretLength, length)
	}
	onVisits, err := env.Flag("CRON_ON_VISITS", true)
	if err != nil {
		return err
	}
	if !onVisits && held.secret == "" {
		return fmt.Errorf("%s: must stay true while %s is empty, or no cron job ever runs",
			env.Key("CRON_ON_VISITS"), env.Key("CRON_SECRET"))
	}
	held.onVisits = onVisits
	return nil
}

// siteCron holds the site's cron runs and the grace they get to stop.
type siteCron struct {
	runs      *server.CronRuns
	stopGrace time.Duration
}

// cronOf returns the site's cron jobs over the cron store the site holds, the session sweep the only core job.
func cronOf(settings cronSettings, built site, logger *slog.Logger) siteCron {
	runner := cron.New(cron.Config{
		Jobs:      []cron.Job{sessionSweep(built.users, settings.sweepEvery, logger)},
		Store:     built.cronStore,
		TimeLimit: settings.timeLimit,
		Lease:     settings.lease,
		Logger:    logger,
	})
	return siteCron{runs: server.NewCronRuns(runner, settings.pace, time.Now), stopGrace: settings.stopGrace}
}

// sessionSweep returns the job that deletes expired sessions and account links at the time the runner hands it.
func sessionSweep(store authkit.SessionReaper, every time.Duration, logger *slog.Logger) cron.Job {
	return cron.Job{Name: sessionSweepJob, Every: every, Run: func(ctx context.Context, now time.Time) error {
		sessions, tokens, err := authkit.Sweep(ctx, store, now)
		logSwept(logger, sessions, tokens)
		return err
	}}
}

// logSwept logs how many sessions and tokens a sweep removed in the reaper's words, staying quiet about none.
func logSwept(logger *slog.Logger, sessions, tokens int64) {
	if sessions > 0 {
		logger.Info("reaped expired sessions", "count", sessions)
	}
	if tokens > 0 {
		logger.Info("reaped expired tokens", "count", tokens)
	}
}

// serverOf returns the HTTP server for handler, cutting the cron runs as soon as it begins to shut down.
func (c siteCron) serverOf(settings runConfig, handler http.Handler) *http.Server {
	srv := httpServerFrom(settings, handler)
	srv.RegisterOnShutdown(c.runs.Cut)
	return srv
}

// stop returns the server's stop: the cron runs within their own grace, then the plugins within theirs.
func (c siteCron) stop(host *pluginkit.Host, grace time.Duration) func(context.Context) error {
	return func(ctx context.Context) error {
		closing, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.stopGrace)
		defer cancel()
		closed := c.runs.Close(closing)
		return errors.Join(closed, stopPlugins(ctx, host, grace))
	}
}
