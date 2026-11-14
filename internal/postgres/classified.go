// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"

	dbkitpg "github.com/gopherium/framework/dbkit/postgres"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// classified sends statements on conn, marking each failure with its dbkit class.
type classified struct {
	conn db.DBTX
}

// Exec runs a statement that returns no rows.
func (c classified) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	tag, err := c.conn.Exec(ctx, sql, args...)
	return tag, dbkitpg.Classify(err)
}

// Query runs a statement that returns rows.
func (c classified) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	rows, err := c.conn.Query(ctx, sql, args...)
	return classifiedRows{rows}, dbkitpg.Classify(err)
}

// QueryRow runs a statement that returns at most one row.
func (c classified) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return classifiedRow{c.conn.QueryRow(ctx, sql, args...)}
}

// classifiedRows reads the rows of one statement, marking the failure that ends them with its dbkit class.
type classifiedRows struct {
	pgx.Rows
}

// Err returns the failure that ended the rows.
func (r classifiedRows) Err() error {
	return dbkitpg.Classify(r.Rows.Err())
}

// classifiedRow reads the row of one statement, marking a failure with its dbkit class.
type classifiedRow struct {
	pgx.Row
}

// Scan reads the row into dest.
func (r classifiedRow) Scan(dest ...any) error {
	return dbkitpg.Classify(r.Row.Scan(dest...))
}
