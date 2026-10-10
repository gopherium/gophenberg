// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// declareFlexible, declareLayout and declareUnder are the shared fixtures these tests call.
var (
	declareFlexible = contenttest.DeclareFlexible
	declareLayout   = contenttest.DeclareLayout
	declareUnder    = contenttest.DeclareUnder
)

func TestDeletingALayoutReportsASweepItCannotRun(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	features := declareFlexible(t, store)
	hero := declareLayout(t, store, features.ID, "hero")
	declareUnder(t, store, hero.ID, "title")
	plantTyped(t, pool, author, "car", "one", `{"features": [{"hero": {"title": "A"}}]}`)
	sabotage(t, pool, "DROP FUNCTION core.strip_layout(jsonb, text [])")

	err := store.DeleteSubField(t.Context(), hero.ID, nil)

	if err == nil {
		t.Error("DeleteSubField() error = nil, want the sweep it cannot run reported")
	}
}
