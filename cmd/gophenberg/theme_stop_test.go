// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

func TestStartThemeHandsBackAStopThatDoesNothingWhenTheBootFails(t *testing.T) {
	t.Parallel()

	logs := &strings.Builder{}
	settings := runConfig{themesDir: t.TempDir(), theme: "uninstalled", nodeBin: "node"}

	manager, stop, err := startTheme(t.Context(), settings, noSettings{}, testLogger(logs))

	if !errors.Is(err, themehost.ErrNotInstalled) {
		t.Fatalf("startTheme() error = %v, want %v", err, themehost.ErrNotInstalled)
	}
	if !strings.Contains(err.Error(), "uninstalled") {
		t.Errorf("error = %v, want it to name the pinned theme", err)
	}
	if manager != nil {
		t.Errorf("manager = %v, want no manager beside the boot failure", manager)
	}
	if stop == nil {
		t.Fatal("stop = nil, want a stop that does nothing")
	}
	stop()
}
