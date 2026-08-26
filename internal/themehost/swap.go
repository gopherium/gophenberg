// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"context"
	"errors"
	"sync"
	"time"
)

// ErrStartFailed reports a theme the supervisor stopped trying to start.
var ErrStartFailed = errors.New("themehost: the theme did not start")

// ErrStopped reports a theme the supervisor was told to stop.
var ErrStopped = errors.New("themehost: the theme was stopped")

// Await blocks until the theme serves, its start fails, it is stopped, or ctx ends.
func (s *Supervisor) Await(ctx context.Context) error {
	ticker := time.NewTicker(readyPoll)
	defer ticker.Stop()
	for {
		if s.Healthy() {
			return nil
		}
		select {
		case <-s.done:
			if s.StartFailed() {
				return ErrStartFailed
			}
			return ErrStopped
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// Holder is the theme the public site is served through, replaceable while serving.
type Holder struct {
	mu      sync.RWMutex
	current *Supervisor
}

// NewHolder returns a [Holder] serving nothing.
func NewHolder() *Holder {
	return &Holder{}
}

// Healthy reports whether the held theme is serving.
func (h *Holder) Healthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.current != nil && h.current.Healthy()
}

// StartFailed reports whether the held theme stopped trying to start.
func (h *Holder) StartFailed() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.current != nil && h.current.StartFailed()
}

// Target returns the address the held theme serves on, empty while none does.
func (h *Holder) Target() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.current == nil {
		return ""
	}
	return h.current.Target()
}

// Serving returns the theme answering the public site, and whether it is answering.
func (h *Holder) Serving() (string, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.current == nil {
		return "", false
	}
	return h.current.Name(), h.current.Healthy()
}

// Swap holds next instead, returning the supervisor it held.
func (h *Holder) Swap(next *Supervisor) *Supervisor {
	h.mu.Lock()
	defer h.mu.Unlock()
	previous := h.current
	h.current = next
	return previous
}
