// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/gopherium/framework/pluginkit"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gopherium/gophenberg/sdk"
)

// errRefused is a connection the database server refused, as a dial answers it.
var errRefused = &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}

// errWrongPassword is PostgreSQL's answer to a password its role does not have.
var errWrongPassword = &pgconn.PgError{Severity: "FATAL", Code: "28P01", Message: "password authentication failed"}

// pingAnswering returns a ping that fails with err the first fails times, then answers, counting each call in calls.
func pingAnswering(fails int, err error, calls *int) func(context.Context) error {
	return func(context.Context) error {
		*calls++
		if *calls <= fails {
			return err
		}
		return nil
	}
}

func TestLoadRunConfigWaitsAMinuteForTheDatabaseTryingEverySecond(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testkit.Getenv(map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL}))

	want := databaseWait{limit: time.Minute, interval: time.Second}
	if err != nil || settings.databaseWait != want {
		t.Errorf("loadRunConfig() = %+v, %v, want %+v", settings.databaseWait, err, want)
	}
}

func TestLoadRunConfigReadsTheDatabaseWaitFromTheEnvironment(t *testing.T) {
	t.Parallel()

	for value, want := range map[string]databaseWait{
		"90s": {limit: 90 * time.Second, interval: 250 * time.Millisecond},
		"0":   {limit: 0, interval: 250 * time.Millisecond},
	} {
		settings, err := loadRunConfig(testkit.Getenv(map[string]string{
			"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_DATABASE_WAIT": value,
			"GOPHENBERG_DATABASE_WAIT_INTERVAL": "250ms",
		}))
		if err != nil || settings.databaseWait != want {
			t.Errorf("loadRunConfig() with a wait of %q = %+v, %v, want %+v", value, settings.databaseWait, err, want)
		}
	}
}

func TestLoadRunConfigRefusesADatabaseWaitItCannotStand(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		key   string
		value string
		want  string
	}{
		"a wait that is not a duration":      {"GOPHENBERG_DATABASE_WAIT", "soon", "must be a duration like 30s"},
		"a wait below zero":                  {"GOPHENBERG_DATABASE_WAIT", "-1s", "must not be negative"},
		"an interval that is not a duration": {"GOPHENBERG_DATABASE_WAIT_INTERVAL", "often", "must be a duration like 30s"},
		"an interval standing at zero":       {"GOPHENBERG_DATABASE_WAIT_INTERVAL", "0", "must stand above zero"},
		"an interval below zero":             {"GOPHENBERG_DATABASE_WAIT_INTERVAL", "-1s", "must stand above zero"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := loadRunConfig(testkit.Getenv(map[string]string{
				"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, asked.key: asked.value,
			}))

			want := fmt.Sprintf("%s: %s, got %q", asked.key, asked.want, asked.value)
			if err == nil || err.Error() != want {
				t.Errorf("loadRunConfig() error = %v, want %q", err, want)
			}
		})
	}
}

// wantNothingLogged fails t when the wait logged anything.
func wantNothingLogged(t *testing.T, logged *bytes.Buffer) {
	t.Helper()
	if logged.Len() != 0 {
		t.Errorf("await() logged %q, want nothing logged", logged.String())
	}
}

func TestDatabaseWaitSaysNothingForADatabaseThatAnswersAtOnce(t *testing.T) {
	t.Parallel()
	var logged bytes.Buffer
	calls := 0

	err := databaseWait{limit: time.Minute, interval: time.Millisecond}.await(
		t.Context(), pingAnswering(0, errRefused, &calls), testLogger(&logged))

	if err != nil || calls != 1 {
		t.Errorf("await() = %v after %d pings, want nil after one", err, calls)
	}
	wantNothingLogged(t, &logged)
}

func TestDatabaseWaitTriesAgainUntilTheDatabaseAnswers(t *testing.T) {
	t.Parallel()
	var logged bytes.Buffer
	calls := 0

	err := databaseWait{limit: time.Minute, interval: time.Millisecond}.await(
		t.Context(), pingAnswering(3, errRefused, &calls), testLogger(&logged))

	if err != nil || calls != 4 {
		t.Fatalf("await() = %v after %d pings, want nil once the fourth one answers", err, calls)
	}
	for _, line := range []string{"level=WARN msg=\"" + waitingLine + "\"", "connection refused", "wait=1m0s",
		"level=INFO msg=\"" + answeredLine + "\" after="} {
		if held := strings.Count(logged.String(), line); held != 1 {
			t.Errorf("logged %q, holding %q %d times, want once", logged.String(), line, held)
		}
	}
}

