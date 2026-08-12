// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

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
func titlesOf(posts []content.Content) []string {
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
	if posts[0].Content != "" {
		t.Errorf("Content = %q, want listings to omit it", posts[0].Content)
	}
}

func TestContentStoreListFiltersByStatus(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	publish(t, store, "Published One", author, time.Now().UTC().Truncate(time.Microsecond))
	mustCreate(t, store, "Draft One", author)

	posts, total, err := store.List(
		t.Context(),
		content.Filter{Type: content.TypePost, Status: content.StatusPublished, Page: 1, PerPage: 10},
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
