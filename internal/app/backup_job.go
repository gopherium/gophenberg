// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
	"github.com/gopherium/gophenberg/internal/updater"
)

// sqliteBackupJob names the core job that backs up the file of a SQLite site each interval.
const sqliteBackupJob = "core/sqlite-backup"

// backupSchedule is how often a SQLite site backs its file up, how long one backup may run, and the backup it takes.
type backupSchedule struct {
	every     time.Duration
	timeLimit time.Duration
	request   sqlite.BackupRequest
}

// backupScheduleFrom reads the backup schedule of the sqlite: address env names into layout's folder, none elsewhere.
func backupScheduleFrom(env gonsole.Env, layout updater.Layout) (backupSchedule, error) {
	address := env.Value("DATABASE_URL")
	if !onSQLite(address) {
		return backupSchedule{}, nil
	}
	settings, err := sqlite.BackupSettingsFrom(env)
	if err != nil {
		return backupSchedule{}, err
	}
	return backupSchedule{every: settings.Interval, timeLimit: settings.TimeLimit, request: sqlite.BackupRequest{
		Folder: backupsFolder(layout, address), Kind: backupfiles.Scheduled, Kept: settings.Kept,
	}}, nil
}

// backupJobs returns the job that backs up the SQLite file behind db as schedule says, none while it names no interval.
func backupJobs(schedule backupSchedule, db *sql.DB, logger *slog.Logger) []cron.Job {
	return scheduledBackup{take: sqlite.TakeBackup, db: db, logger: logger}.jobs(schedule)
}

// scheduledBackup takes the backups of one SQLite file on a schedule and logs what each one left behind.
type scheduledBackup struct {
	take   func(context.Context, *sql.DB, sqlite.BackupRequest) (sqlite.TakenBackup, error)
	db     *sql.DB
	logger *slog.Logger
}

// jobs returns the cron job that takes the backup schedule names each interval, none while it names no interval.
func (s scheduledBackup) jobs(schedule backupSchedule) []cron.Job {
	if schedule.every == 0 {
		return nil
	}
	return []cron.Job{{
		Name: sqliteBackupJob, Every: schedule.every, TimeLimit: schedule.timeLimit,
		Run: func(ctx context.Context, _ time.Time) error { return s.backUp(ctx, schedule.request) },
	}}
}

// backUp takes the backup request names, then logs where it went and each error that left the copy standing.
func (s scheduledBackup) backUp(ctx context.Context, request sqlite.BackupRequest) error {
	taken, err := s.take(ctx, s.db, request)
	if err != nil {
		return err
	}
	s.logger.Info("backup taken", "path", taken.Path)
	if taken.AfterCopy != nil {
		s.logger.Warn("backup step after the copy failed", "path", taken.Path, "error", taken.AfterCopy)
	}
	if taken.Prune != nil {
		s.logger.Warn("backup prune failed", "folder", request.Folder, "error", taken.Prune)
	}
	return nil
}
