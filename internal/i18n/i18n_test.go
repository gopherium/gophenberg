// SPDX-License-Identifier: Apache-2.0

package i18n_test

import (
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/i18n"
)

// published is the day the date tests read.
var published = time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)

func TestGetAnswersTheSourceWordWithoutACatalog(t *testing.T) {
	t.Parallel()

	if held := i18n.For("en-US").Get("Older posts"); held != "Older posts" {
		t.Errorf("got %q, want the source word", held)
	}
}

func TestGetAnswersTheSourceWordForAnUnknownLanguage(t *testing.T) {
	t.Parallel()

	if held := i18n.For("xx-XX").Get("Older posts"); held != "Older posts" {
		t.Errorf("got %q, want the source word", held)
	}
}

func TestGetAnswersTheTranslationTheCatalogCarries(t *testing.T) {
	t.Parallel()

	if held := i18n.For("es-ES").Get("Older posts"); held != "Entradas anteriores" {
		t.Errorf("got %q, want the translation", held)
	}
}

func TestGetAnswersTheSourceWordForAMessageTheCatalogOmits(t *testing.T) {
	t.Parallel()

	if held := i18n.For("es-ES").Get("A message nothing translates"); held != "A message nothing translates" {
		t.Errorf("got %q, want the source word", held)
	}
}

func TestDateReadsInEnglishWithoutACatalog(t *testing.T) {
	t.Parallel()

	if held := i18n.For("en-US").Date(published); held != "16 August 2026" {
		t.Errorf("got %q, want the English date", held)
	}
}

func TestDateFollowsTheCatalogsPatternAndMonth(t *testing.T) {
	t.Parallel()

	if held := i18n.For("es-ES").Date(published); held != "16 de agosto de 2026" {
		t.Errorf("got %q, want the translated date", held)
	}
}

func TestDatePatternNamesWhatEachPartHolds(t *testing.T) {
	t.Parallel()

	if held := i18n.DatePattern; held != "{day} {month} {year}" {
		t.Errorf("DatePattern = %q, want tokens a translator can read", held)
	}
}
