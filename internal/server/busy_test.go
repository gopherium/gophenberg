// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/server"
)

// errBusy is a store failure the engine marked busy, still carrying the words the database used.
var errBusy = fmt.Errorf("postgres: %w: ERROR: deadlock detected (SQLSTATE 40P01)", dbkit.ErrBusy)

// contentSite is a signed in handler over a post store, beside the account it signed in.
type contentSite struct {
	handler http.Handler
	posts   *fakePostStore
	author  uuid.UUID
}

// mediaSite is a signed in handler over a media store.
type mediaSite struct {
	handler http.Handler
	store   *fakeMediaStore
}

// savedThenHooked is a post store double running a hook once an edit is stored.
type savedThenHooked struct {
	*fakePostStore
	after func()
}

// Update stores the edit and then runs the hook.
func (s *savedThenHooked) Update(
	ctx context.Context, p content.Content, expected time.Time, snapshot *content.Revision, revisionCap int,
) (content.Content, error) {
	updated, err := s.fakePostStore.Update(ctx, p, expected, snapshot, revisionCap)
	s.after()
	return updated, err
}

// wantBusyAnswer fails the test unless the answer is the busy one, holding no database words and no retry hint.
func wantBusyAnswer(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatalf("decoding %q: %v", recorder.Body.String(), err)
	}
	want := map[string]any{"error": "database busy", "code": "database_busy"}
	if recorder.Code != http.StatusServiceUnavailable || !maps.Equal(got, want) {
		t.Errorf("answer = %d %s, want %d %v", recorder.Code, recorder.Body.String(),
			http.StatusServiceUnavailable, want)
	}
	if held := recorder.Header().Get("Retry-After"); held != "" {
		t.Errorf("Retry-After = %q, want none so no client saves again on its own", held)
	}
	if held := recorder.Header().Get("Cache-Control"); held != "no-store" {
		t.Errorf("Cache-Control = %q, want no cache to keep the busy answer", held)
	}
}

func TestAContentSaveTheDatabaseIsTooBusyForAnswersDatabaseBusy(t *testing.T) {
	t.Parallel()

	for name, save := range map[string]func(*testing.T, contentSite) *httptest.ResponseRecorder{
		"a new item": func(t *testing.T, site contentSite) *httptest.ResponseRecorder {
			site.posts.createErr = errBusy
			return doRequest(t, site.handler, http.MethodPost, "/api/content", `{"title":"Hello"}`)
		},
		"an edit": func(t *testing.T, site contentSite) *httptest.ResponseRecorder {
			stored := site.posts.add(newPost(t, "Original", site.author))
			site.posts.updateErr = errBusy
			return doRequest(t, site.handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
				versionedBody(t, stored.UpdatedAt, map[string]any{"title": "Edited"}))
		},
		"an autosave": func(t *testing.T, site contentSite) *httptest.ResponseRecorder {
			stored := site.posts.add(newPost(t, "Other", uuid.Must(uuid.NewV7())))
			site.posts.saveAutosaveErr = errBusy
			return doRequest(t, site.handler, http.MethodPost, "/api/content/"+stored.ID.String()+"/autosave",
				autosavePayload(t, stored.UpdatedAt))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, posts, ada := authedPostServer(t)

			wantBusyAnswer(t, save(t, contentSite{handler: handler, posts: posts, author: ada.ID}))
		})
	}
}

func TestAMediaSaveTheDatabaseIsTooBusyForAnswersDatabaseBusy(t *testing.T) {
	t.Parallel()

	for name, save := range map[string]func(*testing.T, mediaSite) *httptest.ResponseRecorder{
		"an upload": func(t *testing.T, site mediaSite) *httptest.ResponseRecorder {
			site.store.createErr = errBusy
			return sendMediaUpload(t, site.handler, "harbor.jpg", smallJPEG(t))
		},
		"an edit": func(t *testing.T, site mediaSite) *httptest.ResponseRecorder {
			created := storedMediaItem(t, site.handler)
			site.store.updateErr = errBusy
			return doRequest(t, site.handler, http.MethodPatch, fmt.Sprintf("/api/media/%d", created.ID),
				patchBody(created.UpdatedAt.Format(time.RFC3339Nano), map[string]string{"title": "Harbor at dawn"}))
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newFakeMediaStore()
			handler := mediaServer(t, mediahost.New(mediahost.Config{Dir: t.TempDir()}), store)

			wantBusyAnswer(t, save(t, mediaSite{handler: handler, store: store}))
		})
	}
}

func TestAThemeChangeTheDatabaseIsTooBusyForAnswersDatabaseBusy(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		themes server.Themes
		method string
		path   string
		body   string
	}{
		"an activation":        {errThemes{err: errBusy}, http.MethodPost, "/api/themes/active", `{"name":"aurora"}`},
		"a deactivation":       {errThemes{err: errBusy}, http.MethodDelete, "/api/themes/active", ""},
		"a rollback":           {errThemes{err: errBusy}, http.MethodPost, "/api/themes/rollback", ""},
		"a listing":            {errThemes{err: errBusy}, http.MethodGet, "/api/themes", ""},
		"a listing's rollback": {listOnlyFailsThemes{errThemes{err: errBusy}}, http.MethodGet, "/api/themes", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler := themeServer(t, asked.themes)

			wantBusyAnswer(t, doRequest(t, handler, asked.method, asked.path, asked.body))
		})
	}
}

func TestPostPatchThatStoredTheEditNeverAnswersDatabaseBusy(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	author := addAda(t, users)
	posts := &savedThenHooked{fakePostStore: newFakePostStore(), after: func() {}}
	types := newFakeTypeStore()
	types.register(spareType())
	handler := authedServerWithStores(t, server.Config{Users: users, Content: posts, Types: types})
	stored := posts.add(newPost(t, "Original", author.ID))
	deactivate := deactivateTheSpareType(t, handler)
	posts.after = func() {
		deactivate()
		types.listErr = errBusy
	}

	recorder := doRequest(t, handler, http.MethodPatch, "/api/content/"+stored.ID.String(),
		versionedBody(t, stored.UpdatedAt, map[string]any{"title": "Edited"}))

	if posts.posts[stored.ID].Title != "Edited" {
		t.Fatalf("stored title = %q, want the edit stored before the read failed", posts.posts[stored.ID].Title)
	}
	if code := errorCode(t, recorder); code == "database_busy" {
		t.Errorf("code = %q after the edit was stored, want no answer telling the reader nothing was saved", code)
	}
}

func TestSettingsPatchThatStoredTheValueNeverAnswersDatabaseBusy(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.duringSave = func() { settings.lookupErr = errBusy }

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"content_per_page":5}`)

	if settings.site[content.PerPageSettingKey] != "5" {
		t.Fatalf("stored page size = %q, want the value stored before the read failed",
			settings.site[content.PerPageSettingKey])
	}
	if code := errorCode(t, recorder); code == "database_busy" {
		t.Errorf("code = %q after the value was stored, want no answer telling the reader nothing was saved", code)
	}
}

func TestSettingsPatchThatStoredNothingAnswersABusyReadAsDatabaseBusy(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.lookupErr = errBusy

	wantBusyAnswer(t, askLocale(handler, http.MethodPatch, "/api/settings", `{}`))
}
