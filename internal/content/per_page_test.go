// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"math"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestPerPageNamesItsKeyAndBounds(t *testing.T) {
	t.Parallel()

	if content.PerPageSettingKey != "content.per_page" {
		t.Errorf("PerPageSettingKey = %q, want content.per_page", content.PerPageSettingKey)
	}
	if content.DefaultPerPage != 20 {
		t.Errorf("DefaultPerPage = %d, want 20", content.DefaultPerPage)
	}
	if content.MaxPerPage != 100 {
		t.Errorf("MaxPerPage = %d, want 100", content.MaxPerPage)
	}
}

func TestParsePerPageTakesASizeInsideTheBounds(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]int{"1": 1, "5": 5, "20": 20, "100": 100} {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			size, err := content.ParsePerPage(raw)

			if err != nil {
				t.Fatalf("ParsePerPage(%q) error = %v, want nil", raw, err)
			}
			if size != want {
				t.Errorf("ParsePerPage(%q) = %d, want %d", raw, size, want)
			}
		})
	}
}

func TestParsePerPageRefusesWhatCannotStand(t *testing.T) {
	t.Parallel()

	for name, raw := range map[string]string{
		"nothing at all":      "",
		"a word":              "many",
		"zero":                "0",
		"a size below zero":   "-3",
		"one past the most":   "101",
		"a size with a space": " 5",
		"a fraction":          "2.5",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := content.ParsePerPage(raw)

			if !errors.Is(err, content.ErrPerPageInvalid) {
				t.Fatalf("ParsePerPage(%q) error = %v, want %v", raw, err, content.ErrPerPageInvalid)
			}
			if code, ok := content.CodeOf(err); !ok || code != "per_page_invalid" {
				t.Errorf("CodeOf() = %q, %v, want the error named", code, ok)
			}
			held, ok := content.DetailsOf(err)
			if !ok || held["value"] != raw || held["max"] != content.MaxPerPage {
				t.Errorf("DetailsOf() = %v, %v, want the value and the most allowed", held, ok)
			}
		})
	}
}

func TestResolvePerPageFallsBackToTheDefault(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		held  string
		found bool
		want  int
	}{
		"a stored size":            {"5", true, 5},
		"the most allowed":         {"100", true, 100},
		"nothing stored":           {"", false, content.DefaultPerPage},
		"an empty row":             {"", true, content.DefaultPerPage},
		"a row holding a word":     {"many", true, content.DefaultPerPage},
		"a row holding zero":       {"0", true, content.DefaultPerPage},
		"a row past the most":      {"101", true, content.DefaultPerPage},
		"a stored size nobody has": {"5", false, content.DefaultPerPage},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if size := content.ResolvePerPage(asked.held, asked.found); size != asked.want {
				t.Errorf("ResolvePerPage(%q, %v) = %d, want %d", asked.held, asked.found, size, asked.want)
			}
		})
	}
}

func TestPageOffsetSkipsThePagesBeforeItUpToTheLargestOffsetAQueryTakes(t *testing.T) {
	t.Parallel()

	lastWhole := math.MaxInt32 / 20
	for name, asked := range map[string]struct {
		page, perPage, want int
	}{
		"the first page":                {1, 20, 0},
		"a middle page":                 {3, 20, 40},
		"the last page under the bound": {lastWhole + 1, 20, lastWhole * 20},
		"the page one past the bound":   {lastWhole + 2, 20, math.MaxInt32},
		"a page far past the bound":     {math.MaxInt32, 20, math.MaxInt32},
		"a page size of zero":           {5, 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := content.PageOffset(asked.page, asked.perPage); got != asked.want {
				t.Errorf("PageOffset(%d, %d) = %d, want %d", asked.page, asked.perPage, got, asked.want)
			}
		})
	}
}
