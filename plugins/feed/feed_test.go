// SPDX-License-Identifier: Apache-2.0

package feed_test

import (
	"context"
	"encoding/xml"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/plugins/feed"
	"github.com/gopherium/gophenberg/sdk"
)

// channel is the RSS channel a reader parses out of the feed.
type channel struct {
	Title string `xml:"channel>title"`
	Link  string `xml:"channel>link"`
	Items []item `xml:"channel>item"`
}

// item is one entry of the parsed feed.
type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	GUID        string `xml:"guid"`
	IsPermaLink string `xml:"guid,attr"`
	PubDate     string `xml:"pubDate"`
}

// stubPosts serves scripted posts and records what was asked of it.
type stubPosts struct {
	posts    []sdk.Post
	postType string
	limit    int
	listErr  error
}

// ListPublished records the request and returns the scripted posts.
func (s *stubPosts) ListPublished(_ context.Context, postType string, limit int) ([]sdk.Post, error) {
	s.postType, s.limit = postType, limit
	return s.posts, s.listErr
}

// samplePost returns a published post carrying the given title and content.
func samplePost(title, content string) sdk.Post {
	return sdk.Post{
		ID:          uuid.MustParse("019fb000-0000-7000-8000-000000000001"),
		Type:        "post",
		Slug:        "a-slug",
		Title:       title,
		Excerpt:     "An excerpt.",
		Content:     content,
		PublishedAt: time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 7, 20, 10, 0, 0, 0, time.UTC),
	}
}

// testEnv returns a getenv double carrying the given values.
func testEnv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

// mustRegister registers the feed plugin over the given posts and environment.
func mustRegister(t *testing.T, posts sdk.PostReader, values map[string]string) sdk.Plugin {
	t.Helper()
	plugin, err := feed.Register(sdk.Deps{Posts: posts, Getenv: testEnv(values)})
	if err != nil {
		t.Fatalf("Register() error = %v, want nil", err)
	}
	return plugin
}

// serveRequest asks the plugin for its feed with the given request and returns the response.
func serveRequest(t *testing.T, plugin sdk.Plugin, request *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	routes, ok := plugin.(interface{ Routes() http.Handler })
	if !ok {
		t.Fatal("plugin does not provide routes")
	}
	recorder := httptest.NewRecorder()
	routes.Routes().ServeHTTP(recorder, request)
	return recorder
}

// serve asks the plugin for its feed and returns the response.
func serve(t *testing.T, plugin sdk.Plugin) *httptest.ResponseRecorder {
	t.Helper()
	return serveRequest(t, plugin, httptest.NewRequest(http.MethodGet, "/rss.xml", nil))
}

func TestFeedRegistersUnderItsOwnID(t *testing.T) {
	t.Parallel()

	plugin := mustRegister(t, &stubPosts{}, map[string]string{})

	if plugin.ID() != "feed" {
		t.Errorf("ID() = %q, want %q", plugin.ID(), "feed")
	}
}

func TestFeedServesRSSAReaderCanParse(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{posts: []sdk.Post{samplePost("A Published Post", "<p>Body</p>")}}
	response := serve(t, mustRegister(t, posts, map[string]string{}))

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	var parsed channel
	if err := xml.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parsing the feed: %v", err)
	}
	if len(parsed.Items) != 1 || parsed.Items[0].Title != "A Published Post" {
		t.Errorf("items = %+v, want the published post", parsed.Items)
	}
	if got := response.Header().Get("Content-Type"); !strings.Contains(got, "xml") {
		t.Errorf("Content-Type = %q, want an xml type", got)
	}
}

func TestFeedNamesItsChannelAndLinksTheSite(t *testing.T) {
	t.Parallel()

	values := map[string]string{"GOPHENBERG_FEED_TITLE": "The Gophenberg Feed"}
	response := serve(t, mustRegister(t, &stubPosts{}, values))

	var parsed channel
	if err := xml.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parsing the feed: %v", err)
	}
	if parsed.Title != "The Gophenberg Feed" {
		t.Errorf("channel title = %q, want %q", parsed.Title, "The Gophenberg Feed")
	}
	if parsed.Link != "http://example.com" {
		t.Errorf("channel link = %q, want %q", parsed.Link, "http://example.com")
	}
}

func TestFeedLinksEachItemAtItsFutureAddress(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{posts: []sdk.Post{samplePost("A Post", "<p>Body</p>")}}
	response := serve(t, mustRegister(t, posts, map[string]string{}))

	var parsed channel
	if err := xml.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parsing the feed: %v", err)
	}
	if parsed.Items[0].Link != "http://example.com/post/a-slug" {
		t.Errorf("item link = %q, want %q", parsed.Items[0].Link, "http://example.com/post/a-slug")
	}
	if parsed.Items[0].GUID != "urn:uuid:019fb000-0000-7000-8000-000000000001" {
		t.Errorf("item guid = %q, want the urn form", parsed.Items[0].GUID)
	}
	if !strings.Contains(response.Body.String(), `isPermaLink="false"`) {
		t.Error("guid does not declare isPermaLink false")
	}
}

