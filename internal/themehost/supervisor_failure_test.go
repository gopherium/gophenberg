// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// missingNodeBin returns a path where no node binary is installed.
func missingNodeBin(t *testing.T) string {
	t.Helper()

	return filepath.Join(t.TempDir(), "no-such-node")
}

// startWithoutNodeBin runs a supervisor over the given absent node binary and stops it when the test ends.
func startWithoutNodeBin(
	t *testing.T,
	absentNodeBin string,
	tune func(*themehost.SupervisorConfig),
) (*themehost.Supervisor, *logBuffer) {
	t.Helper()

	theme, err := themehost.Load(writeTheme(t), "starter")
	if err != nil {
		t.Fatalf("loading the theme: %v", err)
	}
	logs := &logBuffer{}
	config := themehost.SupervisorConfig{
		Theme:        theme,
		NodeBin:      absentNodeBin,
		APIAddr:      "127.0.0.1:8081",
		Logger:       slog.New(slog.NewTextHandler(logs, nil)),
		ReadyTimeout: time.Second,
		Backoff:      10 * time.Millisecond,
		MaxBackoff:   40 * time.Millisecond,
		MaxAttempts:  3,
	}
	if tune != nil {
		tune(&config)
	}
	supervisor := themehost.NewSupervisor(config)
	supervisor.Start()
	t.Cleanup(supervisor.Stop)
	return supervisor, logs
}

func TestSupervisorGivesUpWhenTheThemeProcessNeverStarts(t *testing.T) {
	t.Parallel()

	absentNodeBin := missingNodeBin(t)

	supervisor, logs := startWithoutNodeBin(t, absentNodeBin, nil)

	waitFor(t, "the supervisor to stop retrying a theme it cannot start", func() bool {
		return strings.Contains(logs.String(), "theme start failed")
	})
	if supervisor.Healthy() {
		t.Error("Healthy() = true, want false for a theme that never started")
	}
	if supervisor.Target() != "" {
		t.Errorf("Target() = %q, want empty for a theme that never started", supervisor.Target())
	}
	if supervisor.Group() != 0 {
		t.Errorf("Group() = %d, want zero because no process group was ever made", supervisor.Group())
	}
	if got := strings.Count(logs.String(), "theme starting"); got != 0 {
		t.Errorf("start lines = %d, want none because no process ever ran", got)
	}
	if got := strings.Count(logs.String(), "theme exited"); got != 3 {
		t.Errorf("exit lines = %d, want one per attempt (3)", got)
	}
	if got := strings.Count(logs.String(), absentNodeBin); got != 3 {
		t.Errorf("exit lines naming the absent binary = %d, want 3 because spawning it is what failed", got)
	}
	if strings.Contains(logs.String(), "finding a free port") {
		t.Errorf("logs = %q, want no port failure because the spawn is the only failure wanted", logs.String())
	}
}

func TestSupervisorStopsWhileItIsWaitingToRetry(t *testing.T) {
	t.Parallel()

	const backoff = time.Minute
	const settle = 500 * time.Millisecond

	supervisor, logs := startWithoutNodeBin(t, missingNodeBin(t), func(config *themehost.SupervisorConfig) {
		config.MaxAttempts = 1
		config.Backoff = backoff
		config.MaxBackoff = backoff
	})
	waitFor(t, "the only attempt to fail", func() bool {
		return strings.Contains(logs.String(), "theme exited")
	})
	time.Sleep(settle)

	returned := make(chan struct{})
	go func() {
		supervisor.Stop()
		close(returned)
	}()

	select {
	case <-returned:
	case <-time.After(10 * time.Second):
		t.Fatal("Stop() did not return, want the one minute wait before a retry cut short")
	}
	if got := strings.Count(logs.String(), "theme exited"); got != 1 {
		t.Errorf("exit lines = %d, want one because the retry was still being waited for", got)
	}
	if strings.Contains(logs.String(), "theme start failed") {
		t.Errorf("logs = %q, want no failed start line because the wait was cut short", logs.String())
	}
	if supervisor.Healthy() {
		t.Error("Healthy() = true after Stop(), want false")
	}
}
