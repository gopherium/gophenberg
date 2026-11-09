// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gopherium/framework/gonsole"

	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/backupfiles"
	"github.com/gopherium/gophenberg/internal/updater"
)

// backupCommandName is the name of the command that backs up a SQLite file on demand.
const backupCommandName = "database:backup"

// errPgDumpBackup answers database:backup on a PostgreSQL database, which pg_dump backs up.
var errPgDumpBackup = errors.New(backupCommandName + " copies a SQLite file only, so back up a PostgreSQL database " +
	"with pg_dump, as " + updatesPage + "#what-to-back-up shows")

// databaseBackupCommand returns the command that copies a SQLite file into its backups folder now.
func databaseBackupCommand() gonsole.Command {
	return onDemandBackup{take: sqlite.TakeBackup}.command()
}

// onDemandBackup takes the backup database:backup asks for through the copy it holds.
type onDemandBackup struct {
	take func(context.Context, *sql.DB, sqlite.BackupRequest) (sqlite.TakenBackup, error)
}

// command returns database:backup over the copy b holds.
func (b onDemandBackup) command() gonsole.Command {
	return gonsole.Command{Name: backupCommandName, Summary: "back up the SQLite file into the backups folder", Run: b.run}
}

// run copies the SQLite file of call into its backups folder and prints where the copy went.
func (b onDemandBackup) run(ctx context.Context, call gonsole.Call) error {
	address, err := call.DatabaseURL()
	if err != nil {
		return err
	}
	if !onSQLite(address) {
		return errPgDumpBackup
	}
	layout, err := updater.BackupSettingsFrom(call.Env)
	if err != nil {
		return err
	}
	db, err := call.DB(ctx)
	if err != nil {
		return err
	}
	taken, err := b.backUp(ctx, db, layout, address)
	if err != nil {
		return err
	}
	return answerBackup(call, taken)
}

// backUp makes the data folder of layout, then takes a manual backup of db into the folder of the file at address.
func (b onDemandBackup) backUp(
	ctx context.Context, db *sql.DB, layout updater.Layout, address string,
) (sqlite.TakenBackup, error) {
	if err := layout.MakeData(); err != nil {
		return sqlite.TakenBackup{}, err
	}
	return b.take(ctx, db, sqlite.BackupRequest{Folder: backupsFolder(layout, address), Kind: backupfiles.Manual})
}

// answerBackup prints where taken went, then on a line of its own the step after the copy that failed.
func answerBackup(call gonsole.Call, taken sqlite.TakenBackup) error {
	_, err := fmt.Fprintf(call.Stdout, "backed up the database to %s\n", taken.Path)
	if taken.AfterCopy == nil {
		return err
	}
	_, warned := fmt.Fprintf(call.Stderr, "the backup stands, though a step after the copy failed: %v\n", taken.AfterCopy)
	return errors.Join(err, warned)
}
