// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gopherium/gouncer/authkit/testkit"

	"github.com/gopherium/gophenberg/internal/server"
)

func echoHandler(status int, body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

func newProtectedServer(t *testing.T) (http.Handler, *testkit.Store) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(server.Config{
		Users: users,
		Plugins: map[string]http.Handler{
			"echo": echoHandler(http.StatusOK, "plugin says hi"),
		},
		PluginPublicPaths: map[string][]string{
			"echo": {"/rss.xml"},
		},
	})
	return handler, users
}

func TestLoginRejectsBadCredentials(t *testing.T) {
	t.Parallel()

	handler, _ := newProtectedServer(t)

	recorder := doLogin(t, handler, `{"email":"ada@example.com","password":"wrong"}`)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("login status = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestLoginIssuesASessionForTheSessionEndpoint(t *testing.T) {
	t.Parallel()

	handler, _ := newProtectedServer(t)
	cookie := loginCookie(t, handler)

	request := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("session status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); !strings.Contains(body, "ada@example.com") {
		t.Errorf("session body = %q, want the logged-in user", body)
	}
}

func TestLogoutEndsTheSession(t *testing.T) {
	t.Parallel()

	handler, _ := newProtectedServer(t)
	cookie := loginCookie(t, handler)

	logout := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	logout.AddCookie(cookie)
	handler.ServeHTTP(httptest.NewRecorder(), logout)

	request := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("session status after logout = %d, want %d", recorder.Code, http.StatusUnauthorized)
	}
}

func TestMiddlewareRejectsRequestsWithoutASession(t *testing.T) {
	t.Parallel()

	handler, _ := newProtectedServer(t)

	for _, target := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/users"},
		{http.MethodGet, "/api/version"},
		{http.MethodGet, "/api/plugins/echo/anything"},
	} {
		request := httptest.NewRequest(target.method, target.path, strings.NewReader("{}"))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s %s status = %d, want %d", target.method, target.path, recorder.Code, http.StatusUnauthorized)
		}
	}
}

func TestMiddlewareAdmitsAuthenticatedRequests(t *testing.T) {
	t.Parallel()

	handler, _ := newProtectedServer(t)
	cookie := loginCookie(t, handler)

	request := httptest.NewRequest(http.MethodGet, "/api/plugins/echo/anything", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK || recorder.Body.String() != "plugin says hi" {
		t.Errorf("response = %d %q, want the plugin handler's response", recorder.Code, recorder.Body.String())
	}
}

func TestMiddlewareAdmitsDeclaredPublicPluginPaths(t *testing.T) {
	t.Parallel()

	handler, _ := newProtectedServer(t)

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		request := httptest.NewRequest(method, "/api/plugins/echo/rss.xml", strings.NewReader("{}"))
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Errorf("%s public path status = %d, want %d", method, recorder.Code, http.StatusOK)
		}
	}
}

func TestMiddlewareReportsSessionStoreFailure(t *testing.T) {
	t.Parallel()

	handler, users := newProtectedServer(t)
	cookie := loginCookie(t, handler)
	users.SessionErr = context.DeadlineExceeded

	request := httptest.NewRequest(http.MethodGet, "/api/plugins/echo/anything", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestUsersAdminSurfaceServesTheUserList(t *testing.T) {
	t.Parallel()

	handler, _ := newProtectedServer(t)
	cookie := loginCookie(t, handler)

	request := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if body := recorder.Body.String(); !strings.Contains(body, "ada@example.com") {
		t.Errorf("users body = %q, want it to list the stored user", body)
	}
}
