// SPDX-License-Identifier: Apache-2.0

package mediahost

import "testing"

func TestStemOfFoldsAccentsLikeSlugs(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   string
		want string
	}{
		"precomposed accents fold":         {in: "Café Ærø.jpg", want: "cafe-aero"},
		"stroked letters fold":             {in: "Łódź 2026.png", want: "lodz-2026"},
		"decomposed accent joins its word": {in: "Mu\u0308nchen.jpg", want: "munchen"},
		"letters with no plain form stay":  {in: "写真.jpg", want: "写真"},
		"a nameless stem still falls back": {in: ".png", want: "file"},
		"the lookalike guard still applies": {
			in:   "photo-150x150.jpg",
			want: "photo-150x150-file",
		},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if got := stemOf(tc.in); got != tc.want {
				t.Errorf("stemOf(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
