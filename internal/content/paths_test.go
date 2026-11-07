// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// pageType returns a hierarchical page type answering under pages.
func pageType() content.Type {
	t := postType()
	t.Key, t.SingularLabel, t.PluralLabel, t.Description = "page", "Page", "Pages", ""
	t.RouteWord, t.Hierarchical, t.Default = "pages", true, false
	return t
}

// mustNew returns content of the type, under the parent, or fails the test.
func mustNew(t *testing.T, contentType content.Type, parent *content.Content, title string) content.Content {
	t.Helper()
	built, err := content.New(contentType, parent, title, uuid.Must(uuid.NewV7()))
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	return built
}

func TestNewAddressesRootContentUnderItsRouteWord(t *testing.T) {
	t.Parallel()

	post := mustNew(t, postType(), nil, "Hello World")
	page := mustNew(t, pageType(), nil, "About")

	if post.Path != "hello-world" {
		t.Errorf("post path = %q, want the default type at the root", post.Path)
	}
	if page.Path != "pages/about" {
		t.Errorf("page path = %q, want it under the route word", page.Path)
	}
	if post.ParentID != nil || page.ParentID != nil {
		t.Errorf("parents = %v and %v, want root content to have none", post.ParentID, page.ParentID)
	}
}

func TestNewAddressesAChildUnderItsParent(t *testing.T) {
	t.Parallel()

	about := mustNew(t, pageType(), nil, "About")

	team := mustNew(t, pageType(), &about, "Team")

	if team.Path != "pages/about/team" {
		t.Errorf("path = %q, want the chain of ancestors", team.Path)
	}
	if team.ParentID == nil || *team.ParentID != about.ID {
		t.Errorf("ParentID = %v, want %v", team.ParentID, about.ID)
	}
}

func TestNewRefusesAParentAFlatTypeCannotHold(t *testing.T) {
	t.Parallel()

	hello := mustNew(t, postType(), nil, "Hello World")

	_, err := content.New(postType(), &hello, "Nested", uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrNotHierarchical) {
		t.Errorf("New() error = %v, want %v", err, content.ErrNotHierarchical)
	}
}

func TestNewRefusesAParentOfAnotherType(t *testing.T) {
	t.Parallel()

	hello := mustNew(t, postType(), nil, "Hello World")

	_, err := content.New(pageType(), &hello, "Team", uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrParentType) {
		t.Errorf("New() error = %v, want %v", err, content.ErrParentType)
	}
}

func TestNewRefusesNestingPastTheLimit(t *testing.T) {
	t.Parallel()

	pages := pageType()
	deepest := mustNew(t, pages, nil, "Level 1")
	for level := 2; level <= content.MaxDepth; level++ {
		deepest = mustNew(t, pages, &deepest, "Level")
	}

	_, err := content.New(pages, &deepest, "One Too Many", uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrTooDeep) {
		t.Errorf("New() at depth %d error = %v, want %v", content.MaxDepth+1, err, content.ErrTooDeep)
	}
}

func TestNewRefusesAReservedAddress(t *testing.T) {
	t.Parallel()

	for _, reserved := range content.ReservedRouteWords {
		_, err := content.New(postType(), nil, reserved, uuid.Must(uuid.NewV7()))

		if !errors.Is(err, content.ErrReservedAddress) {
			t.Errorf("New(%q) error = %v, want %v", reserved, err, content.ErrReservedAddress)
		}
	}
}

func TestNewRefusesAReservedAddressBehindAnApostropheAndAMark(t *testing.T) {
	t.Parallel()

	for _, reserved := range content.ReservedRouteWords {
		title := reserved + "'\U0000FE0F"
		_, err := content.New(postType(), nil, title, uuid.Must(uuid.NewV7()))

		if !errors.Is(err, content.ErrReservedAddress) {
			t.Errorf("New(%q) error = %v, want %v", title, err, content.ErrReservedAddress)
		}
	}
}

func TestNewRefusesAReservedAddressBehindADroppedSymbolAndAMark(t *testing.T) {
	t.Parallel()

	for _, reserved := range content.ReservedRouteWords {
		title := reserved + "&\U0000FE0F"
		_, err := content.New(postType(), nil, title, uuid.Must(uuid.NewV7()))

		if !errors.Is(err, content.ErrReservedAddress) {
			t.Errorf("New(%q) error = %v, want %v", title, err, content.ErrReservedAddress)
		}
	}
}

func TestReservedAddressNamesTheAddress(t *testing.T) {
	t.Parallel()

	err := content.ReservedAddress("page/2")

	if !errors.Is(err, content.ErrReservedAddress) {
		t.Errorf("error = %v, want it to carry %v", err, content.ErrReservedAddress)
	}
	if code, _ := content.CodeOf(err); code != "address_reserved" {
		t.Errorf("code = %q, want address_reserved", code)
	}
	if held, _ := content.DetailsOf(err); held["path"] != "page/2" {
		t.Errorf("details = %v, want the path page/2 named", held)
	}
	if !strings.Contains(err.Error(), `"page/2"`) {
		t.Errorf("message = %q, want it to name the address", err.Error())
	}
}

