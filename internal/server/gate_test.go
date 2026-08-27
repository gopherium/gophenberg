// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
)

// route names one method and pattern the router serves.
type route struct {
	method  string
	pattern string
}

// publicRoutes names the API routes served without a session.
var publicRoutes = []route{
	{http.MethodPost, "/api/auth/login"},
	{http.MethodPost, "/api/auth/logout"},
	{http.MethodGet, "/api/locale"},
	{http.MethodGet, "/api/content/v1"},
	{http.MethodGet, "/api/content/v1/items"},
	{http.MethodGet, "/api/content/v1/resolve"},
}

// everyoneRoutes names the API routes every signed in role reaches.
var everyoneRoutes = []route{
	{http.MethodGet, "/api/auth/session"},
	{http.MethodGet, "/api/types"},
	{http.MethodGet, "/api/groups"},
	{http.MethodGet, "/api/groups/params"},
	{http.MethodGet, "/api/content"},
	{http.MethodPost, "/api/content"},
	{http.MethodGet, "/api/content/counts"},
	{http.MethodGet, "/api/content/{id}"},
	{http.MethodPatch, "/api/content/{id}"},
	{http.MethodDelete, "/api/content/{id}"},
	{http.MethodPost, "/api/content/{id}/restore"},
	{http.MethodPost, "/api/content/{id}/autosave"},
	{http.MethodGet, "/api/content/{id}/autosave"},
	{http.MethodGet, "/api/content/{id}/revisions"},
	{http.MethodGet, "/api/content/{id}/revisions/{revisionID}"},
	{http.MethodDelete, "/api/content/{id}/revisions/{revisionID}"},
	{http.MethodGet, "/api/version"},
	{http.MethodPatch, "/api/locale"},
	{http.MethodGet, "/api/settings"},
	{http.MethodGet, "/api/media"},
	{http.MethodPost, "/api/media"},
	{http.MethodGet, "/api/media/{id}"},
	{http.MethodPatch, "/api/media/{id}"},
	{http.MethodDelete, "/api/media/{id}"},
}

// adminRoutes names the API routes only a privileged role reaches.
var adminRoutes = []route{
	{http.MethodGet, "/api/users"},
	{http.MethodPost, "/api/users"},
	{http.MethodPatch, "/api/users/{id}"},
	{http.MethodPut, "/api/users/{id}/role"},
	{http.MethodPost, "/api/types"},
	{http.MethodPatch, "/api/types/{key}"},
	{http.MethodDelete, "/api/types/{key}"},
	{http.MethodPost, "/api/groups"},
	{http.MethodPut, "/api/groups/order"},
	{http.MethodPatch, "/api/groups/{id}"},
	{http.MethodDelete, "/api/groups/{id}"},
	{http.MethodPost, "/api/groups/{id}/fields"},
	{http.MethodPut, "/api/groups/{id}/fields/order"},
	{http.MethodPatch, "/api/groups/{id}/fields/{fieldKey}"},
	{http.MethodDelete, "/api/groups/{id}/fields/{fieldKey}"},
	{http.MethodPost, "/api/groups/{id}/fields/{fieldKey}/move"},
	{http.MethodPatch, "/api/settings"},
	{http.MethodGet, "/api/themes"},
	{http.MethodPost, "/api/themes"},
	{http.MethodPost, "/api/themes/active"},
	{http.MethodDelete, "/api/themes/active"},
	{http.MethodPost, "/api/themes/rollback"},
}

// memorySiteSettings answers the site wide settings from memory.
type memorySiteSettings struct{}

// Lookup answers no stored value.
func (memorySiteSettings) Lookup(context.Context, string) (string, bool, error) {
	return "", false, nil
}

// Save accepts the values without storing them.
func (memorySiteSettings) Save(context.Context, map[string]string) error { return nil }

// memoryReaderSettings answers one reader's settings from memory.
type memoryReaderSettings struct{}

