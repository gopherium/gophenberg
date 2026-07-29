// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
)

type autosaveBody struct {
	Target  string    `json:"target"`
	PostID  uuid.UUID `json:"post_id"`
	Title   string    `json:"title"`
	Content string    `json:"content"`
	Excerpt string    `json:"excerpt"`
	SavedAt time.Time `json:"saved_at"`
}

const autosavePayload = `{"title":"Buffered","content":"<!-- wp:paragraph --><p>Draft</p><!-- /wp:paragraph -->",` +
	`"excerpt":"Summary"}`

func TestAutosaveUpdatesTheRequestersOwnDraftInPlace(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Original", ada.ID))

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave", autosavePayload)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[autosaveBody](t, recorder)
	if body.Target != "post" {
		t.Errorf("target = %q, want the draft written in place", body.Target)
	}
	if posts.posts[stored.ID].Title != "Buffered" {
		t.Errorf("stored title = %q, want the buffer", posts.posts[stored.ID].Title)
	}
	if len(posts.revisions) != 0 {
		t.Errorf("revisions = %d, want an in-place autosave to store none", len(posts.revisions))
	}
	if posts.lastSnapshot != nil {
		t.Errorf("snapshot = %+v, want autosaves to skip revision snapshots", posts.lastSnapshot)
	}
}

func TestAutosaveParksAnotherAuthorsPost(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Someone Else", uuid.Must(uuid.NewV7())))

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave", autosavePayload)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[autosaveBody](t, recorder)
	if body.Target != "autosave" {
		t.Errorf("target = %q, want the buffer parked in an autosave", body.Target)
	}
	if posts.posts[stored.ID].Title != "Someone Else" {
		t.Errorf("stored title = %q, want the post untouched", posts.posts[stored.ID].Title)
	}
	if len(posts.revisions) != 1 || posts.revisions[0].AuthorID != ada.ID {
		t.Errorf("revisions = %+v, want one autosave credited to the requester", posts.revisions)
	}
	if posts.revisions[0].Kind != post.RevisionKindAutosave {
		t.Errorf("kind = %q, want %q", posts.revisions[0].Kind, post.RevisionKindAutosave)
	}
}

func TestAutosaveParksAPublishedPostOfTheRequester(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	published := newPost(t, "Published", ada.ID)
	published.Status = post.StatusPublished
	posts.add(published)

	recorder := doRequest(
		t, handler, http.MethodPost, "/api/posts/"+published.ID.String()+"/autosave", autosavePayload,
	)

	body := decodeBody[autosaveBody](t, recorder)
	if body.Target != "autosave" {
		t.Errorf("target = %q, want a published post to park the buffer", body.Target)
	}
	if posts.posts[published.ID].Title != "Published" {
		t.Errorf("stored title = %q, want the published post untouched", posts.posts[published.ID].Title)
	}
}

func TestAutosaveOverwritesTheWholeBuffer(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := newPost(t, "Own Draft", ada.ID)
	stored.Excerpt = "Summary"
	posts.add(stored)

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave",
		`{"title":"Own Draft","content":"<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if got := posts.posts[stored.ID].Excerpt; got != "" {
		t.Errorf("Excerpt = %q, want the omitted key to blank the field", got)
	}
}

func TestAutosaveSkipsAnUnchangedBuffer(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := newPost(t, "Unchanged", ada.ID)
	stored.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	stored.Excerpt = "Summary"
	posts.add(stored)
	before := posts.posts[stored.ID].UpdatedAt

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave",
		`{"title":"Unchanged","content":"<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->","excerpt":"Summary"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if len(posts.revisions) != 0 {
		t.Errorf("revisions = %d, want an unchanged buffer to store none", len(posts.revisions))
	}
	if !posts.posts[stored.ID].UpdatedAt.Equal(before) {
		t.Error("an unchanged buffer stamped the post, want it left alone")
	}
}

func TestAutosaveClearsTheParkedBufferOnceTheEditorMatchesThePost(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)
	stored := posts.add(newPost(t, "Shared", uuid.Must(uuid.NewV7())))
	doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave", autosavePayload)

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave",
		`{"title":"Shared","content":"","excerpt":""}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body := decodeBody[autosaveBody](t, recorder); body.Target != "post" {
		t.Errorf("target = %q, want the converged buffer reported as the post", body.Target)
	}
	if len(posts.revisions) != 0 {
		t.Errorf("revisions = %d, want the parked buffer cleared", len(posts.revisions))
	}
	read := doRequest(t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/autosave", "")
	if read.Code != http.StatusNotFound {
		t.Errorf("GET autosave after converging = %d, want %d", read.Code, http.StatusNotFound)
	}
}

func TestAutosaveReportsConflictingEdits(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Contended", ada.ID))
	posts.updateErr = post.ErrConflict

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave", autosavePayload)

	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestAutosaveReportsBufferCleanupFailures(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Unchanged", ada.ID))
	posts.deleteAutosaveErr = context.DeadlineExceeded

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave",
		`{"title":"Unchanged","content":"","excerpt":""}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestAutosaveKeepsOneBufferPerAuthor(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)
	stored := posts.add(newPost(t, "Shared", uuid.Must(uuid.NewV7())))

	doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave", autosavePayload)
	doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave",
		`{"title":"Newer Buffer","content":"body","excerpt":""}`)

	if len(posts.revisions) != 1 {
		t.Fatalf("revisions = %d, want repeated autosaves to replace one another", len(posts.revisions))
	}
	if posts.revisions[0].Title != "Newer Buffer" {
		t.Errorf("Title = %q, want the newest buffer", posts.revisions[0].Title)
	}
}

