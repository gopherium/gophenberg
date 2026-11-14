// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// errDeadlock is the failure every statement of a deadlocked connection answers with.
var errDeadlock = &pgconn.PgError{Code: "40P01", Message: "deadlock detected"}

// deadlockedConn is a connection whose every statement fails on a deadlock.
type deadlockedConn struct{}

// Exec fails on a deadlock.
func (deadlockedConn) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, errDeadlock
}

// Query fails on a deadlock and returns rows that end on one.
func (deadlockedConn) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return deadlockedRows{}, errDeadlock
}

// QueryRow returns a row that fails on a deadlock.
func (deadlockedConn) QueryRow(context.Context, string, ...any) pgx.Row {
	return deadlockedRow{}
}

// deadlockedRows are rows that end on a deadlock.
type deadlockedRows struct{ pgx.Rows }

// Err returns the deadlock.
func (deadlockedRows) Err() error { return errDeadlock }

// deadlockedRow is a row that fails on a deadlock.
type deadlockedRow struct{}

// Scan returns the deadlock.
func (deadlockedRow) Scan(...any) error { return errDeadlock }

func TestClassifiedMarksEachFailureOfTheConnectionAsBusy(t *testing.T) {
	t.Parallel()

	conn := classified{conn: deadlockedConn{}}
	cases := map[string]func() error{
		"an exec": func() error {
			_, err := conn.Exec(t.Context(), "UPDATE held")
			return err
		},
		"a query": func() error {
			_, err := conn.Query(t.Context(), "SELECT held")
			return err
		},
		"the rows of a query": func() error {
			rows, _ := conn.Query(t.Context(), "SELECT held")
			return rows.Err()
		},
		"a row": func() error {
			return conn.QueryRow(t.Context(), "SELECT held").Scan()
		},
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := call(); !errors.Is(err, dbkit.ErrBusy) || !errors.Is(err, errDeadlock) {
				t.Errorf("error = %v, want the deadlock kept under dbkit.ErrBusy", err)
			}
		})
	}
}
