// SPDX-License-Identifier: Apache-2.0

package content

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// letterFolds maps the lowercase Latin letters that do not decompose to their plain spelling.
var letterFolds = map[rune]string{
	'ß': "ss",
	'æ': "ae",
	'œ': "oe",
	'ø': "o",
	'đ': "d",
	'ð': "d",
	'þ': "th",
	'ł': "l",
	'ħ': "h",
	'ı': "i",
	'ŧ': "t",
	'ĳ': "ij",
	'ŀ': "l",
	'ĸ': "k",
	'ŉ': "n",
	'ŋ': "n",
	'ſ': "s",
	'ª': "a",
	'º': "o",
	'ɑ': "a",
	'ǝ': "e",
	'ə': "e",
}

// FoldAccents returns s with its accented Latin letters written as plain ones;
// callers lowercase first, as Slugify and the media host do.
func FoldAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	afterLatin := false
	for _, r := range norm.NFD.String(s) {
		fold, folds := letterFolds[r]
		switch {
		case folds:
			b.WriteString(fold)
			afterLatin = true
		case isApostrophe(r):
			b.WriteRune(r)
		case !unicode.Is(unicode.M, r):
			b.WriteRune(r)
			afterLatin = isLatinLetter(r)
		case !afterLatin || !unicode.Is(unicode.Mn, r):
			b.WriteRune(r)
		}
	}
	return norm.NFC.String(b.String())
}

// isApostrophe reports whether r is a straight or curly apostrophe.
func isApostrophe(r rune) bool {
	return r == '\'' || r == '’'
}

// isLatinLetter reports whether r is a letter of the Latin script.
func isLatinLetter(r rune) bool {
	if r < utf8.RuneSelf {
		return unicode.IsLetter(r)
	}
	return unicode.Is(unicode.Latin, r) && unicode.IsLetter(r)
}
