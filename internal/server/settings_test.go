// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/role"
	"github.com/gopherium/gophenberg/internal/server"
)

// answeredSettings returns the settings the answer carries.
func answeredSettings(t *testing.T, recorder *httptest.ResponseRecorder) struct {
	LocaleDefault  string `json:"locale_default"`
	ContentPerPage int    `json:"content_per_page"`
	JPEGQuality    int    `json:"jpeg_quality"`
} {
	t.Helper()
	var held struct {
		LocaleDefault  string `json:"locale_default"`
		ContentPerPage int    `json:"content_per_page"`
		JPEGQuality    int    `json:"jpeg_quality"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("reading the answer: %v, want nil", err)
	}
	return held
}

func TestSettingsGetAnswersTheDefaultsWhenTheSiteChoseNone(t *testing.T) {
	t.Parallel()

	handler, _ := localeServer(t)

	recorder := askLocale(handler, http.MethodGet, "/api/settings", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	held := answeredSettings(t, recorder)
	if held.ContentPerPage != content.DefaultPerPage {
		t.Errorf("content_per_page = %d, want the default %d", held.ContentPerPage, content.DefaultPerPage)
	}
	if held.JPEGQuality != mediahost.DefaultJPEGQuality {
		t.Errorf("jpeg_quality = %d, want the default %d", held.JPEGQuality, mediahost.DefaultJPEGQuality)
	}
}

func TestSettingsGetAnswersWhatTheSiteChose(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.site[content.PerPageSettingKey] = "5"
	settings.site[mediahost.JPEGQualityKey] = "30"

	recorder := askLocale(handler, http.MethodGet, "/api/settings", "")

	held := answeredSettings(t, recorder)
	if held.ContentPerPage != 5 {
		t.Errorf("content_per_page = %d, want 5", held.ContentPerPage)
	}
	if held.JPEGQuality != 30 {
		t.Errorf("jpeg_quality = %d, want 30", held.JPEGQuality)
	}
}

func TestSettingsGetAnswersTheDefaultForARowItCannotUse(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.site[content.PerPageSettingKey] = "many"
	settings.site[mediahost.JPEGQualityKey] = "0"

	recorder := askLocale(handler, http.MethodGet, "/api/settings", "")

	held := answeredSettings(t, recorder)
	if held.ContentPerPage != content.DefaultPerPage {
		t.Errorf("content_per_page = %d, want the default kept", held.ContentPerPage)
	}
	if held.JPEGQuality != mediahost.DefaultJPEGQuality {
		t.Errorf("jpeg_quality = %d, want the default kept", held.JPEGQuality)
	}
}

func TestSettingsPatchStoresThePageSizeAndTheQuality(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)

	recorder := askLocale(handler, http.MethodPatch, "/api/settings",
		`{"content_per_page":5,"jpeg_quality":30}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	held := answeredSettings(t, recorder)
	if held.ContentPerPage != 5 || held.JPEGQuality != 30 {
		t.Errorf("answered %+v, want the values just stored", held)
	}
	if stood := settings.site[content.PerPageSettingKey]; stood != "5" {
		t.Errorf("stored page size = %q, want 5", stood)
	}
	if stood := settings.site[mediahost.JPEGQualityKey]; stood != "30" {
		t.Errorf("stored quality = %q, want 30", stood)
	}
}

func TestSettingsPatchLeavesUnnamedValuesAlone(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.site[content.PerPageSettingKey] = "5"
	settings.site[mediahost.JPEGQualityKey] = "30"

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"locale_default":"es-ES"}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	held := answeredSettings(t, recorder)
	if held.ContentPerPage != 5 || held.JPEGQuality != 30 {
		t.Errorf("answered %+v, want the values the site already chose", held)
	}
	if stood := settings.site[content.PerPageSettingKey]; stood != "5" {
		t.Errorf("stored page size = %q, want it left alone", stood)
	}
}

