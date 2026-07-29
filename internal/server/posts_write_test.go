// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/server"
)

func TestPostCreateStoresADraftAuthoredByTheRequester(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts", `{"title":"Hello World"}`)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusCreated, recorder.Body.String())
	}
	body := decodeBody[postBody](t, recorder)
	if body.Status != string(post.StatusDraft) || body.Slug != "hello-world" {
		t.Errorf("body = %+v, want a draft slugged hello-world", body)
	}
	if body.AuthorID != ada.ID || body.AuthorName != "Ada Lovelace" {
		t.Errorf("author = %v %q, want the requester", body.AuthorID, body.AuthorName)
	}
	if len(posts.posts) != 1 {
		t.Errorf("stored %d posts, want 1", len(posts.posts))
	}
}

func TestPostCreateRejectsUnknownTypesAndMalformedBodies(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)

	unknownType := doRequest(t, handler, http.MethodPost, "/api/posts", `{"title":"Hi","type":"ghost"}`)
	malformed := doRequest(t, handler, http.MethodPost, "/api/posts", `{`)

	if unknownType.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown type status = %d, want %d", unknownType.Code, http.StatusUnprocessableEntity)
	}
	if malformed.Code != http.StatusBadRequest {
		t.Errorf("malformed body status = %d, want %d", malformed.Code, http.StatusBadRequest)
	}
}

func TestPostCreateReportsStoreFailures(t *testing.T) {
	t.Parallel()

	handler, posts, _ := authedPostServer(t)
	posts.createErr = context.DeadlineExceeded

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts", `{"title":"Hello"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestPostPatchEditsFieldsAndStampsUpdatedAt(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Original", ada.ID))

	recorder := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(),
		`{"title":"Edited","content":"<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->","excerpt":"Sum"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[postBody](t, recorder)
	if body.Title != "Edited" || body.Excerpt != "Sum" {
		t.Errorf("body = %+v, want the edited fields", body)
	}
	if !body.UpdatedAt.After(stored.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want it stamped after %v", body.UpdatedAt, stored.UpdatedAt)
	}
	if body.Slug != "original" {
		t.Errorf("Slug = %q, want the title edit to leave it alone", body.Slug)
	}
}

func TestPostPatchReportsConflictingEdits(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Contended", ada.ID))
	posts.updateErr = post.ErrConflict

	recorder := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"title":"Edited"}`)

	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusConflict)
	}
}

func TestPostPatchPublishesAndKeepsTheOriginalDate(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "To Publish", ada.ID))

	published := decodeBody[postBody](t, doRequest(
		t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"status":"published"}`,
	))
	if published.PublishedAt == nil {
		t.Fatalf("PublishedAt = nil, want it stamped on publication")
	}
	first := *published.PublishedAt

	doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"status":"draft"}`)
	republished := decodeBody[postBody](t, doRequest(
		t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"status":"published"}`,
	))

	if republished.PublishedAt == nil || !republished.PublishedAt.Equal(first) {
		t.Errorf("PublishedAt = %v, want the original %v kept", republished.PublishedAt, first)
	}
}

func TestPostPatchRejectsUnreachableAndUnknownStatuses(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Draft", ada.ID))

	scheduled := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"status":"scheduled"}`)
	unknown := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"status":"publsh"}`)

	if scheduled.Code != http.StatusUnprocessableEntity {
		t.Errorf("scheduled status = %d, want %d", scheduled.Code, http.StatusUnprocessableEntity)
	}
	if unknown.Code != http.StatusUnprocessableEntity {
		t.Errorf("unknown status = %d, want %d", unknown.Code, http.StatusUnprocessableEntity)
	}
	if posts.posts[stored.ID].Status != post.StatusDraft {
		t.Errorf("stored status = %q, want it unchanged", posts.posts[stored.ID].Status)
	}
}

func TestPostPatchWithoutFieldsReturnsThePostUnchanged(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Untouched", ada.ID))

	recorder := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	body := decodeBody[postBody](t, recorder)
	if !body.UpdatedAt.Equal(stored.UpdatedAt) || body.Title != "Untouched" {
		t.Errorf("body = %+v, want the post untouched", body)
	}
}

func TestPostPatchNormalizesAnEditedSlug(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Original", ada.ID))

	recorder := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"slug":"A New Slug!"}`)

	body := decodeBody[postBody](t, recorder)
	if body.Slug != "a-new-slug" {
		t.Errorf("Slug = %q, want it normalized", body.Slug)
	}
}

func TestPostPatchRejectsUnknownAndMalformedRequests(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)
	missing := uuid.Must(uuid.NewV7()).String()

	notFound := doRequest(t, handler, http.MethodPatch, "/api/posts/"+missing, `{"title":"Edited"}`)
	malformedID := doRequest(t, handler, http.MethodPatch, "/api/posts/not-a-uuid", `{"title":"Edited"}`)
	malformedBody := doRequest(t, handler, http.MethodPatch, "/api/posts/"+missing, `{`)

	if notFound.Code != http.StatusNotFound {
		t.Errorf("missing post status = %d, want %d", notFound.Code, http.StatusNotFound)
	}
	if malformedID.Code != http.StatusBadRequest {
		t.Errorf("malformed id status = %d, want %d", malformedID.Code, http.StatusBadRequest)
	}
	if malformedBody.Code != http.StatusBadRequest {
		t.Errorf("malformed body status = %d, want %d", malformedBody.Code, http.StatusBadRequest)
	}
}

