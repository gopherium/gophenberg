// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
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
		Path:        slug,
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
	if !strings.Contains(body, `"api":0`) {
		t.Errorf("body = %q, want the api generation", body)
	}
	if !strings.Contains(body, `"kit":["0.9.0","0.10.0","0.11.0","0.12.0"]`) {
		t.Errorf("body = %q, want the kit versions served", body)
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

	recorder := getContent(t, handler, "/api/content/v1/items")

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

	recorder := getContent(t, handler, "/api/content/v1/items?per_page=500")

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
		"/api/content/v1/items?page=nonsense",
		"/api/content/v1/items?page=0",
		"/api/content/v1/items?per_page=0",
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

	recorder := getContent(t, handler, "/api/content/v1/items?page=99")

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

	recorder := getContent(t, handler, "/api/content/v1/items?type=nonexistent")

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

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/hello-world")

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
		"/api/content/v1/resolve?path=/a-draft",
		"/api/content/v1/resolve?path=/never-written",
		"/api/content/v1/resolve?path=/pages/a-draft",
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

	first := getContent(t, handler, "/api/content/v1/resolve?path=/hello-world")
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("ETag = empty, want a validator derived from the post")
	}

	request := httptest.NewRequest(http.MethodGet, "/api/content/v1/resolve?path=/hello-world", nil)
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
		"/api/content/v1/items",
		"/api/content/v1/resolve?path=/hello-world",
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
		"/api/content/v1/items",
		"/api/content/v1/resolve?path=/hello-world",
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

	for _, path := range []string{"/api/content/v1", "/api/content/v1/items"} {
		recorder := getContent(t, handler, path)
		if got := recorder.Header().Get("X-Generator"); got != "" {
			t.Errorf("GET %s carries X-Generator %q, want the api scope free of it", path, got)
		}
		if got := recorder.Header().Get("Link"); got != "" {
			t.Errorf("GET %s carries Link %q, want the api scope free of it", path, got)
		}
	}
}

