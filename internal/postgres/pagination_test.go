// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// rootChild stores a child of the root post type, which the caller has made hierarchical.
func rootChild(
	t *testing.T, store content.Store, parent *content.Content, title string, author uuid.UUID,
) content.Content {
	t.Helper()
	root := contenttest.PostType()
	root.Hierarchical = true
	built, err := content.New(root, parent, title, author)
	if err != nil {
		t.Fatal(err)
	}
	held, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatal(err)
	}
	return held
}

func TestLegacyPaginationItemCanBeEditedWithoutMoving(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	root := contenttest.PostType()
	root.Hierarchical = true
	if _, err := postgres.NewTypeStore(pool).Update(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	parent := rootChild(t, store, nil, "Page", author)
	child := rootChild(t, store, &parent, "About", author)
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content SET slug = '2', path = 'page/2' WHERE id = $1`, child.ID); err != nil {
		t.Fatal(err)
	}
	legacy, err := store.ByID(t.Context(), child.ID)
	if err != nil {
		t.Fatal(err)
	}
	edited := contenttest.EditTitle(legacy, "Updated title")
	updated, err := store.Update(t.Context(), edited, legacy.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("Update() error = %v, want the legacy item editable at its existing address", err)
	}
	if updated.Path != "page/2" || updated.Title != "Updated title" {
		t.Errorf("Update() = %+v, want the edited title at page/2", updated)
	}
	published := contenttest.PublishItem(t, store, updated)
	if published.Path != "page/2" || published.Status != content.StatusPublished {
		t.Errorf("published = %+v, want the legacy item published without moving", published)
	}
}

func TestLegacyPaginationParentCanBeEditedInPlace(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	root := contenttest.PostType()
	root.Hierarchical = true
	if _, err := postgres.NewTypeStore(pool).Update(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	parent := rootChild(t, store, nil, "Page", author)
	child := rootChild(t, store, &parent, "About", author)
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content SET slug = '2', path = 'page/2' WHERE id = $1`, child.ID); err != nil {
		t.Fatal(err)
	}
	edited := contenttest.EditTitle(parent, "Updated title")
	edited.UpdatedAt = time.Now().UTC()
	updated, err := store.Update(t.Context(), edited, parent.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("Update() error = %v, want the parent of a legacy item editable in place", err)
	}
	if updated.Path != "page" || updated.Title != "Updated title" {
		t.Errorf("Update() = %+v, want the edited title at page", updated)
	}
	after, err := store.ByID(t.Context(), child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Path != "page/2" {
		t.Errorf("child path = %q, want the legacy child kept at page/2", after.Path)
	}
}

func TestRestoreKeepsASafeAddressWhenTheOriginalIsReserved(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	root := contenttest.PostType()
	root.Hierarchical = true
	if _, err := postgres.NewTypeStore(pool).Update(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	parent := rootChild(t, store, nil, "Chapter", author)
	child := rootChild(t, store, &parent, "2", author)
	trashed, err := store.Trash(t.Context(), child.ID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	version := parent.UpdatedAt
	parent, err = parent.Rename("page")
	if err != nil {
		t.Fatal(err)
	}
	parent.UpdatedAt = time.Now().UTC()
	if _, err := store.Update(t.Context(), parent, version, nil, 0); err != nil {
		t.Fatal(err)
	}
	restored, err := store.Restore(t.Context(), child.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Restore() error = %v, want a draft at a safe address", err)
	}
	if restored.Path != "page/"+trashed.Slug || restored.Slug != trashed.Slug {
		t.Errorf("Restore() = %+v, want the trashed slug kept under page", restored)
	}
	if restored.Status != content.StatusDraft {
		t.Errorf("status = %q, want draft", restored.Status)
	}
	edited := contenttest.EditTitle(restored, "Recovered item")
	updated, err := store.Update(t.Context(), edited, restored.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("Update() after restore error = %v, want the restored item editable", err)
	}
	published := contenttest.PublishItem(t, store, updated)
	if published.Path != "page/"+trashed.Slug {
		t.Errorf("published path = %q, want the safe restored address kept", published.Path)
	}
	if _, err := store.PublishedByPath(t.Context(), "page/2"); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedByPath(page/2) error = %v, want the pagination address free", err)
	}
}

func TestStoreWritesRefuseNewPaginationAddresses(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	root := contenttest.PostType()
	root.Hierarchical = true
	if _, err := postgres.NewTypeStore(pool).Update(t.Context(), root); err != nil {
		t.Fatal(err)
	}
	parent := rootChild(t, store, nil, "Page", author)
	built, err := content.New(root, &parent, "About", author)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), built.Place("page", "2")); !errors.Is(err, content.ErrReservedAddress) {
		t.Fatalf("Create() error = %v, want the reserved address refused by the store", err)
	}
	if _, err := store.ByID(t.Context(), built.ID); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("ByID() after refused create error = %v, want no item stored", err)
	}
	held := rootChild(t, store, &parent, "About", author)
	moved := held.Place("page", "2")
	moved.UpdatedAt = time.Now().UTC()
	_, err = store.Update(t.Context(), moved, held.UpdatedAt, nil, 0)
	if !errors.Is(err, content.ErrReservedAddress) {
		t.Fatalf("Update() error = %v, want the new reserved address refused", err)
	}
	if code, _ := content.CodeOf(err); code != "address_reserved" {
		t.Errorf("code = %q, want address_reserved", code)
	}
	if named, _ := content.DetailsOf(err); named["path"] != "page/2" {
		t.Errorf("details = %v, want the path page/2 named", named)
	}
	after, err := store.ByID(t.Context(), held.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Path != "page/about" || !after.UpdatedAt.Equal(held.UpdatedAt) {
		t.Errorf("stored item = %+v, want the refused edit rolled back", after)
	}
}

