// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// libraryWith returns a library over a directory holding the named themes.
func libraryWith(t *testing.T, names ...string) *themehost.Library {
	t.Helper()

	library := themehost.NewLibrary(t.TempDir())
	for _, name := range names {
		archive := validArchive(t, name)
		if err := library.Install(name, bytes.NewReader(archive), int64(len(archive))); err != nil {
			t.Fatalf("installing %s: %v", name, err)
		}
	}
	return library
}

func TestAnEmptyLibraryListsNothing(t *testing.T) {
	t.Parallel()

	installed, err := libraryWith(t).List()

	if err != nil {
		t.Fatalf("List() = %v, want an empty library listed", err)
	}
	if len(installed) != 0 {
		t.Errorf("List() = %v, want nothing installed", installed)
	}
}

func TestTheLibraryListsWhatIsInstalledInOrder(t *testing.T) {
	t.Parallel()

	installed, err := libraryWith(t, "riverbed", "aurora").List()

	if err != nil {
		t.Fatalf("List() = %v, want the installed themes", err)
	}
	if len(installed) != 2 {
		t.Fatalf("List() = %v, want both themes", installed)
	}
	if installed[0].Name != "aurora" || installed[1].Name != "riverbed" {
		t.Errorf("List() = %v, want the themes in name order", installed)
	}
	if installed[0].Version != "1.0.0" {
		t.Errorf("Version = %q, want the version the manifest declares", installed[0].Version)
	}
}

func TestTheLibraryReportsAThemeThatWillNotLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	library := themehost.NewLibrary(dir)
	if err := os.MkdirAll(filepath.Join(dir, "broken"), 0o755); err != nil {
		t.Fatalf("planting a broken theme: %v", err)
	}

	installed, err := library.List()

	if err != nil {
		t.Fatalf("List() = %v, want a broken theme listed rather than an error", err)
	}
	if len(installed) != 1 {
		t.Fatalf("List() = %v, want the broken theme listed", installed)
	}
	if installed[0].Broken == "" {
		t.Errorf("Broken = %q, want the reason it will not load", installed[0].Broken)
	}
}

func TestTheLibraryReportsAThemeItCannotEvenRead(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	library := themehost.NewLibrary(dir)
	if err := os.MkdirAll(filepath.Join(dir, "broken", "theme.json"), 0o755); err != nil {
		t.Fatalf("planting an unreadable manifest: %v", err)
	}

	installed, err := library.List()

	if err != nil {
		t.Fatalf("List() = %v, want the theme listed rather than an error", err)
	}
	if len(installed) != 1 || installed[0].Broken == "" {
		t.Errorf("List() = %v, want the theme reported as broken", installed)
	}
}

func TestTheLibraryIgnoresLooseFilesAndStagingDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	library := themehost.NewLibrary(dir)
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("loose"), 0o644); err != nil {
		t.Fatalf("planting a loose file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".aurora-staging-1"), 0o755); err != nil {
		t.Fatalf("planting a staging directory: %v", err)
	}

	installed, err := library.List()

	if err != nil {
		t.Fatalf("List() = %v, want the library listed", err)
	}
	if len(installed) != 0 {
		t.Errorf("List() = %v, want neither loose files nor staging directories listed", installed)
	}
}

func TestTheLibraryReportsADirectoryItDoesNotHaveAsEmpty(t *testing.T) {
	t.Parallel()

	for _, dir := range []string{filepath.Join(t.TempDir(), "absent"), ""} {
		installed, err := themehost.NewLibrary(dir).List()

		if err != nil {
			t.Fatalf("List() on %q = %v, want it read as empty", dir, err)
		}
		if len(installed) != 0 {
			t.Errorf("List() on %q = %v, want nothing installed", dir, installed)
		}
	}
}

func TestTheLibraryRefusesAThemeNameThatIsNotOne(t *testing.T) {
	t.Parallel()

	library := themehost.NewLibrary(t.TempDir())
	archive := validArchive(t, "aurora")

	for _, name := range []string{"", ".", "..", "../escape", "with/slash", ".hidden"} {
		if err := library.Install(name, bytes.NewReader(archive), int64(len(archive))); err == nil {
			t.Errorf("Install(%q) = nil, want a name that is not a theme name refused", name)
		}
	}
}

func TestTheLibraryRefusesAnUploadWithNoDirectoryToInstallInto(t *testing.T) {
	t.Parallel()

	library := themehost.NewLibrary("")
	archive := validArchive(t, "aurora")

	err := library.Install("aurora", bytes.NewReader(archive), int64(len(archive)))

	var refusal *themehost.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("Install() = %v, want a refusal the admin can read", err)
	}
	if refusal.Reason != "no themes directory is configured, set GOPHENBERG_THEMES_DIR" {
		t.Errorf("Reason = %q, want it to name the missing setting", refusal.Reason)
	}
}

func TestTheLibraryCreatesItsDirectoryOnTheFirstInstall(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "themes")
	library := themehost.NewLibrary(dir)
	archive := validArchive(t, "aurora")

	if err := library.Install("aurora", bytes.NewReader(archive), int64(len(archive))); err != nil {
		t.Fatalf("Install() = %v, want the directory created", err)
	}

	if _, err := themehost.Load(dir, "aurora"); err != nil {
		t.Errorf("the installed theme does not load: %v", err)
	}
}