func TestAutosaveKeepsTheRowIDAcrossReplacements(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)
	stored := posts.add(newPost(t, "Shared", uuid.Must(uuid.NewV7())))

	doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave", autosavePayload)
	first := posts.revisions[0].ID
	doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave",
		`{"title":"Newer Buffer","content":"body","excerpt":""}`)

	if posts.revisions[0].ID != first {
		t.Errorf("ID = %s, want the replaced buffer to keep row id %s", posts.revisions[0].ID, first)
	}
}

func TestAutosaveGetReturnsTheRequestersBuffer(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)
	stored := posts.add(newPost(t, "Shared", uuid.Must(uuid.NewV7())))
	doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave", autosavePayload)

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/autosave", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[autosaveBody](t, recorder)
	if body.Target != "autosave" || body.Title != "Buffered" {
		t.Errorf("body = %+v, want the stored buffer", body)
	}
	if body.Content == "" {
		t.Error("Content is empty, want the buffered body")
	}
}

func TestRevisionEndpointsExposeTheParkedAutosave(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)
	stored := posts.add(newPost(t, "Shared", uuid.Must(uuid.NewV7())))
	doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave", autosavePayload)

	listed := decodeBody[revisionListBody](t, doRequest(t, handler, http.MethodGet,
		"/api/posts/"+stored.ID.String()+"/revisions", ""))

	if len(listed.Items) != 1 || listed.Items[0].Kind != "autosave" {
		t.Fatalf("items = %+v, want the parked autosave listed", listed.Items)
	}
	deleted := doRequest(t, handler, http.MethodDelete,
		"/api/posts/"+stored.ID.String()+"/revisions/"+listed.Items[0].ID.String(), "")
	if deleted.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d", deleted.Code, http.StatusNoContent)
	}
	read := doRequest(t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/autosave", "")
	if read.Code != http.StatusNotFound {
		t.Errorf("GET autosave after revision delete = %d, want %d", read.Code, http.StatusNotFound)
	}
}

func TestAutosaveGetReportsAnAbsentBuffer(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Unsaved", ada.ID))

	recorder := doRequest(t, handler, http.MethodGet, "/api/posts/"+stored.ID.String()+"/autosave", "")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestAutosaveRejectsUnknownAndMalformedRequests(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)
	missing := uuid.Must(uuid.NewV7()).String()

	tests := map[string]struct {
		method string
		path   string
		body   string
		want   int
	}{
		"missing post on save": {
			http.MethodPost, "/api/posts/" + missing + "/autosave", autosavePayload, http.StatusNotFound,
		},
		"missing post on read": {
			http.MethodGet, "/api/posts/" + missing + "/autosave", "", http.StatusNotFound,
		},
		"malformed post id": {
			http.MethodPost, "/api/posts/not-a-uuid/autosave", autosavePayload, http.StatusBadRequest,
		},
		"malformed post id on read": {
			http.MethodGet, "/api/posts/not-a-uuid/autosave", "", http.StatusBadRequest,
		},
		"malformed body": {
			http.MethodPost, "/api/posts/" + missing + "/autosave", `{`, http.StatusBadRequest,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if code := doRequest(t, handler, tc.method, tc.path, tc.body).Code; code != tc.want {
				t.Errorf("%s %s = %d, want %d", tc.method, tc.path, code, tc.want)
			}
		})
	}
}

func TestAutosaveRoutesRequireASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	unauthed := serverWithStores(users, newFakePostStore())
	id := uuid.Must(uuid.NewV7()).String()

	for _, target := range []struct {
		method string
		body   string
	}{
		{http.MethodPost, autosavePayload},
		{http.MethodGet, ""},
	} {
		recorder := doRequest(t, unauthed, target.method, "/api/posts/"+id+"/autosave", target.body)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s autosave = %d, want %d", target.method, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestAutosaveReportsStoreFailures(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	own := posts.add(newPost(t, "Own Draft", ada.ID))
	other := posts.add(newPost(t, "Other", uuid.Must(uuid.NewV7())))
	posts.updateErr = context.DeadlineExceeded
	posts.saveAutosaveErr = context.DeadlineExceeded
	posts.autosaveErr = context.DeadlineExceeded

	inPlace := doRequest(t, handler, http.MethodPost, "/api/posts/"+own.ID.String()+"/autosave", autosavePayload)
	parked := doRequest(t, handler, http.MethodPost, "/api/posts/"+other.ID.String()+"/autosave", autosavePayload)
	read := doRequest(t, handler, http.MethodGet, "/api/posts/"+own.ID.String()+"/autosave", "")

	for name, code := range map[string]int{"in place": inPlace.Code, "parked": parked.Code, "read": read.Code} {
		if code != http.StatusInternalServerError {
			t.Errorf("%s status = %d, want %d", name, code, http.StatusInternalServerError)
		}
	}
}

func TestAutosaveReportsSnapshotFailures(t *testing.T) {
	handler, posts, _ := authedPostServer(t)
	stored := posts.add(newPost(t, "Other", uuid.Must(uuid.NewV7())))
	uuid.SetRand(failingReader{})
	defer uuid.SetRand(nil)

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/autosave", autosavePayload)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
