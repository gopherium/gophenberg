// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/sdk"
)

// heldPath is the public route on which the holding plugin holds every request until it is cancelled.
const heldPath = "/held"

// heldEnd is how and when a held request ended.
type heldEnd struct {
	cause error
	at    time.Time
}

// stopSeen is what the holding plugin saw when the host began stopping it.
type stopSeen struct {
	heldEnded bool
	live      bool
	left      time.Duration
}

// holdingPlugin holds every request on its route until the request is cancelled, and reports what its stop saw.
type holdingPlugin struct {
	entered   chan struct{}
	enterOnce sync.Once
	endedWith chan heldEnd
	ended     atomic.Bool
	stopped   chan stopSeen
}

// newHoldingPlugin returns a plugin ready to hold one request.
func newHoldingPlugin() *holdingPlugin {
	return &holdingPlugin{
		entered: make(chan struct{}), endedWith: make(chan heldEnd, 1), stopped: make(chan stopSeen, 1),
	}
}

// ID returns the plugin's identifier.
func (*holdingPlugin) ID() string {
	return "holding"
}

// Start starts nothing.
func (*holdingPlugin) Start(context.Context) error {
	return nil
}

// Stop reports whether the held request had ended, and the context the host stopped the plugin with.
func (p *holdingPlugin) Stop(ctx context.Context) error {
	seen := stopSeen{heldEnded: p.ended.Load(), live: ctx.Err() == nil}
	if deadline, bounded := ctx.Deadline(); bounded {
		seen.left = time.Until(deadline)
	}
	p.stopped <- seen
	return nil
}

// PublicPaths opens the held route to visitors without a session.
func (*holdingPlugin) PublicPaths() []string {
	return []string{heldPath}
}

// Routes returns the handler holding each request until it is cancelled.
func (p *holdingPlugin) Routes() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.enterOnce.Do(func() { close(p.entered) })
		<-r.Context().Done()
		p.endedWith <- heldEnd{cause: context.Cause(r.Context()), at: time.Now()}
		p.ended.Store(true)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
}

// closeOnListen closes its channel once the server logs that it is listening.
type closeOnListen struct {
	listening chan struct{}
	once      *sync.Once
}

// Write scans log output for the listening line.
func (w closeOnListen) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "listening") {
		w.once.Do(func() { close(w.listening) })
	}
	return len(p), nil
}

// holdRequest sends a request to the held route that the end of the test abandons.
func holdRequest(t *testing.T, addr string) {
	t.Helper()
	request, err := http.NewRequestWithContext(
		t.Context(), http.MethodGet, "http://"+addr+"/api/plugins/holding"+heldPath, nil)
	if err != nil {
		t.Fatalf("building the held request: %v", err)
	}
	answered := make(chan struct{})
	go func() {
		defer close(answered)
		if response, err := http.DefaultClient.Do(request); err == nil {
			_ = response.Body.Close()
		}
	}()
	t.Cleanup(func() { <-answered })
}

func TestRunStopsThePluginsWhenThePinnedThemeCannotLoad(t *testing.T) {
	t.Parallel()

	plugin := newHoldingPlugin()
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL":        emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":                "localhost:0",
		"GOPHENBERG_THEMES_DIR":          t.TempDir(),
		"GOPHENBERG_THEME":               "missing",
		"GOPHENBERG_SHUTDOWN_STOP_GRACE": "4s",
	}

	err := run(t.Context(), testGetenv(env), io.Discard,
		func(sdk.Deps) ([]sdk.Plugin, error) { return []sdk.Plugin{plugin}, nil })

	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("run() error = %v, want the pinned theme reported", err)
	}
	select {
	case seen := <-plugin.stopped:
		if !seen.live || seen.left <= 0 || seen.left > 4*time.Second {
			t.Errorf("the plugin stopped with live = %v and %v left, want a live context under its own 4s grace",
				seen.live, seen.left)
		}
	default:
		t.Error("the plugin never stopped, want the plugins stopped once the theme failed to start")
	}
}

func TestRunCancelsARequestStillRunningAtTheGraceBeforeThePluginsStop(t *testing.T) {
	t.Parallel()

	plugin := newHoldingPlugin()
	addr := "localhost:" + freePort(t)
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL":          emptyDatabaseURL(t),
		"GOPHENBERG_ADDR":                  addr,
		"GOPHENBERG_SHUTDOWN_GRACE":        "50ms",
		"GOPHENBERG_SHUTDOWN_CANCEL_GRACE": "3s",
		"GOPHENBERG_SHUTDOWN_STOP_GRACE":   "4s",
	}
	ctx, signal := context.WithCancel(t.Context())
	defer signal()
	listening := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- run(ctx, testGetenv(env), closeOnListen{listening: listening, once: &sync.Once{}},
			func(sdk.Deps) ([]sdk.Plugin, error) { return []sdk.Plugin{plugin}, nil })
	}()
	select {
	case <-listening:
	case <-time.After(30 * time.Second):
		t.Fatal("run() never reported listening")
	}
	holdRequest(t, addr)
	select {
	case <-plugin.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("the held request never reached the plugin")
	}

	signalled := time.Now()
	signal()

	var runErr error
	select {
	case runErr = <-finished:
	case <-time.After(30 * time.Second):
		t.Fatal("run() never returned after the signal")
	}
	if runErr != nil {
		t.Errorf("run() error = %v, want a normal stop once every cancelled request ended", runErr)
	}
	select {
	case end := <-plugin.endedWith:
		if !errors.Is(end.cause, gonsole.ErrGraceRanOut) {
			t.Errorf("the held request ended with %v, want %v", end.cause, gonsole.ErrGraceRanOut)
		}
		if waited := end.at.Sub(signalled); waited < 50*time.Millisecond || waited > time.Second {
			t.Errorf("the held request was cancelled %v after the signal, want soon after the 50ms grace ran out",
				waited)
		}
	default:
		t.Error("the held request still ran once run() returned, want it cancelled at the grace")
	}
	seen := <-plugin.stopped
	if !seen.heldEnded {
		t.Error("the plugin began stopping while the held request still ran, want the request ended first")
	}
	if !seen.live || seen.left <= 0 || seen.left > 4*time.Second {
		t.Errorf("the plugin stopped with live = %v and %v left, want a live context under its own 4s grace",
			seen.live, seen.left)
	}
}
