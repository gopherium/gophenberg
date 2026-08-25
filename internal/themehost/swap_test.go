// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"context"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/themehost"
)

func TestAwaitReturnsOnceTheThemeServes(t *testing.T) {
	t.Parallel()

	supervisor, _ := startSupervisor(t, "healthy", nil)

	if err := supervisor.Await(t.Context()); err != nil {
		t.Fatalf("Await() = %v, want the theme reported serving", err)
	}
	if !supervisor.Healthy() {
		t.Error("want the theme healthy once Await returned")
	}
}

func TestAwaitReportsAThemeThatGivesUp(t *testing.T) {
	t.Parallel()

	supervisor, _ := startSupervisor(t, "crash", func(c *themehost.SupervisorConfig) {
		c.MaxAttempts = 2
		c.Backoff = time.Millisecond
	})

	err := supervisor.Await(t.Context())

	if err == nil {
		t.Fatal("Await() = nil, want the failed start reported")
	}
	if supervisor.Healthy() {
		t.Error("want the theme unhealthy after its start failed")
	}
}

func TestAwaitStopsWhenTheCallerDoes(t *testing.T) {
	t.Parallel()

	supervisor, _ := startSupervisor(t, "deaf", nil)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	if err := supervisor.Await(ctx); err == nil {
		t.Error("Await() = nil, want the caller's cancellation reported")
	}
}

func TestHolderWithoutAThemeIsNotServing(t *testing.T) {
	t.Parallel()

	holder := themehost.NewHolder()

	if holder.Healthy() {
		t.Error("want an empty holder to report nothing serving")
	}
	if holder.Target() != "" {
		t.Errorf("Target() = %q, want empty", holder.Target())
	}
	if holder.StartFailed() {
		t.Error("want an empty holder to report no failed start")
	}
}

func TestHolderSaysWhenTheHeldThemeFailedItsStart(t *testing.T) {
	t.Parallel()

	supervisor, _ := startSupervisor(t, "deaf", func(c *themehost.SupervisorConfig) {
		c.ReadyTimeout = 150 * time.Millisecond
		c.MaxAttempts = 1
	})
	holder := themehost.NewHolder()
	holder.Swap(supervisor)

	waitFor(t, "the held theme to fail its start", holder.StartFailed)

	if holder.Healthy() {
		t.Error("want a theme whose start failed reported as not serving")
	}
}

func TestHolderReportsTheThemeItHolds(t *testing.T) {
	t.Parallel()

	supervisor, _ := startSupervisor(t, "healthy", nil)
	holder := themehost.NewHolder()
	holder.Swap(supervisor)

	waitFor(t, "the held theme to report ready", holder.Healthy)

	if holder.Target() != supervisor.Target() {
		t.Errorf("Target() = %q, want the held theme's address", holder.Target())
	}
}

func TestHolderHandsBackWhatItHeld(t *testing.T) {
	t.Parallel()

	first, _ := startSupervisor(t, "healthy", nil)
	second, _ := startSupervisor(t, "healthy", nil)
	holder := themehost.NewHolder()
	holder.Swap(first)

	previous := holder.Swap(second)

	if previous != first {
		t.Error("Swap() did not hand back the theme it held")
	}
	waitFor(t, "the new theme to report ready", holder.Healthy)
	if holder.Target() != second.Target() {
		t.Errorf("Target() = %q, want the new theme's address", holder.Target())
	}
}

func TestHolderReturnsToNothingServing(t *testing.T) {
	t.Parallel()

	supervisor, _ := startSupervisor(t, "healthy", nil)
	holder := themehost.NewHolder()
	holder.Swap(supervisor)
	waitFor(t, "the held theme to report ready", holder.Healthy)

	holder.Swap(nil)

	if holder.Healthy() {
		t.Error("want nothing serving once the holder is emptied")
	}
}
