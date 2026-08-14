// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/server"
)

// authedServerWithStores returns a handler carrying a session over the given stores.
func authedServerWithStores(t *testing.T, cfg server.Config) http.Handler {
	t.Helper()
	handler := server.NewServer(cfg)
	cookie := loginCookie(t, handler)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
}

func TestPostDetailReportsAuthorLookupFailures(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	posts := newFakePostStore()
	stored := posts.add(newPost(t, "Stored", uuid.Must(uuid.NewV7())))
	handler := authedServerWithStores(t, server.Config{Users: failingUserStore{Store: users}, Content: posts})

	recorder := doRequest(t, handler, http.MethodGet, "/api/content/"+stored.ID.String(), "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestPostListRefusesTypesTheRegistryDoesNotServe(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	types := newFakeTypeStore()
	retired := postType()
	retired.Key, retired.Active, retired.Default = "briefing", false, false
	retired.RouteWord = "briefings"
	types.register(retired)
	handler := authedServerWithStores(t,
		server.Config{Users: users, Content: newFakePostStore(), Types: types})

	for _, contentType := range []string{"page", "briefing"} {
		list := doRequest(t, handler, http.MethodGet, "/api/content?type="+contentType, "")
		counts := doRequest(t, handler, http.MethodGet, "/api/content/counts?type="+contentType, "")

		if list.Code != http.StatusUnprocessableEntity {
			t.Errorf("listing %q = %d, want %d", contentType, list.Code, http.StatusUnprocessableEntity)
		}
		if counts.Code != http.StatusUnprocessableEntity {
			t.Errorf("counting %q = %d, want %d", contentType, counts.Code, http.StatusUnprocessableEntity)
		}
	}
}

func TestPostErrorsFallBackToTheAuthMapping(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	posts := newFakePostStore()
	posts.createErr = gouncer.ErrUserNotFound
	handler := authedServerWithStores(t, server.Config{Users: users, Content: posts, Types: newFakeTypeStore()})

	recorder := doRequest(t, handler, http.MethodPost, "/api/content", `{"title":"Hi"}`)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want the auth mapping's %d", recorder.Code, http.StatusNotFound)
	}
}