func TestDatabaseWaitStopsOnceTheWaitRunsOut(t *testing.T) {
	t.Parallel()
	var logged bytes.Buffer
	calls := 0

	err := databaseWait{limit: 50 * time.Millisecond, interval: 10 * time.Millisecond}.await(
		t.Context(), pingAnswering(1<<30, errRefused, &calls), testLogger(&logged))

	want := "the database did not answer within 50ms, the time GOPHENBERG_DATABASE_WAIT allows: "
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("await() = %v, want %q", err, want)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Errorf("await() = %v, want the last failure kept", err)
	}
	if calls < 2 || calls > 6 {
		t.Errorf("await() pinged %d times, want from 2 to 6, one ping and one after each pause of 10ms in 50ms", calls)
	}
	if held := strings.Count(logged.String(), waitingLine); held != 1 {
		t.Errorf("logged %q, holding the wait %d times, want once", logged.String(), held)
	}
	if strings.Contains(logged.String(), answeredLine) {
		t.Errorf("logged %q, want no answer logged", logged.String())
	}
}

func TestDatabaseWaitOfZeroStopsAtTheFirstFailure(t *testing.T) {
	t.Parallel()
	var logged bytes.Buffer
	calls := 0

	err := databaseWait{limit: 0, interval: time.Millisecond}.await(
		t.Context(), pingAnswering(1, errRefused, &calls), testLogger(&logged))

	want := "the database did not answer within 0s, the time GOPHENBERG_DATABASE_WAIT allows: "
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("await() = %v, want %q", err, want)
	}
	if calls != 1 {
		t.Errorf("await() pinged %d times, want once", calls)
	}
	wantNothingLogged(t, &logged)
}

func TestDatabaseWaitStopsAtOnceOnAFailureWaitingCannotFix(t *testing.T) {
	t.Parallel()
	var logged bytes.Buffer
	calls := 0

	err := databaseWait{limit: time.Minute, interval: time.Millisecond}.await(
		t.Context(), pingAnswering(1, errWrongPassword, &calls), testLogger(&logged))

	if !errors.Is(err, errWrongPassword) || calls != 1 {
		t.Errorf("await() = %v after %d pings, want the wrong password after one", err, calls)
	}
	wantNothingLogged(t, &logged)
}

// wantRanOut fails t unless err says the wait of within ran out, the setting it names included.
func wantRanOut(t *testing.T, err error, within string) {
	t.Helper()
	want := "the database did not answer within " + within + ", the time GOPHENBERG_DATABASE_WAIT allows: "
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("await() = %v, want %q", err, want)
	}
}

// pingHanging is a ping that hears nothing until its context ends, then fails with that end.
func pingHanging(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// silentAddress listens on loopback and never takes a connection, so a client connects and then hears nothing.
func silentAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening on loopback: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener.Addr().String()
}

func TestDatabaseWaitBoundsEachTryByTheTimeTheWaitHasLeft(t *testing.T) {
	t.Parallel()
	var ends []time.Time
	ping := func(ctx context.Context) error {
		end, _ := ctx.Deadline()
		ends = append(ends, end)
		if len(ends) < 3 {
			return errRefused
		}
		return nil
	}
	began := time.Now()

	err := databaseWait{limit: time.Minute, interval: time.Millisecond}.await(
		t.Context(), ping, testLogger(&bytes.Buffer{}))

	if err != nil || len(ends) != 3 {
		t.Fatalf("await() = %v after %d pings, want nil once the third one answers", err, len(ends))
	}
	if ends[0].Before(began.Add(time.Minute)) || ends[0].After(time.Now().Add(time.Minute)) {
		t.Errorf("the first ping ran until %v, want the wait's end, a minute after %v", ends[0], began)
	}
	for i, end := range ends {
		if !end.Equal(ends[0]) {
			t.Errorf("ping %d ran until %v, want the wait's end %v as every other ping", i+1, end, ends[0])
		}
	}
}

