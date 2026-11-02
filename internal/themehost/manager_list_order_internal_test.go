// SPDX-License-Identifier: Apache-2.0

package themehost

import "testing"

// wantOrdered fails the test unless each name compares below every name after it, asked both ways round.
func wantOrdered(t *testing.T, ordered []string) {
	t.Helper()

	for at, lower := range ordered {
		for _, higher := range ordered[at+1:] {
			if got := compareNaturalNames(lower, higher); got >= 0 {
				t.Errorf("compareNaturalNames(%q, %q) = %d, want it negative", lower, higher, got)
			}
			if got := compareNaturalNames(higher, lower); got <= 0 {
				t.Errorf("compareNaturalNames(%q, %q) = %d, want it positive", higher, lower, got)
			}
		}
	}
}

func TestCompareNaturalNamesOrdersNamesAsStrnatcasecmpDoes(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"a prefix sorts before the longer name":          {"the", "theme", "theme2"},
		"an underscore sorts after the letters":          {"my-theme", "mytheme", "my_theme"},
		"spaces are skipped":                             {"theme 2", "theme3", "theme10", "theme 20"},
		"every ASCII white space is skipped":             {"a1", "a\t2", "a\n3", "a\v4", "a\f5", "a\r6"},
		"a run opening with a zero reads digit by digit": {"theme010", "theme05", "theme1", "theme9", "theme10"},
		"zeros opening a name are skipped":               {"9", "010"},
		"zeros opening both names are skipped":           {"0010", "011"},
		"a zero before a letter stays":                   {"0b", "1a"},
		"a run ends past the digit nine":                 {"a9", "a10", "a:"},
		"a run starts at the digit zero":                 {"a//", "a1"},
		"names go on after a run":                        {"a1b2", "a1b10"},
		"a digit sorts before a letter":                  {"1a", "a"},
		"a space right after a run is compared":          {"1 z", "1a"},
		"a name can end in a space":                      {"a ", "a b"},
		"a name sorts before itself with a space after":  {"a", "a "},
		"a lone zero is not skipped":                     {"0", "1"},
		"a run can be longer than any integer":           {"theme9", "theme10000000000000000000000"},
		"letters past ASCII keep their lower case order": {"ÿ", "ā"},
		"accented capitals sit with their lower case":    {"Ea", "zoe", "áa", "Ábc", "Éa"},
	}
	for name, ordered := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			wantOrdered(t, ordered)
		})
	}
}
