// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

// storedSettings holds what a site and its readers chose, with per-method error injection.
type storedSettings struct {
	mu         sync.Mutex
	site       map[string]string
	reader     map[string]string
	lookupErr  error
	readerErr  error
	saveErr    error
	duringSave func()
}

// newStoredSettings returns an empty settings store.
func newStoredSettings() *storedSettings {
	return &storedSettings{site: map[string]string{}, reader: map[string]string{}}
}

// Lookup returns what the site chose for a key.
func (s *storedSettings) Lookup(_ context.Context, key string) (string, bool, error) {
	if s.lookupErr != nil {
		return "", false, s.lookupErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	held, found := s.site[key]
	return held, found, nil
}

// Save stores what the site chose.
func (s *storedSettings) Save(_ context.Context, values map[string]string) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	if s.duringSave != nil {
		s.duringSave()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, value := range values {
		s.site[key] = value
	}
	return nil
}

// readerStore answers one reader's settings out of the same store.
type readerStore struct {
	*storedSettings
}

// Lookup returns what one reader chose for a key.
func (s readerStore) Lookup(_ context.Context, _ uuid.UUID, key string) (string, bool, error) {
	if s.readerErr != nil {
		return "", false, s.readerErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	held, found := s.reader[key]
	return held, found, nil
}

// Save stores what one reader chose.
func (s readerStore) Save(_ context.Context, _ uuid.UUID, key, value string) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reader[key] = value
	return nil
}

// localeServer returns a signed in handler over a settings store, and the store behind it.
func localeServer(t *testing.T) (http.Handler, *storedSettings) {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	settings := newStoredSettings()
	cfg := serverConfig(users, newFakePostStore())
	cfg.Settings = settings
	cfg.Readers = readerStore{settings}
	handler := server.NewServer(cfg)
	cookie := loginCookie(t, handler)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
	return authed, settings
}

// askLocale returns the response to a request against the locale routes.
func askLocale(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)
	return recorder
}

// answeredLocale returns the locale a response names.
func answeredLocale(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var answered struct {
		Locale    string   `json:"locale"`
		Supported []string `json:"supported"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("reading the locale answer: %v", err)
	}
	if len(answered.Supported) == 0 {
		t.Error("the answer names no supported locales, want the list a client picks from")
	}
	return answered.Locale
}

func TestLocaleFollowsTheReaderOverTheSiteAndTheBrowser(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.site[content.LocaleSettingKey] = "en-US"
	settings.reader[content.LocaleSettingKey] = "es-ES"

	recorder := askLocale(handler, http.MethodGet, "/api/locale", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := answeredLocale(t, recorder); got != "es-ES" {
		t.Errorf("locale = %q, want the reader's own choice", got)
	}
}

func TestLocaleFollowsTheSiteWhenTheReaderChoseNothing(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.site[content.LocaleSettingKey] = "es-ES"

	recorder := askLocale(handler, http.MethodGet, "/api/locale", "")

	if got := answeredLocale(t, recorder); got != "es-ES" {
		t.Errorf("locale = %q, want the site default", got)
	}
}

func TestLocaleReportsASiteLookupFailure(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.lookupErr = context.DeadlineExceeded

	recorder := askLocale(handler, http.MethodGet, "/api/locale", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestLocaleReportsAReaderLookupFailure(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.readerErr = context.DeadlineExceeded

	recorder := askLocale(handler, http.MethodGet, "/api/locale", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestLocaleFollowsTheBrowserForAVisitorCarryingNoSession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	settings := newStoredSettings()
	cfg := serverConfig(users, newFakePostStore())
	cfg.Settings = settings
	cfg.Readers = readerStore{settings}
	handler := server.NewServer(cfg)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/locale", nil)
	request.Header.Set("Accept-Language", "es-ES")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := answeredLocale(t, recorder); got != "es-ES" {
		t.Errorf("locale = %q, want the language the browser asked for", got)
	}
}

func TestLocaleFollowsTheBrowserForACookieNamingNoSession(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	settings := newStoredSettings()
	cfg := serverConfig(users, newFakePostStore())
	cfg.Settings = settings
	cfg.Readers = readerStore{settings}
	handler := server.NewServer(cfg)
	named := loginCookie(t, handler).Name

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/locale", nil)
	request.Header.Set("Accept-Language", "es-ES")
	request.AddCookie(&http.Cookie{Name: named, Value: "a token no session was ever issued for"})
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := answeredLocale(t, recorder); got != "es-ES" {
		t.Errorf("locale = %q, want the language the browser asked for", got)
	}
}

func TestLocaleFollowsTheBrowserWhenNothingStoresAReaderChoice(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	handler := server.NewServer(serverConfig(users, newFakePostStore()))

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/locale", nil)
	request.Header.Set("Accept-Language", "es-ES")
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := answeredLocale(t, recorder); got != "es-ES" {
		t.Errorf("locale = %q, want the language the browser asked for", got)
	}
}

func TestLocalePatchStoresTheReadersOwnLanguage(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)

	recorder := askLocale(handler, http.MethodPatch, "/api/locale", `{"locale":"es-ES"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := answeredLocale(t, recorder); got != "es-ES" {
		t.Errorf("locale = %q, want the language just stored", got)
	}
	if held := settings.reader[content.LocaleSettingKey]; held != "es-ES" {
		t.Errorf("stored = %q, want the language the reader chose", held)
	}
}

func TestLocalePatchRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		body string
		want int
	}{
		"a body that is not json": {"{", http.StatusBadRequest},
		"a language the site does not answer in": {
			`{"locale":"xx-XX"}`, http.StatusUnprocessableEntity,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _ := localeServer(t)

			recorder := askLocale(handler, http.MethodPatch, "/api/locale", test.body)

			if recorder.Code != test.want {
				t.Errorf("status = %d, want %d", recorder.Code, test.want)
			}
		})
	}
}

