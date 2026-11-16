// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content/contentstore"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// contentTx sends the statements of one content write and opens savepoints on it.
type contentTx struct {
	contentQueries
	tx *sql.Tx
}

// contentInTx returns the transactor running each content write in one transaction on handle, marking its failure.
func contentInTx(handle *sql.DB, queries *db.Queries) contentstore.Transactor {
	return func(ctx context.Context, write func(contentstore.Tx) error) error {
		return dbsqlite.Classify(dbkit.InTx(ctx, handle, func(tx *sql.Tx) error {
			return write(contentTx{contentQueries: contentQueries{typeQueries{queries: queries.WithTx(tx)}}, tx: tx})
		}))
	}
}

const (
	// markAttempt marks where a restore attempt starts inside its write.
	markAttempt = "SAVEPOINT restore_attempt"
	// undoAttempt rolls the write back to where the restore attempt started.
	undoAttempt = "ROLLBACK TO restore_attempt"
	// releaseAttempt drops the mark of the restore attempt, keeping what the write holds.
	releaseAttempt = "RELEASE restore_attempt"
)

// Savepoint runs attempt on a savepoint of the transaction, rolling back to it when attempt fails.
func (q contentTx) Savepoint(ctx context.Context, attempt func(contentstore.Queries) error) error {
	if _, err := q.tx.ExecContext(ctx, markAttempt); err != nil {
		return err
	}
	if err := attempt(q.contentQueries); err != nil {
		return q.undo(ctx, err)
	}
	_, err := q.tx.ExecContext(ctx, releaseAttempt)
	return err
}

// undo rolls the write back to the savepoint and releases it, returning failed, or its own failure with failed's text.
func (q contentTx) undo(ctx context.Context, failed error) error {
	for _, statement := range []string{undoAttempt, releaseAttempt} {
		if _, err := q.tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("undoing the restore attempt after %v: %w", failed, err)
		}
	}
	return failed
}
