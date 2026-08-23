// SPDX-License-Identifier: Apache-2.0

package themehost_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// manifestFor returns a manifest body naming a theme built on the kit this release serves.
func manifestFor(name, version string) string {
	return fmt.Sprintf(`{"name":%q,"version":%q,"kit":%q}`, name, version, themehost.NewestKit())
}

// writeTheme lays out a valid theme directory and returns the themes directory holding it.
func writeTheme(t *testing.T) string {
	t.Helper()

	const name = "starter"
	themesDir := t.TempDir()
	dir := filepath.Join(themesDir, name)
	if err := os.MkdirAll(filepath.Join(dir, "server"), 0o755); err != nil {
		t.Fatalf("making server dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "client"), 0o755); err != nil {
		t.Fatalf("making client dir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "theme.json"), manifestFor(name, "0.1.0"))
	writeFile(t, filepath.Join(dir, "server", "entry.mjs"), "export const handler = () => {}\n")
	writeFile(t, filepath.Join(dir, "client", "tokens.css"), "body{margin:0}\n")
	return themesDir
}

// writeFile writes a file the test depends on.
func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func TestLoadReadsAValidThemeDirectory(t *testing.T) {
	t.Parallel()

	themesDir := writeTheme(t)

	theme, err := themehost.Load(themesDir, "starter")

	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if theme == nil {
		t.Fatal("Load() theme = nil, want a theme")
	}
	for _, field := range []struct {
		name string
		got  string
		want string
	}{
		{name: "Name", got: theme.Name, want: "starter"},
		{name: "Version", got: theme.Version, want: "0.1.0"},
		{name: "Kit", got: theme.Kit, want: themehost.NewestKit()},
		{name: "Dir", got: theme.Dir, want: filepath.Join(themesDir, "starter")},
		{name: "Entry", got: theme.Entry, want: filepath.Join(themesDir, "starter", "server", "entry.mjs")},
	} {
		if field.got != field.want {
			t.Errorf("%s = %q, want %q", field.name, field.got, field.want)
		}
	}
}

func TestLoadServesTheRendererWhenNoThemeIsNamed(t *testing.T) {
	t.Parallel()

	theme, err := themehost.Load(filepath.Join(t.TempDir(), "no-such-directory"), "")

	if err != nil {
		t.Fatalf("Load() error = %v, want nil", err)
	}
	if theme != nil {
		t.Errorf("Load() theme = %v, want nil", theme)
	}
}

func TestLoadRefusesADirectoryThatBreaksTheContract(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		break_  func(t *testing.T, dir string)
		wantAll []string
	}{
		{
			name:    "no manifest",
			break_:  func(t *testing.T, dir string) { remove(t, filepath.Join(dir, "theme.json")) },
			wantAll: []string{"theme.json"},
		},
		{
			name:    "manifest is not json",
			break_:  func(t *testing.T, dir string) { writeFile(t, filepath.Join(dir, "theme.json"), "{not json") },
			wantAll: []string{"theme.json"},
		},
		{
			name: "manifest names another theme",
			break_: func(t *testing.T, dir string) {
				writeFile(t, filepath.Join(dir, "theme.json"), manifestFor("other", "0.1.0"))
			},
			wantAll: []string{"theme.json", "other", "starter"},
		},
		{
			name:    "no server entry",
			break_:  func(t *testing.T, dir string) { remove(t, filepath.Join(dir, "server", "entry.mjs")) },
			wantAll: []string{"server/entry.mjs"},
		},
		{
			name:    "no client assets",
			break_:  func(t *testing.T, dir string) { remove(t, filepath.Join(dir, "client")) },
			wantAll: []string{"client"},
		},
		{
			name: "a symlink hides in the tree",
			break_: func(t *testing.T, dir string) {
				link(t, "/etc/passwd", filepath.Join(dir, "client", "secrets.css"))
			},
			wantAll: []string{"symlink", "secrets.css"},
		},
		{
			name: "the tree is larger than the cap",
			break_: func(t *testing.T, dir string) {
				grow(t, filepath.Join(dir, "client", "huge.bin"), themehost.MaxSize+1)
			},
			wantAll: []string{"larger"},
		},
		{
			name: "the server entry is a directory",
			break_: func(t *testing.T, dir string) {
				entry := filepath.Join(dir, "server", "entry.mjs")
				remove(t, entry)
				if err := os.Mkdir(entry, 0o755); err != nil {
					t.Fatalf("making %s a directory: %v", entry, err)
				}
			},
			wantAll: []string{"server/entry.mjs", "wrong kind"},
		},
		{
			name: "the client assets are a file",
			break_: func(t *testing.T, dir string) {
				client := filepath.Join(dir, "client")
				remove(t, client)
				writeFile(t, client, "not a directory")
			},
			wantAll: []string{"client", "wrong kind"},
		},
		{
			name: "the manifest cannot be read",
			break_: func(t *testing.T, dir string) {
				denyRead(t, filepath.Join(dir, "theme.json"))
			},
			wantAll: []string{"theme.json"},
		},
		{
			name: "a directory in the tree cannot be read",
			break_: func(t *testing.T, dir string) {
				denyRead(t, filepath.Join(dir, "client"))
			},
			wantAll: []string{"starter"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			themesDir := writeTheme(t)
			testCase.break_(t, filepath.Join(themesDir, "starter"))

			theme, err := themehost.Load(themesDir, "starter")

			if err == nil {
				t.Fatalf("Load() error = nil, want it refused naming %v", testCase.wantAll)
			}
			if theme != nil {
				t.Errorf("Load() theme = %v, want nil", theme)
			}
			for _, want := range testCase.wantAll {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("Load() error = %q, want it to name %q", err, want)
				}
			}
		})
	}
}

func TestLoadRefusesAThemeThatIsNotInstalled(t *testing.T) {
	t.Parallel()

	_, err := themehost.Load(t.TempDir(), "missing")

	if err == nil {
		t.Fatal("Load() error = nil, want it refused")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("Load() error = %q, want it to name the theme", err)
	}
}

// remove deletes a path the test no longer wants.
func remove(t *testing.T, path string) {
	t.Helper()

	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("removing %s: %v", path, err)
	}
}