func TestLocalePatchReportsAStoreFailure(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.saveErr = context.DeadlineExceeded

	recorder := askLocale(handler, http.MethodPatch, "/api/locale", `{"locale":"es-ES"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

// answeredDefault returns the site default a settings response names.
func answeredDefault(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()
	var answered struct {
		LocaleDefault string `json:"locale_default"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("reading the settings answer: %v", err)
	}
	return answered.LocaleDefault
}

func TestSettingsReportWhatTheSiteChose(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.site[content.LocaleSettingKey] = "es-ES"

	recorder := askLocale(handler, http.MethodGet, "/api/settings", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := answeredDefault(t, recorder); got != "es-ES" {
		t.Errorf("locale_default = %q, want the stored site default", got)
	}
}

func TestSettingsReportALookupFailure(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.lookupErr = context.DeadlineExceeded

	recorder := askLocale(handler, http.MethodGet, "/api/settings", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestSettingsPatchStoresTheSiteDefault(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"locale_default":"es-ES"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := answeredDefault(t, recorder); got != "es-ES" {
		t.Errorf("locale_default = %q, want the default just stored", got)
	}
	if held := settings.site[content.LocaleSettingKey]; held != "es-ES" {
		t.Errorf("stored = %q, want the default the site chose", held)
	}
}

func TestSettingsPatchClearsTheSiteDefault(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.site[content.LocaleSettingKey] = "es-ES"

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"locale_default":""}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if held := settings.site[content.LocaleSettingKey]; held != "" {
		t.Errorf("stored = %q, want the site back on the browser's language", held)
	}
}

func TestSettingsPatchLeavesTheDefaultAloneWhenTheBodyNamesNone(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.site[content.LocaleSettingKey] = "es-ES"

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if held := settings.site[content.LocaleSettingKey]; held != "es-ES" {
		t.Errorf("stored = %q, want the default untouched", held)
	}
}

func TestSettingsPatchRefusesWhatItCannotRead(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		body string
		want int
	}{
		"a body that is not json":                {"{", http.StatusBadRequest},
		"a language the site does not answer in": {`{"locale_default":"xx-XX"}`, http.StatusUnprocessableEntity},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, _ := localeServer(t)

			recorder := askLocale(handler, http.MethodPatch, "/api/settings", test.body)

			if recorder.Code != test.want {
				t.Errorf("status = %d, want %d", recorder.Code, test.want)
			}
		})
	}
}

func TestSettingsPatchReportsAStoreFailure(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.saveErr = context.DeadlineExceeded

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"locale_default":"es-ES"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
