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

// filingField returns the relation field a post and a category are filed through.
func filingField() content.Field {
	return content.Field{
		ID: 11, Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	}
}

// contentServerOver returns a public handler over a store the caller has prepared.
func contentServerOver(store *fakePostStore, posts ...content.Content) http.Handler {
	for _, p := range posts {
		store.add(p)
	}
	types := &fakeTypeStore{types: []content.Type{{
		Key: content.TypePost, SingularLabel: "Post", PluralLabel: "Posts",
		Revisions: true, RevisionCap: 100, PageKind: content.PageKindSingle,
		Default: true, Active: true, Fields: []content.Field{filingField()},
	}}}
	types.register(content.Type{
		Key: "category", SingularLabel: "Category", PluralLabel: "Categories",
		RouteWord: "categories", PageKind: content.PageKindArchive, Active: true,
		Fields: []content.Field{filingField()},
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
	filed.Fields = content.Values{"categories": []any{term.ID.String()}}
	term.Fields = content.Values{"categories": []any{filed.ID.String()}}
	return term, filed
}

func TestContentAPIReportsAnItemWhoseTargetsItCannotRead(t *testing.T) {
	t.Parallel()

	store := newFakePostStore()
	term, filed := termFixture(t)
	handler := contentServerOver(store, term, filed)
	store.targetsErr = context.DeadlineExceeded

	recorder := getContent(t, handler, "/api/content/v1/resolve?path=/a-filed-post")

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
