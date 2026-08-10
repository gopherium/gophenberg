// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"sync"
	"syscall"
	"time"
)

// healthPath is where a theme server reports that it is ready to serve.
const healthPath = "/_gophenberg/health"

// loopbackHost is the address a theme server binds, never reachable from outside the machine.
const loopbackHost = "127.0.0.1"

// defaultStopGrace is how long a theme process group has to exit before it is killed outright.
const defaultStopGrace = 3 * time.Second

// readyPoll is how often a starting theme is asked whether it is ready.
const readyPoll = 20 * time.Millisecond

// SupervisorConfig is what a supervisor needs to run one theme.
type SupervisorConfig struct {
	Theme        *Theme
	NodeBin      string
	APIAddr      string
	Logger       *slog.Logger
	ReadyTimeout time.Duration
	Backoff      time.Duration
	MaxBackoff   time.Duration
	MaxAttempts  int
	StopGrace    time.Duration
	StubDelay    time.Duration
}

// Supervisor runs one theme server and keeps it running.
type Supervisor struct {
	config SupervisorConfig
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once

	mu      sync.RWMutex
	port    int
	group   int
	healthy bool
}

// NewSupervisor returns a supervisor for one theme.
func NewSupervisor(config SupervisorConfig) *Supervisor {
	return &Supervisor{config: withDefaults(config), done: make(chan struct{})}
}

// withDefaults returns the config with every unset timing filled in.
func withDefaults(config SupervisorConfig) SupervisorConfig {
	for _, timing := range []struct {
		at  *time.Duration
		set time.Duration
	}{
		{at: &config.ReadyTimeout, set: 30 * time.Second},
		{at: &config.Backoff, set: 500 * time.Millisecond},
		{at: &config.MaxBackoff, set: 30 * time.Second},
		{at: &config.StopGrace, set: defaultStopGrace},
	} {
		if *timing.at == 0 {
			*timing.at = timing.set
		}
	}
	if config.MaxAttempts == 0 {
		config.MaxAttempts = 5
	}
	return config
}

// Start runs the theme until Stop is called.
func (s *Supervisor) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go s.supervise(ctx)
}

// Stop ends the theme and waits for its process group to go.
func (s *Supervisor) Stop() {
	s.once.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		<-s.done
	})
}

// Name returns the theme the supervisor runs.
func (s *Supervisor) Name() string { return s.config.Theme.Name }

// Healthy reports whether the theme is serving.
func (s *Supervisor) Healthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.healthy
}

// Target returns the address the theme serves on, empty while it is down.
func (s *Supervisor) Target() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if !s.healthy {
		return ""
	}
	return "http://" + net.JoinHostPort(loopbackHost, strconv.Itoa(s.port))
}

// Group returns the process group the theme runs in, zero while it is down.
func (s *Supervisor) Group() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.group
}

// supervise restarts the theme until it is told to stop or has failed too often.
func (s *Supervisor) supervise(ctx context.Context) {
	defer close(s.done)

	wait := s.config.Backoff
	for attempt := 1; attempt <= s.config.MaxAttempts; attempt++ {
		served := s.attempt(ctx)
		if ctx.Err() != nil {
			return
		}
		if served {
			attempt, wait = 0, s.config.Backoff
		}
		if !sleep(ctx, wait) {
			return
		}
		wait = min(wait*2, s.config.MaxBackoff)
	}
	s.config.Logger.Error("theme gave up", "theme", s.config.Theme.Name, "attempts", s.config.MaxAttempts)
}

