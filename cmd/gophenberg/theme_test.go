// SPDX-License-Identifier: Apache-2.0

package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testLogger returns a logger writing structured lines the test reads back.
func testLogger(into io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(into, nil))
}

// writeThemeDir lays out a theme directory the boot can load.
func writeThemeDir(t *testing.T, name string) string {
	t.Helper()

	themesDir := t.TempDir()
	dir := filepath.Join(themesDir, name)
	for _, part := range []string{"server", "client"} {
		if err := os.MkdirAll(filepath.Join(dir, part), 0o755); err != nil {
			t.Fatalf("making %s: %v", part, err)
		}
	}
	manifest := `{"name":"` + name + `","version":"0.1.0","kit":"^0.1.0"}`
	if err := os.WriteFile(filepath.Join(dir, "theme.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("writing the manifest: %v", err)
	}
	entry := filepath.Join(dir, "server", "entry.mjs")
	if err := os.WriteFile(entry, []byte("// a theme that is never spawned here\n"), 0o644); err != nil {
		t.Fatalf("writing the entry: %v", err)
	}
	return themesDir
}

func TestLoadRunConfigReadsTheThemeRuntimeSettings(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
		"GOPHENBERG_THEMES_DIR":   "/srv/themes",
		"GOPHENBERG_THEME":        "starter",
		"GOPHENBERG_NODE_BIN":     "/usr/bin/node",
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if settings.themesDir != "/srv/themes" {
		t.Errorf("themesDir = %q, want /srv/themes", settings.themesDir)
	}
	if settings.theme != "starter" {
		t.Errorf("theme = %q, want starter", settings.theme)
	}
	if settings.nodeBin != "/usr/bin/node" {
		t.Errorf("nodeBin = %q, want /usr/bin/node", settings.nodeBin)
	}
}

func TestLoadRunConfigServesTheRendererWhenNoThemeIsNamed(t *testing.T) {
	t.Parallel()

	settings, err := loadRunConfig(testGetenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": unreachableDatabaseURL,
	}))

	if err != nil {
		t.Fatalf("loadRunConfig() error = %v, want nil", err)
	}
	if settings.theme != "" {
		t.Errorf("theme = %q, want empty so the renderer serves", settings.theme)
	}
	if settings.nodeBin != "node" {
		t.Errorf("nodeBin = %q, want node from PATH by default", settings.nodeBin)
	}
}

func TestStartThemeServesTheRendererWhenNoThemeIsNamed(t *testing.T) {
	t.Parallel()

	logs := &strings.Builder{}

	theme, stop, err := startTheme(runConfig{}, testLogger(logs))

	if err != nil {
		t.Fatalf("startTheme() error = %v, want nil", err)
	}
	if theme != nil {
		t.Errorf("theme = %v, want nil so the renderer serves", theme)
	}
	stop()
	if !strings.Contains(logs.String(), "renderer") {
		t.Errorf("logs = %q, want the boot to name the renderer mode", logs.String())
	}
}

func TestStartThemeReportsAThemeThatIsNotInstalled(t *testing.T) {
	t.Parallel()

	logs := &strings.Builder{}
	settings := runConfig{themesDir: t.TempDir(), theme: "missing", nodeBin: "node"}

	_, _, err := startTheme(settings, testLogger(logs))

	if err == nil {
		t.Fatal("startTheme() error = nil, want the boot to refuse a theme it cannot load")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error = %v, want it to name the theme", err)
	}
}

func TestStartThemeNamesTheThemeItBootsWith(t *testing.T) {
	t.Parallel()

	logs := &strings.Builder{}
	settings := runConfig{themesDir: writeThemeDir(t, "starter"), theme: "starter", nodeBin: "node"}

	theme, stop, err := startTheme(settings, testLogger(logs))

	if err != nil {
		t.Fatalf("startTheme() error = %v, want nil", err)
	}
	if theme == nil {
		t.Fatal("theme = nil, want the supervised theme")
	}
	stop()
	if !strings.Contains(logs.String(), "starter") {
		t.Errorf("logs = %q, want the boot to name the theme", logs.String())
	}
}
