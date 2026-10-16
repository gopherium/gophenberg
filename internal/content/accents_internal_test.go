// SPDX-License-Identifier: Apache-2.0

package content

import (
	"testing"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

func TestLetterFoldsHoldEveryLatinLetterThatDoesNotDecompose(t *testing.T) {
	t.Parallel()

	for r := rune(0x00C0); r <= 0x017F; r++ {
		if !unicode.IsLower(r) || norm.NFD.String(string(r)) != string(r) {
			continue
		}
		if _, ok := letterFolds[r]; !ok {
			t.Errorf("letterFolds has no plain spelling for %q (%U)", r, r)
		}
	}
}

func TestFoldAccentsReadsAnApostropheAsPartOfTheWord(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   string
		want string
	}{
		"apostrophe and mark after latin dropped":       {in: "cafe'\U00001DC0", want: "cafe'"},
		"curly apostrophe and mark after latin dropped": {in: "don\U00002019\U00000301t", want: "don\U00002019t"},
		"apostrophe and mark after devanagari kept":     {in: "\U00000939'\U0000093F", want: "\U00000939'\U0000093F"},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if got := FoldAccents(tc.in); got != tc.want {
				t.Errorf("FoldAccents(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
