// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/mediahost"
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

func TestSettingsPatchStoresNothingWhenTheSettingsCannotBeRead(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.lookupErr = context.DeadlineExceeded

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"content_per_page":5}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d, body %s",
			recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	settings.lookupErr = nil
	if held, stored, _ := settings.Lookup(t.Context(), content.PerPageSettingKey); stored {
		t.Errorf("the page size was stored as %q, want a refused answer to store nothing", held)
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

func TestSettingsPatchReportsAStoreThatWillNotWrite(t *testing.T) {
	t.Parallel()

	handler, settings := localeServer(t)
	settings.saveErr = context.DeadlineExceeded

	recorder := askLocale(handler, http.MethodPatch, "/api/settings", `{"content_per_page":5}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}
