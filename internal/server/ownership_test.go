// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
)

// ownedWrites names the routes that change one item and ask who authored it.
var ownedWrites = []route{
	{http.MethodPatch, "/api/content/{id}"},
	{http.MethodDelete, "/api/content/{id}"},
	{http.MethodPost, "/api/content/{id}/restore"},
	{http.MethodDelete, "/api/content/{id}/revisions/{revisionID}"},
	{http.MethodPatch, "/api/media/{id}"},
	{http.MethodDelete, "/api/media/{id}"},
}

// unownedWrites names the open routes that change nothing another account owns,
// the two creates stamping the writer and the other two writing the session's own row.
var unownedWrites = []route{
	{http.MethodPost, "/api/content"},
	{http.MethodPost, "/api/media"},
	{http.MethodPost, "/api/content/{id}/autosave"},
	{http.MethodPatch, "/api/locale"},
}

// ownedWorld is a server holding one post and one media item authored by the admin.
type ownedWorld struct {
	handler http.Handler
	postID  uuid.UUID
	mediaID int64
	version time.Time
}

// newOwnedWorld returns a server whose stored work belongs to the administrator.
func newOwnedWorld(t *testing.T) *ownedWorld {
	t.Helper()
	users := newFakeUserStore()
	ada := addAda(t, users)
	addRanked(t, users, "author@example.com", "Maria Perez", role.Author)
	addRanked(t, users, "editor@example.com", "Grace Hopper", role.Editor)
	posts := newFakePostStore()
	stored := posts.add(newPost(t, "Owned by the admin", ada.ID))
	mediaStore := newFakeMediaStore()
	item, err := mediaStore.Create(t.Context(), media.Media{
		Title: "Owned by the admin", File: "owned.png", AuthorID: ada.ID, UpdatedAt: stored.UpdatedAt,
	})
	if err != nil {
		t.Fatalf("planting the media item: %v", err)
	}
	handler := server.NewServer(server.Config{
		Users:      users,
		Content:    posts,
		Types:      newFakeTypeStore(),
		Media:      mediahost.New(mediahost.Config{Dir: t.TempDir()}),
		MediaStore: mediaStore,
	})
	return &ownedWorld{handler: handler, postID: stored.ID, mediaID: item.ID, version: stored.UpdatedAt}
}

// signIn returns the session cookie of the given account.
func (w *ownedWorld) signIn(t *testing.T, email string) *http.Cookie {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":%q}`, email, testPassword)
	recorder := doLogin(t, w.handler, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s login status = %d, want %d", email, recorder.Code, http.StatusOK)
	}
	return sessionCookie(t, recorder)
}

// ask sends one write at the stored work, answering the recorder.
func (w *ownedWorld) ask(t *testing.T, cookie *http.Cookie, write route) *httptest.ResponseRecorder {
	t.Helper()
	path := strings.NewReplacer(
		"{id}", w.postID.String(),
		"{revisionID}", uuid.New().String(),
	).Replace(write.pattern)
	if strings.HasPrefix(write.pattern, "/api/media/") {
		path = fmt.Sprintf("/api/media/%d", w.mediaID)
	}
	body := fmt.Sprintf(`{"updated_at":%q}`, w.version.Format(time.RFC3339Nano))
	request := httptest.NewRequest(write.method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	w.handler.ServeHTTP(recorder, request)
	return recorder
}

func TestEveryOpenWriteEitherAsksWhoOwnsItOrOwnsNothing(t *testing.T) {
	t.Parallel()

	classified := map[route]bool{}
	for _, group := range [][]route{ownedWrites, unownedWrites} {
		for _, write := range group {
			classified[write] = true
		}
	}

	for _, open := range everyoneRoutes {
		if open.method == http.MethodGet {
			continue
		}
		if !classified[open] {
			t.Errorf("%s %s changes something without asking who owns it, want it checked or declared unowned",
				open.method, open.pattern)
		}
	}
	for write := range classified {
		if !slices.Contains(everyoneRoutes, write) {
			t.Errorf("%s %s is classified but no longer an open route", write.method, write.pattern)
		}
	}
}

func TestAnAuthorIsRefusedEveryWriteOverAnotherAccountsWork(t *testing.T) {
	t.Parallel()

	world := newOwnedWorld(t)
	cookie := world.signIn(t, "author@example.com")

	for _, write := range ownedWrites {
		recorder := world.ask(t, cookie, write)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d for an author who did not write it",
				write.method, write.pattern, recorder.Code, http.StatusForbidden)
			continue
		}
		if body := recorder.Body.String(); !strings.Contains(body, "not_the_author") {
			t.Errorf("%s %s refused with %q, want the not_the_author code",
				write.method, write.pattern, body)
		}
	}
}

func TestAnEditorReachesEveryWriteOverAnotherAccountsWork(t *testing.T) {
	t.Parallel()

	world := newOwnedWorld(t)
	cookie := world.signIn(t, "editor@example.com")

	for _, write := range ownedWrites {
		recorder := world.ask(t, cookie, write)

		if recorder.Code == http.StatusForbidden {
			t.Errorf("%s %s refused an editor with %d, want every account's work open to it",
				write.method, write.pattern, recorder.Code)
		}
	}
}

func TestAnAuthorReachesEveryWriteOverItsOwnWork(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	maria := addRanked(t, users, "author@example.com", "Maria Perez", role.Author)
	posts := newFakePostStore()
	stored := posts.add(newPost(t, "Written by Maria", maria.ID))
	mediaStore := newFakeMediaStore()
	item, err := mediaStore.Create(t.Context(), media.Media{
		Title: "Uploaded by Maria", File: "hers.png", AuthorID: maria.ID, UpdatedAt: stored.UpdatedAt,
	})
	if err != nil {
		t.Fatalf("planting the media item: %v", err)
	}
	world := &ownedWorld{
		handler: server.NewServer(server.Config{
			Users:      users,
			Content:    posts,
			Types:      newFakeTypeStore(),
			Media:      mediahost.New(mediahost.Config{Dir: t.TempDir()}),
			MediaStore: mediaStore,
		}),
		postID: stored.ID, mediaID: item.ID, version: stored.UpdatedAt,
	}
	cookie := world.signIn(t, "author@example.com")

	for _, write := range ownedWrites {
		recorder := world.ask(t, cookie, write)

		if recorder.Code == http.StatusForbidden {
			t.Errorf("%s %s refused the author of the item with %d, want its own work open to it",
				write.method, write.pattern, recorder.Code)
		}
	}
}
