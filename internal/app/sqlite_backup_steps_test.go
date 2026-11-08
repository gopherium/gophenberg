// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cucumber/godog"

	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// sqliteBackupsFeature is the feature the backups of a SQLite file answer to.
	sqliteBackupsFeature = "../../test/features/features/sqlite-backups.feature"
	// migratedByLeft stores a version as the one that last migrated a SQLite file.
	migratedByLeft = `INSERT INTO core_settings (key, value) VALUES ('migrated_by', ?)
ON CONFLICT (key) DO UPDATE SET value = excluded.value RETURNING true`
	// migratedByHeld reports whether a SQLite file records a version as the one that last migrated it.
	migratedByHeld = "SELECT EXISTS (SELECT 1 FROM core_settings WHERE key = 'migrated_by' AND value = ?)"
)

// backupScenario is what one SQLite backup scenario keeps beside the operator scenario.
type backupScenario struct {
	*operatorScenario
}

// initializeSQLiteBackups binds the steps of the SQLite backup scenarios to s.
func initializeSQLiteBackups(sc *godog.ScenarioContext, s *operatorScenario) {
	b := &backupScenario{operatorScenario: s}
	sc.Given(`^the settings point at an empty (SQLite file|PostgreSQL database)$`, b.pointAtEmpty)
	sc.Given(`^the file records Gophenberg "([^"]*)" as the version that last migrated it$`, b.recordMigratedBy)
	sc.Given(`^GOPHENBERG_SQLITE_SKIP_BACKUP_FOR names the version of this binary$`, b.skipThisVersion)
	sc.Given(`^the settings name an install root$`, b.nameInstallRoot)
	sc.Given(`^a (?:backup|file) named "([^"]*)" waits in the backups folder$`, b.plant)
	sc.Then(`^the backups folder holds no backup$`, b.holdsNoBackup)
	sc.Then(`^the backups folder holds (\d+) (update|scheduled|manual) backups?$`, b.holdsBackups)
	sc.Then(`^the backups folder still holds "([^"]*)"$`, b.stillHolds)
	sc.Then(`^the backups folder no longer holds "([^"]*)"$`, b.noLongerHolds)
	sc.Then(`^the update backup records Gophenberg "([^"]*)" as the version that last migrated it$`, b.backupRecords)
	sc.Then(`^the file records this binary as the version that last migrated it$`, b.recordsThisBinary)
	sc.Then(`^the answer names where it put the manual backup$`, b.namesManualBackup)
	sc.Then(`^the backups kept are "([^"]*)", "([^"]*)", "([^"]*)" and "([^"]*)", the newest first$`, b.keptInOrder)
}

// pointAtEmpty points the settings at a fresh database of kind, skipping the scenario on the other engine.
func (b *backupScenario) pointAtEmpty(kind string) error {
	if kind != b.engine.kind {
		return godog.ErrSkip
	}
	b.pointAtAnEmptyDatabase()
	return nil
}

// recordMigratedBy stores release as the version that last migrated the scenario's file.
func (b *backupScenario) recordMigratedBy(ctx context.Context, release string) error {
	if _, err := b.answers(ctx, migratedByLeft, release); err != nil {
		return fmt.Errorf("recording %s as the version that last migrated the file: %w", release, err)
	}
	return nil
}

// skipThisVersion names the version of this binary as the one that may migrate with no backup.
func (b *backupScenario) skipThisVersion() {
	b.env["GOPHENBERG_SQLITE_SKIP_BACKUP_FOR"] = version.Version()
}

// nameInstallRoot points the install root setting at a fresh folder.
func (b *backupScenario) nameInstallRoot() {
	b.env["GOPHENBERG_INSTALL_ROOT"] = b.t.TempDir()
}

// folder returns the backups folder of the scenario, the install root's, else the one beside the file.
func (b *backupScenario) folder() string {
	if root := b.env["GOPHENBERG_INSTALL_ROOT"]; root != "" {
		return filepath.Join(root, "backups")
	}
	file := strings.TrimPrefix(b.env["GOPHENBERG_DATABASE_URL"], sqlitePrefix)
	return filepath.Join(filepath.Dir(file), "backups")
}

// plant writes an empty file called name into the backups folder.
func (b *backupScenario) plant(name string) error {
	if err := os.MkdirAll(b.folder(), 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(b.folder(), name), nil, 0o600)
}

// names returns the name of every entry in the backups folder, none when the folder is missing.
func (b *backupScenario) names() ([]string, error) {
	entries, err := os.ReadDir(b.folder())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, err
}