func TestDatabaseWaitEndsATryThatHangsOnceTheWaitRunsOut(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	began := time.Now()

	err := databaseWait{limit: 50 * time.Millisecond, interval: 10 * time.Millisecond}.await(
		ctx, pingHanging, testLogger(&bytes.Buffer{}))

	took := time.Since(began)
	wantRanOut(t, err, "50ms")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("await() = %v, want the hanging try's failure kept", err)
	}
	if took > 300*time.Millisecond {
		t.Errorf("await() took %v, want it to end soon after its wait of 50ms", took)
	}
}

func TestDatabaseWaitNeverTakesAnAnswerThatComesAfterTheWait(t *testing.T) {
	t.Parallel()
	late := func(context.Context) error {
		time.Sleep(100 * time.Millisecond)
		return nil
	}

	err := databaseWait{limit: 50 * time.Millisecond, interval: 10 * time.Millisecond}.await(
		t.Context(), late, testLogger(&bytes.Buffer{}))

	wantRanOut(t, err, "50ms")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("await() = %v, want the wait's end kept as the failure", err)
	}
}

func TestDatabaseWaitPingsNoMoreOnceItsLastPauseReachesTheWaitsEnd(t *testing.T) {
	t.Parallel()
	calls := 0

	err := databaseWait{limit: 50 * time.Millisecond, interval: time.Hour}.await(
		t.Context(), pingAnswering(1, errRefused, &calls), testLogger(&bytes.Buffer{}))

	wantRanOut(t, err, "50ms")
	if calls != 1 {
		t.Errorf("await() pinged %d times, want once, as its only pause ends at the wait's end", calls)
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Errorf("await() = %v, want the last failure kept", err)
	}
}

func TestDatabaseWaitOfZeroTriesOnceWithNoTimeBound(t *testing.T) {
	t.Parallel()
	var bounded []bool
	ping := func(ctx context.Context) error {
		_, held := ctx.Deadline()
		bounded = append(bounded, held)
		return errRefused
	}

	err := databaseWait{limit: 0, interval: time.Millisecond}.await(t.Context(), ping, testLogger(&bytes.Buffer{}))

	wantRanOut(t, err, "0s")
	if !slices.Equal(bounded, []bool{false}) {
		t.Errorf("await() pinged with time bounds %v, want one ping with no time bound", bounded)
	}
}

func TestDatabaseWaitAnswersTheEndOfItsContextDuringATry(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	ping := func(tried context.Context) error {
		cancel()
		<-tried.Done()
		return fmt.Errorf("failed to connect: %w", tried.Err())
	}

	err := databaseWait{limit: time.Minute, interval: time.Millisecond}.await(ctx, ping, testLogger(&bytes.Buffer{}))

	if !errors.Is(err, context.Canceled) || err.Error() != context.Canceled.Error() {
		t.Errorf("await() = %v, want the end of its context as it is", err)
	}
}

func TestDatabaseWaitStopsOnceTheWaitRunsOutOnAServerThatNeverAnswers(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	handle, err := dbkitpg.Open("postgres://maria@"+silentAddress(t)+"/blog?sslmode=disable",
		dbkitpg.Options{MaxConns: 2})
	if err != nil {
		t.Fatalf("opening a pool: %v", err)
	}
	defer func() { _ = handle.Close() }()
	began := time.Now()

	err = databaseWait{limit: 100 * time.Millisecond, interval: 10 * time.Millisecond}.await(
		ctx, handle.Pool.Ping, testLogger(&bytes.Buffer{}))

	took := time.Since(began)
	wantRanOut(t, err, "100ms")
	if took > 300*time.Millisecond {
		t.Errorf("await() took %v, want it to end soon after its wait of 100ms", took)
	}
}

func TestDatabaseWaitEndsWithTheContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	calls := 0
	ping := func(context.Context) error {
		calls++
		cancel()
		return errRefused
	}

	err := databaseWait{limit: time.Minute, interval: time.Minute}.await(ctx, ping, testLogger(&bytes.Buffer{}))

	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Errorf("await() = %v after %d pings, want the context's end after one", err, calls)
	}
}

