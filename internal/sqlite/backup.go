// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
)

// mainFileLookup reads the path of the main database file of a handle.
const mainFileLookup = "SELECT file FROM pragma_database_list WHERE name = 'main'"

var (
	// geteuid returns the user the process runs as, zero for root.
	geteuid = os.Geteuid
	// chmod sets the permission bits of a file.
	chmod = os.Chmod
)

// errRootLeavesFolder stops a run as root at an existing backups folder its group or others may open.
var errRootLeavesFolder = errors.New("its group or others may open it, and a run as root changes no folder it " +
	"did not make, so run the command as the site's user or set the folder to 0700")

// BackupRequest is one backup to take: the folder it goes to, what takes it, and how many update and scheduled to keep.
type BackupRequest struct {
	Folder string
	Kind   backupfiles.Kind
	Kept   int
}

// TakenBackup is a backup in place: its path, and the errors after the copy landed, which leave the copy standing.
type TakenBackup struct {
	Path      string
	AfterCopy error
	Prune     error
}

// TakeBackup copies db as request asks, keeps the copy from other users, then runs PruneBackups unless it is an update.
func TakeBackup(ctx context.Context, db *sql.DB, request BackupRequest) (TakenBackup, error) {
	backup, err := request.backup(ctx, db)
	if err != nil {
		return TakenBackup{}, err
	}
	target := filepath.Join(request.Folder, backup.Name())
	snapshot, err := dbsqlite.NewSnapshotter(db).Snapshot(ctx, target)
	if snapshot.Path == "" {
		return TakenBackup{}, failedCopy(target, err)
	}
	if modeErr := chmod(snapshot.Path, 0o600); modeErr != nil {
		return TakenBackup{}, errors.Join(
			fmt.Errorf("sqlite: keep the backup %s from other users: %w", snapshot.Path, modeErr), os.Remove(snapshot.Path))
	}
	taken := TakenBackup{Path: snapshot.Path, AfterCopy: err}
	if request.Kind != backupfiles.Update {
		taken.Prune = PruneBackups(request, taken)
	}
	return taken, nil
}

// PruneBackups prunes the folder of request to its count, sparing taken, unless it is manual or its AfterCopy failed.
func PruneBackups(request BackupRequest, taken TakenBackup) error {
	if taken.AfterCopy != nil || request.Kind == backupfiles.Manual {
		return nil
	}
	backup, _ := backupfiles.Read(filepath.Base(taken.Path))
	return backupfiles.Folder(request.Folder).Prune(request.Kept, backup)
}

// failedCopy returns the error of a copy that did not land, ErrSnapshotRunning when another run took that very backup.
func failedCopy(target string, err error) error {
	if _, held := os.Lstat(target); held == nil {
		return fmt.Errorf("sqlite: another run took the backup %s: %w", target, dbsqlite.ErrSnapshotRunning)
	}
	return fmt.Errorf("sqlite: take the backup %s: %w", target, err)
}

// backup makes the folder of the request and returns the backup to take, named for the build that last migrated db.
func (r BackupRequest) backup(ctx context.Context, db *sql.DB) (backupfiles.Backup, error) {
	if err := makeBackupFolder(ctx, db, r.Folder); err != nil {
		return backupfiles.Backup{}, err
	}
	build, err := LastMigratedBy(ctx, db)
	version := cmp.Or(build.Version, backupfiles.UnknownVersion)
	return backupfiles.Backup{Version: version, At: time.Now(), Kind: r.Kind}, err
}

// makeBackupFolder makes a missing folder only its owner opens, as root owned like db's file, or keeps it from others.
func makeBackupFolder(ctx context.Context, db *sql.DB, folder string) error {
	owner, asRoot, err := databaseOwner(ctx, db)
	if err != nil {
		return err
	}
	err = os.Mkdir(folder, 0o700)
	if errors.Is(err, fs.ErrExist) {
		return keepFromOthers(folder, asRoot)
	}
	if err != nil {
		return fmt.Errorf("sqlite: make the backups folder %s: %w", folder, err)
	}
	if !asRoot {
		return nil
	}
	return giveFolder(folder, owner)
}

// keepFromOthers sets an existing folder its group or others may open to 0700, through any link, refusing as root.
func keepFromOthers(folder string, asRoot bool) error {
	info, err := os.Stat(folder)
	if err == nil && othersMayOpen(info) {
		err = closeFolder(folder, asRoot)
	}
	if err != nil {
		return fmt.Errorf("sqlite: keep the backups folder %s from other users: %w", folder, err)
	}
	return nil
}

// closeFolder sets folder to 0700, or as root leaves it as it is and returns errRootLeavesFolder.
func closeFolder(folder string, asRoot bool) error {
	if asRoot {
		return errRootLeavesFolder
	}
	return chmod(folder, 0o700)
}

// othersMayOpen reports whether info is of a folder its group or others may open.
func othersMayOpen(info fs.FileInfo) bool {
	return info.IsDir() && info.Mode().Perm()&0o077 != 0
}

// databaseOwner returns the owner and the group of the main database file of db, and whether the process runs as root.
func databaseOwner(ctx context.Context, db *sql.DB) (syscall.Stat_t, bool, error) {
	var owner syscall.Stat_t
	if geteuid() != 0 {
		return owner, false, nil
	}
	var path string
	err := db.QueryRowContext(ctx, mainFileLookup).Scan(&path)
	if err == nil {
		err = syscall.Stat(path, &owner)
	}
	if err != nil {
		return owner, true, fmt.Errorf("sqlite: read the owner of the database file: %w", err)
	}
	return owner, true, nil
}

// giveFolder gives folder the owner and the group of owner through a handle that never follows a link.
func giveFolder(folder string, owner syscall.Stat_t) error {
	made, err := os.OpenFile(folder, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err == nil {
		err = errors.Join(made.Chown(int(owner.Uid), int(owner.Gid)), made.Close())
	}
	if err != nil {
		return errors.Join(
			fmt.Errorf("sqlite: give the backups folder %s the owner of the database file: %w", folder, err),
			os.Remove(folder))
	}
	return nil
}
