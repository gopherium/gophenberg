// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// nodeBin returns the node the stubs run under, skipping when the machine has none.
func nodeBin(t *testing.T) string {
	t.Helper()

	found, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not on PATH, so the theme runtime cannot be exercised here")
	}
	return found
}

// installStub lays out a theme directory serving the named stub and returns it.
func installStub(t *testing.T, stub string) *themehost.Theme {
	t.Helper()

	themesDir := writeTheme(t)
	source, err := os.ReadFile(filepath.Join("testdata", stub+".mjs"))
	if err != nil {
		t.Fatalf("reading stub %s: %v", stub, err)
	}
	writeFile(t, filepath.Join(themesDir, "starter", "server", "entry.mjs"), string(source))
	theme, err := themehost.Load(themesDir, "starter")
	if err != nil {
		t.Fatalf("loading the stub theme: %v", err)
	}
	return theme
}

// logBuffer is a writer the test reads structured log lines back from.
type logBuffer struct {
	mu    sync.Mutex
	lines strings.Builder
}

// Write records a log line.
func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lines.Write(p)
}

// String returns what has been logged so far.
func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.lines.String()
}

// startSupervisor runs a supervisor over a stub and stops it when the test ends.
func startSupervisor(
	t *testing.T,
	stub string,
	tune func(*themehost.SupervisorConfig),
) (*themehost.Supervisor, *logBuffer) {
	t.Helper()

	logs := &logBuffer{}
	config := themehost.SupervisorConfig{
		Theme:       installStub(t, stub),
		NodeBin:     nodeBin(t),
		APIAddr:     "127.0.0.1:8081",
		Logger:      slog.New(slog.NewTextHandler(logs, nil)),
		Backoff:     10 * time.Millisecond,
		MaxBackoff:  40 * time.Millisecond,
		MaxAttempts: 1,
	}
	if tune != nil {
		tune(&config)
	}
	supervisor := themehost.NewSupervisor(config)
	supervisor.Start()
	t.Cleanup(supervisor.Stop)
	return supervisor, logs
}

// waitPoll is how often a test wait checks its condition.
const waitPoll = 5 * time.Millisecond

// waitFor polls until the condition holds or the deadline passes.
func waitFor(t *testing.T, why string, condition func() bool) {
	t.Helper()

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(waitPoll)
	}
	t.Fatalf("timed out waiting for %s", why)
}

// readiness is what a readiness wait watches, a theme that either serves or gives up starting.
type readiness interface {
	Healthy() bool
	StartFailed() bool
}

// waitReady polls until the theme serves, failing with the supervisor logs once its start fails or time runs out.
func waitReady(t *testing.T, why string, theme readiness, logs *logBuffer) {
	t.Helper()

	deadline, bounded := t.Deadline()
	grace := time.Until(deadline) / 20
	for !theme.Healthy() {
		if theme.StartFailed() || (bounded && time.Until(deadline) < 2*grace) {
			t.Fatalf("timed out waiting for %s\nsupervisor logs:\n%s", why, logs)
		}
		time.Sleep(waitPoll)
	}
}

