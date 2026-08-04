// SPDX-License-Identifier: Apache-2.0

// Package themehost validates and runs the prebuilt theme artifacts a site is served from.
package themehost

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// MaxSize is the largest a theme directory may be.
const MaxSize = 64 << 20

// ErrNotInstalled reports that no theme is installed under the name.
var ErrNotInstalled = errors.New("themehost: theme not installed")

// entryPath is where a theme directory keeps the server the supervisor spawns.
var entryPath = filepath.Join("server", "entry.mjs")

// clientPath is where a theme directory keeps the assets the server serves.
const clientPath = "client"

// manifestPath is where a theme directory declares itself.
const manifestPath = "theme.json"

// Theme is a validated theme directory.
type Theme struct {
	Name    string
	Version string
	Kit     string
	Dir     string
	Entry   string
}

// manifest is what a theme directory declares about itself.
type manifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Kit     string `json:"kit"`
}

// Load returns the theme installed under a name, or nothing when no theme is named.
func Load(themesDir, name string) (*Theme, error) {
	if name == "" {
		return nil, nil
	}
	dir := filepath.Join(themesDir, name)
	declared, err := inspect(dir, name)
	if err != nil {
		return nil, err
	}
	return &Theme{
		Name:    declared.Name,
		Version: declared.Version,
		Kit:     declared.Kit,
		Dir:     dir,
		Entry:   filepath.Join(dir, entryPath),
	}, nil
}

// inspect returns what a theme directory declares once every rule holds.
func inspect(dir, name string) (manifest, error) {
	declared, err := readManifest(dir, name)
	if err != nil {
		return declared, err
	}
	if declared.Name != name {
		return declared, fmt.Errorf("themehost: %s names theme %q, but it is installed as %q",
			filepath.Join(name, manifestPath), declared.Name, name)
	}
	if err := requireParts(dir, name); err != nil {
		return declared, err
	}
	return declared, walk(dir, name)
}

// readManifest returns the manifest a theme directory declares itself with.
func readManifest(dir, name string) (manifest, error) {
	var declared manifest
	raw, err := os.ReadFile(filepath.Join(dir, manifestPath))
	if errors.Is(err, fs.ErrNotExist) {
		return declared, fmt.Errorf("%w: %s declares no %s", ErrNotInstalled, name, manifestPath)
	}
	if err != nil {
		return declared, fmt.Errorf("themehost: reading %s: %w", filepath.Join(name, manifestPath), err)
	}
	if err := json.Unmarshal(raw, &declared); err != nil {
		return declared, fmt.Errorf("themehost: %s is not valid JSON: %w", filepath.Join(name, manifestPath), err)
	}
	return declared, nil
}

// requireParts reports whether a theme directory holds the server and the assets it is served from.
func requireParts(dir, name string) error {
	for _, part := range []struct {
		path  string
		isDir bool
	}{
		{path: entryPath, isDir: false},
		{path: clientPath, isDir: true},
	} {
		info, err := os.Stat(filepath.Join(dir, part.path))
		if err != nil {
			return fmt.Errorf("themehost: %s holds no %s", name, filepath.ToSlash(part.path))
		}
		if info.IsDir() != part.isDir {
			return fmt.Errorf("themehost: %s is the wrong kind of file in %s", filepath.ToSlash(part.path), name)
		}
	}
	return nil
}

// walk reports whether a theme tree is free of symlinks and within the size cap.
func walk(dir, name string) error {
	var total int64
	return filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("themehost: reading %s: %w", name, err)
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("themehost: %s holds a symlink at %s, which a theme may not do",
				name, relative(dir, path))
		}
		size, err := sizeOf(entry)
		if err != nil {
			return fmt.Errorf("themehost: reading %s: %w", relative(dir, path), err)
		}
		total += size
		if total > MaxSize {
			return fmt.Errorf("themehost: %s is larger than the %d byte cap", name, int64(MaxSize))
		}
		return nil
	})
}

// sizeOf returns the bytes a directory entry occupies.
func sizeOf(entry fs.DirEntry) (int64, error) {
	if entry.IsDir() {
		return 0, nil
	}
	info, err := entry.Info()
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// relative returns a path as it reads from the theme directory.
func relative(dir, path string) string {
	return filepath.ToSlash(strings.TrimPrefix(path, dir+string(filepath.Separator)))
}