// attempt runs the theme once and reports whether it ever served.
func (s *Supervisor) attempt(ctx context.Context) bool {
	port, err := freePort()
	if err != nil {
		s.config.Logger.Error("theme exited", "theme", s.config.Theme.Name, "reason", err)
		return false
	}
	command := s.spawn(port)
	if err := command.Start(); err != nil {
		s.config.Logger.Error("theme exited", "theme", s.config.Theme.Name, "reason", err)
		return false
	}
	s.config.Logger.Info("theme starting", "theme", s.config.Theme.Name, "port", port, "pid", command.Process.Pid)
	s.began(port, command.Process.Pid)
	waited, exited := watch(command)
	served := s.awaitReady(ctx, port, exited)
	exit := s.settle(ctx, command, waited, served)
	s.ended()
	s.config.Logger.Info("theme exited", "theme", s.config.Theme.Name, "reason", exit)
	return served
}

// spawn returns the command running the theme with only the environment it is allowed.
func (s *Supervisor) spawn(port int) *exec.Cmd {
	command := exec.Command(s.config.NodeBin, s.config.Theme.Entry)
	command.Dir = s.config.Theme.Dir
	command.Env = []string{
		"HOST=" + loopbackHost,
		"PORT=" + strconv.Itoa(port),
		"GOPHENBERG_API_URL=http://" + s.config.APIAddr,
	}
	if s.config.StubDelay > 0 {
		command.Env = append(command.Env,
			"GOPHENBERG_STUB_DELAY_MS="+strconv.FormatInt(s.config.StubDelay.Milliseconds(), 10))
	}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return command
}

// began records that a theme process is running.
func (s *Supervisor) began(port, pid int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.port, s.group, s.healthy = port, pid, false
}

// ended records that no theme process is running.
func (s *Supervisor) ended() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.port, s.group, s.healthy = 0, 0, false
}

// watch returns the exit of a running command, both as a value and as a signal.
func watch(command *exec.Cmd) (chan error, chan struct{}) {
	waited := make(chan error, 1)
	exited := make(chan struct{})
	go func() {
		waited <- command.Wait()
		close(exited)
	}()
	return waited, exited
}

// awaitReady reports whether the theme answered its probe before it exited or the timeout passed.
func (s *Supervisor) awaitReady(ctx context.Context, port int, exited <-chan struct{}) bool {
	probe := "http://" + net.JoinHostPort(loopbackHost, strconv.Itoa(port)) + healthPath
	deadline := time.Now().Add(s.config.ReadyTimeout)
	for time.Now().Before(deadline) {
		if stopped(exited) {
			return false
		}
		if ready(ctx, probe) {
			s.mu.Lock()
			s.healthy = true
			s.mu.Unlock()
			s.config.Logger.Info("theme ready", "theme", s.config.Theme.Name, "port", port)
			return true
		}
		if !sleep(ctx, readyPoll) {
			return false
		}
	}
	return false
}

// stopped reports whether a watched command has exited.
func stopped(exited <-chan struct{}) bool {
	select {
	case <-exited:
		return true
	default:
		return false
	}
}

// settle waits for the theme to exit, ending it when it never served or the supervisor is stopping.
func (s *Supervisor) settle(ctx context.Context, command *exec.Cmd, waited chan error, served bool) error {
	if !served {
		return s.terminate(command, waited)
	}
	select {
	case err := <-waited:
		return err
	case <-ctx.Done():
		return s.terminate(command, waited)
	}
}

// terminate ends a theme process group, killing it outright when it will not go.
func (s *Supervisor) terminate(command *exec.Cmd, waited chan error) error {
	_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
	select {
	case err := <-waited:
		return err
	case <-time.After(s.config.StopGrace):
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		return <-waited
	}
}

// ready reports whether a probe answered that the theme is serving.
func ready(ctx context.Context, probe string) bool {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, probe, nil)
	if err != nil {
		return false
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return false
	}
	defer func() { _ = response.Body.Close() }()
	return response.StatusCode == http.StatusOK
}

// freePort returns a port nothing is listening on.
func freePort() (int, error) {
	listener, err := net.Listen("tcp", net.JoinHostPort(loopbackHost, "0"))
	if err != nil {
		return 0, fmt.Errorf("themehost: finding a free port: %w", err)
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// sleep waits for a duration and reports whether the wait finished.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
