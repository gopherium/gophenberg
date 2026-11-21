// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// rootPaginationCases are the content cases that keep the root listing's numbered pages off every item.
var rootPaginationCases = []Case{
	{"CreateRefusesARootPaginationAddress", createRefusesARootPaginationAddress},
	{"UpdateRefusesAMoveOntoARootPaginationAddress", updateRefusesAMoveOntoARootPaginationAddress},
	{"UpdateRefusesCarryingAChildOntoRootPagination", updateRefusesCarryingAChildOntoRootPagination},
	{"RestoreKeepsTheTrashedSlugOffRootPagination", restoreKeepsTheTrashedSlugOffRootPagination},
	{"UpdateRenamesARootItemSharingARouteWord", updateRenamesARootItemSharingARouteWord},
}

// typeRootPaginationCases are the type cases that keep the page word and the root listing's numbered pages free.
var typeRootPaginationCases = []Case{
	{"UpdateRefusesThePageRouteWord", updateRefusesThePageRouteWord},
	{"UpdateKeepsALegacyPageRouteWordUntilItMoves", updateKeepsALegacyPageRouteWordUntilItMoves},
	{"UpdateRefusesAHandOverWhoseOldRootLabelGivesPage", updateRefusesAHandOverWhoseOldRootLabelGivesPage},
	{"UpdateRefusesAPromotionCarryingAnItemOntoRootPagination", updateRefusesAPromotionCarryingAnItemOntoRootPagination},
}

// nestingPosts stores the built-in post type told to nest and returns it.
func nestingPosts(t *testing.T, types content.TypeStore) content.Type {
	t.Helper()
	root := PostType()
	root.Hierarchical, root.UpdatedAt = true, time.Now().UTC()
	stored, err := types.Update(t.Context(), root)
	if err != nil {
		t.Fatalf("letting the default type nest: %v", err)
	}
	return stored
}

// derivedWordReserved fails the case unless the error carries derived_route_word_reserved, the type and the word.
func derivedWordReserved(t *testing.T, err error, key, name, word string) {
	t.Helper()
	if code, _ := content.CodeOf(err); code != "derived_route_word_reserved" {
		t.Errorf("code = %q, want derived_route_word_reserved", code)
	}
	if held, _ := content.DetailsOf(err); held["type"] != key || held["name"] != name || held["word"] != word {
		t.Errorf("details = %v, want the type %s named %q and the word %q", held, key, name, word)
	}
}

// carriedOnto fails the case unless the error carries address_reserved and names the path.
func carriedOnto(t *testing.T, err error, path string) {
	t.Helper()
	if code, _ := content.CodeOf(err); code != "address_reserved" {
		t.Errorf("code = %q, want address_reserved", code)
	}
	if held, _ := content.DetailsOf(err); held["path"] != path {
		t.Errorf("details = %v, want the path %s named", held, path)
	}
}

// unmoved fails the case unless the store still holds each item at the address and stamp it had.
func unmoved(t *testing.T, store content.Store, items ...content.Content) {
	t.Helper()
	for _, before := range items {
		after, err := store.ByID(t.Context(), before.ID)
		if err != nil {
			t.Fatalf("ByID(%q) error = %v, want nil", before.Title, err)
		}
		if after.Path != before.Path || !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Errorf("%q = %s at %v, want %s at %v kept",
				before.Title, after.Path, after.UpdatedAt, before.Path, before.UpdatedAt)
		}
	}
}

// createRefusesARootPaginationAddress refuses to store an item at page and a number, with or without a parent.
func createRefusesARootPaginationAddress(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	root := nestingPosts(t, s.Types)
	parent := StoreItem(t, s.Content, root, nil, "Page", author)
	child, err := content.New(root, &parent, "About", author)
	if err != nil {
		t.Fatalf("New(About) error = %v, want nil", err)
	}
	flat := MustPost(t, "2", author).Place(content.PageWord, "2")
	flat.Type = legacyPageType(t, s.Types).Key

	for name, built := range map[string]content.Content{
		"flat":   flat,
		"nested": child.Place(content.PageWord, "2"),
	} {
		_, err := s.Content.Create(t.Context(), built)

		if !errors.Is(err, content.ErrReservedAddress) {
			t.Errorf("%s Create() error = %v, want %v", name, err, content.ErrReservedAddress)
		}
		carriedOnto(t, err, "page/2")
		if StillStored(t, s.Content, built) {
			t.Errorf("%s item stored, want the refused create to store nothing", name)
		}
	}
}

