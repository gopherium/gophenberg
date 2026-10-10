// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// carType is the shared fixture under the name these tests use.
var carType = contenttest.CarType

func TestTypeStoreWrapsDatabaseFailures(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	store := postgres.NewTypeStore(pool)
	pool.Close()

	_, list := store.List(t.Context())
	_, byKey := store.ByKey(t.Context(), content.TypePost)
	_, created := store.Create(t.Context(), carType(t))
	_, updated := store.Update(t.Context(), carType(t))
	deleted := store.Delete(t.Context(), content.TypePost)

	for name, err := range map[string]error{
		"List": list, "ByKey": byKey, "Create": created, "Update": updated, "Delete": deleted,
	} {
		if err == nil {
			t.Errorf("%s() on a closed pool error = nil, want a failure", name)
		}
	}
}

// closedTypeStore returns a type store whose pool is already closed.
func closedTypeStore(t *testing.T) *postgres.TypeStore {
	t.Helper()
	_, _, pool := newContentStoreWithPool(t)
	store := postgres.NewTypeStore(pool)
	pool.Close()
	return store
}

func TestTypeStoreReportsADatabaseItCannotReach(t *testing.T) {
	t.Parallel()

	store := closedTypeStore(t)
	field := content.Field{TypeKey: content.TypePost, Key: "color", Label: "Colour", Kind: content.FieldKindText}

	for name, run := range map[string]func() error{
		"listing the registry": func() error {
			_, err := store.List(t.Context())
			return err
		},
		"reading one type": func() error {
			_, err := store.ByKey(t.Context(), content.TypePost)
			return err
		},
		"editing a field": func() error {
			_, err := store.UpdateFieldInGroup(t.Context(), 1, field, field.UpdatedAt, nil)
			return err
		},
		"removing a field": func() error {
			return store.DeleteFieldInGroup(t.Context(), 1, "color", nil)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := run(); err == nil {
				t.Errorf("%s: error = nil, want the closed pool reported", name)
			}
		})
	}
}
