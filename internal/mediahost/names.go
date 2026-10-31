// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gopherium/gophenberg/internal/content"
)

// maxStemBytes bounds the bytes of the stem an upload name gives, before any suffix.
const maxStemBytes = 200

// maxNameBytes bounds the bytes of an upload name read for its stem.
const maxNameBytes = 4 * maxStemBytes

// lookalikeSuffix matches a stem that could collide with sub-size naming.
var lookalikeSuffix = regexp.MustCompile(`-(\d+x\d+|scaled|rotated)$`)

// baseName returns name without any leading path, whichever separator wrote it.
func baseName(name string) string {
	return name[strings.LastIndexAny(name, `/\`)+1:]
}

// titleOf returns the upload's name without its path and extension.
func titleOf(name string) string {
	base := baseName(name)
	return strings.TrimSuffix(base, path.Ext(base))
}

// stemOf returns the sanitized file stem an upload stores under.
func stemOf(name string) string {
	var b strings.Builder
	separated := false
	for _, r := range content.FoldAccents(prefixOf(titleOf(name), maxNameBytes)) {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			separated = true
			continue
		}
		if separated && b.Len() > 0 {
			b.WriteRune('-')
		}
		separated = false
		b.WriteRune(r)
	}
	stem := strings.TrimSuffix(prefixOf(b.String(), maxStemBytes), "-")
	if stem == "" {
		stem = "file"
	}
	if lookalikeSuffix.MatchString(stem) {
		stem += "-file"
	}
	return stem
}

// prefixOf returns the longest prefix of s that holds whole runes within limit bytes.
func prefixOf(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// numberedStem returns stem for the first attempt, and stem with the attempt
// number appended after that.
func numberedStem(stem string, attempt int) string {
	if attempt == 1 {
		return stem
	}
	return stem + "-" + strconv.Itoa(attempt)
}