func TestFeedLinksCarryTheHostTheReaderDialed(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{posts: []sdk.Post{samplePost("A Post", "<p>Body</p>")}}
	request := httptest.NewRequest(http.MethodGet, "/rss.xml", nil)
	request.Host = "localhost:8081"

	response := serveRequest(t, mustRegister(t, posts, map[string]string{}), request)

	var parsed channel
	if err := xml.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parsing the feed: %v", err)
	}
	if parsed.Link != "http://localhost:8081" {
		t.Errorf("channel link = %q, want %q", parsed.Link, "http://localhost:8081")
	}
	if parsed.Items[0].Link != "http://localhost:8081/post/a-slug" {
		t.Errorf("item link = %q, want %q", parsed.Items[0].Link, "http://localhost:8081/post/a-slug")
	}
}

func TestFeedInfersHTTPSFromATLSConnection(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{posts: []sdk.Post{samplePost("A Post", "<p>Body</p>")}}
	request := httptest.NewRequest(http.MethodGet, "https://example.com/rss.xml", nil)

	response := serveRequest(t, mustRegister(t, posts, map[string]string{}), request)

	var parsed channel
	if err := xml.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parsing the feed: %v", err)
	}
	if parsed.Link != "https://example.com" {
		t.Errorf("channel link = %q, want %q", parsed.Link, "https://example.com")
	}
}

func TestFeedHonorsTheForwardedProtocol(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{posts: []sdk.Post{samplePost("A Post", "<p>Body</p>")}}
	request := httptest.NewRequest(http.MethodGet, "/rss.xml", nil)
	request.Host = "example.com"
	request.Header.Set("X-Forwarded-Proto", "https")

	response := serveRequest(t, mustRegister(t, posts, map[string]string{}), request)

	var parsed channel
	if err := xml.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parsing the feed: %v", err)
	}
	if parsed.Link != "https://example.com" {
		t.Errorf("channel link = %q, want %q", parsed.Link, "https://example.com")
	}
	if parsed.Items[0].Link != "https://example.com/post/a-slug" {
		t.Errorf("item link = %q, want %q", parsed.Items[0].Link, "https://example.com/post/a-slug")
	}
}

func TestFeedStripsBlockCommentsFromTheContent(t *testing.T) {
	t.Parallel()

	content := "<!-- wp:paragraph -->\n<p>Body</p>\n<!-- /wp:paragraph -->\n\n" +
		"<!-- wp:heading {\"level\":3} -->\n<h3>Heading</h3>\n<!-- /wp:heading -->"
	posts := &stubPosts{posts: []sdk.Post{samplePost("A Post", content)}}

	response := serve(t, mustRegister(t, posts, map[string]string{}))

	body := response.Body.String()
	if strings.Contains(body, "wp:") {
		t.Errorf("feed carries block comments, want them stripped: %s", body)
	}
	var parsed channel
	if err := xml.Unmarshal(response.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parsing the feed: %v", err)
	}
	if !strings.Contains(parsed.Items[0].Description, "<p>Body</p>") {
		t.Errorf("description = %q, want it to keep the markup", parsed.Items[0].Description)
	}
	if !strings.Contains(parsed.Items[0].Description, "<h3>Heading</h3>") {
		t.Errorf("description = %q, want it to keep the heading", parsed.Items[0].Description)
	}
}

func TestFeedAsksOnlyForPublishedPostsUpToItsCap(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{}

	serve(t, mustRegister(t, posts, map[string]string{}))

	if posts.postType != "post" {
		t.Errorf("asked for type %q, want %q", posts.postType, "post")
	}
	if posts.limit != 20 {
		t.Errorf("asked for %d posts, want the default cap of 20", posts.limit)
	}
}

func TestFeedTakesTheCapFromTheEnvironment(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{}

	serve(t, mustRegister(t, posts, map[string]string{"GOPHENBERG_FEED_ITEMS": "5"}))

	if posts.limit != 5 {
		t.Errorf("asked for %d posts, want 5", posts.limit)
	}
}

func TestFeedReportsPostsItCouldNotRead(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{listErr: errors.New("database down")}

	response := serve(t, mustRegister(t, posts, map[string]string{}))

	if response.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

// breakingWriter is a response writer refusing to carry a body.
type breakingWriter struct {
	header http.Header
}

// Header returns the headers the writer collected.
func (w *breakingWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

// Write refuses whatever it is handed.
func (w *breakingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("connection gone")
}

// WriteHeader accepts the status.
func (w *breakingWriter) WriteHeader(_ int) {}

func TestFeedGivesUpOnAReaderThatWentAway(t *testing.T) {
	t.Parallel()

	posts := &stubPosts{posts: []sdk.Post{samplePost("A Post", "<p>Body</p>")}}
	plugin := mustRegister(t, posts, map[string]string{})
	routes, ok := plugin.(interface{ Routes() http.Handler })
	if !ok {
		t.Fatal("plugin does not provide routes")
	}

	routes.Routes().ServeHTTP(&breakingWriter{}, httptest.NewRequest(http.MethodGet, "/rss.xml", nil))
}

func TestFeedLeavesItsAddressReachableWithoutASession(t *testing.T) {
	t.Parallel()

	plugin := mustRegister(t, &stubPosts{}, map[string]string{})

	public, ok := plugin.(interface{ PublicPaths() []string })
	if !ok {
		t.Fatal("plugin does not declare public paths")
	}
	if paths := public.PublicPaths(); len(paths) != 1 || paths[0] != "/rss.xml" {
		t.Errorf("PublicPaths() = %v, want [/rss.xml]", paths)
	}
}