func TestCheckPaginationPathsPassesAddressesTheRootListingLeavesAlone(t *testing.T) {
	t.Parallel()

	for name, paths := range map[string][]string{
		"no addresses":                     nil,
		"addresses no root page names":     {"about", "books/page/2", "pages/about/page/3"},
		"a page word without a page count": {"page", "page/two"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := content.CheckPaginationPaths(paths); err != nil {
				t.Errorf("CheckPaginationPaths(%q) error = %v, want nil", paths, err)
			}
		})
	}
}

func TestCheckPaginationPathsNamesTheFirstAddressTheRootListingTakes(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		paths []string
		want  string
	}{
		"one root page":             {[]string{"page/2"}, "page/2"},
		"a root page after a free":  {[]string{"about", "page/3"}, "page/3"},
		"the first of two named":    {[]string{"about", "page/3", "page/2"}, "page/3"},
		"a root page before a free": {[]string{"page/4", "about"}, "page/4"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := content.CheckPaginationPaths(test.paths)

			if !errors.Is(err, content.ErrReservedAddress) {
				t.Fatalf("CheckPaginationPaths(%q) error = %v, want %v", test.paths, err, content.ErrReservedAddress)
			}
			if held, _ := content.DetailsOf(err); held["path"] != test.want {
				t.Errorf("details = %v, want the path %s named", held, test.want)
			}
		})
	}
}

func TestNewNamesTheReservedAddressItRefuses(t *testing.T) {
	t.Parallel()

	root := postType()
	root.Hierarchical = true
	parent := mustNew(t, root, nil, "Page")
	for _, tc := range []struct {
		parent      *content.Content
		title, path string
	}{
		{nil, "Admin", "admin"},
		{&parent, "2", "page/2"},
	} {
		_, err := content.New(root, tc.parent, tc.title, uuid.Must(uuid.NewV7()))

		if code, _ := content.CodeOf(err); code != "address_reserved" {
			t.Errorf("New(%q) code = %q, want address_reserved", tc.title, code)
		}
		if held, _ := content.DetailsOf(err); held["path"] != tc.path {
			t.Errorf("New(%q) details = %v, want the path %s named", tc.title, held, tc.path)
		}
	}
}

func TestNewLeavesAReservedWordAloneBelowTheRoot(t *testing.T) {
	t.Parallel()

	about := mustNew(t, pageType(), nil, "About")

	admin := mustNew(t, pageType(), &about, "Admin")

	if admin.Path != "pages/about/admin" {
		t.Errorf("path = %q, want a reserved word allowed away from the first segment", admin.Path)
	}
}

func TestAddressUnderRebuildsAChildAddress(t *testing.T) {
	t.Parallel()

	got := content.AddressUnder("pages/company", "team")

	if got != "pages/company/team" {
		t.Errorf("AddressUnder() = %q, want the child under its parent", got)
	}
	if root := content.AddressUnder("", "hello"); root != "hello" {
		t.Errorf("AddressUnder() at the root = %q, want %q", root, "hello")
	}
}

func TestAddressPrefixReadsWhatASlugHangsFrom(t *testing.T) {
	t.Parallel()

	prefixes := map[string]string{
		"hello":            "",
		"pages/about":      "pages",
		"pages/about/team": "pages/about",
	}

	for path, want := range prefixes {
		slug := path[strings.LastIndex(path, "/")+1:]

		if got := content.AddressPrefix(path, slug); got != want {
			t.Errorf("AddressPrefix(%q, %q) = %q, want %q", path, slug, got, want)
		}
	}
}

