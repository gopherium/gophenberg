// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"testing"
	"time"
)

// timedConfig returns a run config carrying distinct timings the wiring must carry through.
func timedConfig() runConfig {
	return runConfig{
		nodeBin:            "/usr/bin/node",
		addr:               "localhost:9999",
		mediaDir:           "/srv/media",
		themeReadyTimeout:  45 * time.Second,
		themeBackoff:       250 * time.Millisecond,
		themeMaxBackoff:    time.Minute,
		themeStopGrace:     5 * time.Second,
		themeProxyTimeout:  20 * time.Second,
		themeStartAttempts: 9,
		mediaUploadCap:     64 << 20,
	}
}

func TestSupervisionCarriesEveryTimingTheEnvironmentNamed(t *testing.T) {
	t.Parallel()

	settings := timedConfig()

	held := supervisionFrom(settings, testLogger(io.Discard))

	for name, asked := range map[string]struct {
		stood time.Duration
		want  time.Duration
	}{
		"ReadyTimeout": {held.ReadyTimeout, settings.themeReadyTimeout},
		"Backoff":      {held.Backoff, settings.themeBackoff},
		"MaxBackoff":   {held.MaxBackoff, settings.themeMaxBackoff},
		"StopGrace":    {held.StopGrace, settings.themeStopGrace},
	} {
		if asked.stood != asked.want {
			t.Errorf("%s = %v, want %v", name, asked.stood, asked.want)
		}
	}
	if held.MaxAttempts != settings.themeStartAttempts {
		t.Errorf("MaxAttempts = %d, want %d", held.MaxAttempts, settings.themeStartAttempts)
	}
	if held.NodeBin != settings.nodeBin || held.APIAddr != settings.addr {
		t.Errorf("NodeBin = %q APIAddr = %q, want the run config's own", held.NodeBin, held.APIAddr)
	}
	if held.Logger == nil {
		t.Error("Logger = nil, want the run logger carried")
	}
}

func TestMediaConfigCarriesTheUploadCapTheEnvironmentNamed(t *testing.T) {
	t.Parallel()

	settings := timedConfig()

	held := mediaConfigFrom(settings)

	if held.MaxSize != settings.mediaUploadCap {
		t.Errorf("MaxSize = %d, want %d", held.MaxSize, settings.mediaUploadCap)
	}
	if held.Dir != settings.mediaDir {
		t.Errorf("Dir = %q, want %q", held.Dir, settings.mediaDir)
	}
}
