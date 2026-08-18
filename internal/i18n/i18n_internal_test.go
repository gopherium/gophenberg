// SPDX-License-Identifier: Apache-2.0

package i18n

import "testing"

func TestParseAnswersAnEmptyTranslatorForBytesThatAreNotACatalog(t *testing.T) {
	t.Parallel()

	if held := parse([]byte("not a catalog")).Get("Older posts"); held != "Older posts" {
		t.Errorf("got %q, want the source word", held)
	}
}
