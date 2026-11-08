// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// besideFolder names the backups folder beside the database file of a site with no data folder or install root.
const besideFolder = "backups"

// beforeMigrating backs a SQLite file up before a new build migrates it, through the rule that asks and the copy.
type beforeMigrating struct {
	needs func(context.Context, *sql.DB, sqlite.Build) (bool, error)
	take  func(context.Context, *sql.DB, sqlite.BackupRequest) (sqlite.TakenBackup, error)
}

// fileBackups is the backup before migrating that reads and copies the file itself.
var fileBackups = beforeMigrating{needs: sqlite.NeedsBackup, take: sqlite.TakeBackup}

// backedUp returns apply, run once the file holds a backup whenever this build must back it up first.
func backedUp(
	env gonsole.Env, apply func(context.Context, *sql.DB, sqlite.MigrateOptions) error,
) func(context.Context, *sql.DB, sqlite.MigrateOptions) error {
	return fileBackups.around(env, apply)
}

// around returns apply, run once b backed the file up as the settings env holds say.
func (b beforeMigrating) around(
	env gonsole.Env, apply func(context.Context, *sql.DB, sqlite.MigrateOptions) error,
) func(context.Context, *sql.DB, sqlite.MigrateOptions) error {
	return func(ctx context.Context, db *sql.DB, opts sqlite.MigrateOptions) error {
		plan, err := planBackup(env, opts)
		if err == nil {
			err = b.backUp(ctx, db, plan)
		}
		if err != nil {
			return err
		}
		return apply(ctx, db, opts)
	}
}

// backupPlan is one backup before migrating: the copy, its folders, the skip setting, the wait and the floor.
type backupPlan struct {
	request sqlite.BackupRequest
	layout  updater.Layout
	skipFor string
	wait    sqlite.MigrateOptions
	names   gonsole.Env
	rising  bool
}

// planBackup reads the backup before migrating from env, waiting for another run's backup as opts waits for the lock.
func planBackup(env gonsole.Env, opts sqlite.MigrateOptions) (backupPlan, error) {
	settings, settingsErr := sqlite.BackupSettingsFrom(env)
	layout, layoutErr := updater.BackupSettingsFrom(env)
	if err := errors.Join(settingsErr, layoutErr); err != nil {
		return backupPlan{}, err
	}
	request := sqlite.BackupRequest{
		Folder: backupsFolder(layout, env.Value("DATABASE_URL")), Kind: backupfiles.Update, Kept: settings.Kept,
	}
	return backupPlan{request: request, layout: layout, skipFor: settings.SkipFor, wait: opts, names: env}, nil
}

// backupsFolder returns the folder the file at address backs up into: the data folder's, the root's, else beside it.
func backupsFolder(layout updater.Layout, address string) string {
	if layout.Kept() == "" {
		return filepath.Join(filepath.Dir(strings.TrimPrefix(address, sqlitePrefix)), besideFolder)
	}
	return layout.Backups()
}

// backUp backs db up as plan says when this build must, unless the skip setting lets this version go without.
func (b beforeMigrating) backUp(ctx context.Context, db *sql.DB, plan backupPlan) error {
	needed, err := b.needs(ctx, db, thisBuild())
	if err != nil || !needed {
		return err
	}
	stored, err := sqlite.StoredMinimumVersion(ctx, db)
	if err != nil {
		return err
	}
	plan.rising = version.Before(stored, version.Minimum())
	if plan.skips() {
		return nil
	}
	return b.copyFile(ctx, db, plan)
}

// skips reports whether the skip setting names this version and the file's floor stays where it is.
func (p backupPlan) skips() bool {
	return !p.rising && p.skipFor == version.Version()
}

// copyFile takes the backup plan names, or waits for another run's, then records the copy once it lands.
func (b beforeMigrating) copyFile(ctx context.Context, db *sql.DB, plan backupPlan) error {
	err := plan.layout.MakeData()
	if err == nil {
		_, err = b.take(ctx, db, plan.request)
	}
	switch {
	case errors.Is(err, dbsqlite.ErrSnapshotRunning):
		return b.awaitOther(ctx, db, plan)
	case err != nil:
		return plan.failed(err)
	}
	return sqlite.RecordBackupTaken(ctx, db, thisBuild())
}

// awaitOther reads the rule again every poll of plan until another backup lets db migrate, or the wait passes.
func (b beforeMigrating) awaitOther(ctx context.Context, db *sql.DB, plan backupPlan) error {
	deadline := time.NewTimer(plan.wait.LockWait)
	defer deadline.Stop()
	poll := time.NewTicker(plan.wait.LockPoll)
	defer poll.Stop()
	for {
		needed, err := b.needs(ctx, db, thisBuild())
		if err != nil || !needed {
			return err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("take the backup before migrating: wait for another backup into %s: %w",
				plan.request.Folder, context.Cause(ctx))
		case <-deadline.C:
			return plan.overdue()
		case <-poll.C:
		}
	}
}

// failed returns the error of a backup before migrating that did not land, naming the skip setting where it applies.
func (p backupPlan) failed(cause error) error {
	skip := p.names.Key("SQLITE_SKIP_BACKUP_FOR")
	if p.rising {
		return fmt.Errorf("take the backup before migrating: %w, and %s does not apply since %s raises the minimum "+
			"version of the file, so free room by moving backups off the server", cause, skip, version.Version())
	}
	return fmt.Errorf("take the backup before migrating: %w, so free room by moving backups off the server, "+
		"or set %s=%s to migrate this version with no backup", cause, skip, version.Version())
}

// overdue returns the error of a run whose wait for another backup passed while the file still needed its own.
func (p backupPlan) overdue() error {
	return fmt.Errorf("take the backup before migrating: another backup was writing into %s and the file still "+
		"needed its own after %s of %v, checked every %s of %v, so run the command again once that backup ends",
		p.request.Folder, p.names.Key("SQLITE_MIGRATE_LOCK_WAIT"), p.wait.LockWait,
		p.names.Key("SQLITE_MIGRATE_LOCK_POLL"), p.wait.LockPoll)
}
