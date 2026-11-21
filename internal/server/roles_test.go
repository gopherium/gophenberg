// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/gouncer/authkit/testkit"

	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
)

// refusal is the code and meta a refused request answers.
type refusal struct {
	Code string         `json:"code"`
	Meta map[string]any `json:"meta"`
}

// signedInAs returns a server over roles, a cookie for an account holding held, and the account store.
func signedInAs(t *testing.T, roles *goncierge.Registry, held string) (http.Handler, *http.Cookie, *testkit.Store) {
	t.Helper()
	users := newFakeUserStore()
	addWithRole(t, users, "holder@example.com", "Maria Perez", held)
	handler := server.NewServer(server.Config{
		Users:      users,
		Content:    newFakePostStore(),
		Types:      newFakeTypeStore(),
		Themes:     listOnlyFailsThemes{},
		Settings:   memorySiteSettings{},
		Readers:    memoryReaderSettings{},
		Media:      mediahost.New(mediahost.Config{Dir: t.TempDir()}),
		MediaStore: newFakeMediaStore(),
		Roles:      roles,
		Updates:    &storedUpdates{},

		InstalledPlugins: heldPlugins{},
	})
	recorder := doLogin(t, handler, `{"email":"holder@example.com","password":"correct horse battery"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login as %s status = %d, want %d", held, recorder.Code, http.StatusOK)
	}
	return handler, sessionCookie(t, recorder), users
}

// call is one request a test sends, its method, path and JSON body.
type call struct {
	method, path, body string
}

// ask sends the call with the cookie, answering the recorded reply.
func ask(handler http.Handler, cookie *http.Cookie, sent call) *httptest.ResponseRecorder {
	request := httptest.NewRequest(sent.method, sent.path, strings.NewReader(sent.body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

// capabilityOf returns the capability an admin route asks for, none for the users routes.
func capabilityOf(gated route) string {
	switch {
	case strings.HasPrefix(gated.pattern, "/api/users"):
		return ""
	case strings.HasPrefix(gated.pattern, "/api/settings"):
		return string(role.ManageSettings)
	case strings.HasPrefix(gated.pattern, "/api/themes"):
		return string(role.ManageThemes)
	case strings.HasPrefix(gated.pattern, "/api/updates"):
		return string(role.UpdateCore)
	case gated.pattern == "/api/plugin-admin":
		return string(role.ActivatePlugins)
	default:
		return string(role.ManageTypes)
	}
}

// refusalOf returns the refusal a recorded reply carries.
func refusalOf(t *testing.T, recorder *httptest.ResponseRecorder) refusal {
	t.Helper()
	var held refusal
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("decoding the refusal %q: %v", recorder.Body.String(), err)
	}
	return held
}

func TestEachAdminRouteRefusesAnEditorWithWhatItAsksFor(t *testing.T) {
	t.Parallel()

	handler, cookie, _ := signedInAs(t, nil, role.Editor)

	for _, gated := range adminRoutes {
		recorder := ask(handler, cookie, call{gated.method, concrete(gated.pattern), "{}"})

		asked, want := capabilityOf(gated), "capability_missing"
		if asked == "" {
			want = "role_insufficient"
		}
		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s answered %d for an editor, want 403", gated.method, gated.pattern, recorder.Code)
		}
		got := refusalOf(t, recorder)
		if named, _ := got.Meta["capability"].(string); got.Code != want || named != asked {
			t.Errorf("%s %s refused an editor with %+v, want %s naming %q", gated.method, gated.pattern, got, want, asked)
		}
	}
}

func TestARoleCarryingTheThemesCapabilityReachesTheThemesAlone(t *testing.T) {
	t.Parallel()

	roles := role.Registry()
	if err := roles.Grant("studio", "designer", string(role.ManageThemes)); err != nil {
		t.Fatalf("Grant() = %v, want the plugin role granted", err)
	}
	handler, cookie, _ := signedInAs(t, roles, "designer")

	for _, gated := range adminRoutes {
		refused := ask(handler, cookie, call{gated.method, concrete(gated.pattern), "{}"}).Code == http.StatusForbidden

		if themes := capabilityOf(gated) == string(role.ManageThemes); refused == themes {
			t.Errorf("%s %s refused %v for a designer, want only the theme routes reached",
				gated.method, gated.pattern, refused)
		}
	}
}

func TestTheSessionCarriesWhatAnAdministratorMayDoAndGive(t *testing.T) {
	t.Parallel()

	handler, cookie, _ := signedInAs(t, nil, role.Admin)

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/auth/session", ""})

	var held struct {
		Capabilities []string `json:"capabilities"`
		Grantable    []string `json:"grantable"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("decoding the session %q: %v", recorder.Body.String(), err)
	}
	every := []string{
		string(role.AccessAdmin), string(role.ActivatePlugins), string(role.ChangeOthersWork),
		string(role.ManageSettings), string(role.ManageThemes), string(role.ManageTypes), string(role.ManageUsers),
		string(role.UpdateCore),
	}
	widestFirst := []string{role.Admin, role.Editor, role.Author, role.Member}
	if !slices.Equal(held.Capabilities, every) || !slices.Equal(held.Grantable, widestFirst) {
		t.Errorf("the session carries %v and may give %v, want every core capability and the four core roles widest first",
			held.Capabilities, held.Grantable)
	}
}

