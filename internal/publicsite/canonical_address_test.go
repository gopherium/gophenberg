// SPDX-License-Identifier: Apache-2.0

package publicsite_test

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

// storedAt returns a published item of the type stored at the path.
func storedAt(kind, title, path string) content.Content {
	stored := publishedPost(title, path, blockMarkup, time.Now().UTC())
	stored.Type = kind
	return stored
}

func TestSiteSendsAListingPageToTheOneAddressItAnswersAt(t *testing.T) {
	t.Parallel()

	news := content.Content{
		ID: uuid.Must(uuid.NewV7()), Type: "category", Status: content.StatusPublished,
		Path: "categories/news", Slug: "news", Title: "News", Content: blockMarkup,
	}
	handler, _ := siteWith(append(manyPosts(25), news)...)

	for _, tc := range []struct {
		requested string
		want      string
	}{
		{"/page/1", "/"},
		{"/page/01", "/"},
		{"/page/02", "/page/2"},
		{"/page/+2", "/page/2"},
		{"/page/1?utm_source=feed", "/?utm_source=feed"},
		{"/page/02?utm_source=feed", "/page/2?utm_source=feed"},
		{"/pages/page/1", "/pages"},
		{"/pages/", "/pages"},
		{"/pages/page/2/", "/pages/page/2"},
		{"/page/2/", "/page/2"},
		{"/categories/news/", "/categories/news"},
		{"/pages/page/007", "/pages/page/7"},
		{"/categories/news/page/1", "/categories/news"},
		{"/categories/news/page/03", "/categories/news/page/3"},
	} {
		recorder := get(t, handler, tc.requested)

		if recorder.Code != http.StatusMovedPermanently {
			t.Errorf("GET %s status = %d, want %d", tc.requested, recorder.Code, http.StatusMovedPermanently)
		}
		if got := recorder.Header().Get("Location"); got != tc.want {
			t.Errorf("GET %s Location = %q, want %q", tc.requested, got, tc.want)
		}
	}
}

func TestSiteKeepsTheAddressesAListingAlreadyAnswersAt(t *testing.T) {
	t.Parallel()

	news := content.Content{
		ID: uuid.Must(uuid.NewV7()), Type: "category", Status: content.StatusPublished,
		Path: "categories/news", Slug: "news", Title: "News", Content: blockMarkup,
	}
	nested := publishedPost("One", "1", blockMarkup, time.Now().UTC())
	nested.Type, nested.Path = "page", "pages/page/1"
	handler, _ := siteWith(append(manyPosts(25), news, nested)...)

	for _, address := range []string{
		"/", "/page/2", "/pages", "/pages/page/2", "/pages/page/1",
		"/categories/news", "/categories/news/page/2",
	} {
		recorder := get(t, handler, address)

		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", address, recorder.Code, http.StatusOK)
		}
	}
}

func TestSiteKeepsAListingPageWhoseOneAddressHoldsContent(t *testing.T) {
	t.Parallel()

	two := storedAt("page", "Two", "pages/page/2")
	guide := storedAt(content.TypePost, "Guide", "pages")
	news := storedAt("category", "News", "categories/news")
	three := storedAt("category", "Three", "categories/news/page/3")

	for _, tc := range []struct {
		stored    []content.Content
		requested string
		want      string
	}{
		{[]content.Content{two}, "/pages/page/02", "Pages | A Test Site"},
		{[]content.Content{two}, "/pages/page/+2", "Pages | A Test Site"},
		{[]content.Content{guide}, "/pages/page/1", "Pages | A Test Site"},
		{[]content.Content{guide}, "/pages/page/01", "Pages | A Test Site"},
		{[]content.Content{news, three}, "/categories/news/page/03", "News | A Test Site"},
	} {
		handler, _ := siteWith(tc.stored...)

		recorder := get(t, handler, tc.requested)

		if recorder.Code != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", tc.requested, recorder.Code, http.StatusOK)
		}
		if body := recorder.Body.String(); !strings.Contains(body, "<title>"+tc.want+"</title>") {
			t.Errorf("GET %s body = %q, want the listing titled %q", tc.requested, body, tc.want)
		}
	}
}

func TestSiteServesTheListingWhenItCannotCheckTheOneAddress(t *testing.T) {
	t.Parallel()

	handler, reader := siteWith(manyPosts(25)...)
	reader.readErr = errors.New("database down")

	recorder := get(t, handler, "/page/1")

	if recorder.Code != http.StatusOK {
		t.Errorf("GET /page/1 status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestSiteRedirectsOnlyAReadOfAListingPage(t *testing.T) {
	t.Parallel()

	handler, _ := siteWith(manyPosts(25)...)

	for method, want := range map[string]int{
		http.MethodGet:     http.StatusMovedPermanently,
		http.MethodHead:    http.StatusMovedPermanently,
		http.MethodPost:    http.StatusOK,
		http.MethodPut:     http.StatusOK,
		http.MethodPatch:   http.StatusOK,
		http.MethodDelete:  http.StatusOK,
		http.MethodOptions: http.StatusOK,
	} {
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, httptest.NewRequest(method, "/page/1", nil))

		if recorder.Code != want {
			t.Errorf("%s /page/1 status = %d, want %d", method, recorder.Code, want)
		}
	}
}

func TestSiteSendsNothingButTheRedirect(t *testing.T) {
	t.Parallel()

	handler, _ := siteWith(manyPosts(25)...)

	recorder := get(t, handler, "/page/1?utm_source=feed")

	if recorder.Code != http.StatusMovedPermanently {
		t.Fatalf("GET /page/1 status = %d, want %d", recorder.Code, http.StatusMovedPermanently)
	}
	if recorder.Body.Len() != 0 {
		t.Errorf("GET /page/1 body = %q, want none", recorder.Body.String())
	}
}
