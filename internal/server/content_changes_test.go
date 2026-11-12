// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// savedWrite is one write the server handed over: the status and title before and after, after empty for a delete.
type savedWrite struct {
	before, beforeTitle string
	after, afterTitle   string
	lasting             bool
}

var _ server.ContentChanges = &recordedChanges{}

// recordedChanges keeps each saved write the server hands over.
type recordedChanges struct {
	mu    sync.Mutex
	saved []savedWrite
}

// Saved keeps the write, and whether its context outlives the request.
func (r *recordedChanges) Saved(ctx context.Context, before content.Content, after *content.Content) {
	r.mu.Lock()
	defer r.mu.Unlock()
	write := savedWrite{before: string(before.Status), beforeTitle: before.Title, lasting: ctx.Done() == nil}
	if after != nil {
		write.after, write.afterTitle = string(after.Status), after.Title
	}
	r.saved = append(r.saved, write)
}

// writes returns the writes the server handed over.
func (r *recordedChanges) writes() []savedWrite {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saved
}

// changesServer returns a signed-in handler that hands each saved write to the returned changes, and its post store.
func changesServer(t *testing.T) (http.Handler, *fakePostStore, *recordedChanges, gouncer.User) {
	t.Helper()
	users := newFakeUserStore()
	ada := addAda(t, users)
	posts := newFakePostStore()
	posts.authors = map[uuid.UUID]string{ada.ID: ada.Name}
	changes := &recordedChanges{}
	cfg := serverConfig(users, posts)
	cfg.ContentChanges = changes
	return authedServerWithStores(t, cfg), posts, changes, ada
}

// storedPost stores a post titled title in status, written by author.
func storedPost(
	t *testing.T, posts *fakePostStore, author gouncer.User, title string, status content.Status,
) content.Content {
	t.Helper()
	held := newPost(t, title, author.ID)
	held.Status = status
	return posts.add(held)
}

func TestAnEditHandsOverTheItemBeforeAndAfterIt(t *testing.T) {
	t.Parallel()

	handler, posts, changes, ada := changesServer(t)
	stored := storedPost(t, posts, ada, "Hello", content.StatusDraft)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{"title": "Hello again", "status": "published"}))

	want := []savedWrite{{
		before: "draft", beforeTitle: "Hello", after: "published", afterTitle: "Hello again", lasting: true,
	}}
	if recorder.Code != http.StatusOK || !slices.Equal(changes.writes(), want) {
		t.Errorf("PATCH answered %d and handed over %+v, want 200 and %+v", recorder.Code, changes.writes(), want)
	}
}

func TestATrashHandsOverTheItemBeforeAndAfterIt(t *testing.T) {
	t.Parallel()

	handler, posts, changes, ada := changesServer(t)
	stored := storedPost(t, posts, ada, "Hello", content.StatusPublished)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/content/"+stored.ID.String(), "")

	want := []savedWrite{{before: "published", beforeTitle: "Hello", after: "trash", afterTitle: "Hello", lasting: true}}
	if recorder.Code != http.StatusOK || !slices.Equal(changes.writes(), want) {
		t.Errorf("DELETE answered %d and handed over %+v, want 200 and %+v", recorder.Code, changes.writes(), want)
	}
}

func TestADeleteForGoodHandsOverTheItemBeforeItAlone(t *testing.T) {
	t.Parallel()

	handler, posts, changes, ada := changesServer(t)
	stored := storedPost(t, posts, ada, "Hello", content.StatusPublished)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/content/"+stored.ID.String()+"?force=true", "")

	want := []savedWrite{{before: "published", beforeTitle: "Hello", lasting: true}}
	if recorder.Code != http.StatusNoContent || !slices.Equal(changes.writes(), want) {
		t.Errorf("DELETE answered %d and handed over %+v, want 204 and %+v", recorder.Code, changes.writes(), want)
	}
}

func TestAWriteThatFailsHandsOverNothing(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		fail   func(*fakePostStore)
		method string
		query  string
		edit   bool
	}{
		"an edit": {
			fail: func(posts *fakePostStore) { posts.updateErr = content.ErrConflict }, method: http.MethodPatch, edit: true,
		},
		"a trash": {fail: func(posts *fakePostStore) { posts.trashErr = content.ErrConflict }, method: http.MethodDelete},
		"a delete for good": {
			fail:   func(posts *fakePostStore) { posts.deleteErr = content.ErrNotFound },
			method: http.MethodDelete, query: "?force=true",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, posts, changes, ada := changesServer(t)
			stored := storedPost(t, posts, ada, "Hello", content.StatusPublished)
			tc.fail(posts)
			body := ""
			if tc.edit {
				body = versionedBody(t, stored.UpdatedAt, map[string]any{"title": "Hello again"})
			}

			recorder := doRequest(t, handler, tc.method, "/api/content/"+stored.ID.String()+tc.query, body)

			if recorder.Code < http.StatusBadRequest || len(changes.writes()) != 0 {
				t.Errorf("%s answered %d and handed over %+v, want a failure and nothing", name, recorder.Code,
					changes.writes())
			}
		})
	}
}

func TestAnEditThatChangesNothingHandsOverNothing(t *testing.T) {
	t.Parallel()

	handler, posts, changes, ada := changesServer(t)
	stored := storedPost(t, posts, ada, "Hello", content.StatusPublished)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{"title": "Hello"}))

	if recorder.Code != http.StatusOK || len(changes.writes()) != 0 {
		t.Errorf("PATCH answered %d and handed over %+v, want 200 and nothing", recorder.Code, changes.writes())
	}
}

func TestACreateAndARestoreHandOverNothing(t *testing.T) {
	t.Parallel()

	handler, posts, changes, ada := changesServer(t)
	trashed := storedPost(t, posts, ada, "Gone", content.StatusTrash)

	created := doRequest(t, handler, http.MethodPost, "/api/content", `{"type":"post","title":"Fresh"}`)
	restored := doRequest(t, handler, http.MethodPost, "/api/content/"+trashed.ID.String()+"/restore", "")

	if created.Code != http.StatusCreated || restored.Code != http.StatusOK || len(changes.writes()) != 0 {
		t.Errorf("create answered %d, restore %d, and %+v was handed over, want 201, 200 and nothing",
			created.Code, restored.Code, changes.writes())
	}
}

func TestASiteWithoutContentChangesStillSaves(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Hello", ada.ID))

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{"status": "published"}))

	if recorder.Code != http.StatusOK {
		t.Errorf("PATCH answered %d, want 200 with no one hearing the change", recorder.Code)
	}
}
