// SPDX-License-Identifier: Apache-2.0

package publicsite_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/publicsite"
)

// blockMarkup is the serialized block content a published fixture carries.
const blockMarkup = "<!-- wp:paragraph --><p>Body text.</p><!-- /wp:paragraph -->"

// fakeReader serves posts from memory and records the filter it was asked for.
type fakeReader struct {
	posts   []content.Content
	filter  content.Filter
	listErr error
	readErr error
}

// List records the filter and returns the matching page without content.
func (r *fakeReader) List(_ context.Context, f content.Filter) ([]content.Content, int, error) {
	r.filter = f
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	matched := make([]content.Content, 0, len(r.posts))
	for _, p := range r.posts {
		if p.Type != f.Type || p.Status != f.Status {
			continue
		}
		p.Content = ""
		matched = append(matched, p)
	}
	total := len(matched)
	start := min((f.Page-1)*f.PerPage, total)
	return matched[start:min(start+f.PerPage, total)], total, nil
}

// PublishedByPath returns the stored published item answering at the address.
func (r *fakeReader) PublishedByPath(_ context.Context, path string) (content.Content, error) {
	if r.readErr != nil {
		return content.Content{}, r.readErr
	}
	for _, p := range r.posts {
		if p.Path == path && p.Status == content.StatusPublished {
			return p, nil
		}
	}
	return content.Content{}, content.ErrNotFound
}

// fakeTypes answers which type answers under a route word.
type fakeTypes struct{ err error }

// ByRouteWord returns the post type at the root and the page type under its word.
func (t fakeTypes) ByRouteWord(_ context.Context, word string) (content.Type, error) {
	if t.err != nil {
		return content.Type{}, t.err
	}
	switch word {
	case "":
		return content.Type{Key: content.TypePost, PluralLabel: "Posts", Default: true, Active: true}, nil
	case "pages":
		return content.Type{
			Key: "page", PluralLabel: "Pages", RouteWord: "pages", Hierarchical: true, Active: true,
		}, nil
	}
	return content.Type{}, content.ErrTypeNotFound
}

// publishedPost returns a published post carrying the given title, slug, and body.
func publishedPost(title, slug, body string, at time.Time) content.Content {
	return content.Content{
		ID:          uuid.Must(uuid.NewV7()),
		Type:        content.TypePost,
		Slug:        slug,
		Path:        slug,
		Title:       title,
		Excerpt:     "An excerpt.",
		Content:     body,
		Status:      content.StatusPublished,
		PublishedAt: &at,
		CreatedAt:   at,
		UpdatedAt:   at,
	}
}

// siteWith returns a handler serving the given posts, and the reader behind it.
func siteWith(posts ...content.Content) (http.Handler, *fakeReader) {
	reader := &fakeReader{posts: posts}
	return publicsite.New(publicsite.Config{
		Content: reader,
		Types:   fakeTypes{},
		Title:   "A Test Site",
		Version: "1.2.3",
	}), reader
}

// get returns the response the site serves at path.
func get(t *testing.T, handler http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	return recorder
}

func TestSiteListsPublishedPostsNewestFirst(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	newer := publishedPost("A Newer Post", "newer", blockMarkup, now)
	older := publishedPost("An Older Post", "older", blockMarkup, now.Add(-time.Hour))
	draft := publishedPost("A Draft", "a-draft", blockMarkup, now)
	draft.Status = content.StatusDraft
	handler, reader := siteWith(newer, older, draft)

	recorder := get(t, handler, "/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "A Draft") {
		t.Errorf("body = %q, want the draft hidden", body)
	}
	if strings.Index(body, "A Newer Post") > strings.Index(body, "An Older Post") {
		t.Errorf("body = %q, want the order the store served kept", body)
	}
	if !strings.Contains(body, `href="/newer"`) {
		t.Errorf("body = %q, want each post linked at its address", body)
	}
	if reader.filter.Status != content.StatusPublished {
		t.Errorf("filter status = %q, want %q", reader.filter.Status, content.StatusPublished)
	}
	if reader.filter.OrderBy != content.OrderByDate || reader.filter.Order != content.OrderDesc {
		t.Errorf("filter ordering = %q %q, want the newest published first",
			reader.filter.OrderBy, reader.filter.Order)
	}
	if reader.filter.Type != content.TypePost {
		t.Errorf("filter type = %q, want %q", reader.filter.Type, content.TypePost)
	}
}

