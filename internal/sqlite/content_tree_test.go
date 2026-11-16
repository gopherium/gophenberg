// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// loopWait bounds how long a write over a planted loop may walk before the test calls it endless.
const loopWait = 10 * time.Second

// pageTree is a planted site holding About with Team and Deep below it, and Contact with Directions below it.
type pageTree struct {
	plantedSite
	pages map[string]content.Content
}

// newPageTree returns a fresh page tree, every page stamped at the start moment.
func newPageTree(t *testing.T) pageTree {
	t.Helper()
	site := newPageSite(t)
	about := site.page(t, nil, "About", writtenAt)
	team := site.page(t, &about, "Team", writtenAt)
	contact := site.page(t, nil, "Contact", writtenAt)
	return pageTree{plantedSite: site, pages: map[string]content.Content{
		"About": about, "Team": team, "Deep": site.page(t, &team, "Deep", writtenAt),
		"Contact": contact, "Directions": site.page(t, &contact, "Directions", writtenAt),
	}}
}

// moved returns the page filed under the page named under, or at the top for no name, carrying slug a second later.
func (tree pageTree) moved(title, under, slug string) content.Content {
	page := tree.pages[title]
	prefix := "pages"
	page.ParentID = nil
	if parent, ok := tree.pages[under]; ok {
		prefix, page.ParentID = parent.Path, &parent.ID
	}
	page = page.Place(prefix, slug)
	page.UpdatedAt = page.UpdatedAt.Add(time.Second)
	return page
}

// stored returns the item the store holds under the id of the page named title.
func (tree pageTree) stored(t *testing.T, title string) content.Content {
	t.Helper()
	held, err := tree.store.ByID(t.Context(), tree.pages[title].ID)
	if err != nil {
		t.Fatalf("ByID(%q) error = %v, want nil", title, err)
	}
	return held
}

// unmoved fails t unless each page named still answers at its address with its stamp.
func (tree pageTree) unmoved(t *testing.T, titles ...string) {
	t.Helper()
	for _, title := range titles {
		before, after := tree.pages[title], tree.stored(t, title)
		if after.Path != before.Path || !after.UpdatedAt.Equal(before.UpdatedAt) {
			t.Errorf("%s = %s at %v, want %s at %v kept", title, after.Path, after.UpdatedAt, before.Path,
				before.UpdatedAt)
		}
	}
}

// carriedAs fails t unless each page named answers at its new address with the parent's stamp.
func (tree pageTree) carriedAs(t *testing.T, carried map[string]string, stamp time.Time) {
	t.Helper()
	for title, path := range carried {
		after := tree.stored(t, title)
		if after.Path != path || !after.UpdatedAt.Equal(stamp) {
			t.Errorf("%s = %s at %v, want %s at the parent's %v", title, after.Path, after.UpdatedAt, path, stamp)
		}
	}
}

// plantLoop files About under Team by plain SQL, so each stands above the other.
func (tree pageTree) plantLoop(t *testing.T) {
	t.Helper()
	mustExec(t, tree.handle, "UPDATE core_content SET parent_id = ? WHERE id = ?", tree.pages["Team"].ID,
		tree.pages["About"].ID)
}

func TestContentMoveCarriesEveryDescendantStampedAsTheParent(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, title, under, slug string
		carried                  map[string]string
	}{
		{"a parent renamed onto a name that starts like its old one", "About", "", "about-us",
			map[string]string{"Team": "pages/about-us/team", "Deep": "pages/about-us/team/deep"}},
		{"a parent moved down under another page", "About", "Contact", "about",
			map[string]string{"Team": "pages/contact/about/team", "Deep": "pages/contact/about/team/deep"}},
		{"a nested parent moved up to the top", "Team", "", "team", map[string]string{"Deep": "pages/team/deep"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tree := newPageTree(t)

			moved, err := tree.store.Update(t.Context(), tree.moved(tc.title, tc.under, tc.slug),
				tree.pages[tc.title].UpdatedAt, nil, 0)

			if err != nil {
				t.Fatalf("Update() error = %v, want nil", err)
			}
			tree.carriedAs(t, tc.carried, moved.UpdatedAt)
			tree.unmoved(t, "Directions")
		})
	}
}

