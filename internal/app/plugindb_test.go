// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"
	"github.com/gopherium/framework/gonsole/testkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/sdk"
)

// lender is a plugin that keeps the dependencies it was lent and what its database answered while it registered.
type lender struct {
	id          string
	deps        sdk.Deps
	registering error
}

// ID returns the plugin's identifier.
func (l *lender) ID() string {
	return l.id
}

// Start readies nothing.
func (*lender) Start(context.Context) error {
	return nil
}

// Stop stops nothing.
func (*lender) Stop(context.Context) error {
	return nil
}

// lending registers one lender per id, each keeping its dependencies and trying its database while it registers.
func lending(lenders ...*lender) func(sdk.Deps) ([]sdk.Plugin, error) {
	return func(deps sdk.Deps) ([]sdk.Plugin, error) {
		plugins := make([]sdk.Plugin, 0, len(lenders))
		for _, l := range lenders {
			l.deps = deps
			_, l.registering = deps.DB.Exec(context.Background(), "SELECT 1")
			plugins = append(plugins, l)
		}
		return plugins, nil
	}
}

// lentSite composes a site over an empty database with the settings, lending the plugins their share.
func lentSite(t *testing.T, settings map[string]string, lenders ...*lender) site {
	t.Helper()
	built, err := compose(t.Context(), composeConfig{
		databaseURL: emptyDatabaseURL(t), fieldDepth: 4, getenv: testkit.Getenv(settings),
	}, lending(lenders...))
	if err != nil {
		t.Fatalf("compose() error = %v, want the plugins lent their share", err)
	}
	t.Cleanup(built.close)
	return built
}

// heldRows opens count result sets on the share, each holding a slot until the test ends.
func heldRows(t *testing.T, db sdk.DB, count int) {
	t.Helper()
	for range count {
		rows, err := db.Query(t.Context(), "SELECT 1")
		if err != nil {
			t.Fatalf("Query() error = %v, want a free slot", err)
		}
		t.Cleanup(func() { _ = rows.Close() })
	}
}

// probing returns a context that gives a statement a short wait for a slot, well inside the held rows' timeout.
func probing(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	t.Cleanup(cancel)
	return ctx
}

func TestAPluginReachesTheSiteDatabaseThroughItsShare(t *testing.T) {
	t.Parallel()

	notes := &lender{id: "notes"}
	lentSite(t, nil, notes)
	db := notes.deps.DB

	_, createErr := db.Exec(t.Context(), "CREATE TABLE plugin_notes__notes (body text)")
	_, insertErr := db.Exec(t.Context(), "INSERT INTO plugin_notes__notes (body) VALUES ($1)", "bread at six")
	var body string
	readErr := db.QueryRow(t.Context(), "SELECT body FROM plugin_notes__notes").Scan(&body)

	if createErr != nil || insertErr != nil || readErr != nil || body != "bread at six" {
		t.Errorf("create %v, insert %v, read %q with %v, want the plugin's own table written and read",
			createErr, insertErr, body, readErr)
	}
	if db.Engine() != sdk.Postgres {
		t.Errorf("Engine() = %q, want postgres", db.Engine())
	}
}

func TestAPluginShareRefusesUseWhileThePluginsRegister(t *testing.T) {
	t.Parallel()

	notes := &lender{id: "notes"}

	lentSite(t, nil, notes)

	if notes.registering == nil || !strings.Contains(notes.registering.Error(), "registered") {
		t.Errorf("Exec() during Register = %v, want a refusal saying the share opens once every plugin registered",
			notes.registering)
	}
}

