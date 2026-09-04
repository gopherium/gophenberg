// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestValidGroupKeyStandsForTheKeysAGroupMayCarry(t *testing.T) {
	t.Parallel()

	for name, key := range map[string]string{
		"a plain word":       "details",
		"a hyphenated word":  "article-details",
		"a word with digits": "plans2026",
		"no key at all":      "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := content.ValidGroupKey(key); err != nil {
				t.Errorf("ValidGroupKey(%q) error = %v, want nil", key, err)
			}
		})
	}
}

func TestValidGroupKeyRefusesAKeyNoGroupMayCarry(t *testing.T) {
	t.Parallel()

	for name, key := range map[string]string{
		"a capital letter":  "Details",
		"a space":           "article details",
		"an underscore":     "article_details",
		"a leading digit":   "2026-plans",
		"a leading hyphen":  "-details",
		"an accented vowel": "detalles-año",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := content.ValidGroupKey(key); !errors.Is(err, content.ErrInvalidGroupKey) {
				t.Errorf("ValidGroupKey(%q) error = %v, want %v", key, err, content.ErrInvalidGroupKey)
			}
		})
	}
}
