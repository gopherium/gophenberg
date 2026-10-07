// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// pathCases are the cases over nested addresses, the slugs inside them and the pages below an item.
var pathCases = []Case{
	{"AddressesNestedContent", addressesNestedContent},
	{"LetsSiblingsOfDifferentParentsShareASlug", letsSiblingsOfDifferentParentsShareASlug},
	{"SuffixesASlugTakenBesideASibling", suffixesASlugTakenBesideASibling},
	{"CarriesDescendantsWhenAParentIsRenamed", carriesDescendantsWhenAParentIsRenamed},
	{"CarriesDescendantsWhenAParentMoves", carriesDescendantsWhenAParentMoves},
	{"SuffixesAnAddressARenameCollidesWith", suffixesAnAddressARenameCollidesWith},
	{"KeepsAParentThatHoldsChildren", keepsAParentThatHoldsChildren},
	{"FreesAnAddressWhenContentIsTrashed", freesAnAddressWhenContentIsTrashed},
	{"RestoresAnAddressWithItsSlug", restoresAnAddressWithItsSlug},
	{"ServesPublishedContentByAddress", servesPublishedContentByAddress},
	{"CountsChildren", countsChildren},
	{"DepthMeasuresHowFarContentNestsBelow", depthMeasuresHowFarContentNestsBelow},
	{"TrashRefusesContentAlreadyOnItsWayOut", trashRefusesContentAlreadyOnItsWayOut},
	{"RestoreReturnsAnItemToTheAddressItLeft", restoreReturnsAnItemToTheAddressItLeft},
}

// addressesNestedContent files a page under the route word and its child under the ancestor chain.
func addressesNestedContent(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)

	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)

	if about.Path != "pages/about" {
		t.Errorf("about path = %q, want it under the route word", about.Path)
	}
	if team.Path != "pages/about/team" {
		t.Errorf("team path = %q, want the ancestor chain", team.Path)
	}
	if team.ParentID == nil || *team.ParentID != about.ID {
		t.Errorf("team parent = %v, want %v", team.ParentID, about.ID)
	}
}

// letsSiblingsOfDifferentParentsShareASlug keeps one slug for pages under different parents at distinct addresses.
func letsSiblingsOfDifferentParentsShareASlug(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	careers := MustNest(t, s.Content, nil, "Careers", author)

	first := MustNest(t, s.Content, &about, "Team", author)
	second := MustNest(t, s.Content, &careers, "Team", author)

	if first.Slug != "team" || second.Slug != "team" {
		t.Errorf("slugs = %q and %q, want both to keep the stem", first.Slug, second.Slug)
	}
	if first.Path == second.Path {
		t.Errorf("both teams answer at %q, want distinct addresses", first.Path)
	}
}

// suffixesASlugTakenBesideASibling numbers a slug a sibling holds and carries the suffix into the address.
func suffixesASlugTakenBesideASibling(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	MustNest(t, s.Content, &about, "Team", author)

	second := MustNest(t, s.Content, &about, "Team", author)

	if second.Slug != "team-2" {
		t.Errorf("slug = %q, want the sibling suffix", second.Slug)
	}
	if second.Path != "pages/about/team-2" {
		t.Errorf("path = %q, want the suffix carried into the address", second.Path)
	}
}

