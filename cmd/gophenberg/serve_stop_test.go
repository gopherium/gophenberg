// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync"
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
	var served error
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		serve := func() error { return httpServer.Serve(listener) }
		served = serveUntilDone(t.Context(), httpServer, serve, host, slog.New(slog.DiscardHandler))
	}()
	openGate := sync.OnceFunc(func() { close(listener.gate) })
	t.Cleanup(func() {
		openGate()
		_ = inner.Close()
		<-finished
	})
	client := &http.Client{Transport: &http.Transport{}, Timeout: 5 * time.Second}
	probe := "http://" + inner.Addr().String() + "/"
	first, err := client.Get(probe)
	if err != nil {
		t.Fatalf("the first request: %v, want it served", err)
	}
	_ = first.Body.Close()

	openGate()
	<-finished
	if !errors.Is(served, errListenerGone) {
		t.Errorf("serveUntilDone() error = %v, want %v", served, errListenerGone)
	}
	if !stopped.Load() {
		t.Fatal("the plugins never stopped, want the host stopped once serving failed")
	}
	if second, err := client.Get(probe); err == nil {
		_ = second.Body.Close()
	}

	if servedAfterStop.Load() {
		t.Error("a request reached the server after the plugins stopped, want every open connection closed first")
	}
}
