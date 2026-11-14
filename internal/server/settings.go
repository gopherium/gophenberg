// SPDX-License-Identifier: Apache-2.0

package server

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/gopherium/gouncer/authkit"
	"github.com/gopherium/gouncer/avatar"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/mediahost"
)

// The list settings served when the environment names none.
const (
	DefaultListPageSize        = 20
	DefaultListPageCap         = 100
	DefaultPickerPageSize      = 50
	DefaultToastDuration       = 6 * time.Second
	DefaultToastActionDuration = 10 * time.Second
	DefaultFormatLocale        = "es-ES"
)

// DefaultListPageSizes returns the page sizes a list offers when the environment names none.
func DefaultListPageSizes() []int {
	return []int{10, 20, 50, 100}
}

// DefaultPickerPageSizeAmong returns DefaultPickerPageSize when the sizes offer it, else the opening size.
func DefaultPickerPageSizeAmong(sizes []int, opening int) int {
	if slices.Contains(sizes, DefaultPickerPageSize) {
		return DefaultPickerPageSize
	}
	return opening
}

// ListSettings carries what the admin lists read once, as the environment named it.
type ListSettings struct {
	// PageSizes lists the page sizes a list offers, from the smallest up. Empty serves the defaults.
	PageSizes []int
	// PageSize is the page a list opens on and the size a request naming none is answered at.
	PageSize int
	// PickerPageSize is the page the media picker opens on. Zero serves its default among the page sizes.
	PickerPageSize int
	// PageCap is the most items one admin page carries, a larger request answered at the cap.
	PageCap int
	// ToastDuration is how long a confirmation toast with no action stays on screen.
	ToastDuration time.Duration
	// ToastActionDuration is how long a confirmation toast with an action, such as Undo, stays on screen.
	ToastActionDuration time.Duration
	// FormatLocale is the locale dates and numbers are written in, whatever the interface language.
	FormatLocale string
}

// listsOf returns the list settings the configuration names, each zero value replaced by its default.
func listsOf(cfg Config) ListSettings {
	held := cfg.Lists
	if len(held.PageSizes) == 0 {
		held.PageSizes = DefaultListPageSizes()
	}
	held.PageSize = cmp.Or(held.PageSize, DefaultListPageSize)
	held.PickerPageSize = cmp.Or(held.PickerPageSize, DefaultPickerPageSizeAmong(held.PageSizes, held.PageSize))
	held.PageCap = cmp.Or(held.PageCap, DefaultListPageCap)
	held.ToastDuration = cmp.Or(held.ToastDuration, DefaultToastDuration)
	held.ToastActionDuration = cmp.Or(held.ToastActionDuration, DefaultToastActionDuration)
	held.FormatLocale = cmp.Or(held.FormatLocale, DefaultFormatLocale)
	return held
}

// settingsResponse names the values the site chose for itself and the settings it serves read only.
type settingsResponse struct {
	LocaleDefault           string `json:"locale_default"`
	ContentPerPage          int    `json:"content_per_page"`
	JPEGQuality             int    `json:"jpeg_quality"`
	ListPageSizes           []int  `json:"list_page_sizes"`
	ListPageSize            int    `json:"list_page_size"`
	PickerPageSize          int    `json:"picker_page_size"`
	ToastMilliseconds       int64  `json:"toast_milliseconds"`
	ToastActionMilliseconds int64  `json:"toast_action_milliseconds"`
	FormatLocale            string `json:"format_locale"`
	UpdatesProgress         int64  `json:"updates_progress_milliseconds"`
	UpdatesRestart          int64  `json:"updates_restart_wait_milliseconds"`
	ShowAvatars             bool   `json:"show_avatars"`
	DefaultRole             string `json:"default_role"`
}

// settingsRequest names the values a caller asks the site to choose.
type settingsRequest struct {
	LocaleDefault  *string `json:"locale_default"`
	ContentPerPage *int    `json:"content_per_page"`
	JPEGQuality    *int    `json:"jpeg_quality"`
}