func TestPostPatchReportsStoreFailures(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Doomed", ada.ID))
	posts.updateErr = context.DeadlineExceeded

	recorder := doRequest(t, handler, http.MethodPatch, "/api/posts/"+stored.ID.String(), `{"title":"Edited"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestPostDeleteTrashesAndForceDeletes(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	trashed := posts.add(newPost(t, "To Trash", ada.ID))
	doomed := posts.add(newPost(t, "To Delete", ada.ID))

	trashRecorder := doRequest(t, handler, http.MethodDelete, "/api/posts/"+trashed.ID.String(), "")
	forceRecorder := doRequest(t, handler, http.MethodDelete, "/api/posts/"+doomed.ID.String()+"?force=true", "")

	if trashRecorder.Code != http.StatusOK {
		t.Fatalf("trash status = %d, want %d: %s", trashRecorder.Code, http.StatusOK, trashRecorder.Body.String())
	}
	if body := decodeBody[postBody](t, trashRecorder); body.Status != string(post.StatusTrash) {
		t.Errorf("trashed status = %q, want %q", body.Status, post.StatusTrash)
	}
	if _, ok := posts.posts[trashed.ID]; !ok {
		t.Error("trashing removed the post, want it recoverable")
	}
	if forceRecorder.Code != http.StatusNoContent {
		t.Errorf("force delete status = %d, want %d", forceRecorder.Code, http.StatusNoContent)
	}
	if _, ok := posts.posts[doomed.ID]; ok {
		t.Error("force delete left the post stored, want it gone")
	}
}

func TestPostDeleteRejectsUnknownAndMalformedIDs(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)

	missing := doRequest(t, handler, http.MethodDelete, "/api/posts/"+uuid.Must(uuid.NewV7()).String(), "")
	forceMissing := doRequest(
		t, handler, http.MethodDelete, "/api/posts/"+uuid.Must(uuid.NewV7()).String()+"?force=true", "",
	)
	malformed := doRequest(t, handler, http.MethodDelete, "/api/posts/not-a-uuid", "")

	if missing.Code != http.StatusNotFound || forceMissing.Code != http.StatusNotFound {
		t.Errorf("missing statuses = %d and %d, want %d", missing.Code, forceMissing.Code, http.StatusNotFound)
	}
	if malformed.Code != http.StatusBadRequest {
		t.Errorf("malformed id status = %d, want %d", malformed.Code, http.StatusBadRequest)
	}
}

func TestPostRestoreReturnsATrashedPostToDraft(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := newPost(t, "Trashed", ada.ID)
	stored.Status = post.StatusTrash
	stored.Slug += "-trashed-abcd1234"
	posts.add(stored)

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/restore", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	body := decodeBody[postBody](t, recorder)
	if body.Status != string(post.StatusDraft) || body.Slug != "trashed" {
		t.Errorf("body = %+v, want a draft under the original slug", body)
	}
}

func TestPostRestoreRejectsPostsThatAreNotTrashed(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Live Draft", ada.ID))

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/restore", "")

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
}

func TestPostRestoreRejectsUnknownAndMalformedIDs(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)

	missing := doRequest(
		t, handler, http.MethodPost, "/api/posts/"+uuid.Must(uuid.NewV7()).String()+"/restore", "",
	)
	malformed := doRequest(t, handler, http.MethodPost, "/api/posts/not-a-uuid/restore", "")

	if missing.Code != http.StatusNotFound {
		t.Errorf("missing post status = %d, want %d", missing.Code, http.StatusNotFound)
	}
	if malformed.Code != http.StatusBadRequest {
		t.Errorf("malformed id status = %d, want %d", malformed.Code, http.StatusBadRequest)
	}
}

func TestPostRestoreReportsStoreFailures(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := newPost(t, "Trashed", ada.ID)
	stored.Status = post.StatusTrash
	posts.add(stored)
	posts.restoreErr = context.DeadlineExceeded

	recorder := doRequest(t, handler, http.MethodPost, "/api/posts/"+stored.ID.String()+"/restore", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestPostWriteRoutesRequireASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	unauthed := server.NewServer(server.Config{Users: users, Posts: newFakePostStore()})
	id := uuid.Must(uuid.NewV7()).String()

	for _, target := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/posts", `{"title":"Hi"}`},
		{http.MethodPatch, "/api/posts/" + id, `{"title":"Hi"}`},
		{http.MethodDelete, "/api/posts/" + id, ""},
		{http.MethodPost, "/api/posts/" + id + "/restore", ""},
	} {
		recorder := doRequest(t, unauthed, target.method, target.path, target.body)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want %d", target.method, target.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestPostTrashReportsStoreFailures(t *testing.T) {
	t.Parallel()

	handler, posts, ada := authedPostServer(t)
	stored := posts.add(newPost(t, "Doomed", ada.ID))
	posts.trashErr = context.DeadlineExceeded
	posts.deleteErr = context.DeadlineExceeded

	trash := doRequest(t, handler, http.MethodDelete, "/api/posts/"+stored.ID.String(), "")
	force := doRequest(t, handler, http.MethodDelete, "/api/posts/"+stored.ID.String()+"?force=true", "")

	if trash.Code != http.StatusInternalServerError || force.Code != http.StatusInternalServerError {
		t.Errorf("statuses = %d and %d, want %d", trash.Code, force.Code, http.StatusInternalServerError)
	}
}
