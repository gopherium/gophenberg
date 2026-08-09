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

// ErrUnsafeArchive reports an archive whose entries a theme may not carry.
var ErrUnsafeArchive = errors.New("themehost: the archive is unsafe")

// Install unpacks archive as the named theme, replacing it only once it validates.
func Install(themesDir, name string, archive io.ReaderAt, size int64) (*Theme, error) {
	reader, err := zip.NewReader(archive, size)
	if err != nil {
		return nil, fmt.Errorf("themehost: reading the archive: %w", err)
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

// unpack writes every archive entry under dir, refusing what a theme may not carry.
func unpack(reader *zip.Reader, dir string) error {
	var total int64
	for _, file := range reader.File {
		target, err := safeTarget(dir, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%w: it holds a symlink at %s", ErrUnsafeArchive, file.Name)
		}
		if strings.HasSuffix(file.Name, "/") {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("themehost: unpacking %s: %w", file.Name, err)
			}
			continue
		}
		written, err := extract(file, target)
		if err != nil {
			return err
		}
		total += written
		if total > MaxSize {
			return fmt.Errorf("themehost: the archive unpacks to more than the %d byte cap", int64(MaxSize))
		}
	}
	return nil
}

// safeTarget returns the path an entry unpacks to, refusing one that escapes dir.
func safeTarget(dir, entry string) (string, error) {
	target := filepath.Join(dir, filepath.FromSlash(entry))
	if target != dir && !strings.HasPrefix(target, dir+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: %s escapes the theme directory", ErrUnsafeArchive, entry)
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
	retired := installed + ".retired"
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
	return os.RemoveAll(retired)
}
