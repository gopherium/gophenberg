// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestParseOrderByAcceptsEverySortableColumn(t *testing.T) {
	t.Parallel()

	for _, want := range []content.OrderBy{content.OrderByDate, content.OrderByTitle} {
		got, err := content.ParseOrderBy(string(want))

		if err != nil {
			t.Errorf("ParseOrderBy(%q) error = %v, want nil", want, err)
		}
		if got != want {
			t.Errorf("ParseOrderBy(%q) = %q, want %q", want, got, want)
		}
	}
}

func TestParseOrderByRejectsUnknownColumns(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "author", "Title", "status", "id"} {
		if _, err := content.ParseOrderBy(in); !errors.Is(err, content.ErrInvalidOrderBy) {
			t.Errorf("ParseOrderBy(%q) error = %v, want %v", in, err, content.ErrInvalidOrderBy)
		}
	}
}

func TestParseOrderAcceptsEveryDirection(t *testing.T) {
	t.Parallel()

	for _, want := range []content.Order{content.OrderAsc, content.OrderDesc} {
		got, err := content.ParseOrder(string(want))

		if err != nil {
			t.Errorf("ParseOrder(%q) error = %v, want nil", want, err)
		}
		if got != want {
			t.Errorf("ParseOrder(%q) = %q, want %q", want, got, want)
		}
	}
}

func TestParseOrderRejectsUnknownDirections(t *testing.T) {
	t.Parallel()

	for _, in := range []string{"", "ASC", "ascending", "sideways", "1"} {
		if _, err := content.ParseOrder(in); !errors.Is(err, content.ErrInvalidOrder) {
			t.Errorf("ParseOrder(%q) error = %v, want %v", in, err, content.ErrInvalidOrder)
		}
	}
}
