// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"

	"github.com/gopherium/gophenberg/internal/sqlite"
)

// olderBuild is a build older than thisBuild, the one that last migrated the files these tests claim a backup on.
var olderBuild = sqlite.Build{Version: "0.20.0", Commit: "aa11bb22"}

// claimBackup claims on db the backup build takes before migrating it, failing the test on an error.
func claimBackup(t *testing.T, db *sql.DB, build sqlite.Build) bool {
	t.Helper()
	claimed, err := sqlite.ClaimBackup(t.Context(), db, build)
	if err != nil {
		t.Fatalf("ClaimBackup(%+v) error = %v, want nil", build, err)
	}
	return claimed
}

// backupRecords counts the settings of db that record a backup taken before migrating.
func backupRecords(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	return count(t, db, "SELECT count(*) FROM core_settings WHERE key IN ('backed_up_for', 'backed_up_for_commit')")
}

// sameFile returns a second handle on the file of db, closed when t ends.
func sameFile(t *testing.T, db *sql.DB) *sql.DB {
	t.Helper()
	other, err := sqlite.Open("sqlite:"+liveFile(t, db), testSettings())
	if err != nil {
		t.Fatalf("opening a second handle on the file: %v", err)
	}
	t.Cleanup(func() { _ = other.Close() })
	return other
}

// claimsAtOnce claims the backup of thisBuild through every handle at the same moment and returns how many won.
func claimsAtOnce(t *testing.T, handles ...*sql.DB) int {
	t.Helper()
	var won atomic.Int32
	var claims sync.WaitGroup
	start := make(chan struct{})
	for _, handle := range handles {
		claims.Go(func() {
			<-start
			claimed, err := sqlite.ClaimBackup(t.Context(), handle, thisBuild)
			if err != nil {
				t.Errorf("ClaimBackup() at once error = %v, want nil", err)
			}
			if claimed {
				won.Add(1)
			}
		})
	}
	close(start)
	claims.Wait()
	return int(won.Load())
}

func TestTheFirstClaimOfABackupWins(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	recordMigratedBy(t, db, olderBuild)

	if !claimBackup(t, db, thisBuild) {
		t.Error("ClaimBackup() on a file an older build migrated = false, want true")
	}
	if claimBackup(t, db, thisBuild) {
		t.Error("ClaimBackup() a second time for the same build = true, want false")
	}
}

func TestAClaimAfterThisBuildMigratedLosesAndWritesNothing(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	recordMigratedBy(t, db, olderBuild)
	claimBackup(t, db, thisBuild)
	recordMigratedBy(t, db, thisBuild)

	if claimBackup(t, db, thisBuild) {
		t.Error("ClaimBackup() on a file this build migrated = true, want false")
	}
	if got := backupRecords(t, db); got != 0 {
		t.Errorf("the file holds %d backup records after a claim that lost, want none written", got)
	}
}

func TestANewerBuildClaimsABackupOfItsOwn(t *testing.T) {
	t.Parallel()

	db := migrated(t)
	recordMigratedBy(t, db, olderBuild)
	claimBackup(t, db, thisBuild)

	if newer := (sqlite.Build{Version: "0.22.0", Commit: "cc33dd44"}); !claimBackup(t, db, newer) {
		t.Errorf("ClaimBackup(%+v) after the claim of %+v = false, want true", newer, thisBuild)
	}
}

func TestOneOfTwoClaimsAtOnceWins(t *testing.T) {
	t.Parallel()

	for round := range 25 {
		first := migrated(t)
		recordMigratedBy(t, first, olderBuild)

		if won := claimsAtOnce(t, first, sameFile(t, first)); won != 1 {
			t.Fatalf("round %d: %d of two claims at once won, want exactly one", round, won)
		}
	}
}

func TestAClaimOnAFileWithNoSettingsTableWinsWithNothingToRecord(t *testing.T) {
	t.Parallel()

	db := sqlitetest.Open(t, testSettings().Options())

	claimed, err := sqlite.ClaimBackup(t.Context(), db, thisBuild)

	if err != nil || !claimed {
		t.Errorf("ClaimBackup() on a file without core tables = %v, %v, want true and nil, the copy kept", claimed, err)
	}
	if got, err := sqlite.LastMigratedBy(t.Context(), db); err != nil || got != (sqlite.Build{}) {
		t.Errorf("LastMigratedBy() = %+v, %v after it, want no build and nil", got, err)
	}
}

func TestAClaimThatCannotReadOrStoreLosesWithItsError(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, prefix string
		fail         func(*sqlitetest.Faults)
	}{
		{"the settings table lookup", "sqlite: look for the settings table: ",
			func(f *sqlitetest.Faults) { f.FailStatement(sqlite.SettingsLookup, errPlanted) }},
		{"the read of the builds", "sqlite: record the backup taken before migrating: ",
			func(f *sqlitetest.Faults) { f.FailStatement(sqlite.BuildsLookup, errPlanted) }},
		{"the commit", "sqlite: record the backup taken before migrating: ",
			func(f *sqlitetest.Faults) { f.FailNextCommit(errPlanted) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			db, faults := faultyHandle(t)
			recordMigratedBy(t, db, olderBuild)
			tc.fail(faults)

			claimed, err := sqlite.ClaimBackup(t.Context(), db, thisBuild)

			wantPlanted(t, err, tc.prefix)
			if claimed {
				t.Error("ClaimBackup() that failed = true, want false")
			}
			if got := backupRecords(t, db); got != 0 {
				t.Errorf("the file holds %d backup records after a failed claim, want none", got)
			}
		})
	}
}
