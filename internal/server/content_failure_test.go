// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// contentServerOver returns a public handler over a store the caller has prepared.
func contentServerOver(store *fakePostStore, posts ...content.Content) http.Handler {
	for _, p := range posts {
		store.add(p)
	}
	types := newFakeTypeStore()
	types.register(content.Type{
		Key: "category", SingularLabel: "Category", PluralLabel: "Categories",
		RouteWord: "categories", PageKind: content.PageKindArchive, Active: true,
	})
	return server.NewServer(server.Config{Users: newFakeUserStore(), Content: store, Types: types})
}

// termFixture returns a published term and a post filed under it.
func termFixture(t *testing.T) (content.Content, content.Content) {
	t.Helper()
	now := time.Now().UTC()
	term := publishedFixture(t, "news", blockMarkup, now)
	term.Type = "category"
	term.Path = "categories/news"
	filed := publishedFixture(t, "a-filed-post", blockMarkup, now)
	filed.Relations = content.Relations{"categories": {term.ID}}
	return term, filed
}

func TestContentAPIReportsAnItemWhoseTargetsItCannotRead(t *testing.T) {
	t.Parallel()

	store := newFakePostStore()
	handler := contentServerOver(store, publishedFixture(t, "hello-world", blockMarkup, time.Now().UTC()))
	store.targetsErr = context.DeadlineExceeded

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/hello-world")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestContentAPIReportsATermItCannotRead(t *testing.T) {
	t.Parallel()

	term, filed := termFixture(t)

	for name, prepare := range map[string]func(*fakePostStore){
		"the content pointing at it": func(s *fakePostStore) { s.relatedErr = context.DeadlineExceeded },
		"the targets of the term":    func(s *fakePostStore) { s.targetsErr = context.DeadlineExceeded },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newFakePostStore()
			handler := contentServerOver(store, term, filed)
			prepare(store)

			recorder := getContent(t, handler, "/api/content/v1/resolve?path=/categories/news")

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
			}
		})
	}
}

func TestContentAPIRefusesTermPagingItCannotRead(t *testing.T) {
	t.Parallel()

	term, filed := termFixture(t)
	handler := contentServerOver(newFakePostStore(), term, filed)

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/categories/news&per_page=none")

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
