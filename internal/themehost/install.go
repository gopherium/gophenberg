// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Install unpacks archive as the named theme, replacing it only once it validates.
func Install(themesDir, name string, archive io.ReaderAt, size int64) (*Theme, error) {
	reader, err := openArchive(archive, size)
	if err != nil {
		return nil, err
	}
	staging, err := os.MkdirTemp(themesDir, "."+name+"-staging-")
	if err != nil {
		return nil, fmt.Errorf("themehost: staging %s: %w", name, err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	if err := unpack(reader, filepath.Join(staging, name)); err != nil {
		return nil, err
	}
	theme, err := Load(staging, name)
	if err != nil {
		return nil, err
	}
	if err := swapIn(filepath.Join(staging, name), filepath.Join(themesDir, name)); err != nil {
		return nil, err
	}
	theme.Dir = filepath.Join(themesDir, name)
	return theme, nil
}

// openArchive returns the entries an archive carries, refusing one carrying more than the cap.
func openArchive(archive io.ReaderAt, size int64) (*zip.Reader, error) {
	reader, err := zip.NewReader(archive, size)
	if err != nil {
		return nil, refuse("the archive could not be read", "themehost: reading the archive: %w", err)
	}
	if len(reader.File) > MaxEntries {
		return nil, refuse("the archive holds too many files",
			"themehost: the archive holds %d entries, more than the %d cap", len(reader.File), MaxEntries)
	}
	return reader, nil
}

// unpack writes every archive entry under dir, refusing an archive that unpacks past the cap.
func unpack(reader *zip.Reader, dir string) error {
	var total int64
	made := make(map[string]bool)
	for _, file := range reader.File {
		written, err := unpackOne(file, dir, made)
		if err != nil {
			return err
		}
		total += written
		if total > MaxSize {
			return refuse("the theme is too large",
				"themehost: the archive unpacks to more than the %d byte cap", int64(MaxSize))
		}
	}
	return nil
}

// unpackOne writes one archive entry under dir, refusing what a theme may not carry.
func unpackOne(file *zip.File, dir string, made map[string]bool) (int64, error) {
	target, err := safeTarget(dir, file.Name)
	if err != nil {
		return 0, err
	}
	if file.FileInfo().Mode()&fs.ModeSymlink != 0 {
		return 0, refuse("symlinks are not allowed",
			"themehost: the archive holds a symlink at %s", file.Name)
	}
	if strings.HasSuffix(file.Name, "/") {
		if err := os.MkdirAll(target, 0o755); err != nil {
			return 0, fmt.Errorf("themehost: unpacking %s: %w", file.Name, err)
		}
		return dirsCost(target, dir, made), nil
	}
	written, err := extract(file, target)
	return written + dirsCost(filepath.Dir(target), dir, made), err
}

// dirsCost returns what the directories up to root cost, charging each one once.
func dirsCost(path, root string, made map[string]bool) int64 {
	var cost int64
	for len(path) > len(root) && !made[path] {
		made[path] = true
		cost += dirSize
		path = filepath.Dir(path)
	}
	return cost
}

// safeTarget returns the path an entry unpacks to, refusing one that escapes dir.
func safeTarget(dir, entry string) (string, error) {
	target := filepath.Join(dir, filepath.FromSlash(entry))
	if target != dir && !strings.HasPrefix(target, dir+string(os.PathSeparator)) {
		return "", refuse("the archive is unsafe", "themehost: %s escapes the theme directory", entry)
	}
	return target, nil
}

// extract writes one archive entry to target and returns how much it wrote.
func extract(file *zip.File, target string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, fmt.Errorf("themehost: unpacking %s: %w", file.Name, err)
	}
	source, err := file.Open()
	if err != nil {
		return 0, fmt.Errorf("themehost: reading %s from the archive: %w", file.Name, err)
	}
	defer func() { _ = source.Close() }()

	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("themehost: writing %s: %w", file.Name, err)
	}
	written, err := io.Copy(out, io.LimitReader(source, MaxSize+1))
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return written, fmt.Errorf("themehost: writing %s: %w", file.Name, err)
	}
	return written, nil
}

// swapIn moves the staged theme into place, replacing what the name held.
func swapIn(staged, installed string) error {
	retired := filepath.Join(filepath.Dir(installed), "."+filepath.Base(installed)+".retired")
	if err := os.RemoveAll(retired); err != nil {
		return fmt.Errorf("themehost: clearing %s: %w", retired, err)
	}
	switch err := os.Rename(installed, retired); {
	case err == nil, errors.Is(err, fs.ErrNotExist):
	default:
		return fmt.Errorf("themehost: retiring the installed theme: %w", err)
	}
	if err := os.Rename(staged, installed); err != nil {
		_ = os.Rename(retired, installed)
		return fmt.Errorf("themehost: installing the theme: %w", err)
	}
	_ = os.RemoveAll(retired)
	return nil
}
