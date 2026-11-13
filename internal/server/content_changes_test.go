// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// savedWrite is one write the server handed over: the status and title before and after, after empty for a delete.
type savedWrite struct {
	before, beforeTitle string
	after, afterTitle   string
	// storedTitle is the title the store held when the server readied the write, empty when it held none.
	storedTitle string
	// held reports whether the item stayed held from when the write was readied until it was handed over.
	held    bool
	lasting bool
}

var _ server.ContentChanges = &recordedChanges{}

// recordedChanges keeps each saved write the server hands over and counts the items it holds.
type recordedChanges struct {
	mu    sync.Mutex
	posts *fakePostStore
	// meanwhile is a write that lands while the server waits to hold the item, nil for none.
	meanwhile func(id uuid.UUID)
	holds     int
	saved     []savedWrite
}

// Hold lets the write that lands meanwhile through, then counts the item held until release.
func (r *recordedChanges) Hold(id uuid.UUID) (release func()) {
	if r.meanwhile != nil {
		r.meanwhile(id)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.holds++
	return func() {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.holds--
	}
}

// Before notes what the store holds and whether the item is held, and returns what keeps the write.
func (r *recordedChanges) Before(ctx context.Context, before content.Content) func(context.Context, *content.Content) {
	r.mu.Lock()
	heldBefore := r.holds > 0
	r.mu.Unlock()
	stored := r.posts.posts[before.ID].Title
	lastingBefore := ctx.Done() == nil
	return func(ctx context.Context, after *content.Content) {
		r.mu.Lock()
		defer r.mu.Unlock()
		write := savedWrite{
			before: string(before.Status), beforeTitle: before.Title, storedTitle: stored,
			held: heldBefore && r.holds > 0, lasting: lastingBefore && ctx.Done() == nil,
		}
		if after != nil {
			write.after, write.afterTitle = string(after.Status), after.Title
		}
		r.saved = append(r.saved, write)
	}
}

// writes returns the writes the server handed over.
func (r *recordedChanges) writes() []savedWrite {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saved
}

// holding returns how many items the server holds and has not released.
func (r *recordedChanges) holding() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.holds
}

// changesServer returns a signed-in handler that hands each saved write to the returned changes, and its post store.
func changesServer(t *testing.T) (http.Handler, *fakePostStore, *recordedChanges, gouncer.User) {
	t.Helper()
	users := newFakeUserStore()
	ada := addAda(t, users)
	posts := newFakePostStore()
	posts.authors = map[uuid.UUID]string{ada.ID: ada.Name}
	changes := &recordedChanges{posts: posts}
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
		before: "draft", beforeTitle: "Hello", after: "published", afterTitle: "Hello again", storedTitle: "Hello",
		held: true, lasting: true,
	}}
	if recorder.Code != http.StatusOK || !slices.Equal(changes.writes(), want) || changes.holding() != 0 {
		t.Errorf("PATCH answered %d, handed over %+v and still holds %d, want 200, %+v and none", recorder.Code,
			changes.writes(), changes.holding(), want)
	}
}

func TestATrashHandsOverTheItemBeforeAndAfterIt(t *testing.T) {
	t.Parallel()

	handler, posts, changes, ada := changesServer(t)
	stored := storedPost(t, posts, ada, "Hello", content.StatusPublished)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/content/"+stored.ID.String(), "")

	want := []savedWrite{{
		before: "published", beforeTitle: "Hello", after: "trash", afterTitle: "Hello", storedTitle: "Hello",
		held: true, lasting: true,
	}}
	if recorder.Code != http.StatusOK || !slices.Equal(changes.writes(), want) || changes.holding() != 0 {
		t.Errorf("DELETE answered %d, handed over %+v and still holds %d, want 200, %+v and none", recorder.Code,
			changes.writes(), changes.holding(), want)
	}
}

func TestADeleteForGoodHandsOverTheItemBeforeItAlone(t *testing.T) {
	t.Parallel()

	handler, posts, changes, ada := changesServer(t)
	stored := storedPost(t, posts, ada, "Hello", content.StatusPublished)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/content/"+stored.ID.String()+"?force=true", "")

	want := []savedWrite{{before: "published", beforeTitle: "Hello", storedTitle: "Hello", held: true, lasting: true}}
	if recorder.Code != http.StatusNoContent || !slices.Equal(changes.writes(), want) || changes.holding() != 0 {
		t.Errorf("DELETE answered %d, handed over %+v and still holds %d, want 204, %+v and none", recorder.Code,
			changes.writes(), changes.holding(), want)
	}
}

func TestAWriteReadsTheItemOnlyOnceItHoldsIt(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		method string
		edit   bool
	}{
		"an edit":  {method: http.MethodPatch, edit: true},
		"a delete": {method: http.MethodDelete},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, posts, changes, ada := changesServer(t)
			stored := storedPost(t, posts, ada, "Hello", content.StatusPublished)
			landedAt := stored.UpdatedAt.Add(time.Second)
			changes.meanwhile = func(id uuid.UUID) {
				landed := posts.posts[id]
				landed.Title, landed.UpdatedAt = "Edited meanwhile", landedAt
				posts.posts[id] = landed
			}
			body := ""
			if tc.edit {
				body = versionedBody(t, landedAt, map[string]any{"title": "Hello again"})
			}

			recorder := doRequest(t, handler, tc.method, "/api/content/"+stored.ID.String(), body)

			written := changes.writes()
			if recorder.Code != http.StatusOK || len(written) != 1 || written[0].beforeTitle != "Edited meanwhile" {
				t.Errorf("%s answered %d and handed over %+v, want 200 and the item after the write that landed",
					name, recorder.Code, written)
			}
		})
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
		"a read": {fail: func(posts *fakePostStore) { posts.byIDErr = content.ErrNotFound }, method: http.MethodDelete},
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

			if recorder.Code < http.StatusBadRequest || len(changes.writes()) != 0 || changes.holding() != 0 {
				t.Errorf("%s answered %d, handed over %+v and still holds %d, want a failure, nothing and none", name,
					recorder.Code, changes.writes(), changes.holding())
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

	if recorder.Code != http.StatusOK || len(changes.writes()) != 0 || changes.holding() != 0 {
		t.Errorf("PATCH answered %d, handed over %+v and still holds %d, want 200, nothing and none", recorder.Code,
			changes.writes(), changes.holding())
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
	gone := posts.add(newPost(t, "Gone", ada.ID))

	patched := doRequest(t, handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{"status": "published"}))
	deleted := doRequest(t, handler, http.MethodDelete, "/api/content/"+gone.ID.String()+"?force=true", "")

	if patched.Code != http.StatusOK || deleted.Code != http.StatusNoContent {
		t.Errorf("PATCH answered %d and DELETE %d, want 200 and 204 with no one hearing the change", patched.Code,
			deleted.Code)
	}
}
