// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/mediahost"
)

// settingsResponse names the values the site chose for itself.
type settingsResponse struct {
	LocaleDefault  string `json:"locale_default"`
	ContentPerPage int    `json:"content_per_page"`
	JPEGQuality    int    `json:"jpeg_quality"`
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
		held, err := s.chosenSettings(r.Context())
		if err != nil {
			respondDomainError(w, err)
			return
		}
		if len(values) > 0 {
			if err := s.settings.Save(r.Context(), values); err != nil {
				respondDomainError(w, err)
				return
			}
		}
		authkit.Respond(w, http.StatusOK, settingsAfter(held, req))
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
	}, nil
}

// settingsAfter returns what the site holds once the values the request names are stored.
func settingsAfter(held settingsResponse, req settingsRequest) settingsResponse {
	if req.LocaleDefault != nil {
		held.LocaleDefault = *req.LocaleDefault
	}
	if req.ContentPerPage != nil {
		held.ContentPerPage = *req.ContentPerPage
	}
	if req.JPEGQuality != nil {
		held.JPEGQuality = *req.JPEGQuality
	}
	return held
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