// updateRefusesAMoveOntoARootPaginationAddress refuses to move an item to page and a number.
func updateRefusesAMoveOntoARootPaginationAddress(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	root := nestingPosts(t, s.Types)
	parent := StoreItem(t, s.Content, root, nil, "Page", author)
	held := StoreItem(t, s.Content, root, &parent, "About", author)
	moved := held.Place(content.PageWord, "3")
	moved.UpdatedAt = time.Now().UTC()

	_, err := s.Content.Update(t.Context(), moved, held.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrReservedAddress) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrReservedAddress)
	}
	carriedOnto(t, err, "page/3")
	unmoved(t, s.Content, held)
}

// updateRefusesCarryingAChildOntoRootPagination refuses a rename or a move that carries a child to page/2.
func updateRefusesCarryingAChildOntoRootPagination(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	root := nestingPosts(t, s.Types)
	chapter := StoreItem(t, s.Content, root, nil, "Chapter", author)
	renamed := StoreItem(t, s.Content, root, &chapter, "2", author)
	inner := StoreItem(t, s.Content, root, &chapter, "Page", author)
	moved := StoreItem(t, s.Content, root, &inner, "2", author)

	for name, ancestor := range map[string]content.Content{"rename": chapter, "reparent": inner} {
		lifted := ancestor.Place("", content.PageWord)
		lifted.ParentID, lifted.UpdatedAt = nil, time.Now().UTC()

		_, err := s.Content.Update(t.Context(), lifted, ancestor.UpdatedAt, nil, 0)

		if !errors.Is(err, content.ErrReservedAddress) {
			t.Fatalf("%s Update() error = %v, want %v", name, err, content.ErrReservedAddress)
		}
		carriedOnto(t, err, "page/2")
	}
	unmoved(t, s.Content, chapter, renamed, inner, moved)
}

// restoreKeepsTheTrashedSlugOffRootPagination restores to draft under the trashed slug when the original is page/2.
func restoreKeepsTheTrashedSlugOffRootPagination(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	root := nestingPosts(t, s.Types)
	chapter := StoreItem(t, s.Content, root, nil, "Chapter", author)
	child := StoreItem(t, s.Content, root, &chapter, "2", author)
	trashed, err := s.Content.Trash(t.Context(), child.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	renamed, err := chapter.Rename(content.PageWord)
	if err != nil {
		t.Fatalf("Rename() error = %v, want nil", err)
	}
	renamed.UpdatedAt = time.Now().UTC()
	if _, err := s.Content.Update(t.Context(), renamed, chapter.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("renaming the parent to page: %v", err)
	}

	restored, err := s.Content.Restore(t.Context(), child.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Restore() error = %v, want a draft at a free address", err)
	}
	if restored.Status != content.StatusDraft || restored.Slug != trashed.Slug || restored.Path != "page/"+trashed.Slug {
		t.Errorf("Restore() = %+v, want a draft keeping the slug %s under page", restored, trashed.Slug)
	}
	published := PublishItem(t, s.Content, restored)
	if published.Path != restored.Path {
		t.Errorf("published path = %q, want the restored address %q kept", published.Path, restored.Path)
	}
	if _, err := s.Content.PublishedByPath(t.Context(), "page/2"); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedByPath(page/2) error = %v, want %v", err, content.ErrNotFound)
	}
}

// updateRenamesARootItemSharingARouteWord renames a root item slugged like a route word, leaving that type's items.
func updateRenamesARootItemSharingARouteWord(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	page := MustNest(t, s.Content, nil, "2", author)
	post := StoreItem(t, s.Content, PostType(), nil, "Pages", author)
	renamed, err := post.Rename(content.PageWord)
	if err != nil {
		t.Fatalf("Rename() error = %v, want nil", err)
	}
	renamed.UpdatedAt = time.Now().UTC()

	updated, err := s.Content.Update(t.Context(), renamed, post.UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want the root item renamed", err)
	}
	if updated.Path != content.PageWord {
		t.Errorf("post path = %q, want page", updated.Path)
	}
	unmoved(t, s.Content, page)
}

