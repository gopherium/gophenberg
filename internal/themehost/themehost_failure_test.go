// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// denyStat leaves a directory readable but not searchable, skipping when the user keeps the right anyway.
func denyStat(t *testing.T, dir string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which searches a directory whatever its mode says")
	}
	if err := os.Chmod(dir, 0o400); err != nil {
		t.Fatalf("denying search on %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

func TestLoadReportsAnEntryItCannotMeasure(t *testing.T) {
	t.Parallel()

	themesDir := writeTheme(t)
	denyStat(t, filepath.Join(themesDir, "starter", "client"))

	_, err := themehost.Load(themesDir, "starter")

	if err == nil {
		t.Fatal("Load() error = nil, want the entry it cannot measure reported")
	}
	var refused *themehost.Error
	if errors.As(err, &refused) {
		t.Errorf("Load() refused with %q, want a plain read failure", refused.Code)
	}
	const measuring = "themehost: reading client/tokens.css: "
	if !strings.HasPrefix(err.Error(), measuring) {
		t.Errorf("Load() error = %q, want it to open with %q", err, measuring)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("Load() error = %v, want the permission the unsearchable directory denies", err)
	}
	var measured *fs.PathError
	if !errors.As(err, &measured) {
		t.Fatalf("Load() error = %v, want the path it could not measure carried", err)
	}
	if want := filepath.Join(themesDir, "starter", "client", "tokens.css"); measured.Path != want {
		t.Errorf("Load() path = %q, want %q", measured.Path, want)
	}
}