func TestSettingsPatchAnswersWhatStandsWhenItNamesNothing(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.site[content.PerPageSettingKey] = "5"

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	held := answeredSettings(t, recorder)
	if held.ContentPerPage != 5 {
		t.Errorf("content_per_page = %d, want the stored 5, never a zero", held.ContentPerPage)
	}
	if held.JPEGQuality != mediahost.DefaultJPEGQuality {
		t.Errorf("jpeg_quality = %d, want the default, never a zero", held.JPEGQuality)
	}
}

func TestSettingsPatchRefusesAPageSizeOutsideItsBounds(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"zero":              `{"content_per_page":0}`,
		"below zero":        `{"content_per_page":-3}`,
		"one past the most": `{"content_per_page":101}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, settings := localeServer(t)

			recorder := askLocale(handler, http.MethodPatch, "/api/settings", body)

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d, body %s",
					recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
			}
			if code := errorCode(t, recorder); code != "per_page_invalid" {
				t.Errorf("code = %q, want per_page_invalid", code)
			}
			if _, stored := settings.site[content.PerPageSettingKey]; stored {
				t.Error("the page size was stored, want a refused value kept out")
			}
		})
	}
}

func TestSettingsPatchRefusesAQualityOutsideItsBounds(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"zero":              `{"jpeg_quality":0}`,
		"below zero":        `{"jpeg_quality":-10}`,
		"one past the most": `{"jpeg_quality":101}`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, settings := localeServer(t)

			recorder := askLocale(handler, http.MethodPatch, "/api/settings", body)

			if recorder.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status = %d, want %d, body %s",
					recorder.Code, http.StatusUnprocessableEntity, recorder.Body.String())
			}
			if code := errorCode(t, recorder); code != "jpeg_quality_invalid" {
				t.Errorf("code = %q, want jpeg_quality_invalid", code)
			}
			if _, stored := settings.site[mediahost.JPEGQualityKey]; stored {
				t.Error("the quality was stored, want a refused value kept out")
			}
		})
	}
}

func TestSettingsPatchAnswersWithWhatAnotherWriterStoredMeanwhile(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.duringSave = func() { settings.site[mediahost.JPEGQualityKey] = "30" }

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"content_per_page":5}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	answered := answeredSettings(t, recorder)
	if answered.JPEGQuality != 30 || answered.ContentPerPage != 5 {
		t.Errorf("the answer carries per page %d and quality %d, want 5 and the 30 stored meanwhile",
			answered.ContentPerPage, answered.JPEGQuality)
	}
}

func TestSettingsPatchNamesTheHighestQualityItTakes(t *testing.T) {
	t.Parallel()

	handler, _ := localeServer(t)

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"jpeg_quality":101}`)

	var answered struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &answered); err != nil {
		t.Fatalf("decoding the answer: %v", err)
	}
	if answered.Meta["max"] != float64(mediahost.MaxJPEGQuality) {
		t.Errorf("meta max = %v, want %d", answered.Meta["max"], mediahost.MaxJPEGQuality)
	}
}

func TestSettingsPatchStoresEveryNamedValueTogether(t *testing.T) {
	t.Parallel()

	handler, _ := localeServer(t)

	recorder := askLocale(handler, http.MethodPatch, "/api/settings",
		`{"locale_default":"fr-FR","content_per_page":7,"jpeg_quality":45}`)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	held := answeredSettings(t, recorder)
	if held.LocaleDefault != "fr-FR" || held.ContentPerPage != 7 || held.JPEGQuality != 45 {
		t.Errorf("answered %+v, want every named value carried", held)
	}
}