func TestAnUnopenedShareRefusesEveryStatement(t *testing.T) {
	t.Parallel()

	share := &pluginLane{}
	var scanned int

	_, execErr := share.Exec(t.Context(), "SELECT 1")
	rows, queryErr := share.Query(t.Context(), "SELECT 1")
	row := share.QueryRow(t.Context(), "SELECT 1")
	tx, beginErr := share.Begin(t.Context())

	if execErr == nil || queryErr == nil || rows != nil || row.Err() == nil || row.Scan(&scanned) == nil ||
		beginErr == nil || tx != nil {
		t.Errorf("an unopened share answered exec %v, query %v %v, row %v, begin %v %v, want every one refused",
			execErr, rows, queryErr, row.Err(), tx, beginErr)
	}
}

func TestAShareTheOptionsRefuseAnswersTheRefusalToEveryStatement(t *testing.T) {
	t.Parallel()

	pool, err := pgxpool.New(t.Context(), unreachableDatabaseURL)
	if err != nil {
		t.Fatalf("pgxpool.New() error = %v, want a pool", err)
	}
	t.Cleanup(pool.Close)
	share := &pluginLane{}

	share.open(pool, 1, laneSettings{})
	t.Cleanup(share.close)
	_, err = share.Exec(t.Context(), "SELECT 1")

	if err == nil || !strings.Contains(err.Error(), "Slots") {
		t.Errorf("Exec() on a share opened with no slots = %v, want the share's own refusal", err)
	}
}

func TestAPluginQueryThatFailsAnswersNoRows(t *testing.T) {
	t.Parallel()

	notes := &lender{id: "notes"}
	lentSite(t, nil, notes)

	rows, err := notes.deps.DB.Query(t.Context(), "SELEC 1")

	if err == nil || rows != nil {
		t.Errorf("Query() = %v, %v, want a nil Rows and the error", rows, err)
	}
}

func TestThePluginShareHoldsFourSlotsForAPluginByDefault(t *testing.T) {
	t.Parallel()

	notes := &lender{id: "notes"}
	lentSite(t, nil, notes)
	db := notes.deps.DB

	heldRows(t, db, 4)
	_, err := db.Exec(probing(t), "SELECT 1")

	if !errors.Is(err, dbkit.ErrShareFull) {
		t.Errorf("a fifth statement = %v, want the share full after the four slots one plugin adds by default", err)
	}
}

func TestThePluginShareHoldsTheSettingsSlotsForEachRegisteredPlugin(t *testing.T) {
	t.Parallel()

	notes, calendar := &lender{id: "notes"}, &lender{id: "calendar"}
	lentSite(t, map[string]string{"GOPHENBERG_PLUGIN_DB_CONNS": "1"}, notes, calendar)
	db := notes.deps.DB

	heldRows(t, db, 2)
	_, err := db.Exec(probing(t), "SELECT 1")

	if !errors.Is(err, dbkit.ErrShareFull) {
		t.Errorf("a third statement = %v, want the share full after one slot for each of two plugins", err)
	}
}

func TestAPluginTransactionWaitsForAFreeSlot(t *testing.T) {
	t.Parallel()

	notes := &lender{id: "notes"}
	lentSite(t, map[string]string{"GOPHENBERG_PLUGIN_DB_CONNS": "1"}, notes)
	db := notes.deps.DB

	heldRows(t, db, 1)
	tx, err := db.Begin(probing(t))

	if !errors.Is(err, dbkit.ErrShareFull) || tx != nil {
		t.Errorf("Begin() on a full share = %v, %v, want a nil Tx and the share full", tx, err)
	}
}

func TestAPluginTransactionCommitsAndRollsBack(t *testing.T) {
	t.Parallel()

	notes := &lender{id: "notes"}
	lentSite(t, nil, notes)
	db := notes.deps.DB
	if _, err := db.Exec(t.Context(), "CREATE TABLE plugin_notes__notes (body text)"); err != nil {
		t.Fatalf("creating the table: %v", err)
	}

	kept := writeIn(t, db, "kept", func(tx sdk.Tx) error { return tx.Commit() })
	dropped := writeIn(t, db, "dropped", func(tx sdk.Tx) error { return tx.Rollback() })

	var count int
	if err := db.QueryRow(t.Context(), "SELECT count(*) FROM plugin_notes__notes").Scan(&count); err != nil {
		t.Fatalf("counting the notes: %v", err)
	}
	if kept != nil || dropped != nil || count != 1 {
		t.Errorf("commit %v, rollback %v, %d notes left, want the committed note alone", kept, dropped, count)
	}
}

