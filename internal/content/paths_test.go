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
	t.Key, t.SingularLabel, t.PluralLabel = "page", "Page", "Pages"
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