func TestSettingsPatchKeepsEveryValueOutWhenOneIsRefused(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)

	recorder := askLocale(handler, http.MethodPatch, "/api/settings",
		`{"content_per_page":5,"jpeg_quality":500}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	if _, stored := settings.site[content.PerPageSettingKey]; stored {
		t.Error("the page size was stored beside a refused quality, want neither kept")
	}
}

func TestSettingsReportAStoreThatWillNotAnswer(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		method string
		body   string
	}{
		"reading them":      {http.MethodGet, ""},
		"reading them back": {http.MethodPatch, `{"content_per_page":5}`},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			handler, settings := localeServer(t)
			settings.lookupErr = context.DeadlineExceeded

			recorder := askLocale(handler, asked.method, "/api/settings", asked.body)

			if recorder.Code != http.StatusInternalServerError {
				t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
			}
		})
	}
}

// servedLists is what the settings answer carries for the admin lists.
type servedLists struct {
	PageSizes               []int  `json:"list_page_sizes"`
	PageSize                int    `json:"list_page_size"`
	PickerPageSize          int    `json:"picker_page_size"`
	ToastMilliseconds       int64  `json:"toast_milliseconds"`
	ToastActionMilliseconds int64  `json:"toast_action_milliseconds"`
	ToastNameLength         *int   `json:"toast_name_length"`
	FormatLocale            string `json:"format_locale"`
	DialogWriteMilliseconds int64  `json:"dialog_write_timeout_milliseconds"`
}

// listedSettingsServer returns a signed in handler serving the list settings it is given.
func listedSettingsServer(t *testing.T, lists server.ListSettings) http.Handler {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	cfg := serverConfig(users, newFakePostStore())
	cfg.Settings = newStoredSettings()
	cfg.Lists = lists
	return authedServerWithStores(t, cfg)
}

// answeredLists returns what the settings answer carries for the admin lists.
func answeredLists(t *testing.T, recorder *httptest.ResponseRecorder) servedLists {
	t.Helper()
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	var held servedLists
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil {
		t.Fatalf("reading the answer: %v, want nil", err)
	}
	return held
}

func TestSettingsGetServesTheListDefaultsWhenTheEnvironmentNamedNone(t *testing.T) {
	t.Parallel()

	handler := listedSettingsServer(t, server.ListSettings{})

	held := answeredLists(t, askLocale(handler, http.MethodGet, "/api/settings", ""))

	want := servedLists{
		PageSizes: server.DefaultListPageSizes(), PageSize: server.DefaultListPageSize,
		PickerPageSize:          server.DefaultPickerPageSize,
		ToastMilliseconds:       server.DefaultToastDuration.Milliseconds(),
		ToastActionMilliseconds: server.DefaultToastActionDuration.Milliseconds(),
		FormatLocale:            server.DefaultFormatLocale,
		DialogWriteMilliseconds: 60000,
	}
	if !reflect.DeepEqual(held, want) {
		t.Errorf("served %+v, want the defaults %+v", held, want)
	}
}

func TestSettingsGetServesTheListSettingsTheEnvironmentNamed(t *testing.T) {
	t.Parallel()

	handler := listedSettingsServer(t, server.ListSettings{
		PageSizes: []int{5, 15}, PageSize: 15, PickerPageSize: 5, PageCap: 40,
		ToastDuration: 1500 * time.Millisecond, ToastActionDuration: 2500 * time.Millisecond, FormatLocale: "en-GB",
		DialogWriteTimeout: 45 * time.Second,
	})

	held := answeredLists(t, askLocale(handler, http.MethodGet, "/api/settings", ""))

	want := servedLists{
		PageSizes: []int{5, 15}, PageSize: 15, PickerPageSize: 5,
		ToastMilliseconds: 1500, ToastActionMilliseconds: 2500, FormatLocale: "en-GB", DialogWriteMilliseconds: 45000,
	}
	if !reflect.DeepEqual(held, want) {
		t.Errorf("served %+v, want %+v", held, want)
	}
}

func TestSettingsGetOpensThePickerAtTheListPageSizeWhenTheSizesLeaveOutItsDefault(t *testing.T) {
	t.Parallel()

	handler := listedSettingsServer(t, server.ListSettings{PageSizes: []int{5, 15, 45}, PageSize: 15})

	held := answeredLists(t, askLocale(handler, http.MethodGet, "/api/settings", ""))

	if held.PickerPageSize != 15 {
		t.Errorf("picker_page_size = %d, want the list page size 15", held.PickerPageSize)
	}
}

func TestSettingsPatchLeavesTheListSettingsAsTheEnvironmentNamedThem(t *testing.T) {
	t.Parallel()

	handler := listedSettingsServer(t, server.ListSettings{PageSizes: []int{5, 15}, PageSize: 15})

	held := answeredLists(t, askLocale(handler, http.MethodPatch, "/api/settings",
		`{"list_page_sizes":[1,2],"list_page_size":2,"picker_page_size":5,"toast_milliseconds":1,`+
			`"toast_action_milliseconds":1,"format_locale":"fr-FR","dialog_write_timeout_milliseconds":1}`))

	kept := slices.Equal(held.PageSizes, []int{5, 15}) && held.PageSize == 15 && held.PickerPageSize == 15
	timed := held.ToastMilliseconds == server.DefaultToastDuration.Milliseconds() &&
		held.ToastActionMilliseconds == server.DefaultToastActionDuration.Milliseconds() &&
		held.DialogWriteMilliseconds == server.DefaultDialogWriteTimeout.Milliseconds()
	if !kept || !timed || held.FormatLocale != server.DefaultFormatLocale {
		t.Errorf("served %+v, want the list settings the environment named, whatever a write asks", held)
	}
}

func TestSettingsGetTellsTheUpdatesScreenHowOftenToAskAndHowLongToWait(t *testing.T) {
	t.Parallel()

	users := newFakeUserStore()
	addAda(t, users)
	cfg := serverConfig(users, newFakePostStore())
	cfg.Settings = newStoredSettings()
	cfg.UpdatesScreen = server.UpdatesScreen{ProgressInterval: 1500 * time.Millisecond, RestartWait: 2 * time.Minute}
	handler := authedServerWithStores(t, cfg)

	recorder := askLocale(handler, http.MethodGet, "/api/settings", "")

	var held struct {
		Progress *int64 `json:"updates_progress_milliseconds"`
		Wait     *int64 `json:"updates_restart_wait_milliseconds"`
	}
	err := json.Unmarshal(recorder.Body.Bytes(), &held)
	if err != nil || held.Progress == nil || held.Wait == nil || *held.Progress != 1500 || *held.Wait != 120000 {
		t.Errorf("GET /api/settings = %d %s, want 1500 and 120000 milliseconds for the Updates screen", recorder.Code,
			recorder.Body.String())
	}
}

// servedDefaultRole returns the default_role the settings answer carries when the configuration names configured.
func servedDefaultRole(t *testing.T, configured, method, body string) string {
	t.Helper()
	users := newFakeUserStore()
	addAda(t, users)
	cfg := serverConfig(users, newFakePostStore())
	cfg.Settings = newStoredSettings()
	cfg.DefaultRole = configured
	recorder := askLocale(authedServerWithStores(t, cfg), method, "/api/settings", body)
	var held struct {
		DefaultRole *string `json:"default_role"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &held); err != nil || held.DefaultRole == nil {
		t.Fatalf("%s /api/settings = %d %s, want a default_role", method, recorder.Code, recorder.Body.String())
	}
	return *held.DefaultRole
}

func TestSettingsGetServesTheMemberRoleWhenTheEnvironmentNamesNoDefaultRole(t *testing.T) {
	t.Parallel()

	if got := servedDefaultRole(t, "", http.MethodGet, ""); got != role.Member {
		t.Errorf("default_role = %q, want %q", got, role.Member)
	}
}

func TestSettingsGetServesTheDefaultRoleTheEnvironmentNamed(t *testing.T) {
	t.Parallel()

	if got := servedDefaultRole(t, role.Author, http.MethodGet, ""); got != role.Author {
		t.Errorf("default_role = %q, want %q", got, role.Author)
	}
}

func TestSettingsPatchLeavesTheDefaultRoleAsTheEnvironmentNamedIt(t *testing.T) {
	t.Parallel()

	if got := servedDefaultRole(t, role.Author, http.MethodPatch, `{"default_role":"admin"}`); got != role.Author {
		t.Errorf("default_role = %q, want the %q the environment named, whatever a write asks", got, role.Author)
	}
}

func TestSettingsPatchReportsAStoreThatWillNotWrite(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.saveErr = context.DeadlineExceeded

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"content_per_page":5}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