func TestSiteServesAPostWithSanitizedContent(t *testing.T) {
	t.Parallel()

	hostile := `<!-- wp:paragraph --><p onclick="steal()">Body text.</p><script>alert(1)</script><!-- /wp:paragraph -->`
	handler, _ := siteWith(publishedPost("A Post", "a-post", hostile, time.Now().UTC()))

	recorder := get(t, handler, "/a-post")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "onclick") || strings.Contains(body, "alert(1)") {
		t.Errorf("body = %q, want the scriptable markup stripped", body)
	}
	if !strings.Contains(body, "<p>Body text.</p>") {
		t.Errorf("body = %q, want the block markup rendered", body)
	}
	if strings.Contains(body, "wp:") {
		t.Errorf("body = %q, want the block delimiters stripped for a reader", body)
	}
}

func TestSiteIdentifiesItselfOnEveryPage(t *testing.T) {
	t.Parallel()

	handler, _ := siteWith(publishedPost("A Post", "a-post", blockMarkup, time.Now().UTC()))

	for _, path := range []string{"/", "/a-post", "/nothing-here"} {
		body := get(t, handler, path).Body.String()
		if !strings.Contains(body, `<meta name="generator" content="Gophenberg 1.2">`) {
			t.Errorf("GET %s body = %q, want the generator meta at major.minor", path, body)
		}
		if got := strings.Count(body, `href="/gophenberg/blocks.css"`); got != 1 {
			t.Errorf("GET %s references blocks.css %d times, want 1", path, got)
		}
		if got := strings.Count(body, `href="/gophenberg/presets.css"`); got != 1 {
			t.Errorf("GET %s references presets.css %d times, want 1", path, got)
		}
		if !strings.Contains(body, `href="/gophenberg/favicon.svg"`) {
			t.Errorf("GET %s body = %q, want the icon linked", path, body)
		}
		if !strings.Contains(body, `href="/api/plugins/feed/rss.xml"`) {
			t.Errorf("GET %s body = %q, want the feed linked", path, body)
		}
		if !strings.Contains(body, "A Test Site") {
			t.Errorf("GET %s body = %q, want the site title", path, body)
		}
	}
}

func TestSiteAnswersUnservedAddressesWithARenderedNotFound(t *testing.T) {
	t.Parallel()

	draft := publishedPost("A Draft", "a-draft", blockMarkup, time.Now().UTC())
	draft.Status = content.StatusDraft
	handler, _ := siteWith(draft)

	for _, path := range []string{
		"/a-draft",
		"/never-written",
		"/page/a-draft",
		"/one/two/three/four",
		"/pages/page/nonsense",
	} {
		recorder := get(t, handler, path)
		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", path, recorder.Code, http.StatusNotFound)
		}
		if !strings.Contains(recorder.Body.String(), "A Test Site") {
			t.Errorf("GET %s body = %q, want the styled not found page", path, recorder.Body.String())
		}
	}
}

func TestSiteWalksOlderPages(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	posts := make([]content.Content, 0, 25)
	for i := range 25 {
		posts = append(posts, publishedPost("Post", "slug", blockMarkup, now.Add(-time.Duration(i)*time.Minute)))
	}
	handler, reader := siteWith(posts...)

	first := get(t, handler, "/")
	if !strings.Contains(first.Body.String(), `href="/page/2"`) {
		t.Errorf("index body = %q, want a link to the older page", first.Body.String())
	}

	second := get(t, handler, "/page/2")

	if second.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", second.Code, http.StatusOK)
	}
	if reader.filter.Page != 2 {
		t.Errorf("filter page = %d, want 2", reader.filter.Page)
	}
	if !strings.Contains(second.Body.String(), `class="gophenberg-site__newer" href="/"`) {
		t.Errorf("second page body = %q, want a way back to the newest", second.Body.String())
	}
}