func TestContentDescendantWalkStopsAtAPlantedLoop(t *testing.T) {
	t.Parallel()

	tree := newPageTree(t)
	tree.plantLoop(t)
	renamed := tree.moved("About", "Team", "company")
	renamed = renamed.Place("pages", "company")
	ctx, cancel := context.WithTimeout(t.Context(), loopWait)
	defer cancel()

	moved, err := tree.store.Update(ctx, renamed, tree.pages["About"].UpdatedAt, nil, 0)

	if err != nil {
		t.Fatalf("Update() of a page in a planted loop error = %v, want the walk to stop at the loop", err)
	}
	tree.carriedAs(t, map[string]string{"Team": "pages/company/team", "Deep": "pages/company/team/deep"},
		moved.UpdatedAt)
	if got := tree.stored(t, "About").Path; got != "pages/company" {
		t.Errorf("About = %s, want pages/company, not carried as a page below itself", got)
	}
}

func TestContentMoveUnderAParentInAPlantedLoopFilesTheItem(t *testing.T) {
	t.Parallel()

	tree := newPageTree(t)
	tree.plantLoop(t)
	ctx, cancel := context.WithTimeout(t.Context(), loopWait)
	defer cancel()

	filed, err := tree.store.Update(ctx, tree.moved("Contact", "About", "contact"),
		tree.pages["Contact"].UpdatedAt, nil, 0)

	if err != nil || filed.Path != "pages/about/contact" {
		t.Fatalf("Update() under a parent in a planted loop = %s, %v, want pages/about/contact", filed.Path, err)
	}
	tree.carriedAs(t, map[string]string{"Directions": "pages/about/contact/directions"}, filed.UpdatedAt)
}

func TestContentMoveUnderItsOwnDescendantAnswersErrCycle(t *testing.T) {
	t.Parallel()

	for _, under := range []string{"Team", "Deep"} {
		t.Run("under "+under, func(t *testing.T) {
			t.Parallel()
			tree := newPageTree(t)

			_, err := tree.store.Update(t.Context(), tree.moved("About", under, "about"),
				tree.pages["About"].UpdatedAt, nil, 0)

			if !errors.Is(err, content.ErrCycle) {
				t.Errorf("Update() of About under %s error = %v, want %v", under, err, content.ErrCycle)
			}
			tree.unmoved(t, "About", "Team", "Deep")
		})
	}
}

func TestContentDepthCountsTheLevelsBelowAnItem(t *testing.T) {
	t.Parallel()

	tree := newPageTree(t)

	for title, want := range map[string]int{"About": 2, "Team": 1, "Deep": 0, "Contact": 1, "Directions": 0} {
		if got, err := tree.store.Depth(t.Context(), tree.pages[title].ID); err != nil || got != want {
			t.Errorf("Depth(%s) = %d, %v, want %d", title, got, err, want)
		}
	}
}

func TestContentDepthStopsAtAPlantedLoop(t *testing.T) {
	t.Parallel()

	tree := newPageTree(t)
	tree.plantLoop(t)
	ctx, cancel := context.WithTimeout(t.Context(), loopWait)
	defer cancel()

	for title, want := range map[string]int{"About": 2, "Team": 1, "Deep": 0, "Contact": 1, "Directions": 0} {
		if got, err := tree.store.Depth(ctx, tree.pages[title].ID); err != nil || got != want {
			t.Errorf("Depth(%s) over a planted loop = %d, %v, want %d counted up to the loop", title, got, err, want)
		}
	}
}