// updateRefusesThePageRouteWord refuses to move a type under page, the paging word.
func updateRefusesThePageRouteWord(t *testing.T, s Stores) {
	car, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	moved := car
	moved.RouteWord, moved.UpdatedAt = content.PageWord, time.Now().UTC()

	_, err = s.Types.Update(t.Context(), moved)

	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrRouteWordReserved)
	}
	if held, err := s.Types.ByKey(t.Context(), car.Key); err != nil || held.RouteWord != car.RouteWord {
		t.Errorf("car = %+v, %v, want it still under %s", held, err, car.RouteWord)
	}
}

// legacyPageType stores the chapter type under page, a word stored before the paging word was reserved, and returns it.
func legacyPageType(t *testing.T, types content.TypeStore) content.Type {
	t.Helper()
	legacy, err := content.NewType("chapter", "Chapter", "Chapters", content.PageWord)
	if err != nil {
		t.Fatalf("NewType(chapter) error = %v, want nil", err)
	}
	if _, err := types.Create(t.Context(), legacy); err != nil {
		t.Fatalf("Create(chapter) error = %v, want nil", err)
	}
	return legacy
}

// updateKeepsALegacyPageRouteWordUntilItMoves edits a type under page in place and refuses its return after it moved.
func updateKeepsALegacyPageRouteWordUntilItMoves(t *testing.T, s Stores) {
	legacy := legacyPageType(t, s.Types)
	described := legacy
	described.Description, described.UpdatedAt = "Chapters of the site.", time.Now().UTC()
	if held, err := s.Types.Update(t.Context(), described); err != nil || held.RouteWord != content.PageWord {
		t.Fatalf("Update() = %+v, %v, want the legacy type edited under page", held, err)
	}
	moved := described
	moved.RouteWord, moved.UpdatedAt = "chapters", time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), moved); err != nil {
		t.Fatalf("moving the legacy type to chapters: %v", err)
	}
	back := moved
	back.RouteWord, back.UpdatedAt = content.PageWord, time.Now().UTC()

	_, err := s.Types.Update(t.Context(), back)

	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Fatalf("Update() back to page error = %v, want %v", err, content.ErrRouteWordReserved)
	}
	if held, err := s.Types.ByKey(t.Context(), "chapter"); err != nil || held.RouteWord != "chapters" {
		t.Errorf("chapter = %+v, %v, want it still under chapters", held, err)
	}
}

// updateRefusesAHandOverWhoseOldRootLabelGivesPage refuses a hand over that would move the default under page.
func updateRefusesAHandOverWhoseOldRootLabelGivesPage(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	relabelPost(t, s.Types, "Page", "Page")
	post := StoreItem(t, s.Content, PostType(), nil, "Hello World", author)

	_, err := rootPage(t, s.Types)

	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrRouteWordReserved)
	}
	derivedWordReserved(t, err, content.TypePost, "Page", "page")
	if held, err := s.Types.ByKey(t.Context(), content.TypePost); err != nil || !held.Default || held.RouteWord != "" {
		t.Errorf("post = %+v, %v, want it still holding the root", held, err)
	}
	if held, err := s.Types.ByKey(t.Context(), "page"); err != nil || held.Default || held.RouteWord != "pages" {
		t.Errorf("page = %+v, %v, want it still under pages", held, err)
	}
	unmoved(t, s.Content, post)
}

// updateRefusesAPromotionCarryingAnItemOntoRootPagination refuses a hand over that carries a nested page to page/2.
func updateRefusesAPromotionCarryingAnItemOntoRootPagination(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	post := StoreItem(t, s.Content, PostType(), nil, "Hello World", author)
	parent := MustNest(t, s.Content, nil, "Page", author)
	child := MustNest(t, s.Content, &parent, "2", author)

	_, err := rootPage(t, s.Types)

	if !errors.Is(err, content.ErrReservedAddress) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrReservedAddress)
	}
	carriedOnto(t, err, "page/2")
	if held, err := s.Types.ByKey(t.Context(), content.TypePost); err != nil || !held.Default || held.RouteWord != "" {
		t.Errorf("post = %+v, %v, want it still holding the root", held, err)
	}
	if held, err := s.Types.ByKey(t.Context(), "page"); err != nil || held.Default || held.RouteWord != "pages" {
		t.Errorf("page = %+v, %v, want it still under pages", held, err)
	}
	unmoved(t, s.Content, post, parent, child)
}
