// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

func TestServedKitsNamesWhatThisReleaseServes(t *testing.T) {
	t.Parallel()

	served := themehost.ServedKits()

	if !slices.Equal(served, []string{"0.9.0"}) {
		t.Errorf("ServedKits() = %v, want the kit versions this release serves", served)
	}
	if themehost.NewestKit() != "0.9.0" {
		t.Errorf("NewestKit() = %q, want the newest of them", themehost.NewestKit())
	}
}

func TestServedKitsCannotBeChangedByACaller(t *testing.T) {
	t.Parallel()

	themehost.ServedKits()[0] = "9.9.9"

	if themehost.NewestKit() != "0.9.0" {
		t.Errorf("NewestKit() = %q after a caller wrote to the returned slice", themehost.NewestKit())
	}
}

func TestLoadRefusesAThemeBuiltOnAKitThisReleaseDoesNotServe(t *testing.T) {
	t.Parallel()

	themesDir := themeDirDeclaring(t, "aurora", `"0.8.0"`)

	_, err := themehost.Load(themesDir, "aurora")

	assertRefusedWith(t, err, "kit_unsupported")
}

func TestLoadRefusesAManifestNamingNoPlainKitVersion(t *testing.T) {
	t.Parallel()

	for _, declared := range []string{`"^0.9.0"`, `"latest"`, `""`, `null`} {
		t.Run(declared, func(t *testing.T) {
			t.Parallel()

			themesDir := themeDirDeclaring(t, "aurora", declared)

			_, err := themehost.Load(themesDir, "aurora")

			assertRefusedWith(t, err, "kit_missing")
		})
	}
}

func TestLoadReadsAThemeBuiltOnTheKitThisReleaseServes(t *testing.T) {
	t.Parallel()

	themesDir := themeDirDeclaring(t, "aurora", `"`+themehost.NewestKit()+`"`)

	theme, err := themehost.Load(themesDir, "aurora")

	if err != nil {
		t.Fatalf("Load returned %v, want the theme", err)
	}
	if theme.Kit != themehost.NewestKit() {
		t.Errorf("Kit = %q, want the kit it was built on", theme.Kit)
	}
}

// themeDirDeclaring stages a theme directory whose manifest names the given kit value.
func themeDirDeclaring(t *testing.T, name, kit string) string {
	t.Helper()

	themesDir := t.TempDir()
	dir := filepath.Join(themesDir, name)
	for _, part := range []string{"server", "client"} {
		if err := os.MkdirAll(filepath.Join(dir, part), 0o755); err != nil {
			t.Fatalf("making %s dir: %v", part, err)
		}
	}
	writeFile(t, filepath.Join(dir, "theme.json"),
		`{"name":"`+name+`","version":"0.1.0","kit":`+kit+`}`)
	writeFile(t, filepath.Join(dir, "server", "entry.mjs"), "export const handler = () => {}\n")
	writeFile(t, filepath.Join(dir, "client", "tokens.css"), "body{margin:0}\n")
	return themesDir
}

// assertRefusedWith fails unless the error was refused carrying the code.
func assertRefusedWith(t *testing.T, err error, code string) {
	t.Helper()

	if err == nil {
		t.Fatalf("Load returned no error, want the code %q", code)
	}
	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Load returned %v, want it refused carrying %q", err, code)
	}
	if refused.Code != code {
		t.Errorf("code = %q, want %q", refused.Code, code)
	}
}

func TestServesKitRefusesAVersionItCannotRead(t *testing.T) {
	t.Parallel()

	for _, declared := range []string{"", "^0.9.0", "latest", "0.9"} {
		if themehost.ServesKit(declared) {
			t.Errorf("ServesKit(%q) = true, want a version it cannot read refused", declared)
		}
	}
}
