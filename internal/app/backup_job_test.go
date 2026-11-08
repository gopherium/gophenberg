// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/gonsole/testkit"

	"github.com/gopherium/gophenberg/internal/cron"
	"github.com/gopherium/gophenberg/internal/cron/crontest"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
	"github.com/gopherium/gophenberg/internal/updater"
	"github.com/gopherium/gophenberg/internal/version"
)

// scheduledFile is a SQLite file at an absolute path that no scheduled backup test opens.
const scheduledFile = "/srv/site/site.db"

// scheduleBeside returns the backup schedule of a site on the SQLite file at path, with the other settings given.
func scheduleBeside(t *testing.T, path string, settings map[string]string) backupSchedule {
	t.Helper()
	schedule, err := backupScheduleFrom(settingsEnv(migratingWith(path, settings)), updater.Layout{})
	if err != nil {
		t.Fatalf("backupScheduleFrom() error = %v, want nil", err)
	}
	return schedule
}

// onlyJob returns the one job of jobs, failing t unless jobs holds exactly one.
func onlyJob(t *testing.T, jobs []cron.Job) cron.Job {
	t.Helper()
	if len(jobs) != 1 {
		t.Fatalf("the backup jobs are %+v, want one", jobs)
	}
	return jobs[0]
}

// describedJobs returns the name, the interval and the time limit of each job.
func describedJobs(jobs []cron.Job) []string {
	described := make([]string, 0, len(jobs))
	for _, job := range jobs {
		described = append(described, fmt.Sprintf("%s every %s for at most %s", job.Name, job.Every, job.TimeLimit))
	}
	return described
}

// recordedTake returns a copy that lands with no error, and the requests it was handed.
func recordedTake() (func(context.Context, *sql.DB, sqlite.BackupRequest) (sqlite.TakenBackup, error),
	*[]sqlite.BackupRequest,
) {
	var requests []sqlite.BackupRequest
	return func(_ context.Context, _ *sql.DB, request sqlite.BackupRequest) (sqlite.TakenBackup, error) {
		requests = append(requests, request)
		return sqlite.TakenBackup{Path: filepath.Join(request.Folder, "copy.sqlite")}, nil
	}, &requests
}

// scheduledBackupsIn returns every scheduled backup folder holds, the newest first.
func scheduledBackupsIn(t *testing.T, folder string) []backupfiles.Backup {
	t.Helper()
	backups, err := backupfiles.Folder(folder).List()
	if err != nil {
		t.Fatalf("listing the backups in %s: %v", folder, err)
	}
	return slices.DeleteFunc(backups, func(backup backupfiles.Backup) bool {
		return backup.Kind != backupfiles.Scheduled
	})
}

// plantBackups makes the backups folder beside the SQLite file at path and plants an empty file under each name.
func plantBackups(t *testing.T, path string, names ...string) {
	t.Helper()
	if err := os.Mkdir(backupsBeside(path), 0o700); err != nil {
		t.Fatalf("making the backups folder: %v", err)
	}
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(backupsBeside(path), name), nil, 0o600); err != nil {
			t.Fatalf("planting %s: %v", name, err)
		}
	}
}

func TestTheBackupJobRunsOnlyOnSQLite(t *testing.T) {
	t.Parallel()

	data := t.TempDir()
	for address, want := range map[string]struct {
		jobs    []string
		request sqlite.BackupRequest
	}{
		unreachableDatabaseURL: {},
		sqliteAddress: {
			jobs:    []string{"core/sqlite-backup every 2h0m0s for at most 15m0s"},
			request: sqlite.BackupRequest{Folder: filepath.Join(data, "backups"), Kind: backupfiles.Scheduled, Kept: 3},
		},
	} {
		settings, err := loadRunConfig(testkit.Getenv(map[string]string{
			"GOPHENBERG_DATABASE_URL": address, "GOPHENBERG_DATA_DIR": data, "GOPHENBERG_SQLITE_BACKUP_INTERVAL": "2h",
		}))
		if err != nil {
			t.Fatalf("loadRunConfig() on %s error = %v, want nil", address, err)
		}

		jobs := describedJobs(backupJobs(settings.backups, nil, slog.New(slog.DiscardHandler)))

		if !slices.Equal(jobs, want.jobs) || settings.backups.request != want.request {
			t.Errorf("the backup jobs on %s are %q backing up as %+v, want %q backing up as %+v",
				address, jobs, settings.backups.request, want.jobs, want.request)
		}
	}
}

func TestTheBackupJobIsLeftOutWhileTheIntervalIsZero(t *testing.T) {
	t.Parallel()

	schedule := scheduleBeside(t, scheduledFile, map[string]string{"GOPHENBERG_SQLITE_BACKUP_INTERVAL": "0"})

	if jobs := backupJobs(schedule, nil, slog.New(slog.DiscardHandler)); len(jobs) != 0 {
		t.Errorf("the backup jobs with the interval 0 are %q, want none", describedJobs(jobs))
	}
}

func TestTheBackupScheduleRefusesABadBackupSettingOnSQLite(t *testing.T) {
	t.Parallel()

	_, err := loadRunConfig(testkit.Getenv(map[string]string{
		"GOPHENBERG_DATABASE_URL": sqliteAddress, "GOPHENBERG_SQLITE_BACKUPS_KEEP": "0",
	}))

	if err == nil || !strings.HasPrefix(err.Error(), "GOPHENBERG_SQLITE_BACKUPS_KEEP: ") {
		t.Errorf("loadRunConfig() error = %v, want the keep count refused by its name", err)
	}
}

