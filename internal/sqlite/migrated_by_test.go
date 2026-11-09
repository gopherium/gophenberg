// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"testing"

	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

// thisBuild is the build these tests migrate a file with.
var thisBuild = sqlite.Build{Version: "0.21.0", Commit: "3b96de93"}

// recordMigratedBy records build on db as the one that last migrated it, failing the test on an error.
func recordMigratedBy(t *testing.T, db *sql.DB, build sqlite.Build) {
	t.Helper()
	if err := sqlite.RecordMigratedBy(t.Context(), db, build); err != nil {
		t.Fatalf("RecordMigratedBy(%+v) error = %v, want nil", build, err)
	}
}

// needsBackup reports whether thisBuild must back db up before it migrates it, failing the test on an error.
func needsBackup(t *testing.T, db *sql.DB) bool {
	t.Helper()
	needs, err := sqlite.NeedsBackup(t.Context(), db, thisBuild)
	if err != nil {
		t.Fatalf("NeedsBackup() error = %v, want nil", err)
	}
	return needs
}

func TestAFileWithNoTableNeedsNoBackup(t *testing.T) {
	t.Parallel()

	if needsBackup(t, sqlitetest.Open(t, testSettings().Options())) {
		t.Error("NeedsBackup() on a file with no table = true, want false")
	}
}

func TestAFileNoBuildRecordedNeedsABackup(t *testing.T) {
	t.Parallel()

	if !needsBackup(t, migrated(t)) {
		t.Error("NeedsBackup() on a migrated file that records no build = false, want true")
	}
}

func TestAFileAnOlderVersionMigratedNeedsABackup(t *testing.T) {
	t.Parallel()

	for _, older := range []sqlite.Build{
		{Version: "0.20.0", Commit: "3b96de93"},
		{Version: "0.9.0", Commit: "3b96de93"},
		{Version: "0.21.0-rc.1", Commit: "3b96de93"},
	} {
		db := migrated(t)
		recordMigratedBy(t, db, older)

		if !needsBackup(t, db) {
			t.Errorf("NeedsBackup() on a file %+v migrated = false, want true", older)
		}
	}
}

func TestAFileThisBuildMigratedNeedsNone(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	recordMigratedBy(t, db, thisBuild)

	if needsBackup(t, db) {
		t.Error("NeedsBackup() on a file this build migrated = true, want false")
	}
}

func TestAFileANewerVersionMigratedNeedsNone(t *testing.T) {
	t.Parallel()

	for _, newer := range []sqlite.Build{
		{Version: "0.22.0", Commit: "aa11bb22"},
		{Version: "0.21.1-rc.1", Commit: ""},
	} {
		db := migrated(t)
		recordMigratedBy(t, db, newer)

		if needsBackup(t, db) {
			t.Errorf("NeedsBackup() on a file %+v migrated = true, want false", newer)
		}
	}
}

func TestThisVersionFromAnotherCommitNeedsABackup(t *testing.T) {
	t.Parallel()

	for _, other := range []sqlite.Build{
		{Version: "0.21.0", Commit: "aa11bb22"},
		{Version: "0.21.0", Commit: ""},
	} {
		db := migrated(t)
		recordMigratedBy(t, db, other)

		if !needsBackup(t, db) {
			t.Errorf("NeedsBackup() on a file %+v migrated = false, want true", other)
		}
	}
}

func TestLastMigratedByIsEmptyBeforeTheCoreTablesExist(t *testing.T) {
	t.Parallel()

	got, err := sqlite.LastMigratedBy(t.Context(), sqlitetest.Open(t, testSettings().Options()))

	if err != nil || got != (sqlite.Build{}) {
		t.Errorf("LastMigratedBy() = %+v, %v on a file without core tables, want no build and nil", got, err)
	}
}