// handleSettingsGet returns an http.HandlerFunc reporting what the site chose for itself.
func (s *server) handleSettingsGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		held, err := s.chosenSettings(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, held)
	}
}

// handleSettingsPatch returns an http.HandlerFunc storing what the site chose for itself.
func (s *server) handleSettingsPatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := authkit.Decode[settingsRequest](w, r)
		if err != nil {
			authkit.RespondError(w, http.StatusBadRequest, authkit.ErrorResponse{
				Message: "malformed json", Code: "body_malformed",
			})
			return
		}
		values, err := settingsAsked(req)
		if err != nil {
			respondSettingsError(w, err)
			return
		}
		fail := respondDomainError
		if len(values) > 0 {
			if err := s.settings.Save(r.Context(), values); err != nil {
				respondDomainError(w, err)
				return
			}
			fail = respondReadAfterWrite
		}
		held, err := s.chosenSettings(r.Context())
		if err != nil {
			fail(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, held)
	}
}

// chosenSettings returns what the site chose, each value falling back to its own default.
func (s *server) chosenSettings(ctx context.Context) (settingsResponse, error) {
	held := map[string]string{}
	found := map[string]bool{}
	for _, key := range []string{content.LocaleSettingKey, content.PerPageSettingKey, mediahost.JPEGQualityKey} {
		value, stored, err := s.settings.Lookup(ctx, key)
		if err != nil {
			return settingsResponse{}, err
		}
		held[key], found[key] = value, stored
	}
	return settingsResponse{
		LocaleDefault:  held[content.LocaleSettingKey],
		ContentPerPage: content.ResolvePerPage(held[content.PerPageSettingKey], found[content.PerPageSettingKey]),
		JPEGQuality: mediahost.ResolveJPEGQuality(
			held[mediahost.JPEGQualityKey], found[mediahost.JPEGQualityKey]),
		ListPageSizes:           s.lists.PageSizes,
		ListPageSize:            s.lists.PageSize,
		PickerPageSize:          s.lists.PickerPageSize,
		ToastMilliseconds:       s.lists.ToastDuration.Milliseconds(),
		ToastActionMilliseconds: s.lists.ToastActionDuration.Milliseconds(),
		FormatLocale:            s.lists.FormatLocale,
		UpdatesProgress:         s.updatesScreen.ProgressInterval.Milliseconds(),
		UpdatesRestart:          s.updatesScreen.RestartWait.Milliseconds(),
		ShowAvatars:             s.avatars.Mode != avatar.ModeOff,
		DefaultRole:             s.defaultRole,
	}, nil
}

// settingsAsked returns the values the request names, or the reason one of them stands refused.
func settingsAsked(req settingsRequest) (map[string]string, error) {
	values := map[string]string{}
	if req.LocaleDefault != nil {
		if *req.LocaleDefault != "" {
			if err := content.ValidateLocale(*req.LocaleDefault); err != nil {
				return nil, err
			}
		}
		values[content.LocaleSettingKey] = *req.LocaleDefault
	}
	if req.ContentPerPage != nil {
		size := strconv.Itoa(*req.ContentPerPage)
		if _, err := content.ParsePerPage(size); err != nil {
			return nil, err
		}
		values[content.PerPageSettingKey] = size
	}
	if req.JPEGQuality != nil {
		quality := strconv.Itoa(*req.JPEGQuality)
		if _, err := mediahost.ParseJPEGQuality(quality); err != nil {
			return nil, err
		}
		values[mediahost.JPEGQualityKey] = quality
	}
	return values, nil
}

// respondSettingsError writes a refused setting as the reason the operator reads.
func respondSettingsError(w http.ResponseWriter, err error) {
	var refused *mediahost.Error
	if errors.As(err, &refused) {
		authkit.RespondError(w, http.StatusUnprocessableEntity, authkit.ErrorResponse{
			Message: refused.Reason, Code: refused.Code, Meta: refused.Meta,
		})
		return
	}
	respondDomainError(w, err)
}
