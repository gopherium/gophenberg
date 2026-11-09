// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/version"
)

// minimumVersionRefusal returns an error when stored names a minimum version newer than this binary, nil for none.
func minimumVersionRefusal(stored string) error {
	if !version.Before(version.Version(), stored) {
		return nil
	}
	return fmt.Errorf("this database needs Gophenberg %s or newer and this is %s, so restore the backup taken "+
		"before the update to run an older version", stored, version.Version())
}

// guardMinimumVersion returns err, or the refusal of the stored minimum version once it was read.
func guardMinimumVersion(stored string, err error) error {
	if err != nil {
		return err
	}
	return minimumVersionRefusal(stored)
}

// guardPostgres returns an error when db runs too old a PostgreSQL or needs a newer Gophenberg than this one.
func guardPostgres(ctx context.Context, db *sql.DB) error {
	if err := postgres.CheckMajor(ctx, db); err != nil {
		return err
	}
	return guardMinimumVersion(postgres.StoredMinimumVersion(ctx, db))
}

// migratePostgresCore migrates the core schema on db, then raises the stored minimum version to this binary's.
func migratePostgresCore(ctx context.Context, db *sql.DB) error {
	if err := postgres.Migrate(ctx, db); err != nil {
		return err
	}
	return postgres.RaiseMinimumVersion(ctx, db, version.Minimum())
}

// guardedSQLite returns apply, run only once the stored minimum version of the file lets this binary on it.
func guardedSQLite(
	apply func(context.Context, *sql.DB, sqlite.MigrateOptions) error,
) func(context.Context, *sql.DB, sqlite.MigrateOptions) error {
	return func(ctx context.Context, db *sql.DB, opts sqlite.MigrateOptions) error {
		if err := guardMinimumVersion(sqlite.StoredMinimumVersion(ctx, db)); err != nil {
			return err
		}
		return apply(ctx, db, opts)
	}
}

// migrateSQLiteCore migrates the core tables on db, raises the stored minimum version, then records this build.
func migrateSQLiteCore(ctx context.Context, db *sql.DB, opts sqlite.MigrateOptions) error {
	if err := sqlite.Migrate(ctx, db, opts); err != nil {
		return err
	}
	raised := sqlite.RaiseMinimumVersion(ctx, db, version.Minimum())
	return errors.Join(raised, sqlite.RecordMigratedBy(ctx, db, thisBuild()))
}

// thisBuild returns the version and the commit of this binary.
func thisBuild() sqlite.Build {
	return sqlite.Build{Version: version.Version(), Commit: version.Commit()}
}