// Lookup answers no stored value.
func (memoryReaderSettings) Lookup(context.Context, uuid.UUID, string) (string, bool, error) {
	return "", false, nil
}

// Save accepts the value without storing it.
func (memoryReaderSettings) Save(context.Context, uuid.UUID, string, string) error { return nil }

// newGateServer returns a router carrying every optional surface, and an author's cookie.
func newGateServer(t *testing.T) (http.Handler, *http.Cookie) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	addWithRole(t, users, "author@example.com", "Maria Perez", role.Author)
	handler := server.NewServer(server.Config{
		Users:      users,
		Content:    newFakePostStore(),
		Types:      newFakeTypeStore(),
		Themes:     listOnlyFailsThemes{},
		Settings:   memorySiteSettings{},
		Readers:    memoryReaderSettings{},
		Media:      mediahost.New(mediahost.Config{Dir: t.TempDir()}),
		MediaStore: newFakeMediaStore(),
	})
	recorder := doLogin(t, handler, `{"email":"author@example.com","password":"correct horse battery"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("author login status = %d, want %d", recorder.Code, http.StatusOK)
	}
	return handler, sessionCookie(t, recorder)
}

// walkAPIRoutes returns every API route the router serves.
func walkAPIRoutes(t *testing.T, handler http.Handler) map[route]bool {
	t.Helper()
	router, isRouter := handler.(chi.Routes)
	if !isRouter {
		t.Fatal("the server handler is not a chi router, want one the walk can read")
	}
	walked := map[route]bool{}
	err := chi.Walk(router, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.HasPrefix(pattern, "/api/") && !strings.HasPrefix(pattern, "/api/plugins/") {
			walked[route{method, strings.TrimSuffix(pattern, "/")}] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the router: %v", err)
	}
	return walked
}

func TestEveryAPIRouteIsClaimedByExactlyOneGroup(t *testing.T) {
	t.Parallel()

	handler, _ := newGateServer(t)
	claimed := map[route]int{}
	for _, group := range [][]route{publicRoutes, everyoneRoutes, adminRoutes} {
		for _, claim := range group {
			claimed[claim]++
		}
	}

	walked := walkAPIRoutes(t, handler)

	for served := range walked {
		if claimed[served] == 0 {
			t.Errorf("%s %s is served but claimed by no group", served.method, served.pattern)
		}
		if claimed[served] > 1 {
			t.Errorf("%s %s is claimed by %d groups, want exactly one", served.method, served.pattern, claimed[served])
		}
	}
	for claim := range claimed {
		if !walked[claim] {
			t.Errorf("%s %s is claimed but no longer served", claim.method, claim.pattern)
		}
	}
}

// concrete returns the pattern with every parameter replaced by a usable value.
func concrete(pattern string) string {
	replaced := strings.NewReplacer(
		"{id}", uuid.Nil.String(),
		"{revisionID}", uuid.Nil.String(),
		"{key}", "post",
		"{fieldKey}", "color",
	)
	return replaced.Replace(pattern)
}

func TestAnUnprivilegedRoleIsRefusedEveryAdminRoute(t *testing.T) {
	t.Parallel()

	handler, cookie := newGateServer(t)

	for _, gated := range adminRoutes {
		request := httptest.NewRequest(gated.method, concrete(gated.pattern), strings.NewReader("{}"))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusForbidden {
			t.Errorf("%s %s status = %d, want %d for an author",
				gated.method, gated.pattern, recorder.Code, http.StatusForbidden)
		}
	}
}

func TestAnUnprivilegedRoleReachesEveryOpenRoute(t *testing.T) {
	t.Parallel()

	handler, cookie := newGateServer(t)

	for _, open := range everyoneRoutes {
		request := httptest.NewRequest(open.method, concrete(open.pattern), strings.NewReader("{}"))
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(cookie)
		recorder := httptest.NewRecorder()

		handler.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusForbidden {
			t.Errorf("%s %s refused an author with %d, want the role gate to admit it",
				open.method, open.pattern, recorder.Code)
		}
	}
}
