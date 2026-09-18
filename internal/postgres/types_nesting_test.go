// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// nestingStores returns a content store and a type store over one database knowing the page type.
func nestingStores(t *testing.T) (*postgres.ContentStore, *postgres.TypeStore, uuid.UUID) {
	t.Helper()
	items, types, author, _ := nestingStoresWithPool(t)
	return items, types, author
}

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

// flatPage returns the page type told to stop nesting.
func flatPage() content.Type {
	flat := pageType()
	flat.Hierarchical, flat.UpdatedAt = false, time.Now().UTC()
	return flat
}

// nestsStill reports whether the stored page type nests.
func nestsStill(t *testing.T, types *postgres.TypeStore) bool {
	t.Helper()
	stored, err := types.ByKey(t.Context(), "page")
	if err != nil {
		t.Fatalf("ByKey(page) error = %v, want nil", err)
	}
	return stored.Hierarchical
}

func TestUpdateKeepsATypeNestingWhileItsItemsNest(t *testing.T) {
	t.Parallel()

	items, types, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	mustNest(t, items, &about, "Team", author)
	mustNest(t, items, &about, "Offices", author)

	_, err := types.Update(t.Context(), flatPage())

	if !errors.Is(err, content.ErrNestingInUse) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrNestingInUse)
	}
	var refused *content.Error
	if !errors.As(err, &refused) || refused.Held["type"] != "page" || refused.Held["items"] != 2 {
		t.Errorf("details = %v, want the page type and its two nested items named", err)
	}
	if !nestsStill(t, types) {
		t.Errorf("the page type stopped nesting, want the refused edit to have written nothing")
	}
}

func TestUpdateKeepsATypeNestingWhileANestedItemSitsInTheTrash(t *testing.T) {
	t.Parallel()

	items, types, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	team := mustNest(t, items, &about, "Team", author)
	if _, err := items.Trash(t.Context(), team.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash(team) error = %v, want nil", err)
	}

	_, err := types.Update(t.Context(), flatPage())

	if !errors.Is(err, content.ErrNestingInUse) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrNestingInUse)
	}
}

func TestUpdateStopsATypeNestingWhileNoneOfItsItemsNests(t *testing.T) {
	t.Parallel()

	items, types, author := nestingStores(t)
	mustNest(t, items, nil, "About", author)

	updated, err := types.Update(t.Context(), flatPage())

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Hierarchical || nestsStill(t, types) {
		t.Errorf("the page type still nests, want it flat once nothing sits inside another item")
	}
}

func TestUpdateCarriesOtherEditsOntoATypeWhoseItemsNest(t *testing.T) {
	t.Parallel()

	items, types, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	mustNest(t, items, &about, "Team", author)
	relabeled := pageType()
	relabeled.PluralLabel, relabeled.UpdatedAt = "Sections", time.Now().UTC()

	updated, err := types.Update(t.Context(), relabeled)

	if err != nil {
		t.Fatalf("Update() error = %v, want an edit keeping the nesting stored", err)
	}
	if updated.PluralLabel != "Sections" || !updated.Hierarchical {
		t.Errorf("Update() = %+v, want the new label on a type still nesting", updated)
	}
}

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

func TestNestedCountsTheItemsOfTheTypeSittingInsideAnother(t *testing.T) {
	t.Parallel()

	items, types, author := nestingStores(t)
	about := mustNest(t, items, nil, "About", author)
	team := mustNest(t, items, &about, "Team", author)
	mustNest(t, items, &team, "Founders", author)
	mustCreate(t, items, "A post", author)

	pages, err := types.Nested(t.Context(), "page")
	if err != nil {
		t.Fatalf("Nested(page) error = %v, want nil", err)
	}
	posts, err := types.Nested(t.Context(), content.TypePost)
	if err != nil {
		t.Fatalf("Nested(post) error = %v, want nil", err)
	}

	if pages != 2 || posts != 0 {
		t.Errorf("Nested() = %d pages and %d posts, want 2 and 0", pages, posts)
	}
}