func TestContentAPIAdvertisesTheTypesItServes(t *testing.T) {
	t.Parallel()

	handler := contentServer(t)

	recorder := getContent(t, handler, "/api/content/v1")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var handshake struct {
		API   int      `json:"api"`
		Kit   []string `json:"kit"`
		Types []struct {
			Key          string `json:"key"`
			SingularName string `json:"singular_label"`
			PluralName   string `json:"plural_label"`
			RouteWord    string `json:"route_word"`
			Hierarchical bool   `json:"hierarchical"`
			PageKind     string `json:"page_kind"`
			Default      bool   `json:"default"`
		} `json:"types"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &handshake); err != nil {
		t.Fatalf("reading the handshake: %v", err)
	}
	if handshake.API != 0 {
		t.Errorf("api = %d, want 0", handshake.API)
	}
	if !slices.Equal(handshake.Kit, []string{"0.9.0", "0.10.0", "0.11.0", "0.12.0"}) {
		t.Errorf("kit = %v, want the kit versions this release serves", handshake.Kit)
	}
	if len(handshake.Types) != 1 {
		t.Fatalf("types = %d, want the one registered type", len(handshake.Types))
	}
	listed := handshake.Types[0]
	if listed.Key != content.TypePost || !listed.Default || listed.RouteWord != "" {
		t.Errorf("type = %+v, want the default post type at the root", listed)
	}
	if listed.PluralName != "Posts" || listed.PageKind != string(content.PageKindSingle) {
		t.Errorf("type = %+v, want its labels and page kind", listed)
	}
}

func TestContentAPIListsPublishedItemsUnderItems(t *testing.T) {
	t.Parallel()

	handler := contentServer(t, publishedFixture(t, "hello-world", blockMarkup, time.Now().UTC()))

	recorder := getContent(t, handler, "/api/content/v1/items")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), "hello-world") {
		t.Errorf("body = %q, want the published item", recorder.Body.String())
	}
}

func TestContentAPIResolvesAnAddressToItsItem(t *testing.T) {
	t.Parallel()

	handler := contentServer(t, publishedFixture(t, "hello-world", blockMarkup, time.Now().UTC()))

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/hello-world")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var answer struct {
		Kind string `json:"kind"`
		Type struct {
			Key string `json:"key"`
		} `json:"type"`
		Item struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		} `json:"item"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	if answer.Kind != "item" {
		t.Errorf("kind = %q, want %q", answer.Kind, "item")
	}
	if answer.Type.Key != content.TypePost {
		t.Errorf("type = %q, want the item's own type", answer.Type.Key)
	}
	if answer.Item.Path != "hello-world" || !strings.Contains(answer.Item.Content, "Body") {
		t.Errorf("item = %+v, want the addressed item with its content", answer.Item)
	}
}

func TestContentAPIResolvesAnArchiveToItsPage(t *testing.T) {
	t.Parallel()

	handler := contentServer(t, publishedFixture(t, "hello-world", blockMarkup, time.Now().UTC()))

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var answer struct {
		Kind string `json:"kind"`
		Type struct {
			Key string `json:"key"`
		} `json:"type"`
		Page struct {
			Items []struct {
				Path string `json:"path"`
			} `json:"items"`
			Total int `json:"total"`
			Page  int `json:"page"`
		} `json:"page"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	if answer.Kind != "archive" || answer.Type.Key != content.TypePost {
		t.Errorf("answer = %q %q, want the default archive", answer.Kind, answer.Type.Key)
	}
	if answer.Page.Total != 1 || answer.Page.Page != 1 || len(answer.Page.Items) != 1 {
		t.Errorf("page = %+v, want the first page of the archive", answer.Page)
	}
}

func TestContentAPIResolvesAnUnknownAddressToNotFound(t *testing.T) {
	t.Parallel()

	handler := contentServer(t)

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/nowhere")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestContentAPIResolveCarriesTheItemValidator(t *testing.T) {
	t.Parallel()

	handler := contentServer(t, publishedFixture(t, "hello-world", blockMarkup, time.Now().UTC()))

	first := getContent(t, handler, "/api/content/v1/resolve?path=/hello-world")
	etag := first.Header().Get("ETag")
	request := httptest.NewRequest(http.MethodGet, "/api/content/v1/resolve?path=/hello-world", nil)
	request.Header.Set("If-None-Match", etag)
	again := httptest.NewRecorder()
	handler.ServeHTTP(again, request)

	if etag == "" {
		t.Fatal("the resolved item carries no ETag")
	}
	if again.Code != http.StatusNotModified {
		t.Errorf("status = %d, want %d for an unchanged item", again.Code, http.StatusNotModified)
	}
}

func TestContentAPIResolvesAnArchivePageFromItsAddress(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	handler := contentServer(t,
		publishedFixture(t, "first", blockMarkup, now),
		publishedFixture(t, "second", blockMarkup, now.Add(-time.Hour)),
	)

	behind := resolvedPage(t, handler, "/api/content/v1/resolve?path=/page/2&per_page=1")
	front := resolvedPage(t, handler, "/api/content/v1/resolve?path=/&per_page=1")

	if behind.Page != 2 {
		t.Errorf("page = %d, want the number the address carries", behind.Page)
	}
	if len(behind.Items) != 1 || len(front.Items) != 1 {
		t.Fatalf("pages hold %d and %d items, want one each", len(front.Items), len(behind.Items))
	}
	if behind.Items[0].Path == front.Items[0].Path {
		t.Errorf("both pages carry %q, want the address to move the window", behind.Items[0].Path)
	}
	if behind.Total != 2 {
		t.Errorf("total = %d, want every published item counted", behind.Total)
	}
}

// resolvedPage returns the archive page the resolver answers with at address.
func resolvedPage(t *testing.T, handler http.Handler, address string) struct {
	Items []struct {
		Path string `json:"path"`
	} `json:"items"`
	Total int `json:"total"`
	Page  int `json:"page"`
} {
	t.Helper()
	recorder := getContent(t, handler, address)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", address, recorder.Code, http.StatusOK)
	}
	var answer struct {
		Page struct {
			Items []struct {
				Path string `json:"path"`
			} `json:"items"`
			Total int `json:"total"`
			Page  int `json:"page"`
		} `json:"page"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answer); err != nil {
		t.Fatalf("reading the answer to %s: %v", address, err)
	}
	return answer.Page
}

func TestContentAPIMasksAFailedRegistryRead(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	types := newFakeTypeStore()
	types.listErr = errors.New("the registry is unreachable")
	handler := server.NewServer(server.Config{Users: users, Content: newFakePostStore(), Types: types})

	for _, address := range []string{"/api/content/v1", "/api/content/v1/resolve?path=/"} {
		recorder := getContent(t, handler, address)

		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("GET %s status = %d, want %d", address, recorder.Code, http.StatusInternalServerError)
		}
	}
}

func TestContentAPIMasksAFailedArchiveRead(t *testing.T) {
	t.Parallel()

	posts := newFakePostStore()
	posts.listErr = errors.New("the store is unreachable")
	handler := serverWithStores(newFakeUserStore(), posts)

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestContentAPIRefusesMalformedArchivePaging(t *testing.T) {
	t.Parallel()

	handler := contentServer(t)

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/&per_page=0")

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestContentAPIRefusesAnItemOfAnUnregisteredType(t *testing.T) {
	t.Parallel()

	orphan := publishedFixture(t, "orphan", blockMarkup, time.Now().UTC())
	orphan.Type = "ghost"
	handler := contentServer(t, orphan)

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/orphan")

	if recorder.Code == http.StatusOK {
		t.Errorf("status = %d, want the reader refused an item of a type the registry does not hold", recorder.Code)
	}
}

func TestContentAPIListsTheRegistryDefaultRatherThanTheBuiltInType(t *testing.T) {
	t.Parallel()

	types := newFakeTypeStore()
	moved := postType()
	moved.Default = false
	guides := postType()
	guides.Key, guides.PluralLabel, guides.Default = "guide", "Guides", true
	types.types = []content.Type{moved, guides}
	guide := publishedFixture(t, "a-guide", blockMarkup, time.Now().UTC())
	guide.Type = "guide"
	posts := newFakePostStore()
	posts.add(guide)
	posts.add(publishedFixture(t, "a-post", blockMarkup, time.Now().UTC()))
	handler := server.NewServer(server.Config{Users: newFakeUserStore(), Content: posts, Types: types})

	recorder := getContent(t, handler, "/api/content/v1/items")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "a-guide") || strings.Contains(body, "a-post") {
		t.Errorf("body = %q, want the type the registry marks default", body)
	}
}
