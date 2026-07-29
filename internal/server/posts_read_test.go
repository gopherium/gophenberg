// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/server"
)

type postBody struct {
	ID          uuid.UUID  `json:"id"`
	Type        string     `json:"type"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Excerpt     string     `json:"excerpt"`
	Content     string     `json:"content"`
	Status      string     `json:"status"`
	AuthorID    uuid.UUID  `json:"author_id"`
	AuthorName  string     `json:"author_name"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type postListBody struct {
	Items []postBody `json:"items"`
	Total int        `json:"total"`
}

func TestPostRoutesRequireASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(server.Config{Users: users, Posts: newFakePostStore()})

	for _, path := range []string{
		"/api/posts",
		"/api/posts/counts",
		"/api/posts/019f4a00-0000-7000-8000-000000000001",
	} {
		if code := doRequest(t, handler, http.MethodGet, path, "").Code; code != http.StatusUnauthorized {
			t.Errorf("GET %s = %d, want %d", path, code, http.StatusUnauthorized)
		}
	}
}

func TestPostListReturnsItemsWithAuthorNames(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := newPost(t, "Hello World", ada.ID)
	stored.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	posts.add(stored)

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[postListBody](t, recorder)
	if body.Total != 1 || len(body.Items) != 1 {
		t.Fatalf("body = %+v, want one item and a total of 1", body)
	}
	item := body.Items[0]
	if item.AuthorName != "Ada Lovelace" || item.AuthorID != ada.ID {
		t.Errorf("author = %q %v, want %q %v", item.AuthorName, item.AuthorID, "Ada Lovelace", ada.ID)
	}
	if item.Content != "" {
		t.Errorf("Content = %q, want listings to omit it", item.Content)
	}
	if item.Status != string(post.StatusDraft) || item.Slug != "hello-world" {
		t.Errorf("item = %+v, want a draft slugged hello-world", item)
	}
	if item.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want null on a draft", item.PublishedAt)
	}
}

func TestPostListReturnsAnEmptyArrayWithoutPosts(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); !strings.Contains(body, `"items":[]`) {
		t.Errorf("body = %s, want an empty items array", body)
	}
}

func TestPostListFiltersAndPaginates(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	draft := posts.add(newPost(t, "Draft One", ada.ID))
	published := newPost(t, "Published One", ada.ID)
	published.Status = post.StatusPublished
	posts.add(published)

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts?status=published", "")

	body := decodeBody[postListBody](t, recorder)
	if body.Total != 1 || len(body.Items) != 1 || body.Items[0].Title != "Published One" {
		t.Errorf("filtered body = %+v, want only the published post", body)
	}

	page := decodeBody[postListBody](t, doRequest(t, handler, http.MethodGet, "/api/posts?per_page=1&page=2", ""))

	if page.Total != 2 || len(page.Items) != 1 {
		t.Errorf("page two = %+v, want one of two items", page)
	}
	if page.Items[0].ID == draft.ID && body.Items[0].ID == draft.ID {
		t.Error("page two repeated the first page's item")
	}
}

func TestPostListSearchesTitles(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	posts.add(newPost(t, "Gutenberg Editor", ada.ID))
	posts.add(newPost(t, "Something Else", ada.ID))

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts?search=gutenberg", "")

	body := decodeBody[postListBody](t, recorder)
	if body.Total != 1 || body.Items[0].Title != "Gutenberg Editor" {
		t.Errorf("search body = %+v, want the matching post alone", body)
	}
}

func TestPostListRejectsInvalidParameters(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)

	for _, query := range []string{
		"?page=0",
		"?page=-1",
		"?page=many",
		"?per_page=0",
		"?per_page=101",
		"?per_page=lots",
		"?status=publsh",
	} {
		recorder := doRequest(t, handler, http.MethodGet, "/api/posts"+query, "")

		if recorder.Code != http.StatusBadRequest {
			t.Errorf("GET /api/posts%s = %d, want %d", query, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestPostListReportsStoreFailures(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)
	posts.listErr = context.DeadlineExceeded

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestPostListReportsAuthorLookupFailures(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(server.Config{Users: failingUserStore{Store: users}, Posts: newFakePostStore()})
	cookie := loginCookie(t, handler)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})

	recorder := doRequest(t, authed, http.MethodGet, "/api/posts", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestPostGetReturnsTheContent(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := newPost(t, "Hello World", ada.ID)
	stored.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	posts.add(stored)

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts/"+stored.ID.String(), "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[postBody](t, recorder)
	if body.Content != stored.Content {
		t.Errorf("Content = %q, want the stored content", body.Content)
	}
	if body.AuthorName != "Ada Lovelace" {
		t.Errorf("AuthorName = %q, want %q", body.AuthorName, "Ada Lovelace")
	}
}

func TestPostGetRejectsUnknownAndMalformedIDs(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)

	missing := doRequest(t, handler, http.MethodGet, "/api/posts/"+uuid.Must(uuid.NewV7()).String(), "")
	malformed := doRequest(t, handler, http.MethodGet, "/api/posts/not-a-uuid", "")

	if missing.Code != http.StatusNotFound {
		t.Errorf("missing post status = %d, want %d", missing.Code, http.StatusNotFound)
	}
	if malformed.Code != http.StatusBadRequest {
		t.Errorf("malformed id status = %d, want %d", malformed.Code, http.StatusBadRequest)
	}
}

func TestPostCountsReportsEveryStatus(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	posts.add(newPost(t, "Draft One", ada.ID))
	published := newPost(t, "Published One", ada.ID)
	published.Status = post.StatusPublished
	posts.add(published)

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts/counts", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	counts := decodeBody[map[string]int](t, recorder)
	want := map[string]int{"draft": 1, "pending": 0, "private": 0, "published": 1, "trash": 0}
	if len(counts) != len(want) {
		t.Fatalf("counts = %v, want the keys of %v", counts, want)
	}
	for status, total := range want {
		if counts[status] != total {
			t.Errorf("counts[%q] = %d, want %d", status, counts[status], total)
		}
	}
}

func TestPostCountsReportsStoreFailures(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)
	posts.countsErr = context.DeadlineExceeded

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts/counts", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
