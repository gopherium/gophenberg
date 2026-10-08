// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// fieldOn is the shared fixture under the name these tests use.
var fieldOn = contenttest.FieldOn

// storedFields reads the raw fields column of the content row carrying the slug.
func storedFields(t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	var held string
	if err := pool.QueryRow(
		t.Context(), `SELECT fields::text FROM core.content WHERE slug = $1`, slug,
	).Scan(&held); err != nil {
		t.Fatalf("reading the fields of %q: %v, want nil", slug, err)
	}
	return held
}

func TestTypeStoreDeclaresAField(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)

	created, err := types.CreateField(t.Context(), fieldOn(t, "post", "color", content.FieldKindText, ""))

	if err != nil {
		t.Fatalf("CreateField() error = %v, want nil", err)
	}
	if created.ID == 0 {
		t.Errorf("CreateField() id = 0, want a stored identity")
	}
	held, err := types.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 1 || held.Fields[0].Key != "color" {
		t.Fatalf("ByKey() fields = %+v, want the declared field attached", held.Fields)
	}
	listed, err := types.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	for _, stored := range listed {
		if stored.Key == "post" && len(stored.Fields) == 1 {
			return
		}
	}
	t.Errorf("List() = %+v, want the post type carrying its field", listed)
}

func TestTypeStoreKeepsARelationTargetWhole(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v, want nil", err)
	}

	created, err := types.CreateField(t.Context(), fieldOn(t, "post", "pages", content.FieldKindRelation, "page"))

	if err != nil {
		t.Fatalf("CreateField() error = %v, want nil", err)
	}
	if created.RelatesTo != "page" || created.Kind != content.FieldKindRelation {
		t.Errorf("CreateField() = %+v, want the relation shape stored", created)
	}
}

func TestTypeStoreRefusesARelationTargetingAnUnknownType(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)

	_, err := types.CreateField(t.Context(), fieldOn(t, "post", "pages", content.FieldKindRelation, "ghost"))

	if !errors.Is(err, content.ErrTargetUnknown) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrTargetUnknown)
	}
}

func TestTypeStoreRefusesATakenFieldKey(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.CreateField(t.Context(), fieldOn(t, "post", "color", content.FieldKindText, "")); err != nil {
		t.Fatalf("declaring the first field: %v, want nil", err)
	}

	_, err := types.CreateField(t.Context(), fieldOn(t, "post", "color", content.FieldKindNumber, ""))

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

func TestTypeStoreRefusesAFieldOnAnUnknownType(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)

	_, err := types.CreateField(t.Context(), fieldOn(t, "ghost", "color", content.FieldKindText, ""))

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}
