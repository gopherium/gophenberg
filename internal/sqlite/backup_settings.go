// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"errors"
	"fmt"
	"time"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// lowestBackupInterval is the shortest time between two scheduled backups of a file.
	lowestBackupInterval = time.Hour
	// highestBackupInterval is the longest time between two scheduled backups of a file, 31 days.
	highestBackupInterval = 744 * time.Hour
	// highestBackupsKept is the most update and scheduled backups of a file kept together.
	highestBackupsKept = 100
	// highestBackupTimeLimit is the longest one scheduled backup may run.
	highestBackupTimeLimit = 24 * time.Hour
)

// BackupSettings are how a SQLite site backs up its file: the schedule, the one keep count and the version to skip.
type BackupSettings struct {
	Interval  time.Duration
	Kept      int
	TimeLimit time.Duration
	SkipFor   string
}

// BackupSettingsFrom reads the backup settings from env, each one left unset taking its documented default.
func BackupSettingsFrom(env gonsole.Env) (BackupSettings, error) {
	interval, intervalErr := backupIntervalFrom(env)
	kept, keptErr := env.Count("SQLITE_BACKUPS_KEEP", 3, gonsole.AtMost(highestBackupsKept))
	limit, limitErr := env.Duration("SQLITE_BACKUP_TIME_LIMIT", 15*time.Minute,
		gonsole.AtMost(int64(highestBackupTimeLimit)))
	skip, skipErr := gonsole.Parse(env, "SQLITE_SKIP_BACKUP_FOR", "", fullVersion)
	if err := errors.Join(intervalErr, keptErr, limitErr, skipErr); err != nil {
		return BackupSettings{}, err
	}
	return BackupSettings{Interval: interval, Kept: kept, TimeLimit: limit, SkipFor: skip}, nil
}

// backupIntervalFrom reads the time between two scheduled backups, zero turning the schedule off.
func backupIntervalFrom(env gonsole.Env) (time.Duration, error) {
	interval, err := env.Duration("SQLITE_BACKUP_INTERVAL", 24*time.Hour,
		gonsole.AllowZero(), gonsole.AtMost(int64(highestBackupInterval)))
	if err != nil {
		return 0, err
	}
	if interval != 0 && interval < lowestBackupInterval {
		return 0, fmt.Errorf("%s: must be 0 or at least 1h, got %q",
			env.Key("SQLITE_BACKUP_INTERVAL"), env.Value("SQLITE_BACKUP_INTERVAL"))
	}
	return interval, nil
}

// fullVersion reads a full version such as 0.21.0, refusing one that version.Valid does not read.
func fullVersion(value string) (string, error) {
	if !version.Valid(value) {
		return "", fmt.Errorf("must name a version such as 0.21.0, got %q", value)
	}
	return value, nil
}
