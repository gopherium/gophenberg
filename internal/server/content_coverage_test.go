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

func TestPostListAcceptsAnUnregisteredType(t *testing.T) {
	t.Parallel()

	handler, _, _ := authedPostServer(t)

	list := doRequest(t, handler, http.MethodGet, "/api/content?type=page", "")
	counts := doRequest(t, handler, http.MethodGet, "/api/content/counts?type=page", "")

	if list.Code != http.StatusOK {
		t.Errorf("list status = %d, want an unregistered type to list rather than fail", list.Code)
	}
	if counts.Code != http.StatusOK {
		t.Errorf("counts status = %d, want %d", counts.Code, http.StatusOK)
	}
}

func TestPostErrorsFallBackToTheAuthMapping(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	posts := newFakePostStore()
	posts.createErr = gouncer.ErrUserNotFound
	handler := authedServerWithStores(t, server.Config{Users: users, Content: posts})

	recorder := doRequest(t, handler, http.MethodPost, "/api/content", `{"title":"Hi"}`)

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want the auth mapping's %d", recorder.Code, http.StatusNotFound)
	}
}