func TestPaginationEditReportsAMissingStoredItem(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	missing := mustPost(t, "Missing", author).Place("page", "2")
	if _, err := store.Update(t.Context(), missing, missing.UpdatedAt, nil, 0); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("Update() error = %v, want the missing item reported", err)
	}
}

func TestRootHandOverRefusesPageDerivedFromTheOldRootLabel(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	root := contenttest.PostType()
	root.PluralLabel = "Page"
	root, err := types.Update(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	contenttest.RegisterPageType(t, types)
	held := mustCreate(t, store, "Existing post", author)
	promoted := contenttest.PageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()
	_, err = types.Update(t.Context(), promoted)
	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Fatalf("Update() error = %v, want the derived page route refused", err)
	}
	if code, _ := content.CodeOf(err); code != "derived_route_word_reserved" {
		t.Errorf("code = %q, want derived_route_word_reserved", code)
	}
	if named, _ := content.DetailsOf(err); named["type"] != "post" || named["name"] != "Page" || named["word"] != "page" {
		t.Errorf("details = %v, want the type post named Page and the word page", named)
	}
	for _, before := range []content.Type{root, contenttest.PageType()} {
		after, err := types.ByKey(t.Context(), before.Key)
		if err != nil {
			t.Fatal(err)
		}
		if after.Default != before.Default || after.RouteWord != before.RouteWord {
			t.Errorf("type = %+v, want its previous root flag and route kept", after)
		}
	}
	after, err := store.ByID(t.Context(), held.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Path != "existing-post" || !after.UpdatedAt.Equal(held.UpdatedAt) {
		t.Errorf("item = %+v, want its address and timestamp kept", after)
	}
}

func TestTypeRouteMoveRollsBackWhenContentPathsCannotBeWritten(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	contenttest.RegisterPageType(t, types)
	held := contenttest.MustNest(t, store, nil, "About", author)
	raiseOn(t, pool, "core.content", "UPDATE")
	moved := contenttest.PageType()
	moved.RouteWord, moved.UpdatedAt = "chapters", time.Now().UTC()
	if _, err := types.Update(t.Context(), moved); err == nil || !strings.Contains(err.Error(), "sabotaged") {
		t.Fatalf("Update() error = %v, want the failed content move reported", err)
	}
	after, err := types.ByKey(t.Context(), "page")
	if err != nil {
		t.Fatal(err)
	}
	if after.RouteWord != "pages" {
		t.Errorf("route word = %q, want pages after rollback", after.RouteWord)
	}
	item, err := store.ByID(t.Context(), held.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Path != "pages/about" || !item.UpdatedAt.Equal(held.UpdatedAt) {
		t.Errorf("item = %+v, want the stored path and timestamp kept", item)
	}
}

func TestAncestorMovesCannotCreateRootPaginationAddresses(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"rename", "reparent"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			store, author, pool := newContentStoreWithPool(t)
			types := postgres.NewTypeStore(pool)
			root := contenttest.PostType()
			root.Hierarchical = true
			if _, err := types.Update(t.Context(), root); err != nil {
				t.Fatal(err)
			}
			parent := rootChild(t, store, nil, "Chapter", author)
			if action == "reparent" {
				parent = rootChild(t, store, &parent, "Page", author)
			}
			child := rootChild(t, store, &parent, "2", author)
			moved := parent.Place("", "page")
			moved.ParentID, moved.UpdatedAt = nil, time.Now().UTC()
			_, err := store.Update(t.Context(), moved, parent.UpdatedAt, nil, 0)
			if !errors.Is(err, content.ErrReservedAddress) {
				t.Fatalf("Update() error = %v, want %v", err, content.ErrReservedAddress)
			}
			for _, before := range []content.Content{parent, child} {
				after, err := store.ByID(t.Context(), before.ID)
				if err != nil {
					t.Fatal(err)
				}
				if after.Path != before.Path || !after.UpdatedAt.Equal(before.UpdatedAt) {
					t.Errorf("stored item = %+v, want the refused move rolled back", after)
				}
			}
		})
	}
}