func TestTheSessionOfAMemberCarriesAnEmptyListOfCapabilities(t *testing.T) {
	t.Parallel()

	handler, cookie, _ := signedInAs(t, nil, role.Member)

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/auth/session", ""})

	if body := recorder.Body.String(); !strings.Contains(body, `"capabilities":[]`) {
		t.Errorf("the session answers %s, want an empty list of capabilities", body)
	}
}

func TestARoleWriteRefusesARoleNobodyDeclares(t *testing.T) {
	t.Parallel()

	handler, cookie, users := signedInAs(t, nil, role.Admin)
	author := addWithRole(t, users, "author@example.com", "Maria Perez", role.Author)
	path := "/api/users/" + author.ID.String() + "/role"

	recorder := ask(handler, cookie, call{http.MethodPut, path, `{"role":"archivist"}`})

	if got := refusalOf(t, recorder); recorder.Code != http.StatusUnprocessableEntity || got.Code != "role_unknown" {
		t.Errorf("giving an undeclared role answered %d %+v, want 422 role_unknown", recorder.Code, got)
	}
}

func TestARoleCarryingOnlyTheAccountCapabilityCannotReachAnAdministrator(t *testing.T) {
	t.Parallel()

	roles := role.Registry()
	if err := roles.Grant("teams", "team_lead", string(role.ManageUsers)); err != nil {
		t.Fatalf("Grant() = %v, want the plugin role granted", err)
	}
	handler, cookie, users := signedInAs(t, roles, "team_lead")
	member := addWithRole(t, users, "member@example.com", "Maria Perez", role.Member)
	author := addWithRole(t, users, "author@example.com", "Maria Perez", role.Author)
	owner := addWithRole(t, users, "owner@example.com", "Maria Perez", role.Admin)
	refused := []struct {
		name string
		sent call
		want string
	}{
		{
			name: "promoting a member",
			sent: call{http.MethodPut, "/api/users/" + member.ID.String() + "/role", `{"role":"admin"}`},
			want: "role_not_grantable",
		},
		{
			name: "promoting an author",
			sent: call{http.MethodPut, "/api/users/" + author.ID.String() + "/role", `{"role":"admin"}`},
			want: "user_not_editable",
		},
		{
			name: "disabling an administrator",
			sent: call{http.MethodPatch, "/api/users/" + owner.ID.String(), `{"disabled":true}`},
			want: "user_not_editable",
		},
	}

	for _, write := range refused {
		recorder := ask(handler, cookie, write.sent)

		if got := refusalOf(t, recorder); recorder.Code != http.StatusForbidden || got.Code != write.want {
			t.Errorf("%s answered %d %+v for a team lead, want 403 %s", write.name, recorder.Code, got, write.want)
		}
	}
}

func TestTheUserListTellsATeamLeadWhichAccountsItMayEdit(t *testing.T) {
	t.Parallel()

	roles := role.Registry()
	if err := roles.Grant("teams", "team_lead", string(role.ManageUsers)); err != nil {
		t.Fatalf("Grant() = %v, want the plugin role granted", err)
	}
	handler, cookie, users := signedInAs(t, roles, "team_lead")
	addWithRole(t, users, "member@example.com", "Maria Perez", role.Member)
	addWithRole(t, users, "owner@example.com", "Maria Perez", role.Admin)

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/users", ""})

	var listed []struct {
		Email    string `json:"email"`
		Editable bool   `json:"editable"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decoding the user list %q: %v", recorder.Body.String(), err)
	}
	editable := map[string]bool{}
	for _, account := range listed {
		editable[account.Email] = account.Editable
	}
	want := map[string]bool{"holder@example.com": true, "member@example.com": true, "owner@example.com": false}
	if !maps.Equal(editable, want) {
		t.Errorf("the user list marks %v editable, want %v", editable, want)
	}
}
