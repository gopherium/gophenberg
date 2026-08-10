// SPDX-License-Identifier: Apache-2.0

package mediahost

import (
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// lookalikeSuffix matches a stem that could collide with rendition naming.
var lookalikeSuffix = regexp.MustCompile(`-(\d+x\d+|scaled)$`)

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
	for _, r := range strings.ToLower(titleOf(name)) {
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
	stem := b.String()
	if stem == "" {
		stem = "file"
	}
	if lookalikeSuffix.MatchString(stem) {
		stem += "-file"
	}
	return stem
}

// numberedStem returns stem for the first attempt, and stem with the attempt
// number appended after that.
func numberedStem(stem string, attempt int) string {
	if attempt == 1 {
		return stem
	}
	return stem + "-" + strconv.Itoa(attempt)
}