func TestTypePromotionCannotCreateRootPaginationAddresses(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	contenttest.RegisterPageType(t, types)
	rootBefore, err := types.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatal(err)
	}
	parent := contenttest.MustNest(t, store, nil, "Page", author)
	child := contenttest.MustNest(t, store, &parent, "2", author)
	promoted := contenttest.PageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()
	_, err = content.NewRegistry(types).Update(t.Context(), promoted)
	if !errors.Is(err, content.ErrReservedAddress) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrReservedAddress)
	}
	if code, _ := content.CodeOf(err); code != "address_reserved" {
		t.Errorf("code = %q, want address_reserved", code)
	}
	if held, _ := content.DetailsOf(err); held["path"] != "page/2" {
		t.Errorf("details = %v, want the path page/2 named", held)
	}
	held, err := types.ByKey(t.Context(), "page")
	if err != nil {
		t.Fatal(err)
	}
	if held.Default || held.RouteWord != "pages" {
		t.Errorf("type = %+v, want the promotion rolled back", held)
	}
	rootAfter, err := types.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatal(err)
	}
	if !rootAfter.Default || rootAfter.RouteWord != "" || !rootAfter.UpdatedAt.Equal(rootBefore.UpdatedAt) {
		t.Errorf("root type = %+v, want its route, default flag and timestamp kept", rootAfter)
	}
	for _, before := range []content.Content{parent, child} {
		after, err := store.ByID(t.Context(), before.ID)
		if err != nil {
			t.Fatal(err)
		}
		if after.Path != before.Path || !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Errorf("item = %+v, want its address and timestamp kept after rollback", after)
		}
	}
}

func TestStaleLegacyPageTypeCannotReclaimItsReservedRoute(t *testing.T) {
	t.Parallel()

	pool := migratedPool(t)
	store := postgres.NewTypeStore(pool)
	legacy, err := content.NewType("chapter", "Chapter", "Chapters", "page")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), legacy); err != nil {
		t.Fatal(err)
	}
	stale := content.NewRegistry(store)
	if _, err := stale.ByKey(t.Context(), "chapter"); err != nil {
		t.Fatal(err)
	}
	moved := legacy
	moved.RouteWord = "chapters"
	if _, err := content.NewRegistry(store).Update(t.Context(), moved); err != nil {
		t.Fatal(err)
	}
	legacy.Description = "An edit prepared before the move"
	_, err = stale.Update(t.Context(), legacy)
	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Errorf("stale Update() error = %v, want %v", err, content.ErrRouteWordReserved)
	}
	if _, err := store.Update(t.Context(), legacy); !errors.Is(err, content.ErrRouteWordReserved) {
		t.Errorf("store Update() error = %v, want the locked row to refuse the stale route", err)
	}
	stored, err := store.ByKey(t.Context(), "chapter")
	if err != nil {
		t.Fatal(err)
	}
	if stored.RouteWord != "chapters" {
		t.Errorf("route word = %q, want the completed move kept", stored.RouteWord)
	}
}
