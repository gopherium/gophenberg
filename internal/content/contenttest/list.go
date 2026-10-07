// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"cmp"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// listCases are the cases over listing, sorting, narrowing, paging and counting the items of a type.
var listCases = []Case{
	{"ListOrdersByPublicationThenCreation", listOrdersByPublicationThenCreation},
	{"ListOmitsContent", listOmitsContent},
	{"ListFiltersByStatus", listFiltersByStatus},
	{"ListSearchesTitleAndContent", listSearchesTitleAndContent},
	{"ListTreatsWildcardsAsLiterals", listTreatsWildcardsAsLiterals},
	{"ListPaginates", listPaginates},
	{"ListReturnsAnEmptyPagePastTheEnd", listReturnsAnEmptyPagePastTheEnd},
	{"CountsPerStatus", countsPerStatus},
	{"ListSortsByTitle", listSortsByTitle},
	{"ListSortsByDateIndependentlyOfCreationOrder", listSortsByDateIndependentlyOfCreationOrder},
	{"ListDefaultsToNewestPublicationFirst", listDefaultsToNewestPublicationFirst},
	{"ListNamesTheAccountThatWroteEachItem", listNamesTheAccountThatWroteEachItem},
	{"ListKeepsOnlyTheStatusesItIsGiven", listKeepsOnlyTheStatusesItIsGiven},
	{"ListKeepsEveryStatusWhenItNamesNone", listKeepsEveryStatusWhenItNamesNone},
	{"ListKeepsOnlyTheAuthorsItIsGiven", listKeepsOnlyTheAuthorsItIsGiven},
	{"ListKeepsTheItemsDatedBeforeAndAfterADay", listKeepsTheItemsDatedBeforeAndAfterADay},
	{"ListDatesAnUnpublishedItemByItsLastChange", listDatesAnUnpublishedItemByItsLastChange},
	{"ListSortsByTheNameOfTheAuthor", listSortsByTheNameOfTheAuthor},
	{"ListSortsBySlug", listSortsBySlug},
	{"ListSortsByWhenTheParentWasWritten", listSortsByWhenTheParentWasWritten},
	{"ListNestsEachChildUnderItsParent", listNestsEachChildUnderItsParent},
	{"ListNestsAChildWhoseParentIsLeftOutAfterTheRest", listNestsAChildWhoseParentIsLeftOutAfterTheRest},
	{"ListCarriesTheTitleOfEachParent", listCarriesTheTitleOfEachParent},
	{"ListSortsTitlesCaseInsensitively", listSortsTitlesCaseInsensitively},
}

// listed returns the titles and the total the store lists under the filter.
func listed(t *testing.T, store content.Store, f content.Filter) ([]string, int) {
	t.Helper()
	if f.Type == "" {
		f.Type = content.TypePost
	}
	f.Page, f.PerPage = cmp.Or(f.Page, 1), cmp.Or(f.PerPage, 10)
	rows, total, err := store.List(t.Context(), f)
	if err != nil {
		t.Fatalf("List(%+v) error = %v, want nil", f, err)
	}
	return TitlesOf(rows), total
}

