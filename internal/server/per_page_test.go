// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// settledContentServer returns an unauthenticated handler over the store, reading the given site settings.
func settledContentServer(settings *storedSettings, store *fakePostStore, posts ...content.Content) http.Handler {
	for _, p := range posts {
		store.add(p)
	}
	types := newFakeTypeStore()
	types.register(content.Type{
		Key: "category", SingularLabel: "Category", PluralLabel: "Categories",
		RouteWord: "categories", PageKind: content.PageKindArchive, Active: true,
	})
	return server.NewServer(server.Config{
		Users: newFakeUserStore(), Content: store, Types: types, Settings: settings,
	})
}

// settingsChoosing returns a settings store holding the given public page size.
func settingsChoosing(perPage string) *storedSettings {
	settings := newStoredSettings()
	settings.site[content.PerPageSettingKey] = perPage
	return settings
}

func TestContentAPIListsAtThePageSizeTheSiteChose(t *testing.T) {
	t.Parallel()

	handler := settledContentServer(settingsChoosing("5"), newFakePostStore())

	recorder := getContent(t, handler, "/api/content/v1/items")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `"per_page":5`) {
		t.Errorf("body = %q, want the page size the site chose", recorder.Body.String())
	}
}

func TestContentAPILetsAQueryOutrankThePageSizeTheSiteChose(t *testing.T) {
	t.Parallel()

	handler := settledContentServer(settingsChoosing("5"), newFakePostStore())

	recorder := getContent(t, handler, "/api/content/v1/items?per_page=3")

	if !strings.Contains(recorder.Body.String(), `"per_page":3`) {
		t.Errorf("body = %q, want the query's page size to win", recorder.Body.String())
	}
}

func TestContentAPIListsAtTheDefaultForAPageSizeItCannotUse(t *testing.T) {
	t.Parallel()

	for name, stored := range map[string]string{
		"a word":            "many",
		"zero":              "0",
		"one past the most": "101",
		"an empty row":      "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := settledContentServer(settingsChoosing(stored), newFakePostStore())

			recorder := getContent(t, handler, "/api/content/v1/items")

			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
			}
			if !strings.Contains(recorder.Body.String(), `"per_page":20`) {
				t.Errorf("body = %q, want the default page size kept", recorder.Body.String())
			}
		})
	}
}

func TestContentAPIListsAtTheDefaultWhenTheSettingsWillNotAnswer(t *testing.T) {
	t.Parallel()

	settings := newStoredSettings()
	settings.lookupErr = context.DeadlineExceeded
	handler := settledContentServer(settings, newFakePostStore())

	recorder := getContent(t, handler, "/api/content/v1/items")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want the listing served despite the settings, body %s",
			recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"per_page":20`) {
		t.Errorf("body = %q, want the default page size served", recorder.Body.String())
	}
}

func TestContentAPIResolvesAnArchiveAtThePageSizeTheSiteChose(t *testing.T) {
	t.Parallel()

	handler := settledContentServer(settingsChoosing("5"), newFakePostStore(),
		publishedFixture(t, "hello-world", blockMarkup, time.Now().UTC()))

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"per_page":5`) {
		t.Errorf("body = %q, want the archive paged as the site chose", recorder.Body.String())
	}
}

func TestContentAPIResolvesATermAtThePageSizeTheSiteChose(t *testing.T) {
	t.Parallel()

	term, filed := termFixture(t)
	handler := settledContentServer(settingsChoosing("5"), newFakePostStore(), term, filed)

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/categories/news")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"per_page":5`) {
		t.Errorf("body = %q, want the term paged as the site chose", recorder.Body.String())
	}
}

func TestPublicSitePagesAtTheSizeTheSiteChose(t *testing.T) {
	t.Parallel()

	posts := newFakePostStore()
	now := time.Now().UTC()
	for i := range 6 {
		posts.add(publishedFixture(t, "post-"+strconv.Itoa(i), blockMarkup, now.Add(-time.Duration(i)*time.Minute)))
	}
	handler := server.NewServer(server.Config{
		Users:     newFakeUserStore(),
		Content:   posts,
		Types:     newFakeTypeStore(),
		Settings:  settingsChoosing("5"),
		SiteTitle: "A Test Site",
		Version:   "1.2.3",
	})

	recorder := doRequest(t, handler, http.MethodGet, "/", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if !strings.Contains(recorder.Body.String(), `href="/page/2"`) {
		t.Errorf("body = %q, want six posts to run past the page of five the site chose", recorder.Body.String())
	}
}
