// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// themesDirEnv points the artifact test at a directory holding a theme a real build produced.
const themesDirEnv = "GOPHENBERG_TEST_THEMES_DIR"

func TestSupervisorRunsAThemeARealBuildProduced(t *testing.T) {
	t.Parallel()

	themesDir := os.Getenv(themesDirEnv)
	if themesDir == "" {
		t.Skipf("%s is unset, so no built theme is staged to run", themesDirEnv)
	}

	theme, err := themehost.Load(themesDir, "starter")
	if err != nil {
		t.Fatalf("loading the built theme: %v", err)
	}

	supervisor := themehost.NewSupervisor(themehost.SupervisorConfig{
		Theme:        theme,
		NodeBin:      nodeBin(t),
		APIAddr:      "127.0.0.1:8081",
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		ReadyTimeout: 20 * time.Second,
	})
	supervisor.Start()
	t.Cleanup(supervisor.Stop)

	waitFor(t, "the built theme to report ready", supervisor.Healthy)

	response, err := http.Get(supervisor.Target() + "/_gophenberg/health")
	if err != nil {
		t.Fatalf("reading the probe of the built theme: %v", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Errorf("probe status = %d, want 200", response.StatusCode)
	}
}
