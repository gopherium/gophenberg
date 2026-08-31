// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// keyCollision reports whether the error is the unique key refused.
func keyCollision(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// plantSubField stores one field row under the given parent and returns its identity.
func plantSubField(t *testing.T, pool *pgxpool.Pool, groupID int, parent *int, key string) (int, error) {
	t.Helper()
	var id int
	err := pool.QueryRow(t.Context(),
		`INSERT INTO core.content_fields (group_id, parent_field_id, key, label, kind, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 'text', now(), now())
		RETURNING id`,
		groupID, parent, key).Scan(&id)
	return id, err
}

// fieldRowsHolding counts the field rows carrying the key inside the group.
func fieldRowsHolding(t *testing.T, pool *pgxpool.Pool, groupID int, key string) int {
	t.Helper()
	var held int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM core.content_fields WHERE group_id = $1 AND key = $2`,
		groupID, key).Scan(&held); err != nil {
		t.Fatalf("counting field rows: %v, want nil", err)
	}
	return held
}

func TestFieldKeysRepeatAcrossParentsAndCollideInsideOne(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareTypedField(t, store, "car", "specs")
	extras := declareTypedField(t, store, "car", "extras")

	if _, err := plantSubField(t, pool, specs.GroupID, &specs.ID, "title"); err != nil {
		t.Fatalf("planting title under specs: %v, want nil", err)
	}
	if _, err := plantSubField(t, pool, extras.GroupID, &extras.ID, "title"); err != nil {
		t.Errorf("planting title under extras: %v, want both parents to hold one", err)
	}
	if _, err := plantSubField(t, pool, specs.GroupID, &specs.ID, "title"); !keyCollision(err) {
		t.Errorf("planting title twice under specs: error = %v, want the key collision refused", err)
	}
}

func TestFieldKeysStillCollideAtTheTopOfAGroup(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareTypedField(t, store, "car", "specs")

	if _, err := plantSubField(t, pool, specs.GroupID, nil, "specs"); !keyCollision(err) {
		t.Errorf("planting specs twice at the top: error = %v, want the key collision refused", err)
	}
}

func TestDeletingAParentFieldRowTakesEveryDescendant(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareTypedField(t, store, "car", "specs")

	rows, err := plantSubField(t, pool, specs.GroupID, &specs.ID, "rows")
	if err != nil {
		t.Fatalf("planting rows under specs: %v, want nil", err)
	}
	if _, err := plantSubField(t, pool, specs.GroupID, &rows, "title"); err != nil {
		t.Fatalf("planting title under rows: %v, want nil", err)
	}

	if _, err := pool.Exec(t.Context(),
		`DELETE FROM core.content_fields WHERE id = $1`, specs.ID); err != nil {
		t.Fatalf("deleting the parent row: %v, want nil", err)
	}

	if held := fieldRowsHolding(t, pool, specs.GroupID, "title"); held != 0 {
		t.Errorf("rows holding title = %d, want the grandchild gone with its line", held)
	}
}
