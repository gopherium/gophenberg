// SPDX-License-Identifier: Apache-2.0

package updater

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	// backupTime is how the name of a backup writes the time it was taken at, in UTC.
	backupTime = "20060102T150405Z"
	// backupSuffix ends the name of every dump.
	backupSuffix = ".dump"
)

// dumpVersion finds the major in what pg_dump --version prints, like 17 in pg_dump (PostgreSQL) 17.2.
var dumpVersion = regexp.MustCompile(`\(PostgreSQL\) (\d+)`)

// errNotURL refuses a database address a backup cannot hand pg_dump without its password.
var errNotURL = errors.New("updater: the backup reads the database address only as a postgres:// URL")

// Backup is one pg_dump of the database into the backups folder, which keeps it, the one Held names and the newest.
type Backup struct {
	PgDump      string
	DatabaseURL string
	Folder      string
	Name        string
	Held        string
	Keep        int
}

// dated is one backup of the backups folder and the time its name says it was taken at.
type dated struct {
	name string
	at   time.Time
}

// connection is how pg_dump reaches the database: the address it is handed and the variables that carry the password.
type connection struct {
	target string
	env    []string
}

// backupName returns the name of a backup of the database release ran on, taken at at.
func backupName(release string, at time.Time) string {
	return release + "-" + at.UTC().Format(backupTime) + backupSuffix
}

// DumpMajor returns the PostgreSQL major of the pg_dump at bin, refusing one that cannot run or names no version.
func DumpMajor(ctx context.Context, bin string) (int, error) {
	out, err := exec.CommandContext(ctx, bin, "--version").Output()
	if err != nil {
		return 0, fmt.Errorf("updater: run %s --version: %w", bin, err)
	}
	match := dumpVersion.FindSubmatch(out)
	if match == nil {
		return 0, fmt.Errorf("updater: %s --version printed %q, which names no PostgreSQL version", bin,
			bytes.TrimSpace(out))
	}
	major, _ := strconv.Atoi(string(match[1]))
	return major, nil
}

// Take runs pg_dump into a partial file, renames it to the name of the backup, saves the name to disk, then prunes.
func (b Backup) Take(ctx context.Context) error {
	reach, err := dumpTarget(b.DatabaseURL)
	if err != nil {
		return err
	}
	_ = os.MkdirAll(b.Folder, 0o755)
	if err := b.dump(ctx, reach); err != nil {
		_ = os.Remove(b.partial())
		return err
	}
	if err := os.Rename(b.partial(), filepath.Join(b.Folder, b.Name)); err != nil {
		_ = os.Remove(b.partial())
		return fmt.Errorf("updater: keep the backup: %w", err)
	}
	if err := syncFolder(b.Folder); err != nil {
		return fmt.Errorf("updater: save the backup to disk: %w", err)
	}
	b.prune()
	return nil
}

// partial returns the file pg_dump writes the backup into before it takes the name of the backup.
func (b Backup) partial() string {
	return filepath.Join(b.Folder, b.Name+partialSuffix)
}

// dump runs pg_dump in the custom format over the connection reach into a new partial file.
func (b Backup) dump(ctx context.Context, reach connection) error {
	_ = os.Remove(b.partial())
	out, err := os.OpenFile(b.partial(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("updater: write the backup: %w", err)
	}
	var stderr bytes.Buffer
	command := exec.CommandContext(ctx, b.PgDump, "--format=custom", "--no-password", "--dbname="+reach.target)
	command.Stdout, command.Stderr, command.Env = out, &stderr, append(os.Environ(), reach.env...)
	if err := closed(out, command.Run()); err != nil {
		return fmt.Errorf("updater: back up the database with %s: %w: %s", b.PgDump, err,
			strings.TrimSpace(stderr.String()))
	}
	return nil
}

// dumpTarget returns how pg_dump reaches the database at address, with the password moved out of the address.
func dumpTarget(address string) (connection, error) {
	parsed, err := url.Parse(address)
	if err != nil || !postgresURL(parsed) {
		return connection{}, errNotURL
	}
	password, set := parsed.User.Password()
	if set {
		parsed.User = url.User(parsed.User.Username())
	}
	password = movedPassword(parsed, password)
	if password == "" {
		return connection{target: parsed.String()}, nil
	}
	return connection{target: parsed.String(), env: []string{"PGPASSWORD=" + password}}, nil
}

// movedPassword returns the password of the query of parsed, or fallback, and writes the rest back with spaces as %20.
func movedPassword(parsed *url.URL, fallback string) string {
	query := parsed.Query()
	if !query.Has("password") {
		return fallback
	}
	password := cmp.Or(query.Get("password"), fallback)
	query.Del("password")
	parsed.RawQuery = strings.ReplaceAll(query.Encode(), "+", "%20")
	return password
}

// postgresURL reports whether parsed starts with a scheme libpq reads as a PostgreSQL URL.
func postgresURL(parsed *url.URL) bool {
	return parsed.Scheme == "postgres" || parsed.Scheme == "postgresql"
}

// prune keeps the spared dumps and the newest others up to Keep in all, leaving alone each file that is no dump.
func (b Backup) prune() {
	backups := datedBackups(b.Folder)
	others := slices.DeleteFunc(slices.Clone(backups), b.spared)
	room := max(b.Keep-(len(backups)-len(others)), 0)
	for _, old := range others[min(room, len(others)):] {
		_ = os.Remove(filepath.Join(b.Folder, old.name))
	}
}

// spared reports whether old is a backup pruning never deletes: the one just taken or the one Held names.
func (b Backup) spared(old dated) bool {
	return old.name == b.Name || old.name == b.Held
}

// datedBackups returns each dump folder holds with the time it was taken at, the newest first.
func datedBackups(folder string) []dated {
	entries, _ := os.ReadDir(folder)
	var backups []dated
	for _, entry := range entries {
		if at, isBackup := takenAt(entry); isBackup {
			backups = append(backups, dated{name: entry.Name(), at: at})
		}
	}
	slices.SortFunc(backups, func(older, newer dated) int { return newer.at.Compare(older.at) })
	return backups
}

// takenAt returns the time the name of entry says the dump was taken at, and whether entry is a dump.
func takenAt(entry fs.DirEntry) (time.Time, bool) {
	stem, dumped := strings.CutSuffix(entry.Name(), backupSuffix)
	cut := strings.LastIndex(stem, "-")
	if !dumped || cut < 0 {
		return time.Time{}, false
	}
	at, err := time.Parse(backupTime, stem[cut+1:])
	return at, err == nil
}
