// SPDX-License-Identifier: Apache-2.0

package app

import (
	"errors"
	"fmt"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/gonsole"
)

// shortOfSpace words a backup that stopped on too little free space with its megabytes, and returns any other err.
func shortOfSpace(names gonsole.Env, err error) error {
	var short *dbsqlite.SpaceError
	if !errors.As(err, &short) {
		return err
	}
	return fmt.Errorf("the backups folder %s has %d MB free, and the backup needs %d MB for the copy and %s, so free "+
		"some disk space first, for example by moving backups off the server", short.Folder, short.Free>>20,
		(short.Needed-1)>>20+1, names.Key("UPDATE_DISK_MARGIN_MB"))
}
