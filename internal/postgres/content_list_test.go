// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"cmp"
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// publish stores a post already published at the given time.
func publish(t *testing.T, store *postgres.ContentStore, title string, author uuid.UUID, at time.Time) content.Content {
	t.Helper()
	created := mustCreate(t, store, title, author)
	edited := created
	edited.Status = content.StatusPublished
	edited.PublishedAt = &at
	edited.UpdatedAt = at
	updated, err := store.Update(t.Context(), edited, created.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("publishing %q: %v", title, err)
	}
	return updated
}

// titlesOf returns the titles of the listed posts in order.
func titlesOf(posts []content.ListedItem) []string {
	titles := make([]string, len(posts))
	for i, p := range posts {
		titles[i] = p.Title
	}
	return titles
}

func TestContentStoreListOrdersByPublicationThenCreation(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	publish(t, store, "Older Published", author, now.Add(-48*time.Hour))
	publish(t, store, "Newer Published", author, now.Add(-1*time.Hour))
	mustCreate(t, store, "Fresh Draft", author)

	posts, total, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
	want := []string{"Fresh Draft", "Newer Published", "Older Published"}
	got := titlesOf(posts)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order = %v, want %v", got, want)
		}
	}
}

func TestContentStoreListOmitsContent(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "With Body", author)
	edited := created
	edited.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	if _, err := store.Update(t.Context(), edited, created.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	posts, _, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if body := posts[0].Content.Content; body != "" {
		t.Errorf("Content = %q, want listings to omit it", body)
	}
}

func TestContentStoreListFiltersByStatus(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	publish(t, store, "Published One", author, time.Now().UTC().Truncate(time.Microsecond))
	mustCreate(t, store, "Draft One", author)

	posts, total, err := store.List(
		t.Context(),
		content.Filter{Type: content.TypePost, Statuses: []content.Status{content.StatusPublished}, Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 1 || len(posts) != 1 || posts[0].Title != "Published One" {
		t.Errorf("List() = %v with total %d, want only the published post", titlesOf(posts), total)
	}
}

func TestContentStoreListSearchesTitleAndContent(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	mustCreate(t, store, "Gutenberg Editor", author)
	withBody := mustCreate(t, store, "Unrelated Title", author)
	edited := withBody
	edited.Content = "a paragraph mentioning gutenberg inside"
	if _, err := store.Update(t.Context(), edited, withBody.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	mustCreate(t, store, "Something Else", author)

	posts, total, err := store.List(
		t.Context(),
		content.Filter{Type: content.TypePost, Search: "gutenberg", Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 2 || len(posts) != 2 {
		t.Errorf("List() = %v with total %d, want the two gutenberg matches", titlesOf(posts), total)
	}
}

func TestContentStoreListTreatsWildcardsAsLiterals(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	mustCreate(t, store, "100% Coverage", author)
	mustCreate(t, store, "Plain Title", author)

	posts, total, err := store.List(
		t.Context(),
		content.Filter{Type: content.TypePost, Search: "100%", Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 1 || len(posts) != 1 || posts[0].Title != "100% Coverage" {
		t.Errorf("List() = %v with total %d, want only the literal match", titlesOf(posts), total)
	}
}

func TestContentStoreListPaginates(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	publish(t, store, "First", author, now.Add(-3*time.Hour))
	publish(t, store, "Second", author, now.Add(-2*time.Hour))
	publish(t, store, "Third", author, now.Add(-1*time.Hour))

	second, total, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 2, PerPage: 2})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 under the same filter", total)
	}
	if len(second) != 1 || second[0].Title != "First" {
		t.Errorf("page 2 = %v, want the oldest post alone", titlesOf(second))
	}
}

func TestContentStoreListReturnsAnEmptyPagePastTheEnd(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	mustCreate(t, store, "Only One", author)

	posts, total, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 5, PerPage: 10})

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

func TestContentStoreCountsPerStatus(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	publish(t, store, "Published One", author, time.Now().UTC().Truncate(time.Microsecond))
	mustCreate(t, store, "Draft One", author)
	mustCreate(t, store, "Draft Two", author)

	counts, err := store.Counts(t.Context(), content.TypePost)

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

func TestContentStoreCountsReportsDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, _, pool := newContentStoreWithPool(t)
	pool.Close()

	_, err := store.Counts(t.Context(), content.TypePost)

	if err == nil {
		t.Error("Counts() on a closed pool error = nil, want a failure")
	}
}

func TestContentStoreListSortsByTitle(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	mustCreate(t, store, "Beta", author)
	mustCreate(t, store, "Alpha", author)
	mustCreate(t, store, "Gamma", author)

	ascending, _, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if got := titlesOf(ascending); got[0] != "Alpha" || got[2] != "Gamma" {
		t.Errorf("ascending = %v, want Alpha first and Gamma last", got)
	}

	descending, _, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderDesc, Page: 1, PerPage: 10,
	})
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if got := titlesOf(descending); got[0] != "Gamma" || got[2] != "Alpha" {
		t.Errorf("descending = %v, want Gamma first and Alpha last", got)
	}
}

func TestContentStoreListSortsByDateIndependentlyOfCreationOrder(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	publish(t, store, "Published Long Ago", author, now.Add(-72*time.Hour))
	publish(t, store, "Published Recently", author, now.Add(-1*time.Hour))

	oldestFirst, _, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByDate, Order: content.OrderAsc, Page: 1, PerPage: 10,
	})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if got := titlesOf(oldestFirst); got[0] != "Published Long Ago" {
		t.Errorf("ascending by date = %v, want the oldest publication first", got)
	}
}

