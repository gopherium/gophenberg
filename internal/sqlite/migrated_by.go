// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"

	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// migratedByKey is the site setting that holds the version of the build that last migrated the file.
	migratedByKey = "migrated_by"
	// migratedByCommitKey is the site setting that holds the commit of the build that last migrated the file.
	migratedByCommitKey = "migrated_by_commit"
	// backedUpForKey is the site setting that holds the version of the build whose backup before migrating landed.
	backedUpForKey = "backed_up_for"
	// backedUpForCommitKey is the site setting that holds the commit of the build whose backup before migrating landed.
	backedUpForCommitKey = "backed_up_for_commit"
	// tablesLookup reports whether the file holds any table.
	tablesLookup = "SELECT EXISTS (SELECT 1 FROM sqlite_schema WHERE type = 'table')"
	// buildsLookup reads the four build settings in one row, each one empty while none is stored.
	buildsLookup = "SELECT coalesce((SELECT value FROM core_settings WHERE key = ?1), ''), " +
		"coalesce((SELECT value FROM core_settings WHERE key = ?2), ''), " +
		"coalesce((SELECT value FROM core_settings WHERE key = ?3), ''), " +
		"coalesce((SELECT value FROM core_settings WHERE key = ?4), '')"
	// buildStore stores a version and a commit under their two keys in place of any stored before.
	buildStore = `INSERT INTO core_settings (key, value) VALUES (?1, ?2), (?3, ?4)
ON CONFLICT (key) DO UPDATE SET value = excluded.value`
	// buildClear deletes a version and a commit stored under their two keys.
	buildClear = "DELETE FROM core_settings WHERE key IN (?1, ?2)"
)

// Build is one build of Gophenberg: its version and the commit it was built from, empty when the build names none.
type Build struct {
	Version string
	Commit  string
}

// before reports whether b comes before build: an older version, or the same version from another commit.
func (b Build) before(build Build) bool {
	if b.Version == build.Version {
		return b.Commit != build.Commit
	}
	return version.Before(b.Version, build.Version)
}

// builds is what a file records: the build that last migrated it and the one whose backup before migrating landed.
type builds struct {
	migrated Build
	backedUp Build
}

// needBackup reports whether build must back the file up: an older build migrated it and build took no backup yet.
func (r builds) needBackup(build Build) bool {
	return r.migrated.before(build) && r.backedUp != build
}

// rowReader reads one row, as a handle and a transaction both do.
type rowReader interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// NeedsBackup reports whether build must back db up before it migrates it: db holds a table and an older build.
func NeedsBackup(ctx context.Context, db *sql.DB, build Build) (bool, error) {
	var tables bool
	if err := db.QueryRowContext(ctx, tablesLookup).Scan(&tables); err != nil || !tables {
		return false, failed("look for a table", err)
	}
	recorded, err := storedBuilds(ctx, db)
	return err == nil && recorded.needBackup(build), err
}

// LastMigratedBy returns the build db records as the one that last migrated it, none before the core tables exist.
func LastMigratedBy(ctx context.Context, db *sql.DB) (Build, error) {
	recorded, err := storedBuilds(ctx, db)
	return recorded.migrated, err
}

// storedBuilds returns the builds db records, none before the core tables exist.
func storedBuilds(ctx context.Context, db *sql.DB) (builds, error) {
	var ready bool
	if err := db.QueryRowContext(ctx, settingsLookup).Scan(&ready); err != nil || !ready {
		return builds{}, failed("look for the settings table", err)
	}
	return readBuilds(ctx, db)
}

// readBuilds reads the builds the database of reader records.
func readBuilds(ctx context.Context, reader rowReader) (builds, error) {
	var recorded builds
	err := reader.QueryRowContext(ctx, buildsLookup, migratedByKey, migratedByCommitKey, backedUpForKey,
		backedUpForCommitKey).Scan(&recorded.migrated.Version, &recorded.migrated.Commit,
		&recorded.backedUp.Version, &recorded.backedUp.Commit)
	return recorded, failed("read the builds the file records", err)
}

// RecordMigratedBy stores build on db as the one that last migrated it, never in place of a newer version.
func RecordMigratedBy(ctx context.Context, db *sql.DB, build Build) error {
	return failed("record the build that migrated the file", inTx(ctx, db, func(tx *sql.Tx) error {
		recorded, err := readBuilds(ctx, tx)
		if err != nil || version.Before(build.Version, recorded.migrated.Version) {
			return err
		}
		_, err = tx.ExecContext(ctx, buildStore, migratedByKey, build.Version, migratedByCommitKey, build.Commit)
		if err == nil {
			_, err = tx.ExecContext(ctx, buildClear, backedUpForKey, backedUpForCommitKey)
		}
		return err
	}))
}

// ClaimBackup records build's backup before migrating db while build still needs one, reporting whether it did.
func ClaimBackup(ctx context.Context, db *sql.DB, build Build) (bool, error) {
	var ready bool
	if err := db.QueryRowContext(ctx, settingsLookup).Scan(&ready); err != nil || !ready {
		return err == nil, failed("look for the settings table", err)
	}
	var claimed bool
	err := inTx(ctx, db, func(tx *sql.Tx) error {
		won, err := claimIn(ctx, tx, build)
		claimed = won
		return err
	})
	return claimed && err == nil, failed("record the backup taken before migrating", err)
}

// claimIn stores build in tx as the one whose backup landed while build still needs one, reporting whether it did.
func claimIn(ctx context.Context, tx *sql.Tx, build Build) (bool, error) {
	recorded, err := readBuilds(ctx, tx)
	if err != nil || !recorded.needBackup(build) {
		return false, err
	}
	_, err = tx.ExecContext(ctx, buildStore, backedUpForKey, build.Version, backedUpForCommitKey, build.Commit)
	return err == nil, err
}