// writeIn inserts body in a transaction, reads it back inside, and ends the transaction with end.
func writeIn(t *testing.T, db sdk.DB, body string, end func(sdk.Tx) error) error {
	t.Helper()
	tx, err := db.Begin(t.Context())
	if err != nil {
		t.Fatalf("Begin() error = %v, want a transaction", err)
	}
	if _, err := tx.Exec(t.Context(), "INSERT INTO plugin_notes__notes (body) VALUES ($1)", body); err != nil {
		t.Fatalf("inserting %q: %v", body, err)
	}
	var seen string
	inside := tx.QueryRow(t.Context(), "SELECT body FROM plugin_notes__notes WHERE body = $1", body)
	if err := inside.Scan(&seen); err != nil {
		t.Fatalf("reading %q inside the transaction: %v", body, err)
	}
	rows, err := tx.Query(t.Context(), "SELECT body FROM plugin_notes__notes")
	if err != nil {
		t.Fatalf("listing inside the transaction: %v", err)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("closing the listing: %v", err)
	}
	return end(tx)
}

func TestAPluginTransactionQueryThatFailsAnswersNoRows(t *testing.T) {
	t.Parallel()

	notes := &lender{id: "notes"}
	lentSite(t, nil, notes)
	tx, err := notes.deps.DB.Begin(t.Context())
	if err != nil {
		t.Fatalf("Begin() error = %v, want a transaction", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })

	rows, err := tx.Query(t.Context(), "SELEC 1")

	if err == nil || rows != nil {
		t.Errorf("Query() in a transaction = %v, %v, want a nil Rows and the error", rows, err)
	}
}

func TestAPluginTransactionEndsAtTheTransactionTimeout(t *testing.T) {
	t.Parallel()

	notes := &lender{id: "notes"}
	lentSite(t, map[string]string{"GOPHENBERG_PLUGIN_TX_TIMEOUT": "1s"}, notes)
	if _, err := notes.deps.DB.Exec(t.Context(), "SELECT 1"); err != nil {
		t.Fatalf("opening a first connection: %v", err)
	}
	tx, err := notes.deps.DB.Begin(t.Context())
	if err != nil {
		t.Fatalf("Begin() error = %v, want a transaction", err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })

	_, err = tx.Exec(t.Context(), "SELECT pg_sleep(3)")

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a statement past the transaction timeout = %v, want the transaction's deadline", err)
	}
}

func TestComposeRefusesAMalformedPluginDatabaseSetting(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]string{
		"GOPHENBERG_PLUGIN_DB_CONNS":      "0",
		"GOPHENBERG_PLUGIN_QUERY_TIMEOUT": "soon",
		"GOPHENBERG_PLUGIN_TX_TIMEOUT":    "-1s",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := compose(t.Context(), composeConfig{
				databaseURL: unreachableDatabaseURL, fieldDepth: 4, getenv: testkit.Getenv(map[string]string{name: value}),
			}, noPlugins)

			if err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("compose() with %s=%q = %v, want an error naming the setting", name, value, err)
			}
		})
	}
}

func TestClosingTheSiteClosesThePluginShare(t *testing.T) {
	t.Parallel()

	notes := &lender{id: "notes"}
	built, err := compose(t.Context(), composeConfig{databaseURL: emptyDatabaseURL(t), fieldDepth: 4}, lending(notes))
	if err != nil {
		t.Fatalf("compose() error = %v, want nil", err)
	}

	built.close()
	_, err = notes.deps.DB.Exec(t.Context(), "SELECT 1")

	if err == nil {
		t.Error("Exec() after the site closed = nil, want the closed share refused")
	}
}
