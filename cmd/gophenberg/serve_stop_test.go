// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gopherium/pluginkit"
)

// errListenerGone stands for a listener that stops handing over connections for good.
var errListenerGone = errors.New("the listener gave out")

// gatedListener hands over its first connection, then fails every later accept once its gate opens.
type gatedListener struct {
	net.Listener
	gate     chan struct{}
	accepted atomic.Int32
}

// Accept returns the first connection, then waits for the gate and fails for good.
func (l *gatedListener) Accept() (net.Conn, error) {
	if l.accepted.Add(1) == 1 {
		return l.Listener.Accept()
	}
	<-l.gate
	return nil, errListenerGone
}

// stoppingPlugin notes when the host stops it.
type stoppingPlugin struct {
	stopped *atomic.Bool
}

// ID returns the plugin's identifier.
func (stoppingPlugin) ID() string {
	return "stopping"
}

// Start starts nothing.
func (stoppingPlugin) Start(context.Context) error {
	return nil
}

// Stop notes that the host stopped the plugin.
func (p stoppingPlugin) Stop(context.Context) error {
	p.stopped.Store(true)
	return nil
}

func TestAFailedServeClosesOpenConnectionsBeforeThePluginsStop(t *testing.T) {
	t.Parallel()

	var stopped, servedAfterStop atomic.Bool
	inner, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	listener := &gatedListener{Listener: inner, gate: make(chan struct{})}
	httpServer := &http.Server{
		ReadHeaderTimeout: time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if stopped.Load() {
				servedAfterStop.Store(true)
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	}
	host := pluginkit.NewHost(stoppingPlugin{stopped: &stopped})
	if err := host.Start(t.Context()); err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	done := make(chan error, 1)
	go func() {
		serve := func() error { return httpServer.Serve(listener) }
		done <- serveUntilDone(t.Context(), httpServer, serve, host, slog.New(slog.DiscardHandler))
	}()
	client := &http.Client{Transport: &http.Transport{}, Timeout: 5 * time.Second}
	probe := "http://" + inner.Addr().String() + "/"
	first, err := client.Get(probe)
	if err != nil {
		t.Fatalf("the first request: %v, want it served", err)
	}
	_ = first.Body.Close()

	close(listener.gate)
	if err := <-done; !errors.Is(err, errListenerGone) {
		t.Errorf("serveUntilDone() error = %v, want %v", err, errListenerGone)
	}
	if second, err := client.Get(probe); err == nil {
		_ = second.Body.Close()
	}

	if servedAfterStop.Load() {
		t.Error("a request reached the server after the plugins stopped, want every open connection closed first")
	}
}