// backupsOf returns the name of each backup of kind the backups folder holds.
func (b *backupScenario) backupsOf(kind string) ([]string, error) {
	names, err := b.names()
	pattern := regexp.MustCompile(`-` + kind + `(-[0-9a-f]{16})?\.sqlite$`)
	return slices.DeleteFunc(names, func(name string) bool { return !pattern.MatchString(name) }), err
}

// oneBackupOf returns the path of the one backup of kind the backups folder holds.
func (b *backupScenario) oneBackupOf(kind string) (string, error) {
	held, err := b.backupsOf(kind)
	if err != nil || len(held) != 1 {
		return "", fmt.Errorf("the backups folder holds the %s backups %q (%v), want one", kind, held, err)
	}
	return filepath.Join(b.folder(), held[0]), nil
}

// holdsNoBackup fails when the backups folder holds any SQLite backup.
func (b *backupScenario) holdsNoBackup() error {
	names, err := b.names()
	held := slices.DeleteFunc(names, func(name string) bool { return !strings.HasSuffix(name, ".sqlite") })
	if err != nil || len(held) != 0 {
		return fmt.Errorf("the backups folder holds %q (%v), want no backup", held, err)
	}
	return nil
}

// holdsBackups fails unless the backups folder holds count backups of kind.
func (b *backupScenario) holdsBackups(count int, kind string) error {
	held, err := b.backupsOf(kind)
	if err != nil || len(held) != count {
		return fmt.Errorf("the backups folder holds the %s backups %q (%v), want %d", kind, held, err, count)
	}
	return nil
}

// stillHolds fails unless the backups folder holds the file called name.
func (b *backupScenario) stillHolds(name string) error {
	_, err := os.Stat(filepath.Join(b.folder(), name))
	return err
}

// noLongerHolds fails when the backups folder holds the file called name.
func (b *backupScenario) noLongerHolds(name string) error {
	_, err := os.Stat(filepath.Join(b.folder(), name))
	if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("the backups folder holds %s (%v), want it deleted", name, err)
	}
	return nil
}

// backupRecords fails unless the one update backup records release as the version that last migrated it.
func (b *backupScenario) backupRecords(ctx context.Context, release string) error {
	backup, err := b.oneBackupOf("update")
	if err != nil {
		return err
	}
	body, err := os.ReadFile(backup)
	if err != nil {
		return err
	}
	copied := filepath.Join(b.t.TempDir(), "backup.db")
	if err := os.WriteFile(copied, body, 0o600); err != nil {
		return err
	}
	return recordsVersion(ctx, sqlitePrefix+copied, release)
}

// recordsThisBinary fails unless the scenario's file records this binary's version as the one that last migrated it.
func (b *backupScenario) recordsThisBinary(ctx context.Context) error {
	return recordsVersion(ctx, b.env["GOPHENBERG_DATABASE_URL"], version.Version())
}

// recordsVersion fails unless the SQLite file at address records release as the version that last migrated it.
func recordsVersion(ctx context.Context, address, release string) error {
	held, err := sqliteAnswers(ctx, address, migratedByHeld, release)
	if err != nil || !held {
		return fmt.Errorf("the file records %s as the version that last migrated it %v (%v), want it to",
			release, held, err)
	}
	return nil
}

// namesManualBackup fails unless the answer names the path of the one manual backup.
func (b *backupScenario) namesManualBackup() error {
	backup, err := b.oneBackupOf("manual")
	if err != nil {
		return err
	}
	if want := "backed up the database to " + backup; !strings.Contains(b.result.Stdout, want) {
		return fmt.Errorf("stdout %q does not carry %q", b.result.Stdout, want)
	}
	return nil
}

// keptInOrder fails unless update:status lists the four backups named, in the backups folder, in that order.
func (b *backupScenario) keptInOrder(first, second, third, fourth string) error {
	paths := make([]string, 0, 4)
	for _, name := range []string{first, second, third, fourth} {
		paths = append(paths, filepath.Join(b.folder(), name))
	}
	if want := "\nbackups kept: " + strings.Join(paths, ", ") + "\n"; !strings.Contains(b.result.Stdout, want) {
		return fmt.Errorf("stdout %q does not carry %q", b.result.Stdout, want)
	}
	return nil
}

func TestSQLiteBackups(t *testing.T) {
	t.Parallel()

	for name, engine := range map[string]operatorEngine{"postgres": postgresOperators(), "sqlite": sqliteOperators()} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			runOperatorFeature(t, sqliteBackupsFeature, engine)
		})
	}
}
