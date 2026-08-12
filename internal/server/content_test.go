// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// blockMarkup is the serialized block content a published fixture carries.
const blockMarkup = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"

// publishedFixture returns a published post carrying the given slug and body.
func publishedFixture(t *testing.T, slug, body string, published time.Time) content.Content {
	t.Helper()
	return content.Content{
		ID:          uuid.Must(uuid.NewV7()),
		Type:        content.TypePost,
		Slug:        slug,
		Title:       "A Published Post",
		Excerpt:     "An excerpt.",
		Content:     body,
		Status:      content.StatusPublished,
		AuthorID:    uuid.Must(uuid.NewV7()),
		PublishedAt: &published,
		CreatedAt:   published,
		UpdatedAt:   published,
	}
}

// contentServer returns an unauthenticated handler over a store carrying the given posts.
func contentServer(t *testing.T, posts ...content.Content) http.Handler {
	t.Helper()
	store := newFakePostStore()
	for _, p := range posts {
		store.add(p)
	}
	return serverWithStores(newFakeUserStore(), store)
}

// getContent returns the response to an unauthenticated GET of path.
func getContent(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestContentAPIAnswersTheHandshakeWithoutASession(t *testing.T) {
	t.Parallel()

	handler := contentServer(t)

	recorder := getContent(t, handler, "/api/content/v1")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"api":1`) {
		t.Errorf("body = %q, want the api version", body)
	}
	if !strings.Contains(body, `"gophenberg"`) {
		t.Errorf("body = %q, want the product version", body)
	}
}

func TestContentAPIListsPublishedSummariesNewestFirst(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	older := publishedFixture(t, "older", blockMarkup, now.Add(-time.Hour))
	newer := publishedFixture(t, "newer", blockMarkup, now)
	draft := publishedFixture(t, "a-draft", blockMarkup, now)
	draft.Status = content.StatusDraft
	handler := contentServer(t, older, newer, draft)

	recorder := getContent(t, handler, "/api/content/v1/posts")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "a-draft") {
		t.Errorf("body = %q, want the draft hidden", body)
	}
	if strings.Index(body, "newer") > strings.Index(body, "older") {
		t.Errorf("body = %q, want the newest post first", body)
	}
	if strings.Contains(body, `"content"`) {
		t.Errorf("body = %q, want summaries to carry no content", body)
	}
	if !strings.Contains(body, `"total":2`) {
		t.Errorf("body = %q, want the total of published posts", body)
	}
	if !strings.Contains(body, `"page":1`) || !strings.Contains(body, `"per_page":20`) {
		t.Errorf("body = %q, want the effective paging reported", body)
	}
}

func TestContentAPICapsThePageSize(t *testing.T) {
	t.Parallel()

	handler := contentServer(t)

	recorder := getContent(t, handler, "/api/content/v1/posts?per_page=500")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"per_page":100`) {
		t.Errorf("body = %q, want the page size capped at 100", recorder.Body.String())
	}
}

