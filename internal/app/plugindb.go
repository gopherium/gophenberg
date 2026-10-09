// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gopherium/framework/dbkit"
	"github.com/gopherium/framework/gonsole"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/gopherium/gophenberg/sdk"
)

// errLaneUnopened refuses a statement a plugin runs before every plugin registered.
var errLaneUnopened = errors.New("the plugin database share opens once every plugin registered")

// maxPluginDBConns is the most slots one plugin may be given in the share.
const maxPluginDBConns = 1000

// laneSettings is how the plugins' database share is sized and timed, and what of the pool it leaves the core.
type laneSettings struct {
	conns        int
	reserve      int
	queryTimeout time.Duration
	txTimeout    time.Duration
}

// laneSettingsFrom reads the plugins' database share settings, each with its default.
func laneSettingsFrom(getenv func(string) string) (laneSettings, error) {
	env := settingsEnv(getenv)
	conns, err := env.Count("PLUGIN_DB_CONNS", 4, gonsole.AtMost(maxPluginDBConns))
	if err != nil {
		return laneSettings{}, err
	}
	reserve, err := env.Count("PLUGIN_DB_RESERVE", 2, gonsole.AtMost(maxPluginDBConns))
	if err != nil {
		return laneSettings{}, err
	}
	queryTimeout, err := env.Duration("PLUGIN_QUERY_TIMEOUT", 5*time.Second)
	if err != nil {
		return laneSettings{}, err
	}
	txTimeout, err := env.Duration("PLUGIN_TX_TIMEOUT", 30*time.Second)
	if err != nil {
		return laneSettings{}, err
	}
	return laneSettings{conns: conns, reserve: reserve, queryTimeout: queryTimeout, txTimeout: txTimeout}, nil
}

// pluginLane is the one database share every plugin draws from, opened once every plugin registered.
type pluginLane struct {
	sdk.HostOnly
	share atomic.Pointer[dbkit.Share]
	db    *sql.DB
}

// open lends the share over the pool, with the setting's slots for each registered plugin, leaving the reserve free.
func (l *pluginLane) open(pool *pgxpool.Pool, plugins int, lane laneSettings) error {
	connections := int(pool.Config().MaxConns)
	room := connections - lane.reserve
	if room < 1 {
		return fmt.Errorf("GOPHENBERG_PLUGIN_DB_RESERVE=%d leaves plugins no connection in a pool of %d, "+
			"raise pool_max_conns in GOPHENBERG_DATABASE_URL or lower the reserve", lane.reserve, connections)
	}
	db := stdlib.OpenDBFromPool(pool)
	share, err := dbkit.NewShare(db, dbkit.Postgres, dbkit.ShareOptions{
		ID: "plugins", Slots: min(lane.conns*max(plugins, 1), room),
		StatementTimeout: lane.queryTimeout, TransactionTimeout: lane.txTimeout,
	})
	if err != nil {
		_ = db.Close()
		return err
	}
	l.db = db
	l.share.Store(share)
	return nil
}

// close closes the handle the share runs on, once it opened.
func (l *pluginLane) close() {
	if l.db != nil {
		_ = l.db.Close()
	}
}

// opened returns the share, refusing while the plugins register.
func (l *pluginLane) opened() (*dbkit.Share, error) {
	if share := l.share.Load(); share != nil {
		return share, nil
	}
	return nil, errLaneUnopened
}

// Exec runs a statement that returns no rows.
func (l *pluginLane) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	share, err := l.opened()
	if err != nil {
		return nil, err
	}
	return share.Exec(ctx, query, args...)
}

// Query runs a query that returns rows.
func (l *pluginLane) Query(ctx context.Context, query string, args ...any) (sdk.Rows, error) {
	share, err := l.opened()
	if err != nil {
		return nil, err
	}
	return rowsOf(share.Query(ctx, query, args...))
}

// QueryRow runs a query that returns at most one row.
func (l *pluginLane) QueryRow(ctx context.Context, query string, args ...any) sdk.Row {
	share, err := l.opened()
	if err != nil {
		return refusedRow{err: err}
	}
	return share.QueryRow(ctx, query, args...)
}

// Engine names the engine the share runs on.
func (*pluginLane) Engine() sdk.Engine {
	return sdk.Postgres
}

// Begin starts a transaction on the share.
func (l *pluginLane) Begin(ctx context.Context) (sdk.Tx, error) {
	share, err := l.opened()
	if err != nil {
		return nil, err
	}
	tx, err := share.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return laneTx{tx: tx}, nil
}

// rowsOf returns the rows a query answered as the SDK's, a nil Rows when the query failed.
func rowsOf(rows *dbkit.Rows, err error) (sdk.Rows, error) {
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// laneTx is a plugin's transaction on the share, answering the SDK's rows.
type laneTx struct {
	tx *dbkit.Tx
}

// Exec runs a statement in the transaction that returns no rows.
func (t laneTx) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return t.tx.Exec(ctx, query, args...)
}

// Query runs a query in the transaction that returns rows.
func (t laneTx) Query(ctx context.Context, query string, args ...any) (sdk.Rows, error) {
	return rowsOf(t.tx.Query(ctx, query, args...))
}

// QueryRow runs a query in the transaction that returns at most one row.
func (t laneTx) QueryRow(ctx context.Context, query string, args ...any) sdk.Row {
	return t.tx.QueryRow(ctx, query, args...)
}

// Commit ends the transaction, keeping its writes.
func (t laneTx) Commit() error {
	return t.tx.Commit()
}

// Rollback ends the transaction, dropping its writes.
func (t laneTx) Rollback() error {
	return t.tx.Rollback()
}

// refusedRow is the row a refused query answers.
type refusedRow struct {
	err error
}

// Scan answers the refusal.
func (r refusedRow) Scan(...any) error {
	return r.err
}

// Err answers the refusal.
func (r refusedRow) Err() error {
	return r.err
}
