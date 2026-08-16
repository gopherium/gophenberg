// SPDX-License-Identifier: AGPL-3.0-or-later

package content

import (
	"errors"
	"fmt"
	"slices"

	"golang.org/x/text/language"
)

// ErrLocaleUnknown reports that a language is not one the site answers in.
var ErrLocaleUnknown = errors.New("content: locale unknown")

// DefaultLocale is the language the site answers in when nothing else applies.
const DefaultLocale = "en-US"

// SupportedLocales are the languages the site answers in, the fallback first.
var SupportedLocales = []string{DefaultLocale, "es-ES"}

// LocaleSettingKey names the stored site default language.
const LocaleSettingKey = "locale.default"

// LocaleAsked carries what each leg of the resolution chain offered.
type LocaleAsked struct {
	User     string
	Site     string
	Accepted string
}

// matcher holds the supported languages, built once because parsing tags is not free.
var matcher = language.NewMatcher(parseSupported())

// parseSupported returns the supported languages as tags, the fallback first.
func parseSupported() []language.Tag {
	tags := make([]language.Tag, len(SupportedLocales))
	for i, held := range SupportedLocales {
		tags[i] = language.MustParse(held)
	}
	return tags
}

// supported reports whether the site answers in the language.
func supported(locale string) bool {
	return slices.Contains(SupportedLocales, locale)
}

// ValidateLocale reports whether the site answers in the language.
func ValidateLocale(locale string) error {
	if supported(locale) {
		return nil
	}
	return Refuse(ErrLocaleUnknown, "locale_unknown",
		fmt.Sprintf("%s: %s", ErrLocaleUnknown, locale), Details{"value": locale, "allowed": SupportedLocales})
}

// ResolveLocale returns the language to answer in, preferring the reader, then
// the site, then the languages the request accepts.
func ResolveLocale(asked LocaleAsked) string {
	if supported(asked.User) {
		return asked.User
	}
	if supported(asked.Site) {
		return asked.Site
	}
	return accepted(asked.Accepted)
}

// accepted returns the supported language closest to the ones the request named.
func accepted(header string) string {
	if header == "" {
		return DefaultLocale
	}
	wanted, _, err := language.ParseAcceptLanguage(header)
	if err != nil {
		return DefaultLocale
	}
	_, index, _ := matcher.Match(wanted...)
	return SupportedLocales[index]
}
