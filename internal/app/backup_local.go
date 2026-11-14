// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"fmt"
	"io/fs"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"
)

// needsLocalDisk words a backup whose folder sits on a drive that cannot hold one, and returns any other err.
func needsLocalDisk(names gonsole.Env, err error) error {
	var drive *dbsqlite.UnsupportedFileSystemError
	if !errors.As(err, &drive) {
		return err
	}
	advice := "so set " + names.Key("DATA_DIR") + " to a folder on a local disk, then copy the backups off the server"
	if drive.FileSystem != "" {
		return fmt.Errorf("the backups folder %s is on a network, shared or FUSE drive (%s), %s",
			drive.Folder, drive.FileSystem, advice)
	}
	return fmt.Errorf("the drive of the backups folder %s cannot move a file without replacing one, %s%s",
		drive.Folder, advice, leftCopy(err))
}

// leftCopy returns the words that name the copy a failed backup could not remove, or nothing when it removed it.
func leftCopy(err error) string {
	var left *fs.PathError
	if !errors.As(err, &left) || left.Op != "remove" {
		return ""
	}
	return ", and delete the copy " + left.Path + " the backup could not remove"
}