func TestContentChildrenCountsTheItemsRightBelowAndKeepsTheirParentOutOfTheTrash(t *testing.T) {
	t.Parallel()

	tree := newPageTree(t)
	about := tree.pages["About"]
	tree.page(t, &about, "History", writtenAt)

	for title, want := range map[string]int{"About": 2, "Team": 1, "Deep": 0} {
		if got, err := tree.store.Children(t.Context(), tree.pages[title].ID); err != nil || got != want {
			t.Errorf("Children(%s) = %d, %v, want %d", title, got, err, want)
		}
	}
	if _, err := tree.store.Trash(t.Context(), about.ID, writtenAt.Add(time.Second)); !errors.Is(err,
		content.ErrHoldsChildren) {
		t.Errorf("Trash(About) error = %v, want %v", err, content.ErrHoldsChildren)
	}
	tree.unmoved(t, "About")
}

func TestContentNestedListKeepsTwelveSiblingsInTitleOrder(t *testing.T) {
	t.Parallel()

	site := newPageSite(t)
	guide := site.page(t, nil, "Guide", writtenAt)
	steps := []string{
		"Alpha", "Bravo", "Charlie", "Delta", "Echo", "Foxtrot", "Golf", "Hotel", "India", "Juliett", "Kilo", "Lima",
	}
	for _, title := range steps {
		site.page(t, &guide, title, writtenAt)
	}

	got, total := site.titles(t, content.Filter{
		Type: "page", OrderBy: content.OrderByTitle, Order: content.OrderAsc, Hierarchy: true,
	})

	if want := append([]string{"Guide"}, steps...); total != len(want) || !slices.Equal(got, want) {
		t.Errorf("nested = %v of %d, want %v, every sibling in title order below its parent", got, total, want)
	}
}

func TestContentNestedListPutsAChildWhoseParentIsLeftOutAfterTheRest(t *testing.T) {
	t.Parallel()

	site := newPageSite(t)
	about := site.page(t, nil, "About", writtenAt)
	for _, item := range []content.Content{
		site.page(t, &about, "Team", writtenAt),
		site.page(t, nil, "Contact", writtenAt),
		site.page(t, nil, "Zoo", writtenAt),
	} {
		site.publish(t, item, writtenAt.Add(time.Hour))
	}

	got, _ := site.titles(t, content.Filter{
		Type: "page", Statuses: []content.Status{content.StatusPublished},
		OrderBy: content.OrderByTitle, Order: content.OrderAsc, Hierarchy: true,
	})

	if want := []string{"Contact", "Zoo", "Team"}; !slices.Equal(got, want) {
		t.Errorf("nested = %v, want %v, the child of a left out parent after every other item", got, want)
	}
}

func TestContentRenameCarryingAChildOntoRootPaginationIsRejected(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	root := plantNestingPosts(t, site.handle)
	chapter := site.item(t, root, nil, "Chapter", writtenAt)
	child := site.item(t, root, &chapter, "2", writtenAt)
	renamed, err := chapter.Rename(content.PageWord)
	if err != nil {
		t.Fatalf("Rename() error = %v, want nil", err)
	}
	renamed.UpdatedAt = writtenAt.Add(time.Second)

	_, err = site.store.Update(t.Context(), renamed, chapter.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrReservedAddress) {
		t.Errorf("Update() carrying a child onto page/2 error = %v, want %v", err, content.ErrReservedAddress)
	}
	tree := pageTree{plantedSite: site, pages: map[string]content.Content{"Chapter": chapter, "2": child}}
	tree.unmoved(t, "Chapter", "2")
}

