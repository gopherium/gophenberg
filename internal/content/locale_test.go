// SPDX-License-Identifier: AGPL-3.0-or-later

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestResolveLocaleFollowsTheBrowserWhenNothingIsSet(t *testing.T) {
	t.Parallel()

	held := content.ResolveLocale(content.LocaleAsked{Accepted: "es-ES,es;q=0.9"})

	if held != "es-ES" {
		t.Errorf("ResolveLocale() = %q, want the browser's language", held)
	}
}

func TestResolveLocaleFallsBackToTheFirstSupported(t *testing.T) {
	t.Parallel()

	held := content.ResolveLocale(content.LocaleAsked{Accepted: "fr-FR,fr;q=0.9"})

	if held != content.DefaultLocale {
		t.Errorf("ResolveLocale() = %q, want %q", held, content.DefaultLocale)
	}
}

func TestResolveLocalePrefersTheSiteDefault(t *testing.T) {
	t.Parallel()

	held := content.ResolveLocale(content.LocaleAsked{Site: "es-ES", Accepted: "en-US"})

	if held != "es-ES" {
		t.Errorf("ResolveLocale() = %q, want the site's own language", held)
	}
}

func TestResolveLocalePrefersTheReadersOwnChoice(t *testing.T) {
	t.Parallel()

	held := content.ResolveLocale(content.LocaleAsked{User: "es-ES", Site: "en-US", Accepted: "en-US"})

	if held != "es-ES" {
		t.Errorf("ResolveLocale() = %q, want the reader's own language", held)
	}
}

func TestResolveLocaleIgnoresAnUnsupportedSiteDefault(t *testing.T) {
	t.Parallel()

	held := content.ResolveLocale(content.LocaleAsked{Site: "fr-FR", Accepted: "es-ES"})

	if held != "es-ES" {
		t.Errorf("ResolveLocale() = %q, want an unsupported site default passed over", held)
	}
}

func TestResolveLocaleAnswersTheFallbackWhenNothingIsAsked(t *testing.T) {
	t.Parallel()

	if held := content.ResolveLocale(content.LocaleAsked{}); held != content.DefaultLocale {
		t.Errorf("ResolveLocale() = %q, want %q", held, content.DefaultLocale)
	}
}

func TestValidateLocaleAcceptsASupportedLanguage(t *testing.T) {
	t.Parallel()

	if err := content.ValidateLocale("es-ES"); err != nil {
		t.Errorf("ValidateLocale() error = %v, want nil", err)
	}
}

func TestValidateLocaleRefusesAnUnsupportedLanguage(t *testing.T) {
	t.Parallel()

	err := content.ValidateLocale("xx-XX")

	if !errors.Is(err, content.ErrLocaleUnknown) {
		t.Fatalf("ValidateLocale() error = %v, want %v", err, content.ErrLocaleUnknown)
	}
	if code, ok := content.CodeOf(err); !ok || code != "locale_unknown" {
		t.Errorf("CodeOf() = %q, %v, want the refusal named", code, ok)
	}
}