func TestContentStoreListDefaultsToNewestPublicationFirst(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	publish(t, store, "Published Recently", author, now.Add(-1*time.Hour))
	publish(t, store, "Published Long Ago", author, now.Add(-72*time.Hour))

	posts, _, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if got := titlesOf(posts); got[0] != "Published Recently" {
		t.Errorf("default order = %v, want the newest publication first", got)
	}
}

// writer stores a second account, Another Writer, beside the store's own author and returns its id.
func writer(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(t.Context(),
		`INSERT INTO auth.users (id, email, name, password_hash, disabled, created_at)
		VALUES ($1, $2, 'Another Writer', 'hash', false, $3)`,
		id, id.String()+"@example.com", time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("inserting the second account: %v", err)
	}
	return id
}

// listed returns the titles and the total the store lists under the filter.
func listed(t *testing.T, store *postgres.ContentStore, f content.Filter) ([]string, int) {
	t.Helper()
	if f.Type == "" {
		f.Type = content.TypePost
	}
	f.Page, f.PerPage = cmp.Or(f.Page, 1), cmp.Or(f.PerPage, 10)
	rows, total, err := store.List(t.Context(), f)
	if err != nil {
		t.Fatalf("List(%+v) error = %v, want nil", f, err)
	}
	return titlesOf(rows), total
}

// trash sends a stored item to the trash.
func trash(t *testing.T, store *postgres.ContentStore, item content.Content) {
	t.Helper()
	if _, err := store.Trash(t.Context(), item.ID, time.Now().UTC()); err != nil {
		t.Fatalf("trashing %q: %v", item.Title, err)
	}
}

func TestContentStoreListNamesTheAccountThatWroteEachItem(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	other := writer(t, pool)
	mustCreate(t, store, "By Maria", author)
	mustCreate(t, store, "By Another Writer", other)

	rows, _, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Page: 1, PerPage: 10})

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

