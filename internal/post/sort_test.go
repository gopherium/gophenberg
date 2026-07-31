// SPDX-License-Identifier: Apache-2.0

package post_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/post"
)

func TestParseOrderByAcceptsEverySortableColumn(t *testing.T) {
	t.Parallel()

	for _, want := range []post.OrderBy{post.OrderByDate, post.OrderByTitle} {
		got, err := post.ParseOrderBy(string(want))

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
		if _, err := post.ParseOrderBy(in); !errors.Is(err, post.ErrInvalidOrderBy) {
			t.Errorf("ParseOrderBy(%q) error = %v, want %v", in, err, post.ErrInvalidOrderBy)
		}
	}
}

func TestParseOrderAcceptsEveryDirection(t *testing.T) {
	t.Parallel()

	for _, want := range []post.Order{post.OrderAsc, post.OrderDesc} {
		got, err := post.ParseOrder(string(want))

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
		if _, err := post.ParseOrder(in); !errors.Is(err, post.ErrInvalidOrder) {
			t.Errorf("ParseOrder(%q) error = %v, want %v", in, err, post.ErrInvalidOrder)
		}
	}
}
