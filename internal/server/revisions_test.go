// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
)

type revisionBody struct {
	ID         uuid.UUID `json:"id"`
	PostID     uuid.UUID `json:"post_id"`
	Kind       string    `json:"kind"`
	AuthorID   uuid.UUID `json:"author_id"`
	AuthorName string    `json:"author_name"`
	Title      string    `json:"title"`
	Excerpt    string    `json:"excerpt"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

type revisionListBody struct {
	Items []revisionBody `json:"items"`
}

func TestPostPatchSnapshotsThePreviousContent(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "First Title", ada.ID))

	doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"title":"Second Title"}`)

	if posts.lastSnapshot == nil {
		t.Fatal("no snapshot passed to the store, want the previous state captured")
	}
	if posts.lastSnapshot.Title != "First Title" {
		t.Errorf("snapshot title = %q, want the state before the edit", posts.lastSnapshot.Title)
	}
	if posts.lastSnapshot.Kind != post.RevisionKindRevision {
		t.Errorf("snapshot kind = %q, want %q", posts.lastSnapshot.Kind, post.RevisionKindRevision)
	}
	if posts.lastSnapshot.AuthorID != ada.ID || posts.lastSnapshot.PostID != stored.ID {
		t.Errorf("snapshot = %+v, want it credited to the editor and bound to the post", posts.lastSnapshot)
	}
	if posts.lastRevisionCap != 100 {
		t.Errorf("revision cap = %d, want the registry's 100", posts.lastRevisionCap)
	}
}

func TestPostPatchWithoutContentChangesSkipsTheSnapshot(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Only Status", ada.ID))

	doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"status":"published"}`)

	if posts.lastSnapshot != nil {
		t.Errorf("snapshot = %+v, want a status change to store none", posts.lastSnapshot)
	}
}

func TestPostPatchSlugChangeSkipsTheSnapshot(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Slug Only", ada.ID))

	doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"slug":"new-slug"}`)

	if posts.lastSnapshot != nil {
		t.Errorf("snapshot = %+v, want a slug change to store none", posts.lastSnapshot)
	}
}

func TestPostPatchSkipsTheSnapshotForTypesWithoutRevisions(t *testing.T) {
	t.Parallel()

	post.Register(post.Type{Name: "briefing", Label: "Briefings"})
	handler, posts, ada := authedPostServer(t)
	stored := newPost(t, "No History", ada.ID)
	stored.Type = "briefing"
	stored = posts.add(stored)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"title":"Edited"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if posts.lastSnapshot != nil {
		t.Errorf("snapshot = %+v, want a type without revisions to store none", posts.lastSnapshot)
	}
}

func TestPostPatchSkipsTheSnapshotForUnregisteredTypes(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := newPost(t, "Ghost Type", ada.ID)
	stored.Type = "vanished-plugin"
	stored = posts.add(stored)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"title":"Edited"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if posts.lastSnapshot != nil {
		t.Errorf("snapshot = %+v, want an unregistered type to store none", posts.lastSnapshot)
	}
}

func TestRevisionListReturnsNewestFirstWithoutContent(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Revised", ada.ID))
	older := mustRevision(t, stored, "Older", ada.ID)
	newer := mustRevision(t, stored, "Newer", ada.ID)
	newer.CreatedAt = older.CreatedAt.Add(time.Minute)
	posts.revisions = append(posts.revisions, older, newer)

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/revisions", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[revisionListBody](t, recorder)
	if len(body.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(body.Items))
	}
	if body.Items[0].Title != "Newer" {
		t.Errorf("first item = %q, want the newest revision", body.Items[0].Title)
	}
	if body.Items[0].Content != "" {
		t.Errorf("Content = %q, want listings to omit it", body.Items[0].Content)
	}
	if body.Items[0].AuthorName != "Ada Lovelace" {
		t.Errorf("AuthorName = %q, want it resolved", body.Items[0].AuthorName)
	}
}

