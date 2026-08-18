// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Installed describes one theme sitting in the library.
type Installed struct {
	// Name is the directory the theme is installed under.
	Name string
	// Version is what the manifest declares, empty when the theme will not load.
	Version string
	// Broken is why the theme will not load, empty when it loads.
	Broken string
	// Active reports whether this theme is the operator's current choice.
	Active bool
	// Serving reports whether the public site is answered by this theme right now.
	Serving bool
}

// Library is the managed themes directory the admin lists and installs into.
type Library struct {
	dir string
}

// NewLibrary returns the library over a managed themes directory.
func NewLibrary(dir string) *Library { return &Library{dir: dir} }

// Dir returns the directory the library manages.
func (l *Library) Dir() string { return l.dir }

// List returns the installed themes in name order, reporting those that will not load.
func (l *Library) List() ([]Installed, error) {
	entries, err := os.ReadDir(l.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("themehost: reading the themes directory: %w", err)
	}
	installed := make([]Installed, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		installed = append(installed, l.describe(entry.Name()))
	}
	sort.Slice(installed, func(i, j int) bool { return installed[i].Name < installed[j].Name })
	return installed, nil
}

// Install unpacks archive as the named theme, replacing it only once it validates.
func (l *Library) Install(name string, archive io.ReaderAt, size int64) error {
	if err := validName(name); err != nil {
		return err
	}
	if l.dir == "" {
		return refuseHolding("themes_directory_unset",
			"no themes directory is configured, set GOPHENBERG_THEMES_DIR",
			map[string]any{"setting": "GOPHENBERG_THEMES_DIR"},
			"themehost: installing %s with no themes directory", name)
	}
	if err := os.MkdirAll(l.dir, 0o755); err != nil {
		return fmt.Errorf("themehost: creating the themes directory: %w", err)
	}
	_, err := Install(l.dir, name, archive, size)
	return err
}

// describe returns what the library reports about one installed directory.
func (l *Library) describe(name string) Installed {
	theme, err := Load(l.dir, name)
	if err == nil {
		return Installed{Name: name, Version: theme.Version}
	}
	return Installed{Name: name, Broken: reasonOf(err)}
}

// validName reports whether a name may be a directory in the library.
func validName(name string) error {
	if name == "" || strings.HasPrefix(name, ".") || name != filepath.Base(name) {
		return refuseHolding("theme_name_malformed", "the theme name is not allowed",
			map[string]any{"name": name}, "themehost: %q is not a theme name", name)
	}
	return nil
}

// reasonOf returns the refusal reason behind an error, or a general one.
func reasonOf(err error) string {
	var refusal *Refusal
	if errors.As(err, &refusal) {
		return refusal.Reason
	}
	return "the theme could not be read"
}