func TestTheBackupJobBacksUpOnEachDueRun(t *testing.T) {
	t.Parallel()

	take, requests := recordedTake()
	schedule := scheduleBeside(t, scheduledFile, map[string]string{"GOPHENBERG_SQLITE_BACKUP_INTERVAL": "2h"})
	start := time.Date(2026, time.November, 9, 3, 0, 0, 0, time.UTC)
	now := start
	runner := cron.New(cron.Config{
		Jobs:  scheduledBackup{take: take, logger: slog.New(slog.DiscardHandler)}.jobs(schedule),
		Store: crontest.NewMemoryStore(), Clock: func() time.Time { return now },
		TimeLimit: 30 * time.Second, Lease: time.Minute,
	})

	for _, at := range []time.Duration{0, time.Hour, 2 * time.Hour} {
		now = start.Add(at)
		if _, err := runner.RunDue(t.Context()); err != nil {
			t.Fatalf("RunDue() at %s error = %v, want nil", now, err)
		}
	}

	want := sqlite.BackupRequest{Folder: backupsBeside(scheduledFile), Kind: backupfiles.Scheduled, Kept: 3}
	if !slices.Equal(*requests, []sqlite.BackupRequest{want, want}) {
		t.Errorf("the job backed up as %+v over three passes an hour apart, want %+v on the two that fell due",
			*requests, want)
	}
}

func TestTheBackupJobPrunesTheUpdateAndScheduledBackupsToOneCount(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	older, newer := "0.0.1-20200102T031500Z-scheduled.sqlite", "0.0.1-20200103T031500Z-scheduled.sqlite"
	manual := "0.0.1-20200104T031500Z-manual.sqlite"
	plantBackups(t, path, "0.0.1-20200101T031500Z-update.sqlite", older, newer, manual)
	job := onlyJob(t, backupJobs(scheduleBeside(t, path, nil), openSQLiteFile(t, path), slog.New(slog.DiscardHandler)))

	if err := job.Run(t.Context(), time.Now()); err != nil {
		t.Fatalf("the backup job error = %v, want nil", err)
	}

	backups := scheduledBackupsIn(t, backupsBeside(path))
	if len(backups) == 0 || backups[0].Version != version.Version() {
		t.Fatalf("the scheduled backups are %+v, want the newest one of this version's file", backups)
	}
	want := []string{".dbkit-snapshot.lock", older, newer, manual, backups[0].Name()}
	if held := folderNames(t, backupsBeside(path)); !slices.Equal(held, want) {
		t.Errorf("the backups folder holds %q, want %q with the update backup pushed out", held, want)
	}
}

func TestTheBackupJobRunsUnderItsOwnTimeLimit(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	db := openSQLiteFile(t, path)
	var left time.Duration
	take := func(ctx context.Context, _ *sql.DB, _ sqlite.BackupRequest) (sqlite.TakenBackup, error) {
		deadline, _ := ctx.Deadline()
		left = time.Until(deadline)
		return sqlite.TakenBackup{}, nil
	}
	settings, err := cronSettingsWith(nil)
	if err != nil {
		t.Fatalf("cronSettingsWith() error = %v", err)
	}
	schedule := scheduleBeside(t, path, map[string]string{"GOPHENBERG_SQLITE_BACKUP_TIME_LIMIT": "20m"})
	logger := slog.New(slog.DiscardHandler)
	jobs := cronOf(settings, sqliteStores(composeConfig{handle: db, fieldDepth: 4}), logger,
		scheduledBackup{take: take, db: db, logger: logger}.jobs(schedule)...)

	if report, err := jobs.runs.RunNow(); err != nil || report.Ran != 2 {
		t.Fatalf("RunNow() = %+v, %v, want the sweep and the backup run once", report, err)
	}
	if left <= 19*time.Minute || left > 20*time.Minute {
		t.Errorf("the backup ran with %s left, want its own 20m past the runner's %s", left, settings.timeLimit)
	}
}

func TestTheBackupJobLogsACheckpointAndAPruneError(t *testing.T) {
	t.Parallel()

	copied := filepath.Join(backupsBeside(scheduledFile), "copy.sqlite")
	take, _ := stubbedTake(sqlite.TakenBackup{Path: copied, AfterCopy: errAfterCopy, Prune: errRead}, nil)
	logs := &bytes.Buffer{}
	job := onlyJob(t, scheduledBackup{take: take, logger: slog.New(slog.NewTextHandler(logs, nil))}.
		jobs(scheduleBeside(t, scheduledFile, nil)))

	if err := job.Run(t.Context(), time.Now()); err != nil {
		t.Errorf("the backup job error = %v, want nil since the copy stands", err)
	}
	for _, want := range []string{
		`level=INFO msg="backup taken" path=` + copied,
		`level=WARN msg="backup step after the copy failed" path=` + copied + ` error="checkpoint failure"`,
		`level=WARN msg="backup prune failed" folder=` + backupsBeside(scheduledFile) + ` error="read failure"`,
	} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("the backup job logged %q, want it to carry %q", logs.String(), want)
		}
	}
}

func TestTheBackupJobFailsWithACopyThatDidNotLand(t *testing.T) {
	t.Parallel()

	path := migratedSQLiteFile(t)
	blockBackupsFolder(t, path)
	logs := &bytes.Buffer{}
	job := onlyJob(t, backupJobs(scheduleBeside(t, path, nil), openSQLiteFile(t, path),
		slog.New(slog.NewTextHandler(logs, nil))))

	err := job.Run(t.Context(), time.Now())

	if err == nil || !strings.Contains(err.Error(), backupsBeside(path)) {
		t.Errorf("the backup job error = %v, want the failed copy into %s", err, backupsBeside(path))
	}
	if logs.Len() != 0 {
		t.Errorf("the backup job logged %q, want nothing beside the failure the runner logs", logs.String())
	}
}
