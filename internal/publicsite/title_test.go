// SPDX-License-Identifier: Apache-2.0

package publicsite_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestEveryKindOfPageCarriesItsDocumentTitle(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	about := publishedPost("About", "pages/about", blockMarkup, now)
	about.Type = "page"
	news := content.Content{
		ID: uuid.Must(uuid.NewV7()), Type: "category", Status: content.StatusPublished,
		Path: "categories/news", Slug: "news", Title: "News", Content: blockMarkup,
	}
	handler, _ := siteWith(publishedPost(`Ann & "Bob"`, "a-post", blockMarkup, now), about, news)

	for path, want := range map[string]string{
		"/":                "<title>A Test Site</title>",
		"/pages":           "<title>Pages | A Test Site</title>",
		"/a-post":          "<title>Ann &amp; &#34;Bob&#34; | A Test Site</title>",
		"/categories/news": "<title>News | A Test Site</title>",
		"/nothing-here":    "<title>Not found | A Test Site</title>",
	} {
		if body := get(t, handler, path).Body.String(); !strings.Contains(body, want) {
			t.Errorf("GET %s body = %q, want %s", path, body, want)
		}
	}
}