func TestSiteRefusesAPageBelowTheFirst(t *testing.T) {
	t.Parallel()

	handler, _ := siteWith(publishedPost("A Post", "a-post", blockMarkup, time.Now().UTC()))

	for _, address := range []string{"/page/0", "/page/-1", "/pages/page/0"} {
		recorder := get(t, handler, address)

		if recorder.Code != http.StatusNotFound {
			t.Errorf("status at %q = %d, want %d", address, recorder.Code, http.StatusNotFound)
		}
	}
}

func TestSiteServesAPageBeyondTheLastAsEmpty(t *testing.T) {
	t.Parallel()

	handler, _ := siteWith(publishedPost("A Post", "a-post", blockMarkup, time.Now().UTC()))

	recorder := get(t, handler, "/page/99")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if strings.Contains(recorder.Body.String(), "A Post") {
		t.Errorf("body = %q, want no posts that far out", recorder.Body.String())
	}
}

func TestSiteOffersNoOlderPageBeyondTheLast(t *testing.T) {
	t.Parallel()

	handler, _ := siteWith(publishedPost("A Post", "a-post", blockMarkup, time.Now().UTC()))

	for _, path := range []string{"/page/2", "/page/99", "/page/461168601842738791"} {
		body := get(t, handler, path).Body.String()
		if strings.Contains(body, "Older posts") {
			t.Errorf("GET %s body = %q, want no older page past the last", path, body)
		}
	}
}

func TestSiteEscapesWhatAnAuthorTyped(t *testing.T) {
	t.Parallel()

	title := `<script>alert(1)</script>`
	handler, _ := siteWith(publishedPost(title, "a-post", blockMarkup, time.Now().UTC()))

	for _, path := range []string{"/", "/a-post"} {
		body := get(t, handler, path).Body.String()
		if strings.Contains(body, "<script>alert(1)</script>") {
			t.Errorf("GET %s body = %q, want the title escaped", path, body)
		}
		if !strings.Contains(body, "&lt;script&gt;") {
			t.Errorf("GET %s body = %q, want the escaped title rendered", path, body)
		}
	}
}

func TestSiteNamesItselfWhenTheConfigurationDoesNot(t *testing.T) {
	t.Parallel()

	handler := publicsite.New(publicsite.Config{Content: &fakeReader{}, Types: fakeTypes{}, Version: "1.2.3"})

	body := get(t, handler, "/").Body.String()

	if !strings.Contains(body, "<title>Gophenberg</title>") {
		t.Errorf("body = %q, want the default site title", body)
	}
}

func TestSiteDatesAPostThatCarriesNoPublicationInstant(t *testing.T) {
	t.Parallel()

	updated := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	stored := publishedPost("A Post", "a-post", blockMarkup, updated)
	stored.PublishedAt = nil
	stored.UpdatedAt = updated
	handler, _ := siteWith(stored)

	body := get(t, handler, "/a-post").Body.String()

	if !strings.Contains(body, "3 August 2026") {
		t.Errorf("body = %q, want it dated by its last update", body)
	}
}

func TestSiteReportsAStoreItCouldNotRead(t *testing.T) {
	t.Parallel()

	handler, reader := siteWith(publishedPost("A Post", "a-post", blockMarkup, time.Now().UTC()))
	reader.listErr = errors.New("database down")
	reader.readErr = errors.New("database down")

	for _, path := range []string{"/", "/a-post"} {
		recorder := get(t, handler, path)
		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("GET %s status = %d, want %d", path, recorder.Code, http.StatusInternalServerError)
		}
		if strings.Contains(recorder.Body.String(), "database down") {
			t.Errorf("GET %s body = %q, want the internal error masked", path, recorder.Body.String())
		}
	}
}

func TestSiteShowsTheDateAPostWasPublished(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.August, 3, 12, 0, 0, 0, time.UTC)
	handler, _ := siteWith(publishedPost("A Post", "a-post", blockMarkup, at))

	body := get(t, handler, "/a-post").Body.String()

	if !strings.Contains(body, "3 August 2026") {
		t.Errorf("body = %q, want the publication date", body)
	}
	if !strings.Contains(body, `datetime="2026-08-03"`) {
		t.Errorf("body = %q, want a machine readable date", body)
	}
}
