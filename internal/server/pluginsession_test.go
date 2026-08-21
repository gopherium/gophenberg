// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/sdk"
)

// seenSession is what a plugin reports about the request it was reached by.
type seenSession struct {
	Filed        bool   `json:"filed"`
	Email        string `json:"email"`
	Rank         string `json:"rank"`
	ChangesWork  bool   `json:"changes_work"`
	ManagesUsers bool   `json:"manages_users"`
}

// sessionReporter answers what the sdk seam carries on the request.
func sessionReporter() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		held, filed := sdk.SessionFrom(r.Context())
		_ = json.NewEncoder(w).Encode(seenSession{
			Filed:        filed,
			Email:        held.Email,
			Rank:         held.Rank,
			ChangesWork:  held.Can(string(role.ChangeOthersWork)),
			ManagesUsers: held.Can(string(role.ManageUsers)),
		})
	})
}

// pluginServerFor returns a server mounting the reporter, and a cookie for the given rank.
func pluginServerFor(t *testing.T, rank string) (http.Handler, *http.Cookie) {
	t.Helper()
	users := newFakeUserStore()
	addRanked(t, users, "maria@example.com", "Maria Perez", rank)
	handler := server.NewServer(server.Config{
		Users:             users,
		Plugins:           map[string]http.Handler{"reporter": sessionReporter()},
		PluginPublicPaths: map[string][]string{"reporter": {"/open"}},
	})
	recorder := doLogin(t, handler, `{"email":"maria@example.com","password":"correct horse battery"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recorder.Code, http.StatusOK)
	}
	return handler, sessionCookie(t, recorder)
}

// askReporter reads what the plugin saw at the given path.
func askReporter(t *testing.T, handler http.Handler, cookie *http.Cookie, path string) seenSession {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusOK)
	}
	return decodeBody[seenSession](t, recorder)
}

func TestAPluginReadsTheSessionTheHostFiled(t *testing.T) {
	t.Parallel()

	handler, cookie := pluginServerFor(t, role.Editor)

	seen := askReporter(t, handler, cookie, "/api/plugins/reporter/anything")

	if !seen.Filed {
		t.Fatal("the plugin saw no session, want the host to file one on every guarded request")
	}
	if seen.Email != "maria@example.com" || seen.Rank != role.Editor {
		t.Errorf("the plugin saw %+v, want the signed in identity and its rank", seen)
	}
}

func TestAPluginAsksCapabilitiesRatherThanRanks(t *testing.T) {
	t.Parallel()

	editor, editorCookie := pluginServerFor(t, role.Editor)
	admin, adminCookie := pluginServerFor(t, role.Admin)
	author, authorCookie := pluginServerFor(t, role.Author)

	seenEditor := askReporter(t, editor, editorCookie, "/api/plugins/reporter/anything")
	seenAdmin := askReporter(t, admin, adminCookie, "/api/plugins/reporter/anything")
	seenAuthor := askReporter(t, author, authorCookie, "/api/plugins/reporter/anything")

	if !seenEditor.ChangesWork || seenEditor.ManagesUsers {
		t.Errorf("an editor saw %+v, want it changing others work and administering nothing", seenEditor)
	}
	if !seenAdmin.ManagesUsers {
		t.Errorf("an admin saw %+v, want it administering accounts", seenAdmin)
	}
	if seenAuthor.ChangesWork || seenAuthor.ManagesUsers {
		t.Errorf("an author saw %+v, want it holding neither", seenAuthor)
	}
}

func TestAPublicPluginPathCarriesNoSession(t *testing.T) {
	t.Parallel()

	handler, _ := pluginServerFor(t, role.Editor)

	seen := askReporter(t, handler, nil, "/api/plugins/reporter/open")

	if seen.Filed {
		t.Errorf("a public path saw %+v, want no session filed where none was required", seen)
	}
}
