// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/authkit"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/sdk"
)

// longSecret is a synthetic cron secret of the shortest length the site takes.
const longSecret = "0123456789abcdef0123456789abcdef"

// cronSettingsWith reads the cron job settings from a site whose environment adds extra.
func cronSettingsWith(extra map[string]string) (cronSettings, error) {
	env := map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL}
	for key, value := range extra {
		env[key] = value
	}
	settings, err := loadRunConfig(testkit.Getenv(env))
	return settings.cron, err
}

func TestCronSettingsTakeTheirDefaults(t *testing.T) {
	t.Parallel()

	got, err := cronSettingsWith(nil)
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	want := cronSettings{
		onVisits: true, pace: time.Minute, timeLimit: 30 * time.Second, lease: time.Minute,
		stopGrace: 2 * time.Second, sweepEvery: time.Hour,
	}
	if got != want {
		t.Errorf("cron job settings = %+v, want %+v", got, want)
	}
}

func TestCronSettingsReadEachSetting(t *testing.T) {
	t.Parallel()

	got, err := cronSettingsWith(map[string]string{
		"GOPHENBERG_CRON_SECRET":            longSecret,
		"GOPHENBERG_CRON_ON_VISITS":         "false",
		"GOPHENBERG_CRON_VISIT_PACE":        "5m",
		"GOPHENBERG_CRON_TIME_LIMIT":        "10s",
		"GOPHENBERG_CRON_LEASE":             "20s",
		"GOPHENBERG_CRON_STOP_GRACE":        "1s",
		"GOPHENBERG_SESSION_SWEEP_INTERVAL": "30m",
	})
	if err != nil {
		t.Fatalf("loadRunConfig() error = %v", err)
	}
	want := cronSettings{
		secret: longSecret, pace: 5 * time.Minute, timeLimit: 10 * time.Second, lease: 20 * time.Second,
		stopGrace: time.Second, sweepEvery: 30 * time.Minute,
	}
	if got != want {
		t.Errorf("cron job settings = %+v, want %+v", got, want)
	}
}

func TestCronSettingsRefuseALeaseNotAboveTheTimeLimit(t *testing.T) {
	t.Parallel()

	_, err := cronSettingsWith(map[string]string{
		"GOPHENBERG_CRON_TIME_LIMIT": "1m", "GOPHENBERG_CRON_LEASE": "1m",
	})
	if err == nil || !strings.Contains(err.Error(), "GOPHENBERG_CRON_LEASE") ||
		!strings.Contains(err.Error(), "GOPHENBERG_CRON_TIME_LIMIT") {
		t.Errorf("loadRunConfig() error = %v, want the lease refused against the time limit", err)
	}
}

func TestCronSettingsRefuseADurationNotAboveZero(t *testing.T) {
	t.Parallel()

	_, err := cronSettingsWith(map[string]string{"GOPHENBERG_CRON_VISIT_PACE": "0s"})
	if err == nil || !strings.Contains(err.Error(), "GOPHENBERG_CRON_VISIT_PACE") {
		t.Errorf("loadRunConfig() error = %v, want the pace refused by its name", err)
	}
}

func TestCronSettingsRefuseAShortSecretWithoutPrintingIt(t *testing.T) {
	t.Parallel()

	_, err := cronSettingsWith(map[string]string{"GOPHENBERG_CRON_SECRET": "short-synthetic-secret"})
	if err == nil || !strings.Contains(err.Error(), "must be at least 32 characters, got 22") {
		t.Fatalf("loadRunConfig() error = %v, want the short secret refused by its length", err)
	}
	if strings.Contains(err.Error(), "short-synthetic-secret") {
		t.Errorf("loadRunConfig() error = %q, want the secret left out of it", err)
	}
}

func TestCronSettingsRefuseVisitsOffWithNoSecret(t *testing.T) {
	t.Parallel()

	_, err := cronSettingsWith(map[string]string{"GOPHENBERG_CRON_ON_VISITS": "false"})
	if err == nil || !strings.Contains(err.Error(), "GOPHENBERG_CRON_SECRET") {
		t.Errorf("loadRunConfig() error = %v, want visits off refused while no secret lets the host's cron in", err)
	}
}

