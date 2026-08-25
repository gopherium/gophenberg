// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/themehost"
)

// servingThemes is a themes directory that answers every ask, recording what it was told.
type servingThemes struct {
	installed  []themehost.Installed
	previous   string
	offered    bool
	activated  string
	deactivate int
	rolledBack string
	uploaded   string
	size       int64
}

// List returns the themes the directory holds.
func (s *servingThemes) List(context.Context) ([]themehost.Installed, error) {
	return s.installed, nil
}

// Install records the archive it was handed.
func (s *servingThemes) Install(_ context.Context, name string, _ io.ReaderAt, size int64) error {
	s.uploaded, s.size = name, size
	return nil
}

// Activate records the theme the site is served through.
func (s *servingThemes) Activate(_ context.Context, name string) error {
	s.activated = name
	return nil
}

// Deactivate records the return to the built-in renderer.
func (s *servingThemes) Deactivate(context.Context) error {
	s.deactivate++
	return nil
}

// Rollback returns the choice before the current one.
func (s *servingThemes) Rollback(context.Context) (string, error) {
	return s.rolledBack, nil
}

// Previous returns the choice a rollback would return to.
func (s *servingThemes) Previous(context.Context) (string, bool, error) {
	return s.previous, s.offered, nil
}

// askThemes returns the response to a request against the theme routes.
func askThemes(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestThemeListNamesEveryInstalledThemeAndTheRollback(t *testing.T) {
	t.Parallel()

	themes := &servingThemes{
		installed: []themehost.Installed{
			{Name: "aurora", Version: "1.0.0", Active: true, Serving: true},
			{Name: "driftwood", Broken: "the theme kit is not served"},
			{Name: "millpond", Version: "2.0.0", StartFailed: true},
		},
		previous: "driftwood",
		offered:  true,
	}

	recorder := askThemes(themeServer(t, themes), http.MethodGet, "/api/themes", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var listed struct {
		Themes []struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Broken      string `json:"broken"`
			Active      bool   `json:"active"`
			Serving     bool   `json:"serving"`
			StartFailed bool   `json:"startFailed"`
		} `json:"themes"`
		Rollback *struct {
			Theme string `json:"theme"`
		} `json:"rollback"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("reading the listing: %v", err)
	}
	if len(listed.Themes) != 3 {
		t.Fatalf("listed %d themes, want the three installed", len(listed.Themes))
	}
	if !listed.Themes[0].Active || !listed.Themes[0].Serving || listed.Themes[0].StartFailed {
		t.Errorf("aurora = %+v, want it active, serving and still trying", listed.Themes[0])
	}
	if listed.Themes[1].Broken == "" {
		t.Errorf("driftwood = %+v, want the reason it will not load", listed.Themes[1])
	}
	if !listed.Themes[2].StartFailed {
		t.Errorf("millpond = %+v, want the theme that stopped trying marked", listed.Themes[2])
	}
	if listed.Rollback == nil || listed.Rollback.Theme != "driftwood" {
		t.Errorf("rollback = %+v, want the choice before the current one", listed.Rollback)
	}
}

func TestThemeListOffersNoRollbackWhenThereIsNone(t *testing.T) {
	t.Parallel()

	recorder := askThemes(themeServer(t, &servingThemes{}), http.MethodGet, "/api/themes", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var listed map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &listed); err != nil {
		t.Fatalf("reading the listing: %v", err)
	}
	if _, offered := listed["rollback"]; offered {
		t.Error("the listing offers a rollback, want none until a theme has been activated")
	}
}

func TestThemeActivateNamesWhatIsServing(t *testing.T) {
	t.Parallel()

	themes := &servingThemes{}

	recorder := askThemes(themeServer(t, themes), http.MethodPost, "/api/themes/active", `{"name":"aurora"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if themes.activated != "aurora" {
		t.Errorf("activated = %q, want the theme the body named", themes.activated)
	}
}

func TestThemeDeactivateReturnsTheBuiltInRenderer(t *testing.T) {
	t.Parallel()

	themes := &servingThemes{}

	recorder := askThemes(themeServer(t, themes), http.MethodDelete, "/api/themes/active", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if themes.deactivate != 1 {
		t.Errorf("deactivated %d times, want once", themes.deactivate)
	}
}

func TestThemeRollbackNamesWhatItWentBackTo(t *testing.T) {
	t.Parallel()

	themes := &servingThemes{rolledBack: "driftwood"}

	recorder := askThemes(themeServer(t, themes), http.MethodPost, "/api/themes/rollback", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var answered struct {
		Active string `json:"active"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("reading the answer: %v", err)
	}
	if answered.Active != "driftwood" {
		t.Errorf("active = %q, want the choice it went back to", answered.Active)
	}
}

// deadlineRefusingWriter is a response writer whose read deadline cannot be moved.
type deadlineRefusingWriter struct {
	http.ResponseWriter
}

// SetReadDeadline reports that the deadline cannot be moved.
func (deadlineRefusingWriter) SetReadDeadline(time.Time) error {
	return errors.New("the connection will not take a new deadline")
}

func TestThemeUploadReportsADeadlineItCannotExtend(t *testing.T) {
	t.Parallel()

	handler := themeServer(t, &servingThemes{})
	contentType, body := uploadBody(t, "aurora.zip", []byte("an archive"))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/themes", body)
	request.Header.Set("Content-Type", contentType)

	handler.ServeHTTP(deadlineRefusingWriter{recorder}, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestThemeActivateReportsTheReasonAThemeWasRefused(t *testing.T) {
	t.Parallel()

	refused := &themehost.Error{
		Code:   "kit_unsupported",
		Reason: "the theme kit is not served",
		Detail: errors.New("built on 0.1.0"),
		Held:   map[string]any{"declared": "0.1.0"},
	}
	handler := themeServer(t, errThemes{err: refused})

	recorder := askThemes(handler, http.MethodPost, "/api/themes/active", `{"name":"aurora"}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	var answered struct {
		Code string         `json:"code"`
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("reading the refusal: %v", err)
	}
	if answered.Code != "kit_unsupported" {
		t.Errorf("code = %q, want the reason the theme was refused", answered.Code)
	}
	if answered.Meta["declared"] != "0.1.0" {
		t.Errorf("meta = %v, want the versions the refusal carries", answered.Meta)
	}
}
