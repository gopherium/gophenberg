// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// nestingStoresWithPool returns the nesting stores together with the pool they stand on.
func nestingStoresWithPool(t *testing.T) (*postgres.ContentStore, *postgres.TypeStore, uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	items, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v", err)
	}
	return items, types, author, pool
}

// flatPage is the shared fixture under the name these tests use.
var flatPage = contenttest.FlatPage

func TestNestedReportsACountItCannotRead(t *testing.T) {
	t.Parallel()

	_, types, _, pool := nestingStoresWithPool(t)
	sabotage(t, pool, "ALTER TABLE core.content RENAME COLUMN parent_id TO parent_gone")

	if _, err := types.Nested(t.Context(), "page"); err == nil {
		t.Error("Nested() error = nil, want the failing count reported")
	}
}

func TestUpdateReportsNestedItemsItCannotCount(t *testing.T) {
	t.Parallel()

	_, types, _, pool := nestingStoresWithPool(t)
	sabotage(t, pool, "ALTER TABLE core.content RENAME COLUMN parent_id TO parent_gone")

	_, err := types.Update(t.Context(), flatPage())

	if err == nil || errors.Is(err, content.ErrNestingInUse) {
		t.Errorf("Update() error = %v, want the failing count reported as such", err)
	}
}