func TestCronSettingsRefuseAVisitSwitchThatIsNoFlag(t *testing.T) {
	t.Parallel()

	_, err := cronSettingsWith(map[string]string{"GOPHENBERG_CRON_ON_VISITS": "sometimes"})
	if err == nil || !strings.Contains(err.Error(), "GOPHENBERG_CRON_ON_VISITS") {
		t.Errorf("loadRunConfig() error = %v, want the switch refused by its name", err)
	}
}

// sweptStore stands in for the account store, recording the time each sweep asks for and answering counts.
type sweptStore struct {
	sessions, tokens int64
	tokensErr        error
	asked            []time.Time
}

// DeleteExpiredSessions records the time and answers the session count.
func (s *sweptStore) DeleteExpiredSessions(_ context.Context, now time.Time) (int64, error) {
	s.asked = append(s.asked, now)
	return s.sessions, nil
}

// DeleteExpiredTokens records the time and answers the token count or the refusal.
func (s *sweptStore) DeleteExpiredTokens(_ context.Context, now time.Time) (int64, error) {
	s.asked = append(s.asked, now)
	return s.tokens, s.tokensErr
}

// sweepLogged runs the sweep over store at the moment and returns the job, what it logged and its error.
func sweepLogged(store *sweptStore, at time.Time) (cron.Job, string, error) {
	logs := &bytes.Buffer{}
	job := sessionSweep(store, time.Hour, slog.New(slog.NewTextHandler(logs, nil)))
	err := job.Run(context.Background(), at)
	return job, logs.String(), err
}

func TestSessionSweepIsTheCoreJobEveryInterval(t *testing.T) {
	t.Parallel()

	job, _, _ := sweepLogged(&sweptStore{}, time.Now())
	if job.Name != "core/session-sweep" || job.Every != time.Hour {
		t.Errorf("the sweep job is %q every %s, want core/session-sweep every hour", job.Name, job.Every)
	}
}

func TestSessionSweepSweepsAtTheTimeTheRunnerHandsIt(t *testing.T) {
	t.Parallel()

	handed := time.Date(2026, time.October, 13, 10, 0, 0, 0, time.UTC)
	store := &sweptStore{}
	if _, _, err := sweepLogged(store, handed); err != nil {
		t.Fatalf("the sweep failed: %v", err)
	}
	if len(store.asked) != 2 || !store.asked[0].Equal(handed) || !store.asked[1].Equal(handed) {
		t.Errorf("the sweep asked for %v, want sessions and tokens swept at %s", store.asked, handed)
	}
}

func TestSessionSweepLogsWhatItRemovedInTheReapersWords(t *testing.T) {
	t.Parallel()

	_, logged, _ := sweepLogged(&sweptStore{sessions: 2, tokens: 3}, time.Now())
	if !strings.Contains(logged, `msg="reaped expired sessions" count=2`) ||
		!strings.Contains(logged, `msg="reaped expired tokens" count=3`) {
		t.Errorf("the sweep logged %q, want both counts in the reaper's words", logged)
	}
}

func TestSessionSweepLogsTheSessionsOfAFailedTokenHalf(t *testing.T) {
	t.Parallel()

	_, logged, err := sweepLogged(&sweptStore{sessions: 2, tokensErr: errors.New("the store is unreachable")}, time.Now())
	if !errors.Is(err, authkit.ErrSweepTokens) {
		t.Errorf("the sweep error = %v, want the token half named", err)
	}
	if !strings.Contains(logged, `msg="reaped expired sessions" count=2`) {
		t.Errorf("the sweep logged %q, want the sessions removed before the token half failed", logged)
	}
}

func TestSessionSweepStaysQuietWhenNothingExpired(t *testing.T) {
	t.Parallel()

	if _, logged, _ := sweepLogged(&sweptStore{}, time.Now()); logged != "" {
		t.Errorf("the sweep logged %q, want nothing when nothing expired", logged)
	}
}

// stopOrder records the order the stop reached the cron jobs and the plugins in.
type stopOrder struct {
	mu       sync.Mutex
	order    []string
	left     time.Duration
	running  chan struct{}
	stubborn chan struct{}
}

