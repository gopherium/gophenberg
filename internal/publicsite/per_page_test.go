// SPDX-License-Identifier: Apache-2.0

package publicsite_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/publicsite"
)

// fakeSettings answers what the site chose per key, with scripted failure and absence.
type fakeSettings struct {
	held  map[string]string
	err   error
	found bool
}

// Lookup returns what the site chose under key.
func (s fakeSettings) Lookup(_ context.Context, key string) (string, bool, error) {
	if s.err != nil {
		return "", false, s.err
	}
	value, held := s.held[key]
	return value, held && s.found, nil
}

// settingsPaging returns a settings double holding the given public page size.
func settingsPaging(perPage string) fakeSettings {
	return fakeSettings{held: map[string]string{content.PerPageSettingKey: perPage}, found: true}
}

// sitePaging returns a handler serving the posts under the given site settings, and its reader.
func sitePaging(settings publicsite.SiteLocale, posts ...content.Content) (http.Handler, *fakeReader) {
	reader := &fakeReader{posts: posts}
	return publicsite.New(publicsite.Config{
		Content: reader,
		Types:   fakeTypes{},
		Locale:  settings,
		Title:   "A Test Site",
		Version: "1.2.3",
	}), reader
}

// manyPosts returns the given number of published posts, newest first.
func manyPosts(count int) []content.Content {
	now := time.Now().UTC()
	posts := make([]content.Content, 0, count)
	for i := range count {
		posts = append(posts, publishedPost("Post", "slug", blockMarkup, now.Add(-time.Duration(i)*time.Minute)))
	}
	return posts
}

func TestSiteListsAtThePageSizeTheSiteChose(t *testing.T) {
	t.Parallel()

	handler, reader := sitePaging(settingsPaging("5"), manyPosts(12)...)

	recorder := get(t, handler, "/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if reader.filter.PerPage != 5 {
		t.Errorf("filter per page = %d, want the 5 the site chose", reader.filter.PerPage)
	}
}

func TestSiteWalksOlderPagesAtThePageSizeTheSiteChose(t *testing.T) {
	t.Parallel()

	handler, _ := sitePaging(settingsPaging("5"), manyPosts(6)...)

	recorder := get(t, handler, "/")

	if !strings.Contains(recorder.Body.String(), `href="/page/2"`) {
		t.Errorf("body = %q, want six posts to run past a page of five", recorder.Body.String())
	}
}

func TestSiteOffersNoOlderPageWhenTheChosenSizeHoldsThemAll(t *testing.T) {
	t.Parallel()

	handler, _ := sitePaging(settingsPaging("50"), manyPosts(25)...)

	recorder := get(t, handler, "/")

	if strings.Contains(recorder.Body.String(), `href="/page/2"`) {
		t.Errorf("body = %q, want a page of fifty to hold all twenty five", recorder.Body.String())
	}
}

func TestSiteListsAtTheDefaultForAPageSizeItCannotUse(t *testing.T) {
	t.Parallel()

	for name, settings := range map[string]publicsite.SiteLocale{
		"a word":                   settingsPaging("many"),
		"zero":                     settingsPaging("0"),
		"one past the most":        settingsPaging("101"),
		"an empty row":             settingsPaging(""),
		"nothing stored":           fakeSettings{held: map[string]string{}, found: false},
		"a store that will fail":   fakeSettings{err: context.DeadlineExceeded},
		"no settings store at all": nil,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, reader := sitePaging(settings, manyPosts(3)...)

			recorder := get(t, handler, "/")

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want the site served, %d", recorder.Code, http.StatusOK)
			}
			if reader.filter.PerPage != content.DefaultPerPage {
				t.Errorf("filter per page = %d, want the default %d",
					reader.filter.PerPage, content.DefaultPerPage)
			}
		})
	}
}

func TestSiteServesATermAtThePageSizeTheSiteChose(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	term := publishedPost("News", "news", blockMarkup, now)
	term.Type = "category"
	term.Path = "categories/news"
	filed := make([]content.Content, 0, 7)
	for i := range 6 {
		post := publishedPost("Filed", "filed", blockMarkup, now.Add(-time.Duration(i)*time.Minute))
		post.Relations = content.Relations{"categories": {term.ID}}
		filed = append(filed, post)
	}
	handler, _ := sitePaging(settingsPaging("5"), append(filed, term)...)

	recorder := get(t, handler, "/categories/news")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `href="/categories/news/page/2"`) {
		t.Errorf("body = %q, want six filed posts to run past a page of five", recorder.Body.String())
	}
}
