// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"errors"
	"slices"
	"testing"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
)

// marginNoDiskHolds is a margin in bytes, one pebibyte, past the free space of any disk a test runs on.
const marginNoDiskHolds = 1 << 50

func TestABackupStopsWhenItsFolderLacksSpaceForTheCopyAndItsMargin(t *testing.T) {
	t.Parallel()

	folder := backupFolder(t)
	plantBackups(t, folder, "0.19.0-20200101T031500Z-update.sqlite")
	request := sqlite.BackupRequest{Folder: folder, Kind: backupfiles.Scheduled, Kept: 1, Margin: marginNoDiskHolds}

	taken, err := sqlite.TakeBackup(t.Context(), migrated(t), request)

	var short *dbsqlite.SpaceError
	if !errors.Is(err, dbsqlite.ErrNoSpace) || !errors.As(err, &short) || short.Folder != folder || taken.Path != "" {
		t.Errorf("TakeBackup() with a margin no disk holds = %+v, %v, want no path and the space error of %s",
			taken, err, folder)
	}
	want := []string{snapshotLock, "0.19.0-20200101T031500Z-update.sqlite"}
	if got := folderHolds(t, folder); !slices.Equal(got, want) {
		t.Errorf("the folder holds %q, want %q, no copy started and nothing pruned", got, want)
	}
}
