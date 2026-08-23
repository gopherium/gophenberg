// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plantDir makes a directory the test depends on and returns it.
func plantDir(t *testing.T, path string) string {
	t.Helper()

	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("making %s: %v", path, err)
	}
	return path
}

// plantFile writes a file the test depends on.
func plantFile(t *testing.T, path string) {
	t.Helper()

	if err := os.WriteFile(path, []byte("held"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

// denyWrite takes write permission away from a directory, skipping when the user keeps it anyway.
func denyWrite(t *testing.T, path string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which writes to a directory whatever its mode says")
	}
	if err := os.Chmod(path, 0o500); err != nil {
		t.Fatalf("denying write on %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
}

func TestSwapInReportsAnInstalledThemeItCannotRetire(t *testing.T) {
	t.Parallel()

	unwritableThemesDir := t.TempDir()
	installed := plantDir(t, filepath.Join(unwritableThemesDir, "aurora"))
	staged := plantDir(t, filepath.Join(t.TempDir(), "aurora"))
	denyWrite(t, unwritableThemesDir)

	err := swapIn(staged, installed)

	if err == nil {
		t.Fatal("swapIn() = nil, want the theme it cannot retire reported")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("swapIn() = %v, want a failure the missing case does not swallow", err)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("swapIn() = %v, want the denied write reported", err)
	}
	if !strings.Contains(err.Error(), "retiring the installed theme") {
		t.Errorf("swapIn() = %v, want the retirement step named", err)
	}
}

func TestSwapInPutsTheRetiredThemeBackWhenTheStagedOneWillNotMove(t *testing.T) {
	t.Parallel()

	themesDir := t.TempDir()
	installed := plantDir(t, filepath.Join(themesDir, "aurora"))
	plantFile(t, filepath.Join(installed, "theme.json"))
	missingStaged := filepath.Join(themesDir, "no-such-staging", "aurora")
	retired := filepath.Join(themesDir, ".aurora.retired")

	err := swapIn(missingStaged, installed)

	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("swapIn() = %v, want the missing staged theme reported", err)
	}
	if !strings.Contains(err.Error(), "installing the theme") {
		t.Errorf("swapIn() = %v, want the install step named", err)
	}
	if _, statErr := os.Stat(filepath.Join(installed, "theme.json")); statErr != nil {
		t.Errorf("the installed theme reads %v, want it put back", statErr)
	}
	if _, statErr := os.Stat(retired); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("the retired copy reads %v, want the rollback to have moved it back", statErr)
	}
}