func TestRevisionGetReturnsTheContent(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Revised", ada.ID))
	revision := mustRevision(t, stored, "Snapshot", ada.ID)
	revision.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	posts.revisions = append(posts.revisions, revision)

	recorder := doRequest(
		t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/revisions/"+revision.ID.String(), "",
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[revisionBody](t, recorder)
	if body.Content != revision.Content {
		t.Errorf("Content = %q, want the snapshotted body", body.Content)
	}
}

func TestRevisionDeleteRemovesIt(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Revised", ada.ID))
	revision := mustRevision(t, stored, "Doomed", ada.ID)
	posts.revisions = append(posts.revisions, revision)

	recorder := doRequest(
		t, handler, http.MethodDelete, "/api/posts/"+stored.ID.String()+"/revisions/"+revision.ID.String(), "",
	)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if len(posts.revisions) != 0 {
		t.Errorf("revisions = %d, want it removed", len(posts.revisions))
	}
}

func TestRevisionRoutesRejectUnknownAndMalformedIDs(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Revised", ada.ID))
	missing := uuid.Must(uuid.NewV7()).String()

	tests := map[string]struct {
		method string
		path   string
		want   int
	}{
		"missing revision": {
			http.MethodGet, "/api/posts/" + stored.ID.String() + "/revisions/" + missing, http.StatusNotFound,
		},
		"missing revision on delete": {
			http.MethodDelete, "/api/posts/" + stored.ID.String() + "/revisions/" + missing, http.StatusNotFound,
		},
		"malformed post id": {
			http.MethodGet, "/api/posts/not-a-uuid/revisions", http.StatusBadRequest,
		},
		"malformed revision id": {
			http.MethodGet, "/api/posts/" + stored.ID.String() + "/revisions/not-a-uuid", http.StatusBadRequest,
		},
		"malformed revision id on delete": {
			http.MethodDelete, "/api/posts/" + stored.ID.String() + "/revisions/not-a-uuid", http.StatusBadRequest,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if code := doRequest(t, handler, tc.method, tc.path, "").Code; code != tc.want {
				t.Errorf("%s %s = %d, want %d", tc.method, tc.path, code, tc.want)
			}
		})
	}
}

func TestRevisionRoutesReportStoreFailures(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Revised", ada.ID))
	revision := mustRevision(t, stored, "Snapshot", ada.ID)
	posts.revisions = append(posts.revisions, revision)
	posts.revisionsErr = context.DeadlineExceeded
	posts.revisionErr = context.DeadlineExceeded
	posts.deleteRevisionErr = context.DeadlineExceeded

	list := doRequest(t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/revisions", "")
	get := doRequest(
		t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/revisions/"+revision.ID.String(), "",
	)
	remove := doRequest(
		t, handler, http.MethodDelete, "/api/posts/"+stored.ID.String()+"/revisions/"+revision.ID.String(), "",
	)

	for name, recorder := range map[string]int{"list": list.Code, "get": get.Code, "delete": remove.Code} {
		if recorder != http.StatusInternalServerError {
			t.Errorf("%s status = %d, want %d", name, recorder, http.StatusInternalServerError)
		}
	}
}

func TestRevisionRoutesRequireASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	unauthed := serverWithStores(users, newFakePostStore())
	id := uuid.Must(uuid.NewV7()).String()

	for _, target := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/posts/" + id + "/revisions"},
		{http.MethodGet, "/api/posts/" + id + "/revisions/" + id},
		{http.MethodDelete, "/api/posts/" + id + "/revisions/" + id},
	} {
		if code := doRequest(t, unauthed, target.method, target.path, "").Code; code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want %d", target.method, target.path, code, http.StatusUnauthorized)
		}
	}
}

func TestRevisionListReportsAuthorLookupFailures(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	ada := addAda(t, users)
	posts := newFakePostStore()
	stored := posts.add(newPost(t, "Revised", ada.ID))
	handler := authedServerWithStores(t, serverConfig(failingUserStore{Store: users}, posts))

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/revisions", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

// mustRevision returns a revision of the post credited to author.
func mustRevision(t *testing.T, p post.Post, title string, author uuid.UUID) post.Revision {
	t.Helper()
	snapshot := p
	snapshot.Title = title
	revision, err := post.NewRevision(snapshot, post.RevisionKindRevision, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	return revision
}

func TestRevisionGetReportsAuthorLookupFailures(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	ada := addAda(t, users)
	posts := newFakePostStore()
	stored := posts.add(newPost(t, "Revised", ada.ID))
	revision := mustRevision(t, stored, "Snapshot", ada.ID)
	posts.revisions = append(posts.revisions, revision)
	handler := authedServerWithStores(t, serverConfig(failingUserStore{Store: users}, posts))

	recorder := doRequest(
		t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/revisions/"+revision.ID.String(), "",
	)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestRevisionRoutesRejectAMalformedPostID(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)
	id := uuid.Must(uuid.NewV7()).String()

	get := doRequest(t, handler, http.MethodGet, "/api/posts/not-a-uuid/revisions/"+id, "")
	remove := doRequest(t, handler, http.MethodDelete, "/api/posts/not-a-uuid/revisions/"+id, "")

	if get.Code != http.StatusBadRequest || remove.Code != http.StatusBadRequest {
		t.Errorf("statuses = %d and %d, want %d", get.Code, remove.Code, http.StatusBadRequest)
	}
}

func TestPostPatchReportsSnapshotFailures(t *testing.T) {
	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Doomed", ada.ID))
	uuid.SetRand(failingReader{})
	defer uuid.SetRand(nil)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"title":"Edited"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

// failingReader is a random source that always fails.
type failingReader struct{}

// Read reports a failure.
func (failingReader) Read([]byte) (int, error) {
	return 0, errEntropy
}

var errEntropy = errors.New("entropy source failed")
