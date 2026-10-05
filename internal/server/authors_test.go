// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"net/http"
	"slices"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
)

// authorsHeld is the authors list as its readers decode it.
type authorsHeld struct {
	Items []struct {
		ID   uuid.UUID `json:"id"`
		Name string    `json:"name"`
	} `json:"items"`
}

func TestAuthorsListEveryAccountByName(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	ada := addAda(t, users)
	maria := addWithRole(t, users, "maria@example.com", "Maria Perez", role.Author)
	editor := addWithRole(t, users, "editor@example.com", "Editor", role.Editor)
	handler := authedServerWithStores(t, serverConfig(users, newFakePostStore()))

	recorder := doRequest(t, handler, http.MethodGet, "/api/authors", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body)
	}
	held := decodeBody[authorsHeld](t, recorder)
	names := make([]string, len(held.Items))
	ids := make([]uuid.UUID, len(held.Items))
	for i, author := range held.Items {
		names[i], ids[i] = author.Name, author.ID
	}
	if !slices.Equal(names, []string{"Ada Lovelace", "Editor", "Maria Perez"}) {
		t.Errorf("names = %v, want every account sorted by name", names)
	}
	if !slices.Equal(ids, []uuid.UUID{ada.ID, editor.ID, maria.ID}) {
		t.Errorf("ids = %v, want each name beside its own account", ids)
	}
}

func TestAuthorsKeepADisabledAccountThatStillOwnsItems(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	gone := addWithRole(t, users, "maria@example.com", "Maria Perez", role.Author)
	if err := users.SetUserDisabled(t.Context(), gone.ID, true); err != nil {
		t.Fatalf("disabling the account: %v", err)
	}
	handler := authedServerWithStores(t, serverConfig(users, newFakePostStore()))

	held := decodeBody[authorsHeld](t, doRequest(t, handler, http.MethodGet, "/api/authors", ""))

	if len(held.Items) != 2 || held.Items[1].ID != gone.ID {
		t.Errorf("authors = %+v, want the disabled account kept", held.Items)
	}
}

func TestAuthorsSortAccountsSharingANameByTheirID(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	first := addWithRole(t, users, "one@example.com", "Maria Perez", role.Author)
	second := addWithRole(t, users, "two@example.com", "Maria Perez", role.Author)
	handler := authedServerWithStores(t, serverConfig(users, newFakePostStore()))

	held := decodeBody[authorsHeld](t, doRequest(t, handler, http.MethodGet, "/api/authors", ""))

	want := []uuid.UUID{first.ID, second.ID}
	slices.SortFunc(want, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	if len(held.Items) != 3 || held.Items[1].ID != want[0] || held.Items[2].ID != want[1] {
		t.Errorf("authors = %+v, want the two accounts named alike ordered by id", held.Items)
	}
}

func TestAuthorsReportAnAccountStoreThatWillNotAnswer(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := authedServerWithStores(t, server.Config{
		Users: failingUserStore{Store: users}, Content: newFakePostStore(), Types: newFakeTypeStore(),
	})

	recorder := doRequest(t, handler, http.MethodGet, "/api/authors", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestAuthorsNeedASession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(serverConfig(users, newFakePostStore()))

	recorder := doRequest(t, handler, http.MethodGet, "/api/authors", "")

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}
