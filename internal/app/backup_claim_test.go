// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
)

// claimLog logs the claims and the prunes of a backup before migrating in the order they ran.
type claimLog struct {
	won   bool
	steps []string
}

// claim logs a claim and answers whether it won.
func (l *claimLog) claim(context.Context, *sql.DB, sqlite.Build) (bool, error) {
	l.steps = append(l.steps, "claim")
	return l.won, nil
}

// prune logs a prune of the folder of request that spares taken, then fails it.
func (l *claimLog) prune(request sqlite.BackupRequest, taken sqlite.TakenBackup) error {
	l.steps = append(l.steps, prunedStep(request, taken))
	return errRead
}

// prunedStep returns the line a claimLog keeps for a prune of the folder of request that spares taken.
func prunedStep(request sqlite.BackupRequest, taken sqlite.TakenBackup) string {
	return fmt.Sprintf("prune %+v sparing %s", request, taken.Path)
}

// claimingThrough returns the backup before migrating that copies through take and claims and prunes through log.
func claimingThrough(
	take func(context.Context, *sql.DB, sqlite.BackupRequest) (sqlite.TakenBackup, error), log *claimLog,
) beforeMigrating {
	return beforeMigrating{needs: sqlite.NeedsBackup, take: take, claim: log.claim, prune: log.prune}
}

// plantedCopy returns the path of an empty file that stands for the copy a stubbed backup took.
func plantedCopy(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "copy.sqlite")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("planting the copy: %v", err)
	}
	return path
}

func TestAClaimThatLosesDeletesItsOwnCopyAndMigratesWithNoPrune(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	copied := plantedCopy(t)
	take, _ := stubbedTake(sqlite.TakenBackup{Path: copied}, nil)
	log := &claimLog{}
	apply, applied := countedApply()
	run := claimingThrough(take, log).around(settingsEnv(migratingSQLite(path)), apply)

	if err := run(t.Context(), openSQLiteFile(t, path), patientWait); err != nil || *applied != 1 {
		t.Fatalf("a run whose claim lost = %v after %d migrations, want nil after 1", err, *applied)
	}
	if !slices.Equal(log.steps, []string{"claim"}) {
		t.Errorf("a run whose claim lost took the steps %q, want the claim and no prune", log.steps)
	}
	if _, err := os.Lstat(copied); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the copy of a run whose claim lost: %v, want it deleted", err)
	}
}

func TestAClaimThatWinsPrunesAfterItAndKeepsTheCopy(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	taken := sqlite.TakenBackup{Path: plantedCopy(t)}
	take, _ := stubbedTake(taken, nil)
	log := &claimLog{won: true}
	apply, applied := countedApply()
	env := migratingWith(path, map[string]string{"GOPHENBERG_SQLITE_BACKUPS_KEEP": "2"})
	run := claimingThrough(take, log).around(settingsEnv(env), apply)

	if err := run(t.Context(), openSQLiteFile(t, path), patientWait); err != nil || *applied != 1 {
		t.Fatalf("a run whose claim won and whose prune failed = %v after %d migrations, want nil after 1",
			err, *applied)
	}
	request := sqlite.BackupRequest{Folder: backupsBeside(path), Kind: backupfiles.Update, Kept: 2}
	if want := []string{"claim", prunedStep(request, taken)}; !slices.Equal(log.steps, want) {
		t.Errorf("a run whose claim won took the steps %q, want %q", log.steps, want)
	}
	if _, err := os.Lstat(taken.Path); err != nil {
		t.Errorf("the copy of a run whose claim won: %v, want it kept", err)
	}
}

func TestALateRunWhoseClaimLosesLeavesTheWinnersBackup(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	db := openSQLiteFile(t, path)
	winner := olderVersion + "-20200101T031500Z-update.sqlite"
	plantBackups(t, path, winner)
	if won, err := sqlite.ClaimBackup(t.Context(), db, thisBuild()); err != nil || !won {
		t.Fatalf("the winning run's ClaimBackup() = %t, %v, want true", won, err)
	}
	late := fileBackups
	late.needs, _ = stubbedRule(ruleAnswer{needed: true})
	apply, applied := countedApply()
	env := migratingWith(path, map[string]string{"GOPHENBERG_SQLITE_BACKUPS_KEEP": "1"})

	err := late.around(settingsEnv(env), apply)(t.Context(), db, patientWait)

	if err != nil || *applied != 1 {
		t.Fatalf("a late run whose claim lost = %v after %d migrations, want nil after 1", err, *applied)
	}
	want := []string{".dbkit-snapshot.lock", winner}
	if held := folderNames(t, backupsBeside(path)); !slices.Equal(held, want) {
		t.Errorf("the backups folder holds %q after a late run whose claim lost, want %q with the winner's kept",
			held, want)
	}
}

func TestAClaimThatLosesStopsWhenItCannotDeleteItsCopy(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	copied := filepath.Join(t.TempDir(), "copy.sqlite")
	if err := os.MkdirAll(filepath.Join(copied, "held"), 0o700); err != nil {
		t.Fatalf("planting a copy no remove deletes: %v", err)
	}
	take, _ := stubbedTake(sqlite.TakenBackup{Path: copied}, nil)
	apply, applied := countedApply()
	run := claimingThrough(take, &claimLog{}).around(settingsEnv(migratingSQLite(path)), apply)

	err := run(t.Context(), openSQLiteFile(t, path), patientWait)

	if want := "could not delete its own copy " + copied; err == nil || !strings.Contains(err.Error(), want) ||
		*applied != 0 {
		t.Errorf("a run whose claim lost and whose copy stays = %v after %d migrations, want an error naming %q and 0",
			err, *applied, want)
	}
}

func TestAClaimThatLosesGoesOnWhenItsCopyIsAlreadyGone(t *testing.T) {
	t.Parallel()

	path := olderMigratedSQLiteFile(t)
	take, _ := stubbedTake(sqlite.TakenBackup{Path: filepath.Join(t.TempDir(), "pruned.sqlite")}, nil)
	apply, applied := countedApply()
	run := claimingThrough(take, &claimLog{}).around(settingsEnv(migratingSQLite(path)), apply)

	if err := run(t.Context(), openSQLiteFile(t, path), patientWait); err != nil || *applied != 1 {
		t.Errorf("a run whose claim lost after another run pruned its copy = %v after %d migrations, want nil after 1",
			err, *applied)
	}
}