// RunDue marks itself running, then waits for its stop, or for its release when stubborn.
func (s *stopOrder) RunDue(ctx context.Context) (cron.Report, error) {
	close(s.running)
	if s.stubborn != nil {
		<-s.stubborn
	} else {
		<-ctx.Done()
	}
	s.note("jobs")
	return cron.Report{}, nil
}

// note records that the stop reached step.
func (s *stopOrder) note(step string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.order = append(s.order, step)
}

// stoppingPlugin is a plugin that records when it is stopped and how much of its grace is left.
type stoppingPlugin struct {
	order *stopOrder
}

// ID names the plugin.
func (p stoppingPlugin) ID() string { return "stopping" }

// Start starts nothing.
func (stoppingPlugin) Start(context.Context) error { return nil }

// Stop records the stop and the grace left on ctx.
func (p stoppingPlugin) Stop(ctx context.Context) error {
	deadline, _ := ctx.Deadline()
	p.order.mu.Lock()
	p.order.left = time.Until(deadline)
	p.order.mu.Unlock()
	p.order.note("plugins")
	return nil
}

// stopWith starts one cron run, then stops it and the plugin as the server's stop does.
func stopWith(t *testing.T, order *stopOrder, stopGrace, pluginGrace time.Duration) error {
	t.Helper()
	runs := server.NewCronRuns(order, time.Minute, time.Now)
	runs.Visited()
	<-order.running
	host := hostOf([]sdk.Plugin{stoppingPlugin{order: order}})
	if err := host.Start(t.Context(), pluginGrace); err != nil {
		t.Fatalf("starting the plugin: %v", err)
	}
	return (siteCron{runs: runs, stopGrace: stopGrace}).stop(host, pluginGrace)(t.Context())
}

func TestCronStopClosesTheRunsBeforeThePlugins(t *testing.T) {
	t.Parallel()

	order := &stopOrder{running: make(chan struct{})}
	if err := stopWith(t, order, time.Second, time.Second); err != nil {
		t.Fatalf("stop() error = %v", err)
	}

	if strings.Join(order.order, ",") != "jobs,plugins" {
		t.Errorf("the stop reached %v, want the cron jobs before the plugins", order.order)
	}
}

func TestCronStopGivesThePluginsTheirWholeGrace(t *testing.T) {
	t.Parallel()

	order := &stopOrder{running: make(chan struct{}), stubborn: make(chan struct{})}
	defer close(order.stubborn)
	err := stopWith(t, order, 10*time.Millisecond, 4*time.Second)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("stop() error = %v, want the job that ignored its stop reported", err)
	}
	if order.left < 3*time.Second {
		t.Errorf("the plugins had %s of their grace left, want nearly all of it after a job that ignored its stop",
			order.left)
	}
}

// quietJobs stands in for cron jobs that run nothing.
type quietJobs struct{}

// RunDue runs nothing.
func (quietJobs) RunDue(context.Context) (cron.Report, error) {
	return cron.Report{}, nil
}

// futureStore is a cron store whose session sweep falls due an hour from now, counting each read of its slots.
type futureStore struct {
	cron.Store
	reads atomic.Int32
}

// Slots counts one read and holds the session sweep an hour off.
func (s *futureStore) Slots(context.Context) ([]cron.Slot, error) {
	s.reads.Add(1)
	return []cron.Slot{{Name: sessionSweepJob, NextRun: time.Now().Add(time.Hour)}}, nil
}

func TestCronRunsOverTheStoreTheSiteHolds(t *testing.T) {
	t.Parallel()

	settings, err := cronSettingsWith(nil)
	if err != nil {
		t.Fatalf("cronSettingsWith() error = %v", err)
	}
	store := &futureStore{}
	jobs := cronOf(settings, site{cronStore: store}, slog.New(slog.DiscardHandler))

	if _, err := jobs.runs.RunNow(); err != nil {
		t.Fatalf("RunNow() error = %v", err)
	}
	if reads := store.reads.Load(); reads != 1 {
		t.Errorf("the cron jobs read the site's store %d times, want once", reads)
	}
}

func TestOpenStoresPicksThePostgresCronStore(t *testing.T) {
	t.Parallel()

	built, err := openStores(t.Context(), composeConfig{databaseURL: unreachableDatabaseURL})
	if err != nil {
		t.Fatalf("openStores() error = %v", err)
	}
	defer built.pool.Close()

	if _, picked := built.cronStore.(*postgres.CronStore); !picked {
		t.Errorf("openStores() holds the cron store %T, want the PostgreSQL one", built.cronStore)
	}
}

