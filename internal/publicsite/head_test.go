// SPDX-License-Identifier: Apache-2.0

package publicsite_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/publicsite"
	"github.com/gopherium/gophenberg/sdk"
)

// fakeHead adds its tags to every page while on, keeping each page it was asked about.
type fakeHead struct {
	mu   sync.Mutex
	off  bool
	tags []sdk.HeadTag
	seen []sdk.PageMeta
}

// Heads reports whether the fake adds head tags.
func (h *fakeHead) Heads() bool {
	return !h.off
}

// HeadTags keeps the page and answers the fake's tags.
func (h *fakeHead) HeadTags(_ context.Context, page sdk.PageMeta) []sdk.HeadTag {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.seen = append(h.seen, page)
	return h.tags
}

// asked returns the pages the fake was asked about.
func (h *fakeHead) asked() []sdk.PageMeta {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.seen
}

// headedSite returns a handler serving one post, one page and one category under the head source.
func headedSite(head publicsite.HeadSource) (http.Handler, content.Content, content.Content) {
	now := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	post := publishedPost("A Post", "a-post", blockMarkup, now)
	about := publishedPost("About", "pages/about", blockMarkup, now)
	about.Type = "page"
	news := content.Content{
		ID: uuid.Must(uuid.NewV7()), Type: "category", Status: content.StatusPublished,
		Path: "categories/news", Slug: "news", Title: "News", Content: blockMarkup, UpdatedAt: now,
	}
	return publicsite.New(publicsite.Config{
		Content: &fakeReader{posts: []content.Content{post, about, news}},
		Types:   fakeTypes{},
		Title:   "A Test Site",
		Version: "1.2.3",
		Head:    head,
	}), post, news
}

func TestSiteWritesTheHeadTagsPluginsAddOnEveryKindOfPage(t *testing.T) {
	t.Parallel()

	head := &fakeHead{tags: []sdk.HeadTag{{Tag: "meta", Attrs: []sdk.Attr{{Key: "name", Value: "notes-mark"}}}}}
	handler, _, _ := headedSite(head)

	for _, path := range []string{"/", "/pages", "/a-post", "/categories/news", "/nothing-here"} {
		body := get(t, handler, path).Body.String()
		if strings.Count(body, `<meta name="notes-mark">`+"\n</head>") != 1 {
			t.Errorf("GET %s body = %q, want the plugin's tag once at the end of the head", path, body)
		}
	}
}

func TestSiteDescribesEachKindOfPageToThePlugins(t *testing.T) {
	t.Parallel()

	head := &fakeHead{}
	handler, post, news := headedSite(head)
	for _, path := range []string{"/", "/pages", "/a-post", "/categories/news", "/nothing-here"} {
		get(t, handler, path)
	}

	want := []sdk.PageMeta{
		{Kind: sdk.PageArchive, Type: content.TypePost, Path: "/", Page: 1, SiteTitle: "A Test Site"},
		{Kind: sdk.PageArchive, Type: "page", Path: "/pages", Page: 1, SiteTitle: "A Test Site"},
		{Kind: sdk.PageItem, Type: content.TypePost, Path: "/a-post", Item: plainItem(post), SiteTitle: "A Test Site"},
		{
			Kind: sdk.PageTerm, Type: "category", Path: "/categories/news", Page: 1, Item: plainItem(news),
			SiteTitle: "A Test Site",
		},
		{Kind: sdk.PageNotFound, Path: "/nothing-here", SiteTitle: "A Test Site"},
	}
	seen := head.asked()
	if len(seen) != len(want) {
		t.Fatalf("the plugins were asked about %+v, want %+v", seen, want)
	}
	for i := range want {
		if !reflect.DeepEqual(seen[i], want[i]) {
			t.Errorf("page %d was described as %+v with the item %+v, want %+v with %+v",
				i, seen[i], seen[i].Item, want[i], want[i].Item)
		}
	}
}

func TestSiteWritesTheFeedLinkWhereTheShellTemplateWroteIt(t *testing.T) {
	t.Parallel()

	head := &fakeHead{tags: []sdk.HeadTag{{Tag: "link", Attrs: []sdk.Attr{
		{Key: "rel", Value: "alternate"}, {Key: "type", Value: "application/rss+xml"},
		{Key: "title", Value: "A Test Site"}, {Key: "href", Value: "/api/plugins/feed/rss.xml"},
	}}}}
	handler, _, _ := headedSite(head)

	want := `<link rel="stylesheet" href="/gophenberg/presets.css">` + "\n" +
		`<link rel="alternate" type="application/rss+xml" title="A Test Site" href="/api/plugins/feed/rss.xml">` +
		"\n</head>\n"
	for _, path := range []string{"/", "/a-post", "/nothing-here"} {
		if body := get(t, handler, path).Body.String(); !strings.Contains(body, want) {
			t.Errorf("GET %s body = %q, want the head to end as the shell template wrote it %q", path, body, want)
		}
	}
}

func TestSiteLinksNoFeedWhenNoPluginAddsIt(t *testing.T) {
	t.Parallel()

	handler, _ := siteWith(publishedPost("A Post", "a-post", blockMarkup, time.Now().UTC()))

	for _, path := range []string{"/", "/a-post", "/nothing-here"} {
		body := get(t, handler, path).Body.String()
		if strings.Contains(body, "rss.xml") || !strings.Contains(body, `presets.css">`+"\n</head>") {
			t.Errorf("GET %s body = %q, want no feed link and the head ending after the stylesheets", path, body)
		}
	}
}

func TestSiteAsksNoPluginWhenNoneAddsHeadTags(t *testing.T) {
	t.Parallel()

	head := &fakeHead{off: true}
	handler, _, _ := headedSite(head)

	for _, path := range []string{"/", "/a-post", "/categories/news", "/nothing-here"} {
		get(t, handler, path)
	}

	if seen := head.asked(); len(seen) != 0 {
		t.Errorf("the plugins were asked about %+v, want no page asked", seen)
	}
}

// plainItem returns the item a plugin sees of c on a page, its content and fields left out.
func plainItem(c content.Content) *sdk.Item {
	published := c.UpdatedAt
	if c.PublishedAt != nil {
		published = *c.PublishedAt
	}
	return &sdk.Item{
		ID: sdk.ID(c.ID), Type: c.Type, Path: c.Path, Slug: c.Slug, Title: c.Title, Excerpt: c.Excerpt,
		PublishedAt: published, UpdatedAt: c.UpdatedAt,
	}
}
