// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// typeNestingCases are the cases over a type that nests and the items it holds inside one another.
var typeNestingCases = []Case{
	{"UpdateKeepsATypeNestingWhileItsItemsNest", updateKeepsATypeNestingWhileItsItemsNest},
	{"UpdateKeepsATypeNestingWhileANestedItemSitsInTheTrash", updateKeepsATypeNestingWhileANestedItemSitsInTheTrash},
	{"UpdateStopsATypeNestingWhileNoneOfItsItemsNests", updateStopsATypeNestingWhileNoneOfItsItemsNests},
	{"UpdateCarriesOtherEditsOntoATypeWhoseItemsNest", updateCarriesOtherEditsOntoATypeWhoseItemsNest},
	{"NestedCountsTheItemsOfTheTypeSittingInsideAnother", nestedCountsTheItemsOfTheTypeSittingInsideAnother},
}

// nestsStill reports whether the stored page type nests.
func nestsStill(t *testing.T, types content.TypeStore) bool {
	t.Helper()
	stored, err := types.ByKey(t.Context(), "page")
	if err != nil {
		t.Fatalf("ByKey(page) error = %v, want nil", err)
	}
	return stored.Hierarchical
}

// updateKeepsATypeNestingWhileItsItemsNest refuses to stop nesting while items sit inside others, naming them.
func updateKeepsATypeNestingWhileItsItemsNest(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	MustNest(t, s.Content, &about, "Team", author)
	MustNest(t, s.Content, &about, "Offices", author)

	_, err := s.Types.Update(t.Context(), FlatPage())

	if !errors.Is(err, content.ErrNestingInUse) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrNestingInUse)
	}
	var refused *content.Error
	if !errors.As(err, &refused) || refused.Held["type"] != "page" || refused.Held["items"] != 2 {
		t.Errorf("details = %v, want the page type and its two nested items named", err)
	}
	if !nestsStill(t, s.Types) {
		t.Errorf("the page type stopped nesting, want the refused edit to have written nothing")
	}
}

// updateKeepsATypeNestingWhileANestedItemSitsInTheTrash counts a trashed item inside another as still nesting.
func updateKeepsATypeNestingWhileANestedItemSitsInTheTrash(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	if _, err := s.Content.Trash(t.Context(), team.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash(team) error = %v, want nil", err)
	}

	_, err := s.Types.Update(t.Context(), FlatPage())

	if !errors.Is(err, content.ErrNestingInUse) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrNestingInUse)
	}
}

// updateStopsATypeNestingWhileNoneOfItsItemsNests stores a flat type once no item sits inside another.
func updateStopsATypeNestingWhileNoneOfItsItemsNests(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	MustNest(t, s.Content, nil, "About", author)

	updated, err := s.Types.Update(t.Context(), FlatPage())

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Hierarchical || nestsStill(t, s.Types) {
		t.Errorf("the page type still nests, want it flat once nothing sits inside another item")
	}
}

// updateCarriesOtherEditsOntoATypeWhoseItemsNest stores an edit that keeps the nesting while items nest.
func updateCarriesOtherEditsOntoATypeWhoseItemsNest(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	MustNest(t, s.Content, &about, "Team", author)
	relabeled := PageType()
	relabeled.PluralLabel, relabeled.UpdatedAt = "Sections", time.Now().UTC()

	updated, err := s.Types.Update(t.Context(), relabeled)

	if err != nil {
		t.Fatalf("Update() error = %v, want an edit keeping the nesting stored", err)
	}
	if updated.PluralLabel != "Sections" || !updated.Hierarchical {
		t.Errorf("Update() = %+v, want the new label on a type still nesting", updated)
	}
}

// nestedCountsTheItemsOfTheTypeSittingInsideAnother counts only the type's items that sit inside another.
func nestedCountsTheItemsOfTheTypeSittingInsideAnother(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	MustNest(t, s.Content, &team, "Founders", author)
	MustCreate(t, s.Content, "A post", author)

	pages, err := s.Types.Nested(t.Context(), "page")
	if err != nil {
		t.Fatalf("Nested(page) error = %v, want nil", err)
	}
	posts, err := s.Types.Nested(t.Context(), content.TypePost)
	if err != nil {
		t.Fatalf("Nested(post) error = %v, want nil", err)
	}

	if pages != 2 || posts != 0 {
		t.Errorf("Nested() = %d pages and %d posts, want 2 and 0", pages, posts)
	}
}