func TestContentAPIRejectsMalformedPaging(t *testing.T) {
	t.Parallel()

	handler := contentServer(t)

	for _, path := range []string{
		"/api/content/v1/posts?page=nonsense",
		"/api/content/v1/posts?page=0",
		"/api/content/v1/posts?per_page=0",
	} {
		if recorder := getContent(t, handler, path); recorder.Code != http.StatusBadRequest {
			t.Errorf("GET %s status = %d, want %d", path, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestContentAPIServesAPageBeyondTheLastAsEmpty(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := contentServer(t, publishedFixture(t, "only", blockMarkup, now))

	recorder := getContent(t, handler, "/api/content/v1/posts?page=99")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"items":[]`) {
		t.Errorf("body = %q, want an empty page", body)
	}
	if !strings.Contains(body, `"total":1`) {
		t.Errorf("body = %q, want the total kept", body)
	}
}

func TestContentAPIServesAnUnknownTypeAsAnEmptyPage(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := contentServer(t, publishedFixture(t, "only", blockMarkup, now))

	recorder := getContent(t, handler, "/api/content/v1/posts?type=nonexistent")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"items":[]`) {
		t.Errorf("body = %q, want an empty page for an unknown type", recorder.Body.String())
	}
}

func TestContentAPIServesOnePostWithSanitizedContent(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	hostile := `<!-- wp:paragraph --><p onclick="steal()">Body</p><script>alert(1)</script><!-- /wp:paragraph -->`
	handler := contentServer(t, publishedFixture(t, "hello-world", hostile, now))

	recorder := getContent(t, handler, "/api/content/v1/posts/post/hello-world")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "onclick") || strings.Contains(body, "alert(1)") {
		t.Errorf("body = %q, want the scriptable markup stripped", body)
	}
	if !strings.Contains(body, `wp:paragraph`) {
		t.Errorf("body = %q, want the block delimiters the parser needs", body)
	}
}

func TestContentAPIHidesPostsThatAreNotPublished(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	draft := publishedFixture(t, "a-draft", blockMarkup, now)
	draft.Status = content.StatusDraft
	handler := contentServer(t, draft)

	for _, path := range []string{
		"/api/content/v1/posts/post/a-draft",
		"/api/content/v1/posts/post/never-written",
		"/api/content/v1/posts/page/a-draft",
	} {
		recorder := getContent(t, handler, path)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, recorder.Code, http.StatusNotFound)
		}
		if strings.Contains(recorder.Body.String(), "Body") {
			t.Errorf("GET %s body = %q, want no content", path, recorder.Body.String())
		}
	}
}

func TestContentAPIAnswersARepeatedReadWithNotModified(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := contentServer(t, publishedFixture(t, "hello-world", blockMarkup, now))

	first := getContent(t, handler, "/api/content/v1/posts/post/hello-world")
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag = empty, want a validator derived from the post")
	}

	request := httptest.NewRequest(http.MethodGet, "/api/content/v1/posts/post/hello-world", nil)
	request.Header.Set("If-None-Match", etag)
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, request)

	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", second.Code, http.StatusNotModified)
	}
	if second.Body.Len() != 0 {
		t.Errorf("body = %q, want it empty on a 304", second.Body.String())
	}
	if got := second.Header().Get("ETag"); got != etag {
		t.Errorf("ETag = %q, want %q kept on the 304", got, etag)
	}
}

func TestContentAPIMasksAStoreFailure(t *testing.T) {
	t.Parallel()

	store := newFakePostStore()
	store.listErr = errors.New("database down")
	store.publishedErr = errors.New("database down")
	handler := serverWithStores(newFakeUserStore(), store)

	for _, path := range []string{
		"/api/content/v1/posts",
		"/api/content/v1/posts/post/hello-world",
	} {
		recorder := getContent(t, handler, path)
		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("GET %s status = %d, want %d", path, recorder.Code, http.StatusInternalServerError)
		}
		if strings.Contains(recorder.Body.String(), "database down") {
			t.Errorf("GET %s body = %q, want the internal error masked", path, recorder.Body.String())
		}
	}
}

func TestContentAPICarriesTheHeadersAReaderNeeds(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := contentServer(t, publishedFixture(t, "hello-world", blockMarkup, now))

	for _, path := range []string{
		"/api/content/v1",
		"/api/content/v1/posts",
		"/api/content/v1/posts/post/hello-world",
	} {
		recorder := getContent(t, handler, path)
		if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("GET %s allowed origin = %q, want %q", path, got, "*")
		}
		if got := recorder.Header().Get("Cache-Control"); !strings.Contains(got, "s-maxage=60") {
			t.Errorf("GET %s cache control = %q, want it cacheable", path, got)
		}
	}
}

func TestContentAPICarriesNoFrontEndIdentification(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := contentServer(t, publishedFixture(t, "hello-world", blockMarkup, now))

	for _, path := range []string{"/api/content/v1", "/api/content/v1/posts"} {
		recorder := getContent(t, handler, path)
		if got := recorder.Header().Get("X-Generator"); got != "" {
			t.Errorf("GET %s carries X-Generator %q, want the api scope free of it", path, got)
		}
		if got := recorder.Header().Get("Link"); got != "" {
			t.Errorf("GET %s carries Link %q, want the api scope free of it", path, got)
		}
	}
}