// waitRecorder records what serve logs and cancels its context once serve logs that it waits for the database.
type waitRecorder struct {
	cancel context.CancelFunc
	logged *strings.Builder
}

// Write records p and cancels on the line that says serve waits for the database.
func (w waitRecorder) Write(p []byte) (int, error) {
	w.logged.Write(p)
	if strings.Contains(string(p), waitingLine) {
		w.cancel()
	}
	return len(p), nil
}

// stoppedDuringTheWait serves the plugins until serve logs that it waits for the database, then stops it, returning
// what serve logged and returned.
func stoppedDuringTheWait(t *testing.T, plugins registrations) (string, error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	logged := &strings.Builder{}
	env := map[string]string{"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL, "GOPHENBERG_DATABASE_WAIT": "1m"}

	err := run(ctx, testkit.Getenv(env), waitRecorder{cancel: cancel, logged: logged}, plugins)

	return logged.String(), err
}

func TestServeStopsCleanlyWhenItEndsWhileItWaitsForTheDatabase(t *testing.T) {
	t.Parallel()
	registered := false

	logged, err := stoppedDuringTheWait(t, listedAs("late", func(sdk.Deps) (sdk.Plugin, error) {
		registered = true
		return idle("late"), nil
	}))

	if err != nil {
		t.Fatalf("run() = %v, want a clean stop as a stop while it serves ends", err)
	}
	if !strings.Contains(logged, `msg="shutting down"`) {
		t.Errorf("serve logged %q, want it to say it shuts down", logged)
	}
	if registered {
		t.Error("serve registered a plugin before its database answered, want none registered")
	}
}

func TestNotUpYetHoldsOnlyFailuresWaitingCanFix(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		err  error
		want bool
	}{
		"a connection refused": {errRefused, true},
		"a server still starting up": {
			fmt.Errorf("connect: %w", &pgconn.PgError{Code: "57P03", Message: "the database system is starting up"}), true,
		},
		"a wrong password":   {fmt.Errorf("connect: %w", errWrongPassword), false},
		"a missing database": {&pgconn.PgError{Code: "3D000", Message: "database \"blog\" does not exist"}, false},
		"another failure":    {errors.New("tls: handshake failure"), false},
		"no failure":         {nil, false},
	} {
		if held := notUpYet(asked.err); held != asked.want {
			t.Errorf("notUpYet(%s) = %v, want %v", name, held, asked.want)
		}
	}
}

func TestNotUpYetHoldsWhatAPoolAnswersWithNoServerThere(t *testing.T) {
	t.Parallel()

	for name, address := range map[string]string{
		"a port nothing listens on": "postgres://maria@" + testkit.FreeAddr(t) + "/blog?sslmode=disable",
		"a socket not made yet":     "postgres://maria@/blog?host=" + filepath.Join(t.TempDir(), "run"),
	} {
		handle, err := dbkitpg.Open(address, dbkitpg.Options{MaxConns: 2})
		if err != nil {
			t.Fatalf("opening a pool on %s: %v", name, err)
		}
		pinged := handle.Pool.Ping(t.Context())
		_ = handle.Close()
		if !notUpYet(pinged) {
			t.Errorf("notUpYet(%v) = false for %s, want true", pinged, name)
		}
	}
}

func TestReachDatabaseWaitsForTheDatabaseBeforeItAsksAboutTheRole(t *testing.T) {
	t.Parallel()

	handle, err := openPostgres(settingsEnv(testkit.Getenv(nil)), unreachableDatabaseURL)
	if err != nil {
		t.Fatalf("opening the pool: %v", err)
	}
	defer func() { _ = handle.Close() }()
	settings := timedConfig()
	settings.serving = servingDefaults
	settings.databaseWait = databaseWait{limit: 0, interval: time.Millisecond}
	start := boot{settings: settings, built: site{pgHandle: handle}, host: pluginkit.NewHost(),
		logger: testLogger(&bytes.Buffer{})}

	err = start.reachDatabase(t.Context())

	want := "the database did not answer within 0s, the time GOPHENBERG_DATABASE_WAIT allows: "
	if err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Errorf("reachDatabase() = %v, want %q before the role is asked about", err, want)
	}
}
