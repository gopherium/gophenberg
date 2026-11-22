// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/gopherium/framework/dbkit"
	"github.com/gopherium/framework/goncierge"
	"github.com/gopherium/framework/pluginkit"

	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
)

// heldPlugins answers the plugin list from the plugins it holds, or from the failure it was given.
type heldPlugins struct {
	plugins []pluginkit.InstalledPlugin
	err     error
}

// Installed returns the plugins the test stored, or its failure.
func (h heldPlugins) Installed(context.Context) ([]pluginkit.InstalledPlugin, error) {
	return h.plugins, h.err
}

// pluginListServer returns a server over roles answering the plugin list from plugins, and a cookie for held.
func pluginListServer(
	t *testing.T, roles *goncierge.Registry, plugins server.InstalledPlugins, held string,
) (http.Handler, *http.Cookie) {
	t.Helper()
	users := newFakeUserStore()
	addWithRole(t, users, "holder@example.com", "Maria Perez", held)
	handler := server.NewServer(server.Config{Users: users, Roles: roles, InstalledPlugins: plugins})
	recorder := doLogin(t, handler, `{"email":"holder@example.com","password":"correct horse battery"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login as %s status = %d, want %d", held, recorder.Code, http.StatusOK)
	}
	return handler, sessionCookie(t, recorder)
}

func TestThePluginListAnswersEveryPluginUnderTheNamesTheAdminReads(t *testing.T) {
	t.Parallel()

	held := heldPlugins{plugins: []pluginkit.InstalledPlugin{
		{
			ID: "feed", Name: "Feed", Description: "Serves your published posts as an RSS feed.", Version: "0.0.1",
			State: pluginkit.PluginPaused, PausedStep: pluginkit.StepRegister, PausedReason: "the title is too long",
		},
		{ID: "notes", Name: "Notes", State: pluginkit.PluginActive},
	}}
	handler, cookie := pluginListServer(t, nil, held, role.Admin)

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/plugin-admin", ""})

	var answered map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("reading the plugin list %q: %v", recorder.Body.String(), err)
	}
	want := map[string]any{"plugins": []any{
		map[string]any{
			"id": "feed", "name": "Feed", "description": "Serves your published posts as an RSS feed.",
			"version": "0.0.1", "status": "paused", "paused_step": "register", "paused_reason": "the title is too long",
		},
		map[string]any{
			"id": "notes", "name": "Notes", "description": "", "version": "", "status": "active", "paused_step": "",
			"paused_reason": "",
		},
	}}
	if recorder.Code != http.StatusOK || !reflect.DeepEqual(answered, want) {
		t.Errorf("GET /api/plugin-admin = %d %s, want 200 and %v", recorder.Code, recorder.Body.String(), want)
	}
}

func TestThePluginListAnswersARoleCarryingTheCapabilityAlone(t *testing.T) {
	t.Parallel()

	roles := role.Registry()
	if err := roles.Grant("operations", "operator", string(role.ActivatePlugins)); err != nil {
		t.Fatalf("Grant() = %v, want the plugin role granted", err)
	}
	handler, cookie := pluginListServer(t, roles, heldPlugins{}, "operator")

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/plugin-admin", ""})

	if recorder.Code != http.StatusOK {
		t.Errorf("GET /api/plugin-admin = %d %s for a role carrying activate_plugins, want 200", recorder.Code,
			recorder.Body.String())
	}
}

func TestThePluginListAnswersAFailedReadWithTheCodeOfItsCause(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		err    error
		status int
		code   string
	}{
		"a busy database answers busy": {
			fmt.Errorf("postgres: reading the plugin records: %w", dbkit.ErrBusy), http.StatusServiceUnavailable,
			"database_busy",
		},
		"any other failure answers the internal error": {
			errors.New("connection dropped"), http.StatusInternalServerError, "internal",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, cookie := pluginListServer(t, nil, heldPlugins{err: test.err}, role.Admin)

			recorder := ask(handler, cookie, call{http.MethodGet, "/api/plugin-admin", ""})

			if got := answeredFields(t, recorder); recorder.Code != test.status || got["code"] != test.code {
				t.Errorf("GET /api/plugin-admin = %d %v, want %d %s", recorder.Code, got, test.status, test.code)
			}
		})
	}
}

func TestThePluginListIsUnhandledWithoutThePluginsTheSiteHolds(t *testing.T) {
	t.Parallel()

	handler, cookie := pluginListServer(t, nil, nil, role.Admin)

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/plugin-admin", ""})

	if recorder.Code != http.StatusNotFound {
		t.Errorf("GET /api/plugin-admin = %d with no plugin list, want %d", recorder.Code, http.StatusNotFound)
	}
}
