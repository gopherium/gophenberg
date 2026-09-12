// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"

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

// pointedThrough stores the post pointing at the targets through the field key and returns it.
func pointedThrough(
	t *testing.T, store *postgres.ContentStore, post content.Content, key string, targets ...uuid.UUID,
) content.Content {
	t.Helper()
	version := post.UpdatedAt
	post.Fields = content.Values{key: namedTargets(targets)}
	post.UpdatedAt = time.Now().UTC()
	updated, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("pointing the post through %q: %v, want nil", key, err)
	}
	return updated
}

func TestContentStoreListsWhatPointsThroughOneField(t *testing.T) {
	t.Parallel()

	at := pointingStore(t)
	news := publishItem(t, at.store, storedCategory(t, at.store, "News", at.author))
	filed := publishItem(t, at.store, pointedThrough(
		t, at.store, mustCreate(t, at.store, "Filed", at.author), "categories", news.ID))
	publishItem(t, at.store, pointedThrough(
		t, at.store, mustCreate(t, at.store, "Tagged", at.author), "tags", news.ID))

	held, total, err := at.store.PointingAt(t.Context(), news.ID, at.categories.ID, 1, 20)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 1 || len(held) != 1 {
		t.Fatalf("PointingAt() = %d of %d, want the one filed through this field", len(held), total)
	}
	if held[0].ID != filed.ID || held[0].Title != "Filed" || held[0].Type != content.TypePost {
		t.Errorf("PointingAt() = %+v, want the filed post named and typed", held[0])
	}
	if _, tagged, err := at.store.PointingAt(t.Context(), news.ID, at.tags.ID, 1, 20); err != nil ||
		tagged != 1 {
		t.Errorf("PointingAt(tags) = %d, %v, want the tagged post counted apart", tagged, err)
	}
}

func TestContentStoreListsWhatPointsThroughARelationInsideARow(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	inside := rowsPointing(t, pool)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	post := mustCreate(t, store, "Filed", author)
	post.Fields = rowsFiledUnder(news.ID)
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()
	filed, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("filing the post through the row: %v, want nil", err)
	}
	filed = publishItem(t, store, filed)

	held, total, err := store.PointingAt(t.Context(), news.ID, inside.ID, 1, 20)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 1 || len(held) != 1 {
		t.Fatalf("PointingAt() = %d of %d, want the post pointing from inside its row", len(held), total)
	}
	if held[0].ID != filed.ID || held[0].Title != "Filed" {
		t.Errorf("PointingAt() = %+v, want the filed post named", held[0])
	}
}

func TestContentStoreHidesAnUnpublishedPointer(t *testing.T) {
	t.Parallel()

	at := pointingStore(t)
	news := publishItem(t, at.store, storedCategory(t, at.store, "News", at.author))
	pointedThrough(t, at.store, mustCreate(t, at.store, "A draft", at.author), "categories", news.ID)

	held, total, err := at.store.PointingAt(t.Context(), news.ID, at.categories.ID, 1, 20)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 0 || len(held) != 0 {
		t.Errorf("PointingAt() = %+v of %d, want a draft pointer kept back", held, total)
	}
}

func TestContentStorePagesWhatPointsAtAnItem(t *testing.T) {
	t.Parallel()

	at := pointingStore(t)
	news := publishItem(t, at.store, storedCategory(t, at.store, "News", at.author))
	publishItem(t, at.store, pointedThrough(
		t, at.store, mustCreate(t, at.store, "First", at.author), "categories", news.ID))
	second := publishItem(t, at.store, pointedThrough(
		t, at.store, mustCreate(t, at.store, "Second", at.author), "categories", news.ID))

	held, total, err := at.store.PointingAt(t.Context(), news.ID, at.categories.ID, 2, 1)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 2 || len(held) != 1 {
		t.Fatalf("PointingAt() = %d of %d, want the second page holding one", len(held), total)
	}
	if held[0].ID == second.ID {
		t.Errorf("PointingAt() page two = %q, want the older pointer after the newest", held[0].Title)
	}
}

func TestContentStoreCountsNoPointersForAFieldNobodyUses(t *testing.T) {
	t.Parallel()

	at := pointingStore(t)
	news := publishItem(t, at.store, storedCategory(t, at.store, "News", at.author))

	held, total, err := at.store.PointingAt(t.Context(), news.ID, at.tags.ID, 1, 20)

	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	if total != 0 || len(held) != 0 {
		t.Errorf("PointingAt() = %+v of %d, want nothing pointing through it", held, total)
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