// link points a path at a target outside the tree.
func link(t *testing.T, target, path string) {
	t.Helper()

	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("linking %s: %v", path, err)
	}
}

// denyRead takes read permission away from a path, skipping when the user keeps it anyway.
func denyRead(t *testing.T, path string) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which reads a file whatever its mode says")
	}
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("denying read on %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o755) })
}

// grow writes a file of the given size.
func grow(t *testing.T, path string, size int64) {
	t.Helper()

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()
	if err := file.Truncate(size); err != nil {
		t.Fatalf("growing %s: %v", path, err)
	}
}

func TestLoadNamesABrokenSymlinkAsASymlinkRatherThanAMissingFile(t *testing.T) {
	t.Parallel()

	themesDir := writeTheme(t)
	entry := filepath.Join(themesDir, "starter", "server", "entry.mjs")
	remove(t, entry)
	if err := os.Symlink(filepath.Join(themesDir, "nowhere.mjs"), entry); err != nil {
		t.Fatalf("linking the server entry: %v", err)
	}

	_, err := themehost.Load(themesDir, "starter")

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Load() error = %v, want it refused", err)
	}
	if refused.Code != "symlink_present" {
		t.Errorf("Load() code = %q, want symlink_present", refused.Code)
	}
}

func TestLoadCarriesTheMetadataItsErrorTemplateNames(t *testing.T) {
	t.Parallel()

	themesDir := writeTheme(t)
	remove(t, filepath.Join(themesDir, "starter", "theme.json"))

	_, err := themehost.Load(themesDir, "starter")

	var refused *themehost.Error
	if !errors.As(err, &refused) {
		t.Fatalf("Load() error = %v, want it refused", err)
	}
	for _, key := range []string{"name", "file"} {
		if _, found := refused.Held[key]; !found {
			t.Errorf("Load() error holds no %q, so its template cannot be filled", key)
		}
	}
}

func TestLoadReportsAPathItCannotRead(t *testing.T) {
	t.Parallel()

	themesDir := t.TempDir()
	dir := filepath.Join(themesDir, "starter")
	if err := os.MkdirAll(filepath.Join(dir, "client"), 0o755); err != nil {
		t.Fatalf("making the client dir: %v", err)
	}
	writeFile(t, filepath.Join(dir, "theme.json"), manifestFor("starter", "0.1.0"))
	writeFile(t, filepath.Join(dir, "server"), "a file where the directory belongs")

	_, err := themehost.Load(themesDir, "starter")

	if err == nil {
		t.Fatal("Load() error = nil, want the unreadable path reported")
	}
	var refused *themehost.Error
	if errors.As(err, &refused) {
		t.Errorf("Load() refused with %q, want a plain read failure", refused.Code)
	}
}