func TestContentListSortsByWhenTheParentWasWritten(t *testing.T) {
	t.Parallel()

	site := newPageSite(t)
	older := site.page(t, nil, "About", writtenAt)
	newer := site.page(t, nil, "Contact", writtenAt.Add(time.Minute))
	for _, item := range []content.Content{
		site.page(t, &newer, "Directions", writtenAt),
		site.page(t, nil, "Careers", writtenAt),
		site.page(t, &older, "Team", writtenAt),
	} {
		site.publish(t, item, writtenAt.Add(time.Hour))
	}

	for _, tc := range []struct {
		name   string
		order  content.Order
		nested bool
		want   []string
	}{
		{"flat going up", content.OrderAsc, false, []string{"Careers", "Team", "Directions"}},
		{"flat going down", content.OrderDesc, false, []string{"Directions", "Team", "Careers"}},
		{"nested going up", content.OrderAsc, true, []string{"Careers", "Team", "Directions"}},
		{"nested going down", content.OrderDesc, true, []string{"Careers", "Directions", "Team"}},
	} {
		got, _ := site.titles(t, content.Filter{
			Type: "page", Statuses: []content.Status{content.StatusPublished},
			OrderBy: content.OrderByParent, Order: tc.order, Hierarchy: tc.nested,
		})

		if !slices.Equal(got, tc.want) {
			t.Errorf("by parent %s = %v, want %v, the root placed as the direction says and the older parent's "+
				"child going up first", tc.name, got, tc.want)
		}
	}
}

func TestContentRenamedChildNumbersOnlyASlugItsSiblingsHold(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, slug, wantSlug, wantPath string
	}{
		{"onto a sibling's slug", "team", "team-2", "pages/about/team-2"},
		{"onto a root page's slug", "careers", "careers", "pages/about/careers"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			site := newPageSite(t)
			about := site.page(t, nil, "About", writtenAt)
			site.page(t, &about, "Team", writtenAt)
			site.page(t, nil, "Careers", writtenAt)
			history := site.page(t, &about, "History", writtenAt)
			renamed, err := history.Rename(tc.slug)
			if err != nil {
				t.Fatalf("Rename(%q) error = %v, want nil", tc.slug, err)
			}
			renamed.UpdatedAt = writtenAt.Add(time.Second)

			stored, err := site.store.Update(t.Context(), renamed, history.UpdatedAt, nil, 0)

			if err != nil || stored.Slug != tc.wantSlug || stored.Path != tc.wantPath {
				t.Errorf("Update() of History renamed %s = %s at %s, %v, want %s at %s", tc.slug, stored.Slug,
					stored.Path, err, tc.wantSlug, tc.wantPath)
			}
		})
	}
}

func TestContentPostUnderAnotherPostAnswersErrNotHierarchical(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	nesting := contenttest.PostType()
	nesting.Hierarchical = true
	harbor := site.item(t, contenttest.PostType(), nil, "Harbor Notes", writtenAt)
	tides := site.item(t, contenttest.PostType(), nil, "Tide Tables", writtenAt)
	child, err := content.New(nesting, &harbor, "Morning Tide", site.author)
	if err != nil {
		t.Fatalf("New() under a post error = %v, want nil", err)
	}
	child.CreatedAt, child.UpdatedAt = writtenAt, writtenAt
	moved, err := content.Reparent(nesting, tides, &harbor, 0)
	if err != nil {
		t.Fatalf("Reparent() under a post error = %v, want nil", err)
	}
	moved.UpdatedAt = writtenAt.Add(time.Second)

	_, createErr := site.store.Create(t.Context(), child)
	_, updateErr := site.store.Update(t.Context(), moved, tides.UpdatedAt, nil, 0)

	if !errors.Is(createErr, content.ErrNotHierarchical) {
		t.Errorf("Create() under a stored post error = %v, want %v", createErr, content.ErrNotHierarchical)
	}
	if !errors.Is(updateErr, content.ErrNotHierarchical) {
		t.Errorf("Update() moving a stored post under another error = %v, want %v", updateErr,
			content.ErrNotHierarchical)
	}
	tree := pageTree{plantedSite: site, pages: map[string]content.Content{"Tide Tables": tides}}
	tree.unmoved(t, "Tide Tables")
}
