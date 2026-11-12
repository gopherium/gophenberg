// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gopherium/gophenberg/internal/version"
)

const (
	// minimumVersionKey is the site setting that holds the oldest version allowed to run on the database.
	minimumVersionKey = "minimum_version"
	// undefinedTableCode is the PostgreSQL error code of a table that does not exist.
	undefinedTableCode = "42P01"
	// minimumVersionLookup reads the stored minimum version.
	minimumVersionLookup = "SELECT value FROM core.settings WHERE key = $1"
	// minimumVersionRaise stores a minimum version in place of the one read before it, unless one was stored since.
	minimumVersionRaise = `INSERT INTO core.settings (key, value) VALUES ($1, $2)
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value WHERE core.settings.value = $3`
)

// StoredMinimumVersion returns the minimum version stored on db, empty before the core tables exist or while none is.
func StoredMinimumVersion(ctx context.Context, db *sql.DB) (string, error) {
	var minimum string
	err := db.QueryRowContext(ctx, minimumVersionLookup, minimumVersionKey).Scan(&minimum)
	switch {
	case errors.Is(err, sql.ErrNoRows), missingTable(err):
		return "", nil
	case err != nil:
		return "", fmt.Errorf("postgres: read the stored minimum version: %w", dbkitpg.Classify(err))
	}
	return minimum, nil
}

// RaiseMinimumVersion stores minimum on db when the stored one comes before it, reading again after a lost race.
func RaiseMinimumVersion(ctx context.Context, db *sql.DB, minimum string) error {
	for {
		stored, err := StoredMinimumVersion(ctx, db)
		if err != nil || !version.Before(stored, minimum) {
			return err
		}
		raised, err := raiseFrom(ctx, db, stored, minimum)
		if err != nil || raised {
			return err
		}
	}
}

// raiseFrom stores minimum in place of stored, reporting whether stored was still the stored minimum version.
func raiseFrom(ctx context.Context, db *sql.DB, stored, minimum string) (bool, error) {
	result, err := db.ExecContext(ctx, minimumVersionRaise, minimumVersionKey, minimum, stored)
	if err != nil {
		return false, fmt.Errorf("postgres: raise the stored minimum version to %s: %w", minimum, dbkitpg.Classify(err))
	}
	rows, err := result.RowsAffected()
	return rows > 0, err
}

// missingTable reports whether err is PostgreSQL's answer to a query on a table that does not exist.
func missingTable(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == undefinedTableCode
}