// carriesDescendantsWhenAParentIsRenamed moves the whole subtree under the renamed parent's address.
func carriesDescendantsWhenAParentIsRenamed(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	deep := MustNest(t, s.Content, &team, "Maria Perez", author)
	renamed, err := about.Rename("company")
	if err != nil {
		t.Fatalf("Rename error = %v, want nil", err)
	}
	renamed.UpdatedAt = about.UpdatedAt.Add(time.Second)

	moved, err := s.Content.Update(t.Context(), renamed, about.UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if moved.Path != "pages/company" {
		t.Errorf("parent path = %q, want the rename applied", moved.Path)
	}
	if got := AddressOf(t, s.Content, team.ID); got != "pages/company/team" {
		t.Errorf("child path = %q, want it carried along", got)
	}
	if got := AddressOf(t, s.Content, deep.ID); got != "pages/company/team/maria-perez" {
		t.Errorf("grandchild path = %q, want the whole subtree carried", got)
	}
}

// carriesDescendantsWhenAParentMoves moves the whole subtree under the parent's new address.
func carriesDescendantsWhenAParentMoves(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	company := MustNest(t, s.Content, nil, "Company", author)
	moved, err := content.Reparent(PageType(), about, &company, 0)
	if err != nil {
		t.Fatalf("Reparent() error = %v, want nil", err)
	}
	moved.UpdatedAt = about.UpdatedAt.Add(time.Second)

	if _, err := s.Content.Update(t.Context(), moved, about.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if got := AddressOf(t, s.Content, about.ID); got != "pages/company/about" {
		t.Errorf("moved path = %q, want it under its new parent", got)
	}
	if got := AddressOf(t, s.Content, team.ID); got != "pages/company/about/team" {
		t.Errorf("child path = %q, want the subtree carried", got)
	}
}

// suffixesAnAddressARenameCollidesWith numbers a renamed page whose new address another page holds.
func suffixesAnAddressARenameCollidesWith(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	first := MustNest(t, s.Content, nil, "First", author)
	second := MustNest(t, s.Content, nil, "Second", author)
	taking, err := first.Rename(second.Slug)
	if err != nil {
		t.Fatalf("Rename error = %v, want nil", err)
	}
	taking.UpdatedAt = first.UpdatedAt.Add(time.Second)

	_, err = s.Content.Update(t.Context(), taking, first.UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want the suffix to settle the clash", err)
	}
	if got := AddressOf(t, s.Content, first.ID); got != "pages/second-2" {
		t.Errorf("path = %q, want the taken address suffixed", got)
	}
}

// keepsAParentThatHoldsChildren refuses to trash a page that still holds a child.
func keepsAParentThatHoldsChildren(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	MustNest(t, s.Content, &about, "Team", author)

	_, err := s.Content.Trash(t.Context(), about.ID, time.Now().UTC())

	if !errors.Is(err, content.ErrHoldsChildren) {
		t.Errorf("Trash() error = %v, want %v", err, content.ErrHoldsChildren)
	}
}

// freesAnAddressWhenContentIsTrashed moves a trashed page off its address so a new page can take it.
func freesAnAddressWhenContentIsTrashed(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)

	trashed, err := s.Content.Trash(t.Context(), about.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if trashed.Path == "pages/about" {
		t.Errorf("path = %q, want the address freed", trashed.Path)
	}
	replacement := MustNest(t, s.Content, nil, "About", author)
	if replacement.Path != "pages/about" {
		t.Errorf("replacement path = %q, want the freed address reused", replacement.Path)
	}
}

// restoresAnAddressWithItsSlug brings a trashed page back under its original slug and address.
func restoresAnAddressWithItsSlug(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	if _, err := s.Content.Trash(t.Context(), about.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	restored, err := s.Content.Restore(t.Context(), about.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if restored.Slug != "about" || restored.Path != "pages/about" {
		t.Errorf("restored = %q at %q, want the original name and address", restored.Slug, restored.Path)
	}
}

// servesPublishedContentByAddress finds a published page at its address and nothing at an address no page holds.
func servesPublishedContentByAddress(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	published := about
	published.Status = content.StatusPublished
	at := time.Now().UTC()
	published.PublishedAt, published.UpdatedAt = &at, at
	if _, err := s.Content.Update(t.Context(), published, about.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("publishing: %v, want nil", err)
	}

	found, err := s.Content.PublishedByPath(t.Context(), "pages/about")

	if err != nil {
		t.Fatalf("PublishedByPath() error = %v, want nil", err)
	}
	if found.ID != about.ID {
		t.Errorf("PublishedByPath() = %v, want the published page", found.ID)
	}
	if _, err := s.Content.PublishedByPath(t.Context(), "pages/nowhere"); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedByPath(missing) error = %v, want %v", err, content.ErrNotFound)
	}
}

// countsChildren counts the pages nested directly under a parent.
func countsChildren(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	MustNest(t, s.Content, &about, "Team", author)
	MustNest(t, s.Content, &about, "History", author)

	held, err := s.Content.Children(t.Context(), about.ID)

	if err != nil {
		t.Fatalf("Children() error = %v, want nil", err)
	}
	if held != 2 {
		t.Errorf("Children() = %d, want 2", held)
	}
}

// depthMeasuresHowFarContentNestsBelow counts the levels below a page and answers zero for a leaf.
func depthMeasuresHowFarContentNestsBelow(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	MustNest(t, s.Content, &team, "Crew", author)
	alone := MustNest(t, s.Content, nil, "Careers", author)

	deep, err := s.Content.Depth(t.Context(), about.ID)
	if err != nil {
		t.Fatalf("Depth error = %v, want nil", err)
	}
	flat, err := s.Content.Depth(t.Context(), alone.ID)
	if err != nil {
		t.Fatalf("Depth error = %v, want nil", err)
	}

	if deep != 2 {
		t.Errorf("depth below About = %d, want 2", deep)
	}
	if flat != 0 {
		t.Errorf("depth below a leaf = %d, want 0", flat)
	}
}

// trashRefusesContentAlreadyOnItsWayOut refuses to trash a trashed page again and leaves its first address alone.
func trashRefusesContentAlreadyOnItsWayOut(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)

	trashed, err := s.Content.Trash(t.Context(), about.ID, about.UpdatedAt)
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	_, err = s.Content.Trash(t.Context(), about.ID, trashed.UpdatedAt)

	if !errors.Is(err, content.ErrInvalidTransition) {
		t.Fatalf("trashing twice error = %v, want %v", err, content.ErrInvalidTransition)
	}
	if got := AddressOf(t, s.Content, about.ID); got != trashed.Path {
		t.Errorf("path = %q, want the first suffix left alone at %q", got, trashed.Path)
	}
}

// restoreReturnsAnItemToTheAddressItLeft restores a trashed page to the slug and address it left.
func restoreReturnsAnItemToTheAddressItLeft(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)

	trashed, err := s.Content.Trash(t.Context(), about.ID, about.UpdatedAt)
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	restored, err := s.Content.Restore(t.Context(), about.ID, trashed.UpdatedAt)
	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}

	if restored.Path != "pages/about" || restored.Slug != "about" {
		t.Errorf("restored to path %q slug %q, want the address it left", restored.Path, restored.Slug)
	}
}
