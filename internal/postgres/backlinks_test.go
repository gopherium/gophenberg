// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// declaredRelation stores a relation field on the post type pointing at categories, and returns it.
func declaredRelation(t *testing.T, types *postgres.TypeStore, key string) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: "post", Key: key, Label: key,
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	})
	if err != nil {
		t.Fatalf("NewField(%q) error = %v, want nil", key, err)
	}
	stored, err := types.CreateField(t.Context(), built)
	if err != nil {
		t.Fatalf("declaring %q: %v, want nil", key, err)
	}
	return stored
}

// pointing holds what a backlinks read is proven against.
type pointing struct {
	store      *postgres.ContentStore
	author     uuid.UUID
	categories content.Field
	tags       content.Field
	pool       *pgxpool.Pool
}

// pointingStore returns a store, an author, and the two relation fields the post type declares.
func pointingStore(t *testing.T) pointing {
	t.Helper()
	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), categoryType()); err != nil {
		t.Fatalf("registering the category type: %v, want nil", err)
	}
	return pointing{
		store: store, author: author, pool: pool,
		categories: declaredRelation(t, types, "categories"),
		tags:       declaredRelation(t, types, "tags"),
	}
}

func TestContentStoreReportsAPointerCountItCannotRead(t *testing.T) {
	t.Parallel()

	at := pointingStore(t)
	news := publishItem(t, at.store, storedCategory(t, at.store, "News", at.author))
	sabotage(t, at.pool, "ALTER TABLE core.content_relations DROP COLUMN visible CASCADE")

	_, _, err := at.store.PointingAt(t.Context(), news.ID, at.categories.ID, 1, 20)

	if err == nil {
		t.Error("PointingAt() error = nil, want the count it cannot run reported")
	}
}

func TestContentStoreReportsAPointerListingItCannotRead(t *testing.T) {
	t.Parallel()

	at := pointingStore(t)
	news := publishItem(t, at.store, storedCategory(t, at.store, "News", at.author))
	sabotage(t, at.pool, "ALTER TABLE core.content DROP COLUMN title CASCADE")

	_, _, err := at.store.PointingAt(t.Context(), news.ID, at.categories.ID, 1, 20)

	if err == nil {
		t.Error("PointingAt() error = nil, want the listing it cannot run reported")
	}
}
