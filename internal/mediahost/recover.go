// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/internal/media"
)

// notesDir is the library relative folder holding a note for every file an upload is still writing.
const notesDir = ".uploading"

// Saved reports which of the files a stored item names as its own.
type Saved func(ctx context.Context, files []string) (map[string]bool, error)

// noted is a file an upload noted, with the main file of the item it was written for.
type noted struct {
	file string
	main string
}

// notePath returns the absolute path of the note held on a library relative file.
func (l *Library) notePath(rel string) string {
	return l.abs(path.Join(notesDir, rel))
}

// writeNoted notes the file for the item stored at main, then writes it, taking the note back when the write fails.
func (l *Library) writeNoted(rel string, data []byte, main string) error {
	note := l.notePath(rel)
	if err := os.MkdirAll(filepath.Dir(note), 0o755); err != nil {
		return err
	}
	if err := writeExclusive(note, []byte(main)); err != nil {
		return err
	}
	if err := writeExclusive(l.abs(rel), data); err != nil {
		_ = l.unnote(rel)
		return err
	}
	return nil
}

// unnote removes the note held on a library relative file, ignoring one already gone.
func (l *Library) unnote(rel string) error {
	if err := os.Remove(l.notePath(rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// drop deletes a library relative file and then its note, keeping the note while the file stays.
func (l *Library) drop(rel string) error {
	if err := os.Remove(l.abs(rel)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	_ = l.unnote(rel)
	return nil
}

// Finish marks the upload of a saved item finished.
func (l *Library) Finish(m media.Media) {
	for _, file := range filesOf(m) {
		_ = l.unnote(file)
	}
}

// Recover deletes the files of uploads noted before the cutoff whose item was never saved, returning what it deleted.
func (l *Library) Recover(ctx context.Context, before time.Time, saved Saved) ([]string, error) {
	notes, err := l.notedBefore(before)
	if err != nil || len(notes) == 0 {
		return nil, err
	}
	kept, err := saved(ctx, mainsOf(notes))
	if err != nil {
		return nil, err
	}
	deleted := make([]string, 0, len(notes))
	var failures []error
	for _, n := range notes {
		gone, err := l.settle(n, kept[n.main])
		if err != nil {
			failures = append(failures, err)
		}
		if gone {
			deleted = append(deleted, n.file)
		}
	}
	return deleted, errors.Join(failures...)
}

// settle clears the note of a saved item, or deletes an unsaved upload's file first, reporting whether it deleted one.
func (l *Library) settle(n noted, saved bool) (bool, error) {
	if saved {
		return false, l.unnote(n.file)
	}
	err := os.Remove(l.abs(n.file))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return err == nil, l.unnote(n.file)
}

// notedBefore returns the files uploads noted before the cutoff, nothing when no upload was ever noted.
func (l *Library) notedBefore(before time.Time) ([]noted, error) {
	root := l.abs(notesDir)
	var notes []noted
	err := filepath.Walk(root, func(at string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !info.ModTime().Before(before) {
			return err
		}
		main, err := os.ReadFile(at)
		file := filepath.ToSlash(strings.TrimPrefix(at, root+string(filepath.Separator)))
		notes = append(notes, noted{file: file, main: string(main)})
		return err
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return notes, err
}

// mainsOf returns the main files the notes were written for, each once.
func mainsOf(notes []noted) []string {
	seen := make(map[string]bool, len(notes))
	mains := make([]string, 0, len(notes))
	for _, n := range notes {
		if !seen[n.main] {
			seen[n.main] = true
			mains = append(mains, n.main)
		}
	}
	return mains
}