func TestFirstSegmentReadsTheRootOfAnAddress(t *testing.T) {
	t.Parallel()

	addresses := map[string]string{
		"hello":            "hello",
		"pages/about":      "pages",
		"pages/about/team": "pages",
		"":                 "",
	}

	for path, want := range addresses {
		if got := content.FirstSegment(path); got != want {
			t.Errorf("FirstSegment(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestNewRefusesContentDeeperThanTheAddressLimit(t *testing.T) {
	t.Parallel()

	pages := pageType()
	long := mustNew(t, pages, nil, strings.Repeat("a", 200))

	if len(long.Path) > 210 {
		t.Errorf("path length = %d, want the slug cap to bound one segment", len(long.Path))
	}
}

func TestRenameRefusesAReservedAddress(t *testing.T) {
	t.Parallel()

	post := mustNew(t, postType(), nil, "Innocent")

	renamed, err := post.Rename("admin")

	if !errors.Is(err, content.ErrReservedAddress) {
		t.Fatalf("Rename(%q) error = %v, want %v", "admin", err, content.ErrReservedAddress)
	}
	if renamed.Path != "" {
		t.Errorf("path = %q, want nothing when the rename is refused", renamed.Path)
	}
}

func TestRenameCarriesTheItemToItsNewAddress(t *testing.T) {
	t.Parallel()

	about := mustNew(t, pageType(), nil, "About")
	team := mustNew(t, pageType(), &about, "Team")

	renamed, err := team.Rename("crew")
	if err != nil {
		t.Fatalf("Rename(%q) error = %v, want nil", "crew", err)
	}

	if renamed.Path != "pages/about/crew" {
		t.Errorf("path = %q, want the new name under the same parent", renamed.Path)
	}
	if renamed.Slug != "crew" {
		t.Errorf("slug = %q, want %q", renamed.Slug, "crew")
	}
}

func TestRenameLeavesAReservedWordAloneBelowTheRoot(t *testing.T) {
	t.Parallel()

	about := mustNew(t, pageType(), nil, "About")

	renamed, err := about.Rename("admin")
	if err != nil {
		t.Fatalf("Rename(%q) error = %v, want nil below the root", "admin", err)
	}

	if renamed.Path != "pages/admin" {
		t.Errorf("path = %q, want the reserved word to be free below the root", renamed.Path)
	}
}

func TestNewRefusesAParentOnItsWayOut(t *testing.T) {
	t.Parallel()

	about := mustNew(t, pageType(), nil, "About")
	about.Status = content.StatusTrash

	_, err := content.New(pageType(), &about, "Team", uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrParentTrashed) {
		t.Fatalf("New under a trashed parent error = %v, want %v", err, content.ErrParentTrashed)
	}
}

func TestReparentRefusesAParentOnItsWayOut(t *testing.T) {
	t.Parallel()

	about := mustNew(t, pageType(), nil, "About")
	team := mustNew(t, pageType(), nil, "Team")
	about.Status = content.StatusTrash

	_, err := content.Reparent(pageType(), team, &about, 0)

	if !errors.Is(err, content.ErrParentTrashed) {
		t.Fatalf("Reparent under a trashed parent error = %v, want %v", err, content.ErrParentTrashed)
	}
}

func TestReparentCarriesTheItemUnderItsNewParent(t *testing.T) {
	t.Parallel()

	about := mustNew(t, pageType(), nil, "About")
	team := mustNew(t, pageType(), nil, "Team")

	moved, err := content.Reparent(pageType(), team, &about, 0)
	if err != nil {
		t.Fatalf("Reparent error = %v, want nil", err)
	}

	if moved.Path != "pages/about/team" {
		t.Errorf("path = %q, want it under the new parent", moved.Path)
	}
	if moved.ParentID == nil || *moved.ParentID != about.ID {
		t.Errorf("ParentID = %v, want %v", moved.ParentID, about.ID)
	}
}

func TestReparentToTheRootLiftsTheItemBack(t *testing.T) {
	t.Parallel()

	about := mustNew(t, pageType(), nil, "About")
	team := mustNew(t, pageType(), &about, "Team")

	lifted, err := content.Reparent(pageType(), team, nil, 0)
	if err != nil {
		t.Fatalf("Reparent to the root error = %v, want nil", err)
	}

	if lifted.Path != "pages/team" {
		t.Errorf("path = %q, want it back under the route word", lifted.Path)
	}
	if lifted.ParentID != nil {
		t.Errorf("ParentID = %v, want none at the root", lifted.ParentID)
	}
}

func TestReparentRefusesNestingInsideItself(t *testing.T) {
	t.Parallel()

	about := mustNew(t, pageType(), nil, "About")
	team := mustNew(t, pageType(), &about, "Team")

	_, selfErr := content.Reparent(pageType(), about, &about, 0)
	_, descendantErr := content.Reparent(pageType(), about, &team, 0)

	if !errors.Is(selfErr, content.ErrCycle) {
		t.Errorf("Reparent under itself error = %v, want %v", selfErr, content.ErrCycle)
	}
	if !errors.Is(descendantErr, content.ErrCycle) {
		t.Errorf("Reparent under its own child error = %v, want %v", descendantErr, content.ErrCycle)
	}
}

func TestReparentRefusesCarryingAChainPastTheLimit(t *testing.T) {
	t.Parallel()

	deep := mustNew(t, pageType(), nil, "Level 1")
	for level := 2; level <= 9; level++ {
		deep = mustNew(t, pageType(), &deep, "Level")
	}
	branch := mustNew(t, pageType(), nil, "Branch")

	_, fits := content.Reparent(pageType(), branch, &deep, 0)
	_, tooTall := content.Reparent(pageType(), branch, &deep, 2)

	if fits != nil {
		t.Errorf("moving a leaf under level 9 error = %v, want nil", fits)
	}
	if !errors.Is(tooTall, content.ErrTooDeep) {
		t.Errorf("moving a branch two levels tall error = %v, want %v", tooTall, content.ErrTooDeep)
	}
}
