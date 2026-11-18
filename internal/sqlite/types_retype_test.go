// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"maps"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/sqlite"
)

// carriedAt is the stamp the first planted carry writes on every address it moves.
var carriedAt = time.Date(2026, time.March, 5, 9, 0, 0, 0, time.UTC)

// wantAddress fails t unless the item stored under the id answers at the address with the stamp.
func (s plantedSite) wantAddress(t *testing.T, id uuid.UUID, address string, stamp time.Time) {
	t.Helper()
	var path string
	var at dbkit.Time
	if err := s.handle.QueryRowContext(t.Context(), "SELECT path, updated_at FROM core_content WHERE id = ?", id).
		Scan(&path, &at); err != nil {
		t.Fatalf("reading the item %s: %v", id, err)
	}
	if path != address || !at.Equal(stamp) {
		t.Errorf("item %s = %s at %v, want %s at %v", id, path, at.Time, address, stamp)
	}
}

func TestARetypePrefixesEveryPathBeforeItWritesTheNewOnes(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	plantType(t, site.handle, contenttest.PageType())
	team := site.plantItem(t, "page", "team", nil, "{}")
	pages := site.plantItem(t, "page", "pages", nil, "{}")
	nested := site.plantItem(t, "page", "pages/team", &pages, "{}")
	statements := sqlite.TypeStatements(site.handle)

	for _, carry := range []struct {
		name, word, was string
		at              time.Time
		want            map[uuid.UUID]string
	}{
		{"off the root onto pages", "pages", "", carriedAt,
			map[uuid.UUID]string{team: "pages/team", pages: "pages/pages", nested: "pages/pages/team"}},
		{"back onto the root", "", "pages", carriedAt.Add(time.Hour),
			map[uuid.UUID]string{team: "team", pages: "pages", nested: "pages/team"}},
	} {
		paths, err := statements.RetypeContentPaths(t.Context(),
			typestore.TypeRow{Key: "page", RouteWord: carry.word, UpdatedAt: carry.at}, carry.was)

		if err != nil {
			t.Fatalf("RetypeContentPaths() %s error = %v, want nil", carry.name, err)
		}
		if slices.Sort(paths); !slices.Equal(paths, slices.Sorted(maps.Values(carry.want))) {
			t.Errorf("RetypeContentPaths() %s = %v, want every new address", carry.name, paths)
		}
		for id, address := range carry.want {
			site.wantAddress(t, id, address, carry.at)
		}
	}
}
