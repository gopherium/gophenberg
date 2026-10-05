// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
)

// emptiedHeld is the answer to emptying a trash as its readers decode it.
type emptiedHeld struct {
	Deleted int `json:"deleted"`
	Kept    int `json:"kept"`
}

// trashServer returns a handler signed in under the role, the post store behind it and the signed in account.
func trashServer(t *testing.T, held string) (http.Handler, *fakePostStore, gouncer.User) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	maria := addWithRole(t, users, "maria@example.com", "Maria Perez", held)
	posts := newFakePostStore()
	handler := server.NewServer(serverConfig(users, posts))
	recorder := doLogin(t, handler, `{"email":"maria@example.com","password":"correct horse battery"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recorder.Code, http.StatusOK)
	}
	cookie := sessionCookie(t, recorder)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	}), posts, maria
}

// trashed stores a post already in the trash, written by the author.
func trashed(t *testing.T, posts *fakePostStore, title string, author uuid.UUID) content.Content {
	t.Helper()
	item := newPost(t, title, author)
	item.Status = content.StatusTrash
	return posts.add(item)
}

func TestEmptyTrashDeletesEveryTrashedItemOfTheType(t *testing.T) {
	t.Parallel()

	handler, posts, maria := trashServer(t, role.Editor)
	trashed(t, posts, "Mine", maria.ID)
	trashed(t, posts, "Someone Else's", uuid.Must(uuid.NewV7()))
	kept := posts.add(newPost(t, "Draft", maria.ID))

	recorder := doRequest(t, handler, http.MethodDelete, "/api/content/trash?type=post", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	if held := decodeBody[emptiedHeld](t, recorder); held.Deleted != 2 || held.Kept != 0 {
		t.Errorf("answered %+v, want 2 deleted and 0 kept", held)
	}
	if len(posts.posts) != 1 || posts.posts[kept.ID].Title != "Draft" {
		t.Errorf("stored %d items, want the draft alone", len(posts.posts))
	}
}

func TestEmptyTrashLeavesWhatAnAuthorMayNotChange(t *testing.T) {
	t.Parallel()

	handler, posts, maria := trashServer(t, role.Author)
	trashed(t, posts, "Mine", maria.ID)
	theirs := trashed(t, posts, "Someone Else's", uuid.Must(uuid.NewV7()))

	recorder := doRequest(t, handler, http.MethodDelete, "/api/content/trash?type=post", "")

	if held := decodeBody[emptiedHeld](t, recorder); held.Deleted != 1 || held.Kept != 1 {
		t.Errorf("answered %+v, want 1 deleted and 1 kept", held)
	}
	if _, stored := posts.posts[theirs.ID]; !stored || len(posts.posts) != 1 {
		t.Errorf("stored %d items, want only the other account's trashed post", len(posts.posts))
	}
}

func TestEmptyTrashRefusesATypeItCannotName(t *testing.T) {
	t.Parallel()

	for name, query := range map[string]string{
		"no type":                  "",
		"an empty type":            "?type=",
		"a type nobody registered": "?type=ghost",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, posts, maria := trashServer(t, role.Admin)
			trashed(t, posts, "Mine", maria.ID)

			recorder := doRequest(t, handler, http.MethodDelete, "/api/content/trash"+query, "")

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusUnprocessableEntity, recorder.Body)
			}
			if code := errorCode(t, recorder); code != "type_unknown" {
				t.Errorf("code = %q, want type_unknown", code)
			}
			if len(posts.posts) != 1 {
				t.Error("the trash was emptied, want it left alone")
			}
		})
	}
}

func TestEmptyTrashReportsAStoreThatWillNotDelete(t *testing.T) {
	t.Parallel()

	handler, posts, _ := trashServer(t, role.Admin)
	posts.emptyErr = context.DeadlineExceeded

	recorder := doRequest(t, handler, http.MethodDelete, "/api/content/trash?type=post", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestEmptyTrashNeedsASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(serverConfig(users, newFakePostStore()))

	recorder := doRequest(t, handler, http.MethodDelete, "/api/content/trash?type=post", "")

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
