// SPDX-License-Identifier: Apache-2.0

package themehost

import (
	"sort"
	"unicode"
)

// ordered puts the active theme first and sorts the rest by name with case
// ignored and digit runs compared by value. WordPress builds its themes screen
// with wp_prepare_themes_for_js, which keeps the active theme on top and sorts
// the rest with WP_Theme::sort_by_name; raw byte order moved a capitalised or
// accented name out of place and read theme10 before theme2 (#509).
func ordered(themes []Installed) []Installed {
	sort.SliceStable(themes, func(i, j int) bool {
		if themes[i].Active != themes[j].Active {
			return themes[i].Active
		}
		return naturalLess(themes[i].Name, themes[j].Name)
	})
	return themes
}

// naturalLess compares two names the way PHP's strnatcasecmp does: letters
// case-insensitively, and each run of digits by its numeric value so theme2
// comes before theme10.
func naturalLess(a, b string) bool { return naturalCompare(a, b) < 0 }

func naturalCompare(a, b string) int {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		if isDigit(ar[i]) && isDigit(br[j]) {
			ai, bj := i, j
			for ai < len(ar) && isDigit(ar[ai]) {
				ai++
			}
			for bj < len(br) && isDigit(br[bj]) {
				bj++
			}
			if c := compareDigits(string(ar[i:ai]), string(br[j:bj])); c != 0 {
				return c
			}
			i, j = ai, bj
			continue
		}
		al, bl := unicode.ToLower(ar[i]), unicode.ToLower(br[j])
		if al != bl {
			if al < bl {
				return -1
			}
			return 1
		}
		i++
		j++
	}
	switch {
	case i < len(ar):
		return 1
	case j < len(br):
		return -1
	}
	return 0
}

func compareDigits(a, b string) int {
	at, bt := trimLeadingZeros(a), trimLeadingZeros(b)
	if len(at) != len(bt) {
		if len(at) < len(bt) {
			return -1
		}
		return 1
	}
	if at != bt {
		if at < bt {
			return -1
		}
		return 1
	}
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return 0
}

func trimLeadingZeros(s string) string {
	for len(s) > 1 && s[0] == '0' {
		s = s[1:]
	}
	return s
}

func isDigit(r rune) bool { return r >= '0' && r <= '9' }
