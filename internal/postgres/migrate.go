// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
)

// versionTable is goose's default version table, which records the core migrations.
const versionTable = "goose_db_version"

var migrationSource = mustSub(Migrations, "migrations")

// Migrate applies the core schema migrations on db under goose's advisory lock, opening nothing itself.
func Migrate(ctx context.Context, db *sql.DB) error {
	if err := dbkitpg.Migrate(ctx, db, dbkitpg.Migrations{Table: versionTable, FS: migrationSource}); err != nil {
		return fmt.Errorf("postgres: apply migrations: %w", dbkitpg.Classify(err))
	}
	return nil
}

// mustSub returns the dir subtree of fsys and panics if it cannot be created.
func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err)
	}
	return sub
}