func TestRecordingNeverLowersTheVersion(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		stored []sqlite.Build
		record sqlite.Build
		want   sqlite.Build
	}{
		{name: "a first build is stored", record: thisBuild, want: thisBuild},
		{
			name: "an older build is replaced", stored: []sqlite.Build{{Version: "0.20.0", Commit: "aa11bb22"}},
			record: thisBuild, want: thisBuild,
		},
		{
			name: "a newer build stays", stored: []sqlite.Build{{Version: "0.22.0", Commit: "aa11bb22"}},
			record: thisBuild, want: sqlite.Build{Version: "0.22.0", Commit: "aa11bb22"},
		},
		{
			name: "the same version takes the new commit", stored: []sqlite.Build{{Version: "0.21.0", Commit: "aa11bb22"}},
			record: thisBuild, want: thisBuild,
		},
		{
			name: "going back keeps the newer version",
			stored: []sqlite.Build{
				{Version: "0.21.0", Commit: "3b96de93"}, {Version: "0.22.0", Commit: "aa11bb22"},
			},
			record: thisBuild,
			want:   sqlite.Build{Version: "0.22.0", Commit: "aa11bb22"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			db := migrated(t)
			for _, stored := range tc.stored {
				recordMigratedBy(t, db, stored)
			}
			recordMigratedBy(t, db, tc.record)

			got, err := sqlite.LastMigratedBy(t.Context(), db)
			if err != nil || got != tc.want {
				t.Errorf("LastMigratedBy() = %+v, %v, want %+v", got, err, tc.want)
			}
		})
	}
}

func TestRecordingTheVersionLeavesTheMinimumVersionAlone(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	raiseMinimumVersion(t, db, "0.19.0")
	recordMigratedBy(t, db, thisBuild)

	minimum, err := sqlite.StoredMinimumVersion(t.Context(), db)
	if err != nil || minimum != "0.19.0" {
		t.Errorf("StoredMinimumVersion() = %q, %v after recording the build, want 0.19.0", minimum, err)
	}
}

func TestAFileWhoseBackupForThisBuildLandedNeedsNoMore(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	recordMigratedBy(t, db, sqlite.Build{Version: "0.20.0", Commit: "aa11bb22"})
	claimBackup(t, db, thisBuild)

	if needsBackup(t, db) {
		t.Error("NeedsBackup() after this build's backup landed and its migration failed = true, want false")
	}
}

func TestABackupForAnotherBuildLeavesThisBuildNeedingOne(t *testing.T) {
	t.Parallel()

	for _, other := range []sqlite.Build{
		{Version: "0.21.0", Commit: "aa11bb22"},
		{Version: "0.22.0", Commit: "3b96de93"},
	} {
		db := migrated(t)
		recordMigratedBy(t, db, sqlite.Build{Version: "0.20.0", Commit: "aa11bb22"})
		claimBackup(t, db, other)

		if !needsBackup(t, db) {
			t.Errorf("NeedsBackup() after a backup for %+v landed = false, want true for %+v", other, thisBuild)
		}
	}
}

func TestRecordingAMigrationClearsTheBackupTaken(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	recordMigratedBy(t, db, sqlite.Build{Version: "0.20.0", Commit: "aa11bb22"})
	claimBackup(t, db, thisBuild)
	recordMigratedBy(t, db, sqlite.Build{Version: "0.20.5", Commit: "cc33dd44"})

	if !needsBackup(t, db) {
		t.Error("NeedsBackup() after another build migrated the file since this build's backup = false, want true")
	}
}

func TestTheBuildRecordNamesAClosedHandle(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	_ = db.Close()

	if _, err := sqlite.NeedsBackup(t.Context(), db, thisBuild); err == nil {
		t.Error("NeedsBackup() on a closed handle error = nil, want one")
	}
	if _, err := sqlite.LastMigratedBy(t.Context(), db); err == nil {
		t.Error("LastMigratedBy() on a closed handle error = nil, want one")
	}
	if err := sqlite.RecordMigratedBy(t.Context(), db, thisBuild); err == nil {
		t.Error("RecordMigratedBy() on a closed handle error = nil, want one")
	}
	if _, err := sqlite.ClaimBackup(t.Context(), db, thisBuild); err == nil {
		t.Error("ClaimBackup() on a closed handle error = nil, want one")
	}
}