func TestTheServerCutsTheCronJobsWhenItBeginsToShutDown(t *testing.T) {
	t.Parallel()

	runs := server.NewCronRuns(quietJobs{}, time.Minute, time.Now)
	settings := runConfig{addr: "localhost:0", serving: servingDefaults}
	srv := (siteCron{runs: runs, stopGrace: time.Second}).serverOf(settings, http.NotFoundHandler())

	if err := srv.Shutdown(t.Context()); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := runs.RunNow(); errors.Is(err, server.ErrCronClosed) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the cron runs still run after the server began to shut down, want them cut")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestRunRunsTheSessionSweepFromTheCronAddress(t *testing.T) {
	t.Parallel()

	address := testkit.FreeAddr(t)
	databaseURL := emptyDatabaseURL(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL":   databaseURL,
		"GOPHENBERG_ADDR":           address,
		"GOPHENBERG_WEB_DIR":        t.TempDir(),
		"GOPHENBERG_CRON_SECRET":    longSecret,
		"GOPHENBERG_CRON_ON_VISITS": "false",
	}
	if err := seedMigrated(t.Context(), testkit.Getenv(env), new(bytes.Buffer)); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	pool := expiredSessionIn(t, databaseURL)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx, testkit.Getenv(env), io.Discard, noPlugins) }()
	base := "http://" + address
	awaitServer(t, base)

	answer := cronCallTo(t, base)

	if !strings.Contains(answer, `"ran":1`) {
		t.Errorf("the cron call answered %s, want the session sweep run once", answer)
	}
	var left int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM auth.sessions WHERE expires_at < now()").
		Scan(&left); err != nil || left != 0 {
		t.Errorf("%d expired sessions left after the cron call (%v), want none", left, err)
	}
	cancel()
	<-done
}

// expiredSessionIn stores an account with an expired session at databaseURL and returns a pool over it.
func expiredSessionIn(t *testing.T, databaseURL string) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	t.Cleanup(pool.Close)
	users := authkitpg.NewUserStore(pool)
	account, err := gouncer.NewUser("maria@example.com", "Maria Perez", "correct horse battery")
	if err != nil {
		t.Fatalf("building the account: %v", err)
	}
	if err := users.CreateUser(t.Context(), account); err != nil {
		t.Fatalf("storing the account: %v", err)
	}
	expired := time.Now().Add(-time.Hour)
	if err := users.CreateSession(t.Context(), gouncer.Session{
		TokenHash: []byte("expired-session-hash"), UserID: account.ID,
		CreatedAt: expired.Add(-time.Hour), ExpiresAt: expired,
	}); err != nil {
		t.Fatalf("storing the expired session: %v", err)
	}
	return pool
}

// cronCallTo posts the cron call holding the test secret to the site at base and returns its answer.
func cronCallTo(t *testing.T, base string) string {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/api/cron", nil)
	if err != nil {
		t.Fatalf("building the cron call: %v", err)
	}
	request.Header.Set("Authorization", "Bearer "+longSecret)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("sending the cron call: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil || response.StatusCode != http.StatusOK {
		t.Fatalf("the cron call answered %d %s (%v), want 200", response.StatusCode, body, err)
	}
	return string(body)
}

func TestTheSessionSweepRunsAsACronJobOnPostgres(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	if err := migrate(t.Context(), databaseURL); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	pool := expiredSessionIn(t, databaseURL)
	runner := cron.New(cron.Config{
		Jobs:  []cron.Job{sessionSweep(authkitpg.NewUserStore(pool), time.Hour, slog.New(slog.DiscardHandler))},
		Store: postgres.NewCronStore(pool), TimeLimit: 30 * time.Second, Lease: time.Minute,
	})

	if report, err := runner.RunDue(t.Context()); err != nil || report.Ran != 1 {
		t.Fatalf("RunDue() = %+v, %v, want the sweep run once", report, err)
	}
	var left int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM auth.sessions").Scan(&left); err != nil || left != 0 {
		t.Errorf("%d sessions left after the sweep (%v), want the expired one gone", left, err)
	}
}
