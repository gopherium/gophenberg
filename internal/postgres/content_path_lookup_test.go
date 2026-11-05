// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// errStatementHeld ends a read once its statement is held.
var errStatementHeld = errors.New("statement held")

// heldRow is the row a held read answers with.
type heldRow struct{}

// Scan reports that the read never ran.
func (heldRow) Scan(...any) error {
	return errStatementHeld
}

// statementHolder keeps the statement a read sends instead of running it.
type statementHolder struct {
	db.DBTX
	statement string
}

// QueryRow keeps the statement.
func (h *statementHolder) QueryRow(_ context.Context, statement string, _ ...any) pgx.Row {
	h.statement = statement
	return heldRow{}
}

// publishedLookup returns the statement a read of published content by path sends.
func publishedLookup(t *testing.T) string {
	t.Helper()
	holder := &statementHolder{}
	if _, err := db.New(holder).GetPublishedContentByPath(t.Context(), "held"); !errors.Is(err, errStatementHeld) {
		t.Fatalf("holding the statement: %v, want %v", err, errStatementHeld)
	}
	return holder.statement
}

// planOf returns the plan the connection draws for the statement.
func planOf(t *testing.T, held *sql.Conn, statement string, args ...any) string {
	t.Helper()
	rows, err := held.QueryContext(t.Context(), `EXPLAIN (FORMAT TEXT) `+statement, args...)
	if err != nil {
		t.Fatalf("planning the statement: %v, want nil", err)
	}
	defer func() { _ = rows.Close() }()
	var plan strings.Builder
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("reading the plan: %v, want nil", err)
		}
		plan.WriteString(line + "\n")
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the plan: %v, want nil", err)
	}
	return plan.String()
}

func TestPublishedLookupKeepsToThePrefixIndexAfterTheUniqueIndexIsRebuilt(t *testing.T) {
	t.Parallel()

	handle := newTestDB(t)
	if _, err := handle.ExecContext(t.Context(), `REINDEX INDEX CONCURRENTLY core.content_path_unique`); err != nil {
		t.Fatalf("rebuilding the unique index: %v, want nil", err)
	}

	plan := planOf(t, indexedConn(t, handle), publishedLookup(t), "a-stored-path")

	if !strings.Contains(plan, "content_path_prefix_idx") {
		t.Errorf("plan = %q, want the prefix index to answer", plan)
	}
}
