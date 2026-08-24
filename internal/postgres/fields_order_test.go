// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// declaredKeysOf returns the field keys of a stored type in declared order.
func declaredKeysOf(t *testing.T, types *postgres.TypeStore, typeKey string) []string {
	t.Helper()
	held, err := types.ByKey(t.Context(), typeKey)
	if err != nil {
		t.Fatalf("ByKey(%q) error = %v, want nil", typeKey, err)
	}
	keys := make([]string, len(held.Fields))
	for i, f := range held.Fields {
		keys[i] = f.Key
	}
	return keys
}

// threeStoredFields declares three text fields on the post type in a fixed order.
func threeStoredFields(t *testing.T, types *postgres.TypeStore) {
	t.Helper()
	for _, key := range []string{"color", "engine", "doors"} {
		built, err := content.NewField(content.Field{
			TypeKey: content.TypePost, Key: key, Label: key, Kind: content.FieldKindText,
		})
		if err != nil {
			t.Fatalf("NewField(%q) error = %v, want nil", key, err)
		}
		if _, err := types.CreateField(t.Context(), built); err != nil {
			t.Fatalf("CreateField(%q) error = %v, want nil", key, err)
		}
	}
}

func TestFieldsListInTheStoredOrder(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	threeStoredFields(t, types)

	err := types.ReorderFields(t.Context(), content.TypePost, []string{"doors", "color", "engine"})

	if err != nil {
		t.Fatalf("ReorderFields() error = %v, want nil", err)
	}
	want := []string{"doors", "color", "engine"}
	if got := declaredKeysOf(t, types, content.TypePost); !slices.Equal(got, want) {
		t.Errorf("fields = %v, want %v", got, want)
	}
}

func TestAFieldDeclaredAfterAReorderListsLast(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	threeStoredFields(t, types)
	if err := types.ReorderFields(t.Context(), content.TypePost, []string{"doors", "color", "engine"}); err != nil {
		t.Fatalf("ReorderFields() error = %v, want nil", err)
	}

	built, err := content.NewField(content.Field{
		TypeKey: content.TypePost, Key: "seats", Label: "Seats", Kind: content.FieldKindText,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if _, err := types.CreateField(t.Context(), built); err != nil {
		t.Fatalf("CreateField() error = %v, want nil", err)
	}

	want := []string{"doors", "color", "engine", "seats"}
	if got := declaredKeysOf(t, types, content.TypePost); !slices.Equal(got, want) {
		t.Errorf("fields = %v, want the new field declared last: %v", got, want)
	}
}

func TestReorderFieldsReportsAStoreItCannotWrite(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	threeStoredFields(t, types)
	pool.Close()

	err := types.ReorderFields(t.Context(), content.TypePost, []string{"doors", "color", "engine"})

	if err == nil {
		t.Error("ReorderFields() error = nil, want the closed pool reported")
	}
}
