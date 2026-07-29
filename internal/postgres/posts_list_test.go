// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// publish stores a post already published at the given time.
func publish(t *testing.T, store *postgres.PostStore, title string, author uuid.UUID, at time.Time) post.Post {
	t.Helper()
	created := mustCreate(t, store, title, author)
	edited := created
	edited.Status = post.StatusPublished
	edited.PublishedAt = &at
	edited.UpdatedAt = at
	updated, err := store.Update(t.Context(), edited, created.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("publishing %q: %v", title, err)
	}
	return updated
}

// titlesOf returns the titles of the listed posts in order.
func titlesOf(posts []post.Post) []string {
	titles := make([]string, len(posts))
	for i, p := range posts {
		titles[i] = p.Title
	}
	return titles
}

func TestPostStoreListOrdersByPublicationThenCreation(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	publish(t, store, "Older Published", author, now.Add(-48*time.Hour))
	publish(t, store, "Newer Published", author, now.Add(-1*time.Hour))
	mustCreate(t, store, "Fresh Draft", author)

	posts, total, err := store.List(t.Context(), post.Filter{Type: post.TypePost, Page: 1, PerPage: 10})

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

func TestPostStoreListOmitsContent(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "With Body", author)
	edited := created
	edited.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	if _, err := store.Update(t.Context(), edited, created.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	posts, _, err := store.List(t.Context(), post.Filter{Type: post.TypePost, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if posts[0].Content != "" {
		t.Errorf("Content = %q, want listings to omit it", posts[0].Content)
	}
}

func TestPostStoreListFiltersByStatus(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	publish(t, store, "Published One", author, time.Now().UTC().Truncate(time.Microsecond))
	mustCreate(t, store, "Draft One", author)

	posts, total, err := store.List(
		t.Context(),
		post.Filter{Type: post.TypePost, Status: post.StatusPublished, Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 1 || len(posts) != 1 || posts[0].Title != "Published One" {
		t.Errorf("List() = %v with total %d, want only the published post", titlesOf(posts), total)
	}
}

func TestPostStoreListSearchesTitleAndContent(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
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
		post.Filter{Type: post.TypePost, Search: "gutenberg", Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 2 || len(posts) != 2 {
		t.Errorf("List() = %v with total %d, want the two gutenberg matches", titlesOf(posts), total)
	}
}

func TestPostStoreListTreatsWildcardsAsLiterals(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	mustCreate(t, store, "100% Coverage", author)
	mustCreate(t, store, "Plain Title", author)

	posts, total, err := store.List(
		t.Context(),
		post.Filter{Type: post.TypePost, Search: "100%", Page: 1, PerPage: 10},
	)

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 1 || len(posts) != 1 || posts[0].Title != "100% Coverage" {
		t.Errorf("List() = %v with total %d, want only the literal match", titlesOf(posts), total)
	}
}

func TestPostStoreListPaginates(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	now := time.Now().UTC().Truncate(time.Microsecond)
	publish(t, store, "First", author, now.Add(-3*time.Hour))
	publish(t, store, "Second", author, now.Add(-2*time.Hour))
	publish(t, store, "Third", author, now.Add(-1*time.Hour))

	second, total, err := store.List(t.Context(), post.Filter{Type: post.TypePost, Page: 2, PerPage: 2})

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

func TestPostStoreListReturnsAnEmptyPagePastTheEnd(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	mustCreate(t, store, "Only One", author)

	posts, total, err := store.List(t.Context(), post.Filter{Type: post.TypePost, Page: 5, PerPage: 10})

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

func TestPostStoreCountsPerStatus(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	publish(t, store, "Published One", author, time.Now().UTC().Truncate(time.Microsecond))
	mustCreate(t, store, "Draft One", author)
	mustCreate(t, store, "Draft Two", author)

	counts, err := store.Counts(t.Context(), post.TypePost)

	if err != nil {
		t.Fatalf("Counts() error = %v, want nil", err)
	}
	if counts[post.StatusDraft] != 2 || counts[post.StatusPublished] != 1 {
		t.Errorf("counts = %v, want two drafts and one published", counts)
	}
	if _, ok := counts[post.StatusTrash]; ok {
		t.Errorf("counts = %v, want absent statuses omitted by the store", counts)
	}
}

func TestPostStoreCountsReportsDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, _, pool := newPostStoreWithPool(t)
	pool.Close()

	_, err := store.Counts(t.Context(), post.TypePost)

	if err == nil {
		t.Error("Counts() on a closed pool error = nil, want a failure")
	}
}
