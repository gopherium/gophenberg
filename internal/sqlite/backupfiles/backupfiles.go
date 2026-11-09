// SPDX-License-Identifier: Apache-2.0

// Package backupfiles names, lists and prunes the backups of a SQLite site's file, with no database driver.
package backupfiles

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// UnknownVersion stands for the version in the name of a backup of a file that records no version.
	UnknownVersion = "unknown"
	// suffix ends the name of every backup of a SQLite file.
	suffix = ".sqlite"
	// timeForm is how the name of a backup writes the time it was taken at, in UTC, as the names of the dumps do.
	timeForm = "20060102T150405Z"
	// fingerprintLength is how many hex characters of its fingerprint the name of a scheduled backup keeps.
	fingerprintLength = 16
	// hexDigits are the characters a fingerprint is written in.
	hexDigits = "0123456789abcdef"
)

// Kind is what took a backup.
type Kind string

const (
	// Update is a backup taken before a new version migrated the file.
	Update Kind = "update"
	// Scheduled is a backup the schedule took.
	Scheduled Kind = "scheduled"
	// Manual is a backup taken on demand, which no prune deletes.
	Manual Kind = "manual"
)

// Backup is one backup of a SQLite file as its name tells it: the version that last migrated the file, when and why.
type Backup struct {
	Version     string
	At          time.Time
	Kind        Kind
	Fingerprint string
}

// Name returns the file name of the backup, like 0.20.0-20261110T091500Z-update.sqlite.
func (b Backup) Name() string {
	parts := []string{b.Version, b.At.UTC().Format(timeForm), string(b.Kind)}
	if b.Fingerprint != "" {
		parts = append(parts, b.Fingerprint[:min(len(b.Fingerprint), fingerprintLength)])
	}
	return strings.Join(parts, "-") + suffix
}

// sparedBy reports whether a prune that took taken keeps b: b is taken or was taken in a later second.
func (b Backup) sparedBy(taken Backup) bool {
	return b.Name() == taken.Name() || b.At.After(taken.At.Truncate(time.Second))
}

// valid reports whether the backup is one a name may tell: a full or unknown version and a known kind.
func (b Backup) valid() bool {
	if b.Version != UnknownVersion && !version.Valid(b.Version) {
		return false
	}
	switch b.Kind {
	case Scheduled:
		return true
	case Update, Manual:
		return b.Fingerprint == ""
	}
	return false
}

// Read returns the backup name tells, reading it from the right, and whether name is the name of a backup.
func Read(name string) (Backup, bool) {
	backup, told := (&nameReader{rest: strings.TrimSuffix(name, suffix)}).backup()
	if !told || backup.Name() != name {
		return Backup{}, false
	}
	return backup, true
}

// nameReader reads the stem of a backup's name from the right, keeping what it has not read yet.
type nameReader struct {
	rest string
}

// backup returns the backup the stem tells, and whether it tells one.
func (r *nameReader) backup() (Backup, bool) {
	fingerprint := r.fingerprint()
	kind := Kind(r.cut())
	at, err := time.Parse(timeForm, r.cut())
	backup := Backup{Version: r.rest, At: at, Kind: kind, Fingerprint: fingerprint}
	return backup, err == nil && backup.valid()
}

// fingerprint cuts the fingerprint that ends the stem and returns it, empty when none ends it.
func (r *nameReader) fingerprint() string {
	at := strings.LastIndex(r.rest, "-")
	last := r.rest[at+1:]
	if len(last) != fingerprintLength || strings.Trim(last, hexDigits) != "" {
		return ""
	}
	r.rest = r.rest[:max(at, 0)]
	return last
}

// cut cuts the part after the last dash of the stem and returns it, the whole stem when it holds no dash.
func (r *nameReader) cut() string {
	at := strings.LastIndex(r.rest, "-")
	part := r.rest[at+1:]
	r.rest = r.rest[:max(at, 0)]
	return part
}

// Folder is the folder that keeps the backups of a SQLite file.
type Folder string

// List returns every backup the folder holds, the newest first, none when the folder is missing.
func (f Folder) List() ([]Backup, error) {
	entries, err := os.ReadDir(string(f))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("backupfiles: list the backups in %s: %w", f, err)
	}
	var backups []Backup
	for _, entry := range entries {
		if backup, isBackup := Read(entry.Name()); isBackup && entry.Type().IsRegular() {
			backups = append(backups, backup)
		}
	}
	slices.SortFunc(backups, newestFirst)
	return backups, nil
}

// newestFirst orders a before b when a was taken later, the later name first between two taken at once.
func newestFirst(a, b Backup) int {
	return cmp.Or(b.At.Compare(a.At), strings.Compare(b.Name(), a.Name()))
}

// Prune deletes the oldest update and scheduled backups past kept in all, sparing taken, newer and manual backups.
func (f Folder) Prune(kept int, taken Backup) error {
	backups, err := f.List()
	if err != nil {
		return err
	}
	counted := slices.DeleteFunc(backups, func(b Backup) bool { return b.Kind == Manual })
	others := slices.DeleteFunc(slices.Clone(counted), func(b Backup) bool { return b.sparedBy(taken) })
	room := max(kept-(len(counted)-len(others)), 0)
	var failed []error
	for _, old := range others[min(room, len(others)):] {
		failed = append(failed, os.Remove(filepath.Join(string(f), old.Name())))
	}
	return errors.Join(failed...)
}