func TestContentStoreListKeepsOnlyTheStatusesItIsGiven(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	publish(t, store, "Published", author, time.Now().UTC().Truncate(time.Microsecond))
	mustCreate(t, store, "Draft", author)
	trash(t, store, mustCreate(t, store, "Trashed", author))
	pending := mustCreate(t, store, "Pending", author)
	pending.Status = content.StatusPending
	if _, err := store.Update(t.Context(), pending, pending.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	titles, total := listed(t, store, content.Filter{
		Statuses: []content.Status{content.StatusDraft, content.StatusPublished},
	})

	slices.Sort(titles)
	if total != 2 || !slices.Equal(titles, []string{"Draft", "Published"}) {
		t.Errorf("listed %v of %d, want the draft and the published post alone", titles, total)
	}
}

func TestContentStoreListKeepsEveryStatusWhenItNamesNone(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	mustCreate(t, store, "Draft", author)
	trash(t, store, mustCreate(t, store, "Trashed", author))

	titles, total := listed(t, store, content.Filter{})

	if total != 2 || len(titles) != 2 {
		t.Errorf("listed %v of %d, want every status kept, the trash included", titles, total)
	}
}

func TestContentStoreListKeepsOnlyTheAuthorsItIsGiven(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	other := writer(t, pool)
	mustCreate(t, store, "By Maria", author)
	mustCreate(t, store, "By Another Writer", other)

	kept, keptTotal := listed(t, store, content.Filter{Authors: []uuid.UUID{other}})
	left, leftTotal := listed(t, store, content.Filter{ExcludeAuthors: []uuid.UUID{other}})

	if keptTotal != 1 || !slices.Equal(kept, []string{"By Another Writer"}) {
		t.Errorf("kept %v of %d, want only the item the named author wrote", kept, keptTotal)
	}
	if leftTotal != 1 || !slices.Equal(left, []string{"By Maria"}) {
		t.Errorf("left %v of %d, want every item but the excluded author's", left, leftTotal)
	}
}

func TestContentStoreListKeepsTheItemsDatedBeforeAndAfterADay(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	march := time.Date(2026, time.March, 10, 9, 0, 0, 0, time.UTC)
	july := time.Date(2026, time.July, 10, 9, 0, 0, 0, time.UTC)
	publish(t, store, "Spring", author, march)
	publish(t, store, "Summer", author, july)
	mustCreate(t, store, "Fresh Draft", author)
	june := time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC)

	before, beforeTotal := listed(t, store, content.Filter{Before: &june})
	after, afterTotal := listed(t, store, content.Filter{After: &june})
	exactly, _ := listed(t, store, content.Filter{Before: &march})

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

func TestContentStoreListDatesAnUnpublishedItemByItsLastChange(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	first := mustCreate(t, store, "Written First", author)
	mustCreate(t, store, "Written Second", author)
	edited := first
	edited.Title, edited.UpdatedAt = "Written First, Changed Last", time.Now().UTC().Add(time.Minute)
	if _, err := store.Update(t.Context(), edited, first.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	titles, _ := listed(t, store, content.Filter{OrderBy: content.OrderByDate, Order: content.OrderDesc})

	if !slices.Equal(titles, []string{"Written First, Changed Last", "Written Second"}) {
		t.Errorf("newest first = %v, want the draft changed last on top", titles)
	}
}

func TestContentStoreListSortsByTheNameOfTheAuthor(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	other := writer(t, pool)
	mustCreate(t, store, "By Maria", author)
	mustCreate(t, store, "By Another Writer", other)

	ascending, _ := listed(t, store, content.Filter{OrderBy: content.OrderByAuthor, Order: content.OrderAsc})
	descending, _ := listed(t, store, content.Filter{OrderBy: content.OrderByAuthor, Order: content.OrderDesc})

	if !slices.Equal(ascending, []string{"By Another Writer", "By Maria"}) {
		t.Errorf("ascending = %v, want Another Writer's item first", ascending)
	}
	if !slices.Equal(descending, []string{"By Maria", "By Another Writer"}) {
		t.Errorf("descending = %v, want Maria Perez's item first", descending)
	}
}

func TestContentStoreListSortsBySlug(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	mustCreate(t, store, "Banana Bread", author)
	mustCreate(t, store, "Apple Pie", author)
	mustCreate(t, store, "Cherry Tart", author)

	ascending, _ := listed(t, store, content.Filter{OrderBy: content.OrderBySlug, Order: content.OrderAsc})
	descending, _ := listed(t, store, content.Filter{OrderBy: content.OrderBySlug, Order: content.OrderDesc})

	if !slices.Equal(ascending, []string{"Apple Pie", "Banana Bread", "Cherry Tart"}) {
		t.Errorf("ascending = %v, want the slugs in order", ascending)
	}
	if !slices.Equal(descending, []string{"Cherry Tart", "Banana Bread", "Apple Pie"}) {
		t.Errorf("descending = %v, want the slugs in reverse", descending)
	}
}

func TestContentStoreListSortsByWhenTheParentWasWritten(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	contact := mustNest(t, store, nil, "Contact", author)
	mustNest(t, store, &about, "Team", author)
	mustNest(t, store, &contact, "Directions", author)

	ascending, _ := listed(t, store, content.Filter{
		Type: "page", OrderBy: content.OrderByParent, Order: content.OrderAsc,
	})
	descending, _ := listed(t, store, content.Filter{
		Type: "page", OrderBy: content.OrderByParent, Order: content.OrderDesc,
	})

	if !slices.Equal(ascending[2:], []string{"Team", "Directions"}) {
		t.Errorf("ascending = %v, want the items with no parent first, then by the older parent", ascending)
	}
	if !slices.Equal(descending[:2], []string{"Directions", "Team"}) {
		t.Errorf("descending = %v, want the newer parent's child first and the items with no parent last", descending)
	}
}

func TestContentStoreListNestsEachChildUnderItsParent(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	mustNest(t, store, nil, "Contact", author)
	mustNest(t, store, &about, "Team", author)
	mustNest(t, store, &about, "History", author)
	nested := content.Filter{Type: "page", OrderBy: content.OrderByTitle, Order: content.OrderAsc, Hierarchy: true}

	titles, total := listed(t, store, nested)
	flat, _ := listed(t, store, content.Filter{Type: "page", OrderBy: content.OrderByTitle, Order: content.OrderAsc})
	nested.Page, nested.PerPage = 2, 2
	second, _ := listed(t, store, nested)

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

func TestContentStoreListNestsAChildWhoseParentIsLeftOutAfterTheRest(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	for _, item := range []content.Content{
		mustNest(t, store, &about, "Team", author),
		mustNest(t, store, nil, "Contact", author),
		mustNest(t, store, nil, "Zoo", author),
	} {
		edited := item
		at := time.Now().UTC()
		edited.Status, edited.PublishedAt, edited.UpdatedAt = content.StatusPublished, &at, at
		if _, err := store.Update(t.Context(), edited, item.UpdatedAt, nil, 0); err != nil {
			t.Fatalf("publishing %q: %v", item.Title, err)
		}
	}

	titles, _ := listed(t, store, content.Filter{
		Type: "page", Statuses: []content.Status{content.StatusPublished},
		OrderBy: content.OrderByTitle, Order: content.OrderAsc, Hierarchy: true,
	})

	if !slices.Equal(titles, []string{"Contact", "Zoo", "Team"}) {
		t.Errorf("nested = %v, want the child of a parent left out after every item whose parent is kept", titles)
	}
}

func TestContentStoreListCarriesTheTitleOfEachParent(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	about := mustNest(t, store, nil, "About", author)
	team := mustNest(t, store, &about, "Team", author)
	mustNest(t, store, &team, "Volunteers", author)
	untitled := mustNest(t, store, nil, "", author)
	mustNest(t, store, &untitled, "Donations", author)
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
		rows, _, err := store.List(t.Context(), content.Filter{
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

func TestContentStoreListReportsARejectedNestedQuery(t *testing.T) {
	t.Parallel()

	store, _ := newContentStore(t)

	_, _, err := store.List(t.Context(), content.Filter{Type: content.TypePost, Hierarchy: true, Page: 1, PerPage: -1})

	if err == nil {
		t.Error("List() nesting with a negative page size error = nil, want a failure")
	}
}

func TestContentStoreListSortsTitlesCaseInsensitively(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	mustCreate(t, store, "banana", author)
	mustCreate(t, store, "Apricot", author)
	mustCreate(t, store, "cherry", author)

	posts, _, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc, Page: 1, PerPage: 10,
	})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	want := []string{"Apricot", "banana", "cherry"}
	got := titlesOf(posts)
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("titles = %v, want %v, so the database collation is not case insensitive", got, want)
		}
	}
}
