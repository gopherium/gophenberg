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

// MaxEntries is how many files an archive may carry.
const MaxEntries = 10_000

// dirSize is what one unpacked directory is charged against the size cap at install.
const dirSize = 4096

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
		return declared, refuseHolding("manifest_name_mismatch", "the name does not match",
			map[string]any{"file": manifestPath, "declared": declared.Name, "installed": name},
			"themehost: %s names theme %q, but it is installed as %q",
			filepath.Join(name, manifestPath), declared.Name, name)
	}
	if err := requireServedKit(declared.Kit, name); err != nil {
		return declared, err
	}
	if err := requireParts(dir, name); err != nil {
		return declared, err
	}
	return declared, walk(dir, name)
}

// requireServedKit refuses a theme built on a kit version this release does not serve.
func requireServedKit(declared, name string) error {
	if _, ok := parseKit(declared); !ok {
		return refuseHolding("kit_missing", "the manifest names no kit version",
			map[string]any{"file": manifestPath, "name": name, "declared": declared},
			"themehost: %s names the kit as %q, which is not a plain version, so rebuild the theme",
			filepath.Join(name, manifestPath), declared)
	}
	if !ServesKit(declared) {
		return refuseHolding("kit_unsupported", "the theme kit is not served",
			map[string]any{"name": name, "declared": declared, "served": ServedKits()},
			"themehost: %s is built on theme kit %s, and this release serves %s",
			name, declared, strings.Join(servedKits, ", "))
	}
	return nil
}

// readManifest returns the manifest a theme directory declares itself with.
func readManifest(dir, name string) (manifest, error) {
	var declared manifest
	raw, err := os.ReadFile(filepath.Join(dir, manifestPath))
	if errors.Is(err, fs.ErrNotExist) {
		return declared, refuseHolding("manifest_missing", "the manifest is missing",
			map[string]any{"name": name, "file": manifestPath},
			"%w: %s declares no %s", ErrNotInstalled, name, manifestPath)
	}
	if err != nil {
		return declared, fmt.Errorf("themehost: reading %s: %w", filepath.Join(name, manifestPath), err)
	}
	if err := json.Unmarshal(raw, &declared); err != nil {
		return declared, refuseHolding("manifest_malformed", "the manifest is not valid JSON",
			map[string]any{"file": manifestPath, "name": name},
			"themehost: %s is not valid JSON: %w", filepath.Join(name, manifestPath), err)
	}
	return declared, nil
}

// requireParts reports whether a theme directory holds the server and the assets it is served from.
func requireParts(dir, name string) error {
	for _, part := range []struct {
		path   string
		isDir  bool
		code   string
		reason string
	}{
		{path: entryPath, isDir: false, code: "server_entry_missing", reason: "the server entry is missing"},
		{path: clientPath, isDir: true, code: "client_assets_missing", reason: "the client assets are missing"},
	} {
		info, err := os.Lstat(filepath.Join(dir, part.path))
		if errors.Is(err, fs.ErrNotExist) {
			return refuseHolding(part.code, part.reason,
				map[string]any{"name": name, "path": filepath.ToSlash(part.path)},
				"themehost: %s holds no %s", name, filepath.ToSlash(part.path))
		}
		if err != nil {
			return fmt.Errorf("themehost: reading %s in %s: %w", filepath.ToSlash(part.path), name, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return refuseHolding("symlink_present", "symlinks are not allowed",
				map[string]any{"name": name, "path": filepath.ToSlash(part.path)},
				"themehost: %s holds a symlink at %s, which a theme may not do",
				name, filepath.ToSlash(part.path))
		}
		if info.IsDir() != part.isDir {
			return refuseHolding(part.code, part.reason,
				map[string]any{"name": name, "path": filepath.ToSlash(part.path)},
				"themehost: %s is the wrong kind of file in %s",
				filepath.ToSlash(part.path), name)
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
			return refuseHolding("symlink_present", "symlinks are not allowed",
				map[string]any{"name": name, "path": relative(dir, path)},
				"themehost: %s holds a symlink at %s, which a theme may not do",
				name, relative(dir, path))
		}
		if entry.IsDir() {
			return nil
		}
		size, err := sizeOf(entry)
		if err != nil {
			return fmt.Errorf("themehost: reading %s: %w", relative(dir, path), err)
		}
		total += size
		if total > MaxSize {
			return refuseHolding("theme_too_large", "the theme is too large",
				map[string]any{"max": int64(MaxSize)},
				"themehost: %s is larger than the %d byte cap", name, int64(MaxSize))
		}
		return nil
	})
}

// sizeOf returns the bytes a file entry occupies.
func sizeOf(entry fs.DirEntry) (int64, error) {
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
