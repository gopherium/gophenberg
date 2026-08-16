// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer/authkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// SiteSettings persists the values a site chooses for itself.
type SiteSettings interface {
	Lookup(ctx context.Context, key string) (string, bool, error)
	Save(ctx context.Context, values map[string]string) error
}

// ReaderSettings persists the values one reader chooses for themselves.
type ReaderSettings interface {
	Lookup(ctx context.Context, userID uuid.UUID, key string) (string, bool, error)
	Save(ctx context.Context, userID uuid.UUID, key, value string) error
}

// localeResponse names the language a client is answered in.
type localeResponse struct {
	Locale    string   `json:"locale"`
	Supported []string `json:"supported"`
}

// handleLocaleGet returns an http.HandlerFunc naming the language to answer in.
func (s *server) handleLocaleGet() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		asked := content.LocaleAsked{Accepted: r.Header.Get("Accept-Language")}
		if s.settings != nil {
			held, _, err := s.settings.Lookup(r.Context(), content.LocaleSettingKey)
			if err != nil {
				respondDomainError(w, err)
				return
			}
			asked.Site = held
		}
		asked.User = s.readerLocale(r)
		authkit.Respond(w, http.StatusOK, localeResponse{
			Locale:    content.ResolveLocale(asked),
			Supported: content.SupportedLocales,
		})
	}
}

// readerLocale returns the language the signed in reader chose, if a session names one.
func (s *server) readerLocale(r *http.Request) string {
	if s.readers == nil {
		return ""
	}
	cookie, err := r.Cookie(s.auth.CookieName())
	if err != nil {
		return ""
	}
	identity, err := s.auth.SessionIdentity(r.Context(), cookie.Value)
	if err != nil {
		return ""
	}
	held, _, err := s.readers.Lookup(r.Context(), identity.ID, content.LocaleSettingKey)
	if err != nil {
		return ""
	}
	return held
}

// handleLocalePatch returns an http.HandlerFunc storing the reader's own language.
func (s *server) handleLocalePatch() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := authkit.Decode[struct {
			Locale string `json:"locale"`
		}](w, r)
		if err != nil {
			authkit.RespondRefusal(w, http.StatusBadRequest, authkit.Refusal{
				Message: "malformed json", Code: "body_malformed",
			})
			return
		}
		if err := content.ValidateLocale(req.Locale); err != nil {
			respondDomainError(w, err)
			return
		}
		identity := authkit.IdentityFromContext(r.Context())
		if err := s.readers.Save(r.Context(), identity.ID, content.LocaleSettingKey, req.Locale); err != nil {
			respondDomainError(w, err)
			return
		}
		authkit.Respond(w, http.StatusOK, localeResponse{
			Locale:    req.Locale,
			Supported: content.SupportedLocales,
		})
	}
}
