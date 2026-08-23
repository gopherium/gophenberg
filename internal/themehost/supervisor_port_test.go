// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"log/slog"
	"net"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// warmTheNetworkPoller opens and closes a loopback listener, skipping when the loopback refuses one.
func warmTheNetworkPoller(t *testing.T) {
	t.Helper()

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "0"))
	if err != nil {
		t.Skipf("the loopback interface already refuses a listener: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("closing the warm up listener: %v", err)
	}
}

// starveDescriptors leaves the process unable to open any descriptor and returns the call that undoes it.
func starveDescriptors(t *testing.T) func() {
	t.Helper()

	var savedLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &savedLimit); err != nil {
		t.Skipf("reading the descriptor limit: %v", err)
	}
	restore := func() {
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &savedLimit); err != nil {
			t.Errorf("restoring the descriptor limit: %v", err)
		}
	}
	starvedLimit := syscall.Rlimit{Cur: 0, Max: savedLimit.Max}
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &starvedLimit); err != nil {
		t.Skipf("lowering the descriptor limit: %v", err)
	}
	t.Cleanup(restore)
	return restore
}

// loopbackRefusesAListener reports whether a loopback listener fails under the limit in force.
func loopbackRefusesAListener(t *testing.T) bool {
	t.Helper()

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "0"))
	if err != nil {
		return true
	}
	if err := listener.Close(); err != nil {
		t.Fatalf("closing the probe listener: %v", err)
	}
	return false
}

// onlyExitLine returns the single exit line the logs carry, failing when they carry any other count.
func onlyExitLine(t *testing.T, logs string) string {
	t.Helper()

	found := make([]string, 0, 1)
	for _, line := range strings.Split(logs, "\n") {
		if strings.Contains(line, "theme exited") {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Fatalf("exit lines = %d, want exactly one, logs = %q", len(found), logs)
	}
	return found[0]
}

func TestSupervisorReportsAnAttemptThatCannotFindAFreePort(t *testing.T) {
	warmTheNetworkPoller(t)
	logs := &logBuffer{}
	supervisor := themehost.NewSupervisor(themehost.SupervisorConfig{
		Theme:       &themehost.Theme{Name: "starter"},
		NodeBin:     "node",
		APIAddr:     "127.0.0.1:8081",
		Logger:      slog.New(slog.NewTextHandler(logs, nil)),
		Backoff:     time.Millisecond,
		MaxBackoff:  time.Millisecond,
		MaxAttempts: 1,
	})

	restore := starveDescriptors(t)
	starved := loopbackRefusesAListener(t)
	if starved {
		supervisor.Start()
		supervisor.Stop()
	}
	restore()

	if !starved {
		t.Skip("this machine still hands out a loopback listener with no descriptors left")
	}
	exit := onlyExitLine(t, logs.String())
	for _, want := range []string{"finding a free port", "too many open files"} {
		if !strings.Contains(exit, want) {
			t.Errorf("exit line = %q, want a reason saying %q", exit, want)
		}
	}
	if strings.Contains(logs.String(), "theme starting") {
		t.Errorf("logs = %q, want no start line when the attempt never got a port", logs.String())
	}
	if supervisor.Healthy() {
		t.Error("Healthy() = true, want false for a theme that never got a port")
	}
	if supervisor.Target() != "" {
		t.Errorf("Target() = %q, want empty for a theme that never got a port", supervisor.Target())
	}
}
