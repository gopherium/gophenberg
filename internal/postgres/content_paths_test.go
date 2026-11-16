// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// pageType and mustNest are the shared fixtures under the names these tests use.
var (
	pageType = contenttest.PageType
	mustNest = contenttest.MustNest
)

func TestContentDepthStopsAtAPlantedLoop(t *testing.T) {
	t.Parallel()

	items, _, author, pool := nestingStoresWithPool(t)
	about := mustNest(t, items, nil, "About", author)
	team := mustNest(t, items, &about, "Team", author)
	contact := mustNest(t, items, nil, "Contact", author)
	pages := map[string]uuid.UUID{
		"About": about.ID, "Team": team.ID, "Deep": mustNest(t, items, &team, "Deep", author).ID,
		"Contact": contact.ID, "Directions": mustNest(t, items, &contact, "Directions", author).ID,
	}
	if _, err := pool.Exec(t.Context(), `UPDATE core.content SET parent_id = $1 WHERE id = $2`,
		team.ID, about.ID); err != nil {
		t.Fatalf("planting the loop: %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	for title, want := range map[string]int{"About": 2, "Team": 1, "Deep": 0, "Contact": 1, "Directions": 0} {
		if got, err := items.Depth(ctx, pages[title]); err != nil || got != want {
			t.Errorf("Depth(%s) over a planted loop = %d, %v, want %d counted up to the loop", title, got, err, want)
		}
	}
}