func TestSupervisorServesOnceTheThemeReportsReady(t *testing.T) {
	t.Parallel()

	supervisor, logs := startSupervisor(t, "healthy", nil)

	waitReady(t, "the theme to report ready", supervisor, logs)

	response, err := http.Get(supervisor.Target() + "/anything")
	if err != nil {
		t.Fatalf("reading through the target: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	body, _ := io.ReadAll(response.Body)
	if !strings.Contains(string(body), "stub served") {
		t.Errorf("body = %q, want the stub's answer", body)
	}
	for _, want := range []string{"theme starting", "theme ready"} {
		waitFor(t, "a log line saying "+want, func() bool {
			return strings.Contains(logs.String(), want)
		})
	}
}

func TestSupervisorGivesTheChildOnlyTheAllowlist(t *testing.T) {
	t.Setenv("GOPHENBERG_DATABASE_URL", "postgres://should-never-reach-the-child")
	supervisor, logs := startSupervisor(t, "env", nil)
	waitReady(t, "the theme to report ready", supervisor, logs)

	response, err := http.Get(supervisor.Target() + "/env")
	if err != nil {
		t.Fatalf("reading the child environment: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	var childEnv map[string]string
	if err := json.NewDecoder(response.Body).Decode(&childEnv); err != nil {
		t.Fatalf("decoding the child environment: %v", err)
	}

	names := make([]string, 0, len(childEnv))
	for name := range childEnv {
		names = append(names, name)
	}
	slices.Sort(names)
	want := []string{"GOPHENBERG_API_URL", "HOST", "PORT"}
	if !slices.Equal(names, want) {
		t.Errorf("child environment = %v, want exactly %v", names, want)
	}
	if childEnv["GOPHENBERG_API_URL"] != "http://127.0.0.1:8081" {
		t.Errorf("GOPHENBERG_API_URL = %q, want the instance address", childEnv["GOPHENBERG_API_URL"])
	}
	if childEnv["HOST"] != "127.0.0.1" {
		t.Errorf("HOST = %q, want 127.0.0.1", childEnv["HOST"])
	}
}

func TestSupervisorWaitsForASlowBoot(t *testing.T) {
	t.Parallel()

	supervisor, logs := startSupervisor(t, "slow", nil)

	waitReady(t, "the slow theme to report ready", supervisor, logs)
}

func TestSupervisorBacksOffThenFailsTheStartOfAThemeThatNeverBoots(t *testing.T) {
	t.Parallel()

	supervisor, logs := startSupervisor(t, "crash", func(config *themehost.SupervisorConfig) {
		config.MaxAttempts = 3
	})

	waitFor(t, "the supervisor to stop retrying", func() bool {
		return strings.Contains(logs.String(), "theme start failed")
	})
	if supervisor.Healthy() {
		t.Error("Healthy() = true, want false for a theme that never boots")
	}
	if supervisor.Target() != "" {
		t.Errorf("Target() = %q, want empty while the theme is down", supervisor.Target())
	}
	if got := strings.Count(logs.String(), "theme exited"); got != 3 {
		t.Errorf("exit lines = %d, want one per attempt (3)", got)
	}
}

func TestSupervisorLeavesNoOrphanBehind(t *testing.T) {
	t.Parallel()

	supervisor, logs := startSupervisor(t, "healthy", nil)
	waitReady(t, "the theme to report ready", supervisor, logs)
	group := supervisor.Group()
	if group <= 0 {
		t.Fatalf("Group() = %d, want the child process group", group)
	}

	supervisor.Stop()

	waitFor(t, "the process group to be gone", func() bool {
		return syscall.Kill(-group, syscall.Signal(0)) != nil
	})
}

func TestSupervisorKillsAThemeThatSwallowsSigterm(t *testing.T) {
	t.Parallel()

	supervisor, logs := startSupervisor(t, "stubborn", func(config *themehost.SupervisorConfig) {
		config.StopGrace = 100 * time.Millisecond
	})
	waitReady(t, "the stubborn theme to report ready", supervisor, logs)
	group := supervisor.Group()

	supervisor.Stop()

	waitFor(t, "the group kill to end a theme that ignored SIGTERM", func() bool {
		return syscall.Kill(-group, syscall.Signal(0)) != nil
	})
}

func TestSupervisorRestartsAThemeThatDiesAfterServing(t *testing.T) {
	t.Parallel()

	_, logs := startSupervisor(t, "flaky", func(config *themehost.SupervisorConfig) {
		config.MaxAttempts = 10
	})

	waitFor(t, "the theme to be started more than once", func() bool {
		return strings.Count(logs.String(), "theme ready") >= 2
	})
}

func TestSupervisorFailsTheStartOfAThemeThatBootsButNeverReportsReady(t *testing.T) {
	t.Parallel()

	supervisor, logs := startSupervisor(t, "deaf", func(config *themehost.SupervisorConfig) {
		config.ReadyTimeout = 150 * time.Millisecond
	})

	waitFor(t, "the supervisor to fail a theme that never answers", func() bool {
		return strings.Contains(logs.String(), "theme start failed")
	})
	if supervisor.Healthy() {
		t.Error("Healthy() = true, want false for a theme that never answered its probe")
	}
}

func TestTheSupervisorSaysWhenItsStartFailed(t *testing.T) {
	t.Parallel()

	supervisor, _ := startSupervisor(t, "deaf", func(config *themehost.SupervisorConfig) {
		config.ReadyTimeout = 150 * time.Millisecond
		config.MaxAttempts = 1
	})

	if supervisor.StartFailed() {
		t.Error("StartFailed() = true, want false while the supervisor is still trying")
	}
	waitFor(t, "the supervisor to record the failed start", supervisor.StartFailed)
}

func TestSupervisorFailsTheStartOfAThemeThatAcceptsItsProbeButNeverAnswers(t *testing.T) {
	t.Parallel()

	var themeDir string
	supervisor, _ := startSupervisor(t, "mute", func(config *themehost.SupervisorConfig) {
		config.ReadyTimeout = 3 * time.Second
		config.MaxAttempts = 1
		themeDir = config.Theme.Dir
	})

	waitFor(t, "the supervisor to fail a theme whose probe is never answered", supervisor.StartFailed)
	if _, err := os.Stat(filepath.Join(themeDir, "server", "accepted")); err != nil {
		t.Errorf("the theme never accepted a probe (%v), want the start failed while a probe went unanswered", err)
	}
}

func TestAStoppedSupervisorHasNotFailedItsStart(t *testing.T) {
	t.Parallel()

	supervisor, _ := startSupervisor(t, "healthy", nil)
	if err := supervisor.Await(t.Context()); err != nil {
		t.Fatalf("Await() = %v, want the theme serving before it is stopped", err)
	}

	supervisor.Stop()

	if supervisor.StartFailed() {
		t.Error("StartFailed() = true, want a deliberate stop kept apart from a failed start")
	}
}

func TestNewSupervisorFillsInTheTimingsAThemeIsRunUnder(t *testing.T) {
	t.Parallel()

	supervisor := themehost.NewSupervisor(themehost.SupervisorConfig{
		Theme:  &themehost.Theme{Name: "starter"},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	if supervisor.Healthy() {
		t.Error("Healthy() = true before Start(), want false")
	}
	if supervisor.Target() != "" {
		t.Errorf("Target() = %q before Start(), want empty", supervisor.Target())
	}
}

func TestSupervisorStopsCleanlyBeforeTheThemeIsEverReady(t *testing.T) {
	t.Parallel()

	supervisor, _ := startSupervisor(t, "slow", func(config *themehost.SupervisorConfig) {
		config.StubDelay = 5 * time.Second
	})

	supervisor.Stop()

	if supervisor.Healthy() {
		t.Error("Healthy() = true after Stop(), want false")
	}
}