// listOrdersByPublicationThenCreation lists the fresh draft first, then the published posts newest first.
func listOrdersByPublicationThenCreation(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	now := time.Now().UTC().Truncate(time.Microsecond)
	Publish(t, s.Content, "Older Published", author, now.Add(-48*time.Hour))
	Publish(t, s.Content, "Newer Published", author, now.Add(-1*time.Hour))
	MustCreate(t, s.Content, "Fresh Draft", author)

	posts, total, err := s.Content.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	want := []string{"Fresh Draft", "Newer Published", "Older Published"}
	got := TitlesOf(posts)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

// listOmitsContent leaves the body out of each listed row.
func listOmitsContent(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "With Body", author)
	edited := created
	edited.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	if _, err := s.Content.Update(t.Context(), edited, created.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	posts, _, err := s.Content.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if body := posts[0].Content.Content; body != "" {
		t.Errorf("Content = %q, want listings to omit it", body)
	}
}

// listFiltersByStatus lists only the posts in the status asked for.
func listFiltersByStatus(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	Publish(t, s.Content, "Published One", author, time.Now().UTC().Truncate(time.Microsecond))
	MustCreate(t, s.Content, "Draft One", author)

	posts, total, err := s.Content.List(
		t.Context(),
		content.Filter{Type: content.TypePost, Statuses: []content.Status{content.StatusPublished}, Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 1 || len(posts) != 1 || posts[0].Title != "Published One" {
		t.Errorf("List() = %v with total %d, want only the published post", TitlesOf(posts), total)
	}
}

// listSearchesTitleAndContent matches a search in the title or in the body.
func listSearchesTitleAndContent(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "Gutenberg Editor", author)
	withBody := MustCreate(t, s.Content, "Unrelated Title", author)
	edited := withBody
	edited.Content = "a paragraph mentioning gutenberg inside"
	if _, err := s.Content.Update(t.Context(), edited, withBody.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	MustCreate(t, s.Content, "Something Else", author)

	posts, total, err := s.Content.List(
		t.Context(),
		content.Filter{Type: content.TypePost, Search: "gutenberg", Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 2 || len(posts) != 2 {
		t.Errorf("List() = %v with total %d, want the two gutenberg matches", TitlesOf(posts), total)
	}
}

// listTreatsWildcardsAsLiterals matches a percent sign in a search as itself.
func listTreatsWildcardsAsLiterals(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "100% Coverage", author)
	MustCreate(t, s.Content, "Plain Title", author)

	posts, total, err := s.Content.List(
		t.Context(),
		content.Filter{Type: content.TypePost, Search: "100%", Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 1 || len(posts) != 1 || posts[0].Title != "100% Coverage" {
		t.Errorf("List() = %v with total %d, want only the literal match", TitlesOf(posts), total)
	}
}

// listPaginates serves the second page under the total of the whole filter.
func listPaginates(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	now := time.Now().UTC().Truncate(time.Microsecond)
	Publish(t, s.Content, "First", author, now.Add(-3*time.Hour))
	Publish(t, s.Content, "Second", author, now.Add(-2*time.Hour))
	Publish(t, s.Content, "Third", author, now.Add(-1*time.Hour))

	second, total, err := s.Content.List(t.Context(), content.Filter{Type: content.TypePost, Page: 2, PerPage: 2})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 under the same filter", total)
	}
	if len(second) != 1 || second[0].Title != "First" {
		t.Errorf("page 2 = %v, want the oldest post alone", TitlesOf(second))
	}
}

// listReturnsAnEmptyPagePastTheEnd answers an empty page with the full total past the last row.
func listReturnsAnEmptyPagePastTheEnd(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "Only One", author)

	posts, total, err := s.Content.List(t.Context(), content.Filter{Type: content.TypePost, Page: 5, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if posts == nil || len(posts) != 0 {
		t.Errorf("posts = %v, want an empty non-nil slice", posts)
	}
}

// countsPerStatus counts the posts in each status and leaves the absent statuses out.
func countsPerStatus(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	Publish(t, s.Content, "Published One", author, time.Now().UTC().Truncate(time.Microsecond))
	MustCreate(t, s.Content, "Draft One", author)
	MustCreate(t, s.Content, "Draft Two", author)

	counts, err := s.Content.Counts(t.Context(), content.TypePost)

	if err != nil {
		t.Fatalf("Counts() error = %v, want nil", err)
	}
	if counts[content.StatusDraft] != 2 || counts[content.StatusPublished] != 1 {
		t.Errorf("counts = %v, want two drafts and one published", counts)
	}
	if _, ok := counts[content.StatusTrash]; ok {
		t.Errorf("counts = %v, want absent statuses omitted by the store", counts)
	}
}

// listSortsByTitle sorts by title in both directions.
func listSortsByTitle(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "Beta", author)
	MustCreate(t, s.Content, "Alpha", author)
	MustCreate(t, s.Content, "Gamma", author)

	ascending, _, err := s.Content.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if got := TitlesOf(ascending); got[0] != "Alpha" || got[2] != "Gamma" {
		t.Errorf("ascending = %v, want Alpha first and Gamma last", got)
	}

	descending, _, err := s.Content.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderDesc, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if got := TitlesOf(descending); got[0] != "Gamma" || got[2] != "Alpha" {
		t.Errorf("descending = %v, want Gamma first and Alpha last", got)
	}
}

// listSortsByDateIndependentlyOfCreationOrder sorts by the publication date, not by the order of storing.
func listSortsByDateIndependentlyOfCreationOrder(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	now := time.Now().UTC().Truncate(time.Microsecond)
	Publish(t, s.Content, "Published Long Ago", author, now.Add(-72*time.Hour))
	Publish(t, s.Content, "Published Recently", author, now.Add(-1*time.Hour))

	oldestFirst, _, err := s.Content.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByDate, Order: content.OrderAsc, Page: 1, PerPage: 10,
	})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if got := TitlesOf(oldestFirst); got[0] != "Published Long Ago" {
		t.Errorf("ascending by date = %v, want the oldest publication first", got)
	}
}

// listDefaultsToNewestPublicationFirst lists the newest publication first when no order is named.
func listDefaultsToNewestPublicationFirst(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	now := time.Now().UTC().Truncate(time.Microsecond)
	Publish(t, s.Content, "Published Recently", author, now.Add(-1*time.Hour))
	Publish(t, s.Content, "Published Long Ago", author, now.Add(-72*time.Hour))

	posts, _, err := s.Content.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if got := TitlesOf(posts); got[0] != "Published Recently" {
		t.Errorf("default order = %v, want the newest publication first", got)
	}
}

// listNamesTheAccountThatWroteEachItem credits each listed item to the account that wrote it.
func listNamesTheAccountThatWroteEachItem(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	other := s.AddAuthor(t, "Another Writer")
	MustCreate(t, s.Content, "By Maria", author)
	MustCreate(t, s.Content, "By Another Writer", other)

	rows, _, err := s.Content.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	named := map[string]string{}
	for _, row := range rows {
		named[row.Title] = row.AuthorName
	}
	if named["By Maria"] != "Maria Perez" || named["By Another Writer"] != "Another Writer" {
		t.Errorf("authors = %v, want each item credited to the account that wrote it", named)
	}
}

// listKeepsOnlyTheStatusesItIsGiven keeps only the items in the statuses the filter names.
func listKeepsOnlyTheStatusesItIsGiven(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	Publish(t, s.Content, "Published", author, time.Now().UTC().Truncate(time.Microsecond))
	MustCreate(t, s.Content, "Draft", author)
	Trash(t, s.Content, MustCreate(t, s.Content, "Trashed", author))
	pending := MustCreate(t, s.Content, "Pending", author)
	pending.Status = content.StatusPending
	if _, err := s.Content.Update(t.Context(), pending, pending.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	titles, total := listed(t, s.Content, content.Filter{
		Statuses: []content.Status{content.StatusDraft, content.StatusPublished},
	})

	slices.Sort(titles)
	if total != 2 || !slices.Equal(titles, []string{"Draft", "Published"}) {
		t.Errorf("listed %v of %d, want the draft and the published post alone", titles, total)
	}
}

// listKeepsEveryStatusWhenItNamesNone keeps every status, the trash included, when the filter names none.
func listKeepsEveryStatusWhenItNamesNone(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "Draft", author)
	Trash(t, s.Content, MustCreate(t, s.Content, "Trashed", author))

	titles, total := listed(t, s.Content, content.Filter{})

	if total != 2 || len(titles) != 2 {
		t.Errorf("listed %v of %d, want every status kept, the trash included", titles, total)
	}
}

// listKeepsOnlyTheAuthorsItIsGiven keeps the items of the named authors and leaves out the excluded ones.
func listKeepsOnlyTheAuthorsItIsGiven(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	other := s.AddAuthor(t, "Another Writer")
	MustCreate(t, s.Content, "By Maria", author)
	MustCreate(t, s.Content, "By Another Writer", other)

	kept, keptTotal := listed(t, s.Content, content.Filter{Authors: []uuid.UUID{other}})
	left, leftTotal := listed(t, s.Content, content.Filter{ExcludeAuthors: []uuid.UUID{other}})

	if keptTotal != 1 || !slices.Equal(kept, []string{"By Another Writer"}) {
		t.Errorf("kept %v of %d, want only the item the named author wrote", kept, keptTotal)
	}
	if leftTotal != 1 || !slices.Equal(left, []string{"By Maria"}) {
		t.Errorf("left %v of %d, want every item but the excluded author's", left, leftTotal)
	}
}

// listKeepsTheItemsDatedBeforeAndAfterADay keeps the items dated before or after a day, the bound left out.
func listKeepsTheItemsDatedBeforeAndAfterADay(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	march := time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)
	july := time.Date(2026, time.July, 10, 9, 0, 0, 0, time.UTC)
	Publish(t, s.Content, "Spring", author, march)
	Publish(t, s.Content, "Summer", author, july)
	MustCreate(t, s.Content, "Fresh Draft", author)
	june := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

	before, beforeTotal := listed(t, s.Content, content.Filter{Before: &june})
	after, afterTotal := listed(t, s.Content, content.Filter{After: &june})
	exactly, _ := listed(t, s.Content, content.Filter{Before: &march})

	if beforeTotal != 1 || !slices.Equal(before, []string{"Spring"}) {
		t.Errorf("before = %v of %d, want only the item dated before the day", before, beforeTotal)
	}
	if afterTotal != 2 || !slices.Equal(after, []string{"Fresh Draft", "Summer"}) {
		t.Errorf("after = %v of %d, want the summer post and the draft dated by its last change", after, afterTotal)
	}
	if len(exactly) != 0 {
		t.Errorf("before its own date = %v, want an item dated exactly at the bound left out", exactly)
	}
}

// listDatesAnUnpublishedItemByItsLastChange dates a draft by its last change.
func listDatesAnUnpublishedItemByItsLastChange(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	first := MustCreate(t, s.Content, "Written First", author)
	MustCreate(t, s.Content, "Written Second", author)
	edited := first
	edited.Title, edited.UpdatedAt = "Written First, Changed Last", time.Now().UTC().Add(time.Minute)
	if _, err := s.Content.Update(t.Context(), edited, first.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	titles, _ := listed(t, s.Content, content.Filter{OrderBy: content.OrderByDate, Order: content.OrderDesc})

	if !slices.Equal(titles, []string{"Written First, Changed Last", "Written Second"}) {
		t.Errorf("newest first = %v, want the draft changed last on top", titles)
	}
}

// listSortsByTheNameOfTheAuthor sorts by the name of the account that wrote each item.
func listSortsByTheNameOfTheAuthor(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	other := s.AddAuthor(t, "Another Writer")
	MustCreate(t, s.Content, "By Maria", author)
	MustCreate(t, s.Content, "By Another Writer", other)

	ascending, _ := listed(t, s.Content, content.Filter{OrderBy: content.OrderByAuthor, Order: content.OrderAsc})
	descending, _ := listed(t, s.Content, content.Filter{OrderBy: content.OrderByAuthor, Order: content.OrderDesc})

	if !slices.Equal(ascending, []string{"By Another Writer", "By Maria"}) {
		t.Errorf("ascending = %v, want Another Writer's item first", ascending)
	}
	if !slices.Equal(descending, []string{"By Maria", "By Another Writer"}) {
		t.Errorf("descending = %v, want Maria Perez's item first", descending)
	}
}

// listSortsBySlug sorts by slug in both directions.
func listSortsBySlug(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "Banana Bread", author)
	MustCreate(t, s.Content, "Apple Pie", author)
	MustCreate(t, s.Content, "Cherry Tart", author)

	ascending, _ := listed(t, s.Content, content.Filter{OrderBy: content.OrderBySlug, Order: content.OrderAsc})
	descending, _ := listed(t, s.Content, content.Filter{OrderBy: content.OrderBySlug, Order: content.OrderDesc})

	if !slices.Equal(ascending, []string{"Apple Pie", "Banana Bread", "Cherry Tart"}) {
		t.Errorf("ascending = %v, want the slugs in order", ascending)
	}
	if !slices.Equal(descending, []string{"Cherry Tart", "Banana Bread", "Apple Pie"}) {
		t.Errorf("descending = %v, want the slugs in reverse", descending)
	}
}

// listSortsByWhenTheParentWasWritten sorts pages by when their parent was written, those with no parent first.
func listSortsByWhenTheParentWasWritten(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	contact := MustNest(t, s.Content, nil, "Contact", author)
	MustNest(t, s.Content, &about, "Team", author)
	MustNest(t, s.Content, &contact, "Directions", author)

	ascending, _ := listed(t, s.Content, content.Filter{
		Type: "page", OrderBy: content.OrderByParent, Order: content.OrderAsc,
	})
	descending, _ := listed(t, s.Content, content.Filter{
		Type: "page", OrderBy: content.OrderByParent, Order: content.OrderDesc,
	})

	if !slices.Equal(ascending[2:], []string{"Team", "Directions"}) {
		t.Errorf("ascending = %v, want the items with no parent first, then by the older parent", ascending)
	}
	if !slices.Equal(descending[:2], []string{"Directions", "Team"}) {
		t.Errorf("descending = %v, want the newer parent's child first and the items with no parent last", descending)
	}
}

// listNestsEachChildUnderItsParent lists each child under its parent and pages the nested order.
func listNestsEachChildUnderItsParent(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	MustNest(t, s.Content, nil, "Contact", author)
	MustNest(t, s.Content, &about, "Team", author)
	MustNest(t, s.Content, &about, "History", author)
	nested := content.Filter{Type: "page", OrderBy: content.OrderByTitle, Order: content.OrderAsc, Hierarchy: true}

	titles, total := listed(t, s.Content, nested)
	flat, _ := listed(t, s.Content, content.Filter{Type: "page", OrderBy: content.OrderByTitle, Order: content.OrderAsc})
	nested.Page, nested.PerPage = 2, 2
	second, _ := listed(t, s.Content, nested)

	if total != 4 || !slices.Equal(titles, []string{"About", "History", "Team", "Contact"}) {
		t.Errorf("nested = %v of %d, want each child under its parent in title order", titles, total)
	}
	if !slices.Equal(flat, []string{"About", "Contact", "History", "Team"}) {
		t.Errorf("flat = %v, want every item sorted by itself", flat)
	}
	if !slices.Equal(second, []string{"Team", "Contact"}) {
		t.Errorf("second page = %v, want the nested order paged", second)
	}
}

// listNestsAChildWhoseParentIsLeftOutAfterTheRest lists a child whose parent the filter leaves out after the rest.
func listNestsAChildWhoseParentIsLeftOutAfterTheRest(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	for _, item := range []content.Content{
		MustNest(t, s.Content, &about, "Team", author),
		MustNest(t, s.Content, nil, "Contact", author),
		MustNest(t, s.Content, nil, "Zoo", author),
	} {
		edited := item
		at := time.Now().UTC()
		edited.Status, edited.PublishedAt, edited.UpdatedAt = content.StatusPublished, &at, at
		if _, err := s.Content.Update(t.Context(), edited, item.UpdatedAt, nil, 0); err != nil {
			t.Fatalf("publishing %q: %v", item.Title, err)
		}
	}

	titles, _ := listed(t, s.Content, content.Filter{
		Type: "page", Statuses: []content.Status{content.StatusPublished},
		OrderBy: content.OrderByTitle, Order: content.OrderAsc, Hierarchy: true,
	})

	if !slices.Equal(titles, []string{"Contact", "Zoo", "Team"}) {
		t.Errorf("nested = %v, want the child of a parent left out after every item whose parent is kept", titles)
	}
}

// listCarriesTheTitleOfEachParent lists each item with its parent's title, flat, nested and searched.
func listCarriesTheTitleOfEachParent(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	MustNest(t, s.Content, &team, "Volunteers", author)
	untitled := MustNest(t, s.Content, nil, "", author)
	MustNest(t, s.Content, &untitled, "Donations", author)
	everyItem := map[string]string{"About": "", "Team": "About", "Volunteers": "Team", "": "", "Donations": ""}

	for _, listing := range []struct {
		nested bool
		search string
		want   map[string]string
	}{
		{false, "", everyItem},
		{true, "", everyItem},
		{true, "Volunteers", map[string]string{"Volunteers": "Team"}},
	} {
		rows, _, err := s.Content.List(t.Context(), content.Filter{
			Type: "page", Search: listing.search, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
			Hierarchy: listing.nested, Page: 1, PerPage: 10,
		})
		if err != nil {
			t.Fatalf("List(nested %t, search %q) error = %v, want nil", listing.nested, listing.search, err)
		}
		parents := map[string]string{}
		for _, row := range rows {
			parents[row.Title] = row.ParentTitle
		}
		if !maps.Equal(parents, listing.want) {
			t.Errorf("nested %t search %q parents = %v, want %v, each item under its own parent's title",
				listing.nested, listing.search, parents, listing.want)
		}
	}
}

// listSortsTitlesCaseInsensitively sorts titles without regard to their case.
func listSortsTitlesCaseInsensitively(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "banana", author)
	MustCreate(t, s.Content, "Apricot", author)
	MustCreate(t, s.Content, "cherry", author)

	posts, _, err := s.Content.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc, Page: 1, PerPage: 10,
	})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	want := []string{"Apricot", "banana", "cherry"}
	got := TitlesOf(posts)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("titles = %v, want %v, so the database collation is not case insensitive", got, want)
		}
	}
}
