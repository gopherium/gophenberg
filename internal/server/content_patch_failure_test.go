// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// nestingServer returns a signed in handler over stores the caller can break, and the nesting type.
func nestingServer(t *testing.T) (http.Handler, *fakePostStore, *fakeTypeStore, gouncer.User) {
	t.Helper()

	users := newFakeUserStore()
	ada := addAda(t, users)
	posts := newFakePostStore()
	types := newFakeTypeStore()
	types.register(content.Type{
		Key: "page", SingularLabel: "Page", PluralLabel: "Pages", RouteWord: "pages",
		PageKind: content.PageKindSingle, Hierarchical: true, Active: true,
	})
	handler := server.NewServer(server.Config{Users: users, Content: posts, Types: types})
	cookie := loginCookie(t, handler)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
	return authed, posts, types, ada
}

// nestedPages returns a stored parent page and a child page beneath it.
func nestedPages(t *testing.T, posts *fakePostStore, author uuid.UUID) (content.Content, content.Content) {
	t.Helper()
	pageType := content.Type{
		Key: "page", SingularLabel: "Page", PluralLabel: "Pages", RouteWord: "pages",
		PageKind: content.PageKindSingle, Hierarchical: true, Active: true,
	}
	parent, err := content.New(pageType, nil, "A Parent", author)
	if err != nil {
		t.Fatalf("New(parent) error = %v, want nil", err)
	}
	child, err := content.New(pageType, nil, "A Child", author)
	if err != nil {
		t.Fatalf("New(child) error = %v, want nil", err)
	}
	return posts.add(parent), posts.add(child)
}

func TestPatchReportsANestingItCannotResolve(t *testing.T) {
	t.Parallel()

	for name, breakIt := range map[string]func(*fakePostStore, *fakeTypeStore, uuid.UUID){
		"a parent the store will not answer for": func(s *fakePostStore, _ *fakeTypeStore, parent uuid.UUID) {
			s.byIDErrFor = map[uuid.UUID]error{parent: context.DeadlineExceeded}
		},
		"a registry it cannot read": func(_ *fakePostStore, types *fakeTypeStore, _ uuid.UUID) {
			types.listErr = context.DeadlineExceeded
		},
		"a depth it cannot measure": func(s *fakePostStore, _ *fakeTypeStore, _ uuid.UUID) {
			s.depthErr = context.DeadlineExceeded
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, posts, types, ada := nestingServer(t)
			parent, child := nestedPages(t, posts, ada.ID)
			breakIt(posts, types, parent.ID)

			recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+child.ID.String(),
				versionedBody(t, child.UpdatedAt, map[string]any{"parent_id": parent.ID.String()}))

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
			}
		})
	}
}

func TestPatchLeavesTheParentAloneWhenTheBodyNamesTheSameOne(t *testing.T) {
	t.Parallel()

	handler, posts, _, ada := nestingServer(t)
	parent, child := nestedPages(t, posts, ada.ID)
	moved := doRequest(t, handler, http.MethodPatch, "/api/content/"+child.ID.String(),
		versionedBody(t, child.UpdatedAt, map[string]any{"parent_id": parent.ID.String()}))
	if moved.Code != http.StatusOK {
		t.Fatalf("the first move answered %d, want %d", moved.Code, http.StatusOK)
	}
	held := posts.posts[child.ID]

	again := doRequest(t, handler, http.MethodPatch, "/api/content/"+child.ID.String(),
		versionedBody(t, held.UpdatedAt, map[string]any{"parent_id": parent.ID.String()}))

	if again.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", again.Code, http.StatusOK)
	}
}

func TestPatchRefusesAnItemPointingAtItself(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	ada := addAda(t, users)
	posts := newFakePostStore()
	types := newFakeTypeStore()
	types.register(content.Type{
		Key: content.TypePost, SingularLabel: "Post", PluralLabel: "Posts", Default: true, Active: true,
		PageKind: content.PageKindSingle,
		Fields: []content.Field{{
			TypeKey: content.TypePost, Key: "related", Label: "Related",
			Kind: content.FieldKindRelation, RelatesTo: content.TypePost, Many: true,
		}},
	})
	handler := server.NewServer(server.Config{Users: users, Content: posts, Types: types})
	cookie := loginCookie(t, handler)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
	stored := posts.add(newPost(t, "Original", ada.ID))

	recorder := doRequest(t, authed, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{
			"fields": map[string]any{"related": []any{stored.ID.String()}},
		}))

	if recorder.Code == http.StatusOK {
		t.Errorf("status = %d, want an item pointing at itself refused", recorder.Code)
	}
}
