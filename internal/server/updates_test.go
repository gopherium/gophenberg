// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
	"github.com/gopherium/gophenberg/internal/update"
	"github.com/gopherium/gophenberg/internal/updater"
)

// errUpdates is the failure a test's update checks answer with.
var errUpdates = errors.New("update store failure")

// storedUpdates answers the update routes from the result it holds, counting the checks it runs and the starts asked.
type storedUpdates struct {
	mu          sync.Mutex
	result      update.Result
	resultErr   error
	checked     update.Result
	checkErr    error
	checks      int
	overview    server.UpdatesOverview
	overviewErr error
	started     []server.UpdateStart
	startErr    error
}

// Start keeps the start asked for and answers what the test said it answers.
func (s *storedUpdates) Start(_ context.Context, asked server.UpdateStart) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = append(s.started, asked)
	return s.startErr
}

// Overview returns the overview the test stored.
func (s *storedUpdates) Overview(context.Context) (server.UpdatesOverview, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.overview, s.overviewErr
}

// Result returns the result the test stored.
func (s *storedUpdates) Result(context.Context) (update.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result, s.resultErr
}

// Check counts the check and returns what the test said it finds.
func (s *storedUpdates) Check(context.Context) (update.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.checks++
	return s.checked, s.checkErr
}

// updatesServer returns a server whose update routes held answers a minute apart at most, and an admin cookie.
func updatesServer(t *testing.T, held *storedUpdates) (http.Handler, *http.Cookie) {
	t.Helper()
	users := newFakeUserStore()
	addWithRole(t, users, "admin@example.com", "Maria Perez", role.Admin)
	handler := server.NewServer(server.Config{Users: users, Updates: held, UpdatesCheckAge: time.Minute})
	recorder := doLogin(t, handler, `{"email":"admin@example.com","password":"correct horse battery"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", recorder.Code, http.StatusOK)
	}
	return handler, sessionCookie(t, recorder)
}

// answeredResult returns the result a reply carries.
func answeredResult(t *testing.T, recorder *httptest.ResponseRecorder) update.Result {
	t.Helper()
	var result update.Result
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("reading the result %q: %v", recorder.Body.String(), err)
	}
	return result
}

// at returns a pointer to the moment ago before now.
func at(ago time.Duration) *time.Time {
	moment := time.Now().Add(-ago)
	return &moment
}

// answeredFields returns the fields a reply carries, by their names.
func answeredFields(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var fields map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &fields); err != nil {
		t.Fatalf("reading the fields of %q: %v", recorder.Body.String(), err)
	}
	return fields
}

func TestUpdatesGetAnswersTheStoredResultBesideTheInstall(t *testing.T) {
	t.Parallel()

	next := time.Date(2026, 10, 22, 21, 12, 0, 0, time.UTC)
	held := &storedUpdates{overview: server.UpdatesOverview{
		Result:  update.Result{CheckedAt: at(time.Hour), Offer: &update.Found{Version: "0.21.0"}},
		Running: "0.20.0", RunningPostgreSQL: new(15), CanUpdate: true, NextCheck: &next,
		Overview: updater.Overview{Refused: []string{"0.20.5"}, Backups: updater.BackupCount{Kept: 1, Limit: 3}},
	}}
	handler, cookie := updatesServer(t, held)

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/updates", ""})

	if recorder.Code != http.StatusOK || answeredResult(t, recorder).Offer.Version != "0.21.0" {
		t.Errorf("GET /api/updates = %d %s, want 200 and the stored offer", recorder.Code, recorder.Body.String())
	}
	fields := answeredFields(t, recorder)
	want := map[string]any{
		"running": "0.20.0", "running_postgresql": 15.0, "can_update": true, "refused": []any{"0.20.5"}, "job": nil,
		"backups": map[string]any{"kept": 1.0, "limit": 3.0, "newest": nil}, "next_check": "2026-10-22T21:12:00Z",
		"failed_at": nil, "error": "",
	}
	for name, value := range want {
		if held, found := fields[name]; !found || !reflect.DeepEqual(held, value) {
			t.Errorf("GET /api/updates answers %s = %#v (%v), want %#v", name, held, found, value)
		}
	}
	if _, found := fields["cannot_update"]; found {
		t.Errorf("GET /api/updates = %s, want no cannot_update for an install that updates itself", recorder.Body)
	}
}

func TestUpdatesGetNamesWhyAnInstallCannotUpdateItself(t *testing.T) {
	t.Parallel()

	held := &storedUpdates{overview: server.UpdatesOverview{CannotUpdate: updater.NoInstallRoot}}
	handler, cookie := updatesServer(t, held)

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/updates", ""})

	fields := answeredFields(t, recorder)
	if fields["can_update"] != false || fields["cannot_update"] != "no_install_root" {
		t.Errorf("GET /api/updates = %s, want can_update false and cannot_update no_install_root", recorder.Body)
	}
	if _, found := fields["running_postgresql"]; found {
		t.Errorf("GET /api/updates = %s, want no running_postgresql for a site that names no PostgreSQL major",
			recorder.Body)
	}
}

func TestUpdatesGetMasksAStoreItCannotRead(t *testing.T) {
	t.Parallel()

	handler, cookie := updatesServer(t, &storedUpdates{overviewErr: errUpdates})

	recorder := ask(handler, cookie, call{http.MethodGet, "/api/updates", ""})

	if recorder.Code != http.StatusInternalServerError || refusalOf(t, recorder).Code != "internal" {
		t.Errorf("GET /api/updates = %d %s, want 500 and the internal code", recorder.Code, recorder.Body.String())
	}
}

func TestUpdatesCheckRunsACheckWhenTheLastIsOldEnough(t *testing.T) {
	t.Parallel()

	for name, last := range map[string]update.Result{
		"before any check":     {},
		"after an old check":   {CheckedAt: at(time.Hour)},
		"after an old failure": {CheckedAt: at(2 * time.Hour), FailedAt: at(time.Hour), Error: "update: fetch failed"},
	} {
		newer := update.Result{CheckedAt: at(0), Offer: &update.Found{Version: "0.22.0"}}
		held := &storedUpdates{result: last, checked: newer}
		handler, cookie := updatesServer(t, held)

		recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/check", ""})

		if recorder.Code != http.StatusOK || held.checks != 1 || answeredResult(t, recorder).Offer.Version != "0.22.0" {
			t.Errorf("POST /api/updates/check %s = %d %s after %d checks, want 200 and one check", name, recorder.Code,
				recorder.Body.String(), held.checks)
		}
	}
}

func TestUpdatesCheckRefusesACheckSoonAfterTheLast(t *testing.T) {
	t.Parallel()

	for name, last := range map[string]update.Result{
		"after a check":   {CheckedAt: at(10 * time.Second)},
		"after a failure": {CheckedAt: at(time.Hour), FailedAt: at(10 * time.Second), Error: "update: fetch failed"},
	} {
		held := &storedUpdates{result: last}
		handler, cookie := updatesServer(t, held)

		recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/check", ""})

		refused := recorder.Code == http.StatusTooManyRequests && refusalOf(t, recorder).Code == "check_too_soon"
		if !refused || held.checks != 0 {
			t.Errorf("POST /api/updates/check %s = %d %s, want 429 and check_too_soon with no check", name,
				recorder.Code, recorder.Body.String())
		}
		if wait, err := strconv.Atoi(recorder.Header().Get("Retry-After")); err != nil || wait < 1 || wait > 50 {
			t.Errorf("Retry-After = %q %s, want the seconds left, at most 50", recorder.Header().Get("Retry-After"), name)
		}
	}
}

func TestUpdatesCheckAnswersAFailedCheckWithItsStoredResult(t *testing.T) {
	t.Parallel()

	failed := update.Result{FailedAt: at(0), Error: "update: refused the index, which expired at 2026-10-01T00:00:00Z"}
	held := &storedUpdates{checked: failed, checkErr: errors.New(failed.Error)}
	handler, cookie := updatesServer(t, held)

	recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/check", ""})

	if recorder.Code != http.StatusOK || answeredResult(t, recorder).Error != failed.Error {
		t.Errorf("POST /api/updates/check = %d %s, want 200 and the failure beside the last result", recorder.Code,
			recorder.Body.String())
	}
}

func TestUpdatesApplyStartsTheUpdateAskedForAndAnswersTheOverview(t *testing.T) {
	t.Parallel()

	held := &storedUpdates{overview: server.UpdatesOverview{Running: "0.20.0", CanUpdate: true}}
	handler, cookie := updatesServer(t, held)

	recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/apply", `{"version":"0.21.0","retry":true}`})

	if recorder.Code != http.StatusAccepted || answeredFields(t, recorder)["running"] != "0.20.0" {
		t.Errorf("POST /api/updates/apply = %d %s, want 202 and the overview", recorder.Code, recorder.Body.String())
	}
	if want := []server.UpdateStart{{Version: "0.21.0", Retry: true}}; !reflect.DeepEqual(held.started, want) {
		t.Errorf("POST /api/updates/apply started %+v, want %+v", held.started, want)
	}
}

func TestUpdatesApplyAnswersWhyTheStartWasRefused(t *testing.T) {
	t.Parallel()

	reason := "updater: this is a custom build of Gophenberg 0.20.0, so rebuild it on Gophenberg 0.21.0"
	for name, tc := range map[string]struct {
		err     error
		status  int
		code    string
		message string
	}{
		"another update runs": {
			err: fmt.Errorf("take it: %w", updater.ErrUpdateRuns), status: http.StatusConflict, code: "update_runs",
			message: "another update runs",
		},
		"a check that names its code": {
			err:    content.Refuse(server.ErrUpdateRefused, "custom_build", reason, nil),
			status: http.StatusUnprocessableEntity, code: "custom_build", message: reason,
		},
		"a check that names no code": {
			err:    content.Refuse(server.ErrUpdateRefused, "", reason, nil),
			status: http.StatusUnprocessableEntity, code: "update_refused", message: reason,
		},
		"a failure it cannot name": {
			err: errUpdates, status: http.StatusInternalServerError, code: "internal", message: "internal error",
		},
	} {
		handler, cookie := updatesServer(t, &storedUpdates{startErr: tc.err})

		recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/apply", `{"version":"0.21.0"}`})

		var got struct {
			Code    string `json:"code"`
			Message string `json:"error"`
		}
		_ = json.Unmarshal(recorder.Body.Bytes(), &got)
		if recorder.Code != tc.status || got.Code != tc.code || !strings.Contains(got.Message, tc.message) {
			t.Errorf("POST /api/updates/apply when %s = %d %s, want %d and %s saying %q", name, recorder.Code,
				recorder.Body.String(), tc.status, tc.code, tc.message)
		}
	}
}

func TestUpdatesApplyRefusesABodyThatNamesNoVersionToStart(t *testing.T) {
	t.Parallel()

	for body, want := range map[string]struct {
		status int
		code   string
	}{
		`{"retry":true}`:                  {http.StatusUnprocessableEntity, "version_required"},
		`{"version":""}`:                  {http.StatusUnprocessableEntity, "version_required"},
		`{"version":"0.21.0","to":"0.2"}`: {http.StatusBadRequest, "body_unknown_attribute"},
		`{"version":`:                     {http.StatusBadRequest, "body_malformed"},
	} {
		held := &storedUpdates{}
		handler, cookie := updatesServer(t, held)

		recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/apply", body})

		if recorder.Code != want.status || answeredFields(t, recorder)["code"] != want.code || len(held.started) != 0 {
			t.Errorf("POST /api/updates/apply %s = %d %s after %d starts, want %d and %s with no start", body,
				recorder.Code, recorder.Body.String(), len(held.started), want.status, want.code)
		}
	}
}

func TestUpdatesApplyMasksAnOverviewItCannotReadOnceTheUpdateStarted(t *testing.T) {
	t.Parallel()

	held := &storedUpdates{overviewErr: errUpdates}
	handler, cookie := updatesServer(t, held)

	recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/apply", `{"version":"0.21.0"}`})

	if recorder.Code != http.StatusInternalServerError || len(held.started) != 1 {
		t.Errorf("POST /api/updates/apply = %d %s after %d starts, want 500 after one start", recorder.Code,
			recorder.Body.String(), len(held.started))
	}
}

func TestUpdatesCheckMasksAStoreItCannotUse(t *testing.T) {
	t.Parallel()

	failed := update.Result{FailedAt: at(0), Error: "update: fetch failed"}
	notStored := fmt.Errorf("%w: %w", update.ErrNotStored, errUpdates)
	for name, held := range map[string]*storedUpdates{
		"a result it cannot read":   {resultErr: errUpdates},
		"a check it cannot store":   {checkErr: errUpdates},
		"a failure it cannot store": {checked: failed, checkErr: notStored},
	} {
		handler, cookie := updatesServer(t, held)

		recorder := ask(handler, cookie, call{http.MethodPost, "/api/updates/check", ""})

		if recorder.Code != http.StatusInternalServerError || refusalOf(t, recorder).Code != "internal" {
			t.Errorf("POST /api/updates/check with %s = %d %s, want 500 and the internal code", name, recorder.Code,
				recorder.Body.String())
		}
	}
}
