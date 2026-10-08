// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"database/sql"
)

// Engine names the database engine a site runs on.
type Engine string

const (
	// Postgres is the PostgreSQL engine.
	Postgres Engine = "postgres"
	// SQLite is the SQLite engine.
	SQLite Engine = "sqlite"
)

// Querier runs statements written with $1, $2 placeholders, each within the site's statement timeout.
type Querier interface {
	Exec(ctx context.Context, query string, args ...any) (sql.Result, error)
	Query(ctx context.Context, query string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) Row
}

// Rows is the answer of a query, read one row at a time and closed when done.
type Rows interface {
	Next() bool
	NextResultSet() bool
	Scan(dest ...any) error
	Close() error
	Err() error
}

// Row is the answer of a query that returns at most one row.
type Row interface {
	Scan(dest ...any) error
	Err() error
}

// Tx is a transaction on a plugin's share of the site database, ending at the site's transaction timeout.
type Tx interface {
	Querier
	Commit() error
	Rollback() error
}

// DB is a plugin's capped share of the site database, which only the site implements.
type DB interface {
	Querier
	Engine() Engine
	Begin(ctx context.Context) (Tx, error)
	hostOnly()
}

// HostOnly seals an SDK interface to the site, which embeds it in the type it lends.
type HostOnly struct{}

// hostOnly marks the type as the site's.
func (HostOnly) hostOnly() {}
