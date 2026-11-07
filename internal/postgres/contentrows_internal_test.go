// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"math"
	"testing"

	"github.com/gopherium/gophenberg/internal/content/contentstore"
)

func TestRowLimitBindsAPageSizeUpToTheLargestLimitAQueryTakes(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		size int
		want int32
	}{
		"a page of twenty":                   {20, 20},
		"the largest limit a query takes":    {math.MaxInt32, math.MaxInt32},
		"the largest page size an int holds": {math.MaxInt, math.MaxInt32},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := rowLimit(asked.size); got != asked.want {
				t.Errorf("rowLimit(%d) = %d, want %d", asked.size, got, asked.want)
			}
		})
	}
}

func TestAListAskingForMoreRowsThanAQueryTakesBindsTheLargestLimit(t *testing.T) {
	t.Parallel()

	page := contentstore.Page{Limit: math.MaxInt}

	if got := listParams(page).RowLimit; got != math.MaxInt32 {
		t.Errorf("listParams(Limit %d).RowLimit = %d, want %d", page.Limit, got, math.MaxInt32)
	}
	if got := nestedParams(page).RowLimit; got != math.MaxInt32 {
		t.Errorf("nestedParams(Limit %d).RowLimit = %d, want %d", page.Limit, got, math.MaxInt32)
	}
}
