// SPDX-License-Identifier: Apache-2.0

package sdk

import (
	"context"
	"database/sql"
	"testing"
)

// siteDB is a database share the way a site builds one, sealed by HostOnly.
type siteDB struct {
	HostOnly
}

// Exec runs nothing.
func (siteDB) Exec(context.Context, string, ...any) (sql.Result, error) {
	return nil, nil
}

// Query answers no rows.
func (siteDB) Query(context.Context, string, ...any) (Rows, error) {
	return nil, nil
}

// QueryRow answers no row.
func (siteDB) QueryRow(context.Context, string, ...any) Row {
	return nil
}

// Engine names PostgreSQL.
func (siteDB) Engine() Engine {
	return Postgres
}

// Begin starts no transaction.
func (siteDB) Begin(context.Context) (Tx, error) {
	return nil, nil
}

func TestASiteBuildsADatabaseShareByEmbeddingHostOnly(t *testing.T) {
	t.Parallel()

	var share DB = siteDB{}

	share.hostOnly()
	if share.Engine() != Postgres {
		t.Errorf("Engine() = %q, want the engine the site names", share.Engine())
	}
}

func TestEnginesCarryTheirStableNames(t *testing.T) {
	t.Parallel()

	if Postgres != "postgres" || SQLite != "sqlite" {
		t.Errorf("engines are %q and %q, want postgres and sqlite, the names plugins compare", Postgres, SQLite)
	}
}
