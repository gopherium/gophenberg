// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// declareFilterable declares one field of every kind a filter reads on the post type.
func declareFilterable(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	declareField(t, pool, "note", content.FieldKindText)
	declareField(t, pool, "price", content.FieldKindNumber)
	declareField(t, pool, "on-sale", content.FieldKindBoolean)
	declareField(t, pool, "since", content.FieldKindDate)
	declareChoice(t, pool, "colour", false)
	declareChoice(t, pool, "tags", true)
}

// declareChoice declares a choice field offering red, blue and warm, holding several when asked.
func declareChoice(t *testing.T, pool *pgxpool.Pool, key string, multiple bool) {
	t.Helper()
	declared := fieldOn(t, content.TypePost, key, content.FieldKindChoice, "")
	declared.Settings = map[string]any{
		content.SettingChoices: []any{
			map[string]any{"value": "red", "label": "Red"},
			map[string]any{"value": "blue", "label": "Blue"},
			map[string]any{"value": "warm", "label": "Warm"},
		},
		content.SettingMultiple: multiple,
	}
	if _, err := postgres.NewTypeStore(pool).CreateField(t.Context(), declared); err != nil {
		t.Fatalf("declaring the %q field: %v, want nil", key, err)
	}
}

// holding stores a draft post carrying the given field values.
func holding(t *testing.T, store *postgres.ContentStore, title string, author uuid.UUID,
	values content.Values) content.Content {
	t.Helper()
	created := mustCreate(t, store, title, author)
	created.Fields = values
	updated, err := store.Update(t.Context(), created, created.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("storing the values of %q: %v", title, err)
	}
	return updated
}

// listedUnder returns the titles and the total the filter answers, narrowed by the terms.
func listedUnder(t *testing.T, store *postgres.ContentStore, terms map[string]any) ([]string, int) {
	t.Helper()
	rows, total, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 1, PerPage: 20, Fields: terms,
	})
	if err != nil {
		t.Fatalf("List(%v): %v", terms, err)
	}
	return titlesOf(rows), total
}

func TestContentStoreListNarrowsByEachFilterableKind(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareFilterable(t, pool)
	holding(t, store, "Match", author, content.Values{
		"note": "half price", "price": float64(10), "on-sale": true,
		"since": "2026-09-05", "colour": "red", "tags": []any{"red", "warm"},
	})
	holding(t, store, "Miss", author, content.Values{
		"note": "full price", "price": float64(20), "on-sale": false,
		"since": "2026-01-01", "colour": "blue", "tags": []any{"blue"},
	})

	cases := map[string]map[string]any{
		"a number":        {"price": float64(10)},
		"a boolean":       {"on-sale": true},
		"a date":          {"since": "2026-09-05"},
		"a text":          {"note": "half price"},
		"a single choice": {"colour": "red"},
		"a choice member": {"tags": []any{"red"}},
		"two terms anded": {"price": float64(10), "on-sale": true},
	}
	for name, terms := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			rows, total, err := store.List(t.Context(), content.Filter{
				Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
				Page: 1, PerPage: 20, Fields: terms,
			})

			if err != nil {
				t.Fatalf("List(%v): %v", terms, err)
			}
			if titles := titlesOf(rows); len(titles) != 1 || titles[0] != "Match" {
				t.Errorf("titles = %v, want only the matching item", titles)
			}
			if total != 1 {
				t.Errorf("total = %d, want 1", total)
			}
			if len(rows) == 1 && rows[0].Fields["note"] != "half price" {
				t.Errorf("fields = %v, want the values carried onto the narrowed row", rows[0].Fields)
			}
		})
	}
}

func TestContentStoreListLeavesAnUnfilteredListingWhole(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareFilterable(t, pool)
	holding(t, store, "Match", author, content.Values{"price": float64(10)})
	holding(t, store, "Miss", author, content.Values{"price": float64(20)})

	titles, total := listedUnder(t, store, nil)

	if len(titles) != 2 || total != 2 {
		t.Errorf("titles = %v with total %d, want both items", titles, total)
	}
}

func TestContentStoreListCarriesTheValuesOfEveryRow(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareFilterable(t, pool)
	holding(t, store, "Match", author, content.Values{"note": "half price"})

	rows, _, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc, Page: 1, PerPage: 20,
	})

	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want the stored item", len(rows))
	}
	if rows[0].Fields["note"] != "half price" {
		t.Errorf("fields = %v, want the values carried onto the listed row", rows[0].Fields)
	}
}

func TestContentStoreListAnswersNothingWhenNoItemHoldsTheTerm(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareFilterable(t, pool)
	holding(t, store, "Match", author, content.Values{"price": float64(10)})

	titles, total := listedUnder(t, store, map[string]any{"price": float64(99)})

	if len(titles) != 0 || total != 0 {
		t.Errorf("titles = %v with total %d, want an empty page", titles, total)
	}
}

func TestMigrationsIndexTheFieldValuesForContainment(t *testing.T) {
	t.Parallel()

	held := newTestDB(t)

	var definition string
	err := held.QueryRow(
		`SELECT indexdef FROM pg_indexes
		WHERE schemaname = 'core' AND tablename = 'content' AND indexname = 'content_field_values_idx'`,
	).Scan(&definition)

	if err != nil {
		t.Fatalf("querying pg_indexes: %v", err)
	}
	for _, want := range []string{"USING gin", "jsonb_path_ops"} {
		if !strings.Contains(definition, want) {
			t.Errorf("index = %q, want it to carry %q", definition, want)
		}
	}
}

func TestContentStoreListNarrowedKeepsTheOrderAndThePage(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareFilterable(t, pool)
	for _, title := range []string{"Charlie", "Alpha", "Bravo"} {
		holding(t, store, title, author, content.Values{"price": float64(10)})
	}
	holding(t, store, "Delta", author, content.Values{"price": float64(20)})

	first, total, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 1, PerPage: 2, Fields: map[string]any{"price": float64(10)},
	})
	if err != nil {
		t.Fatalf("listing the first page: %v", err)
	}
	second, _, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 2, PerPage: 2, Fields: map[string]any{"price": float64(10)},
	})
	if err != nil {
		t.Fatalf("listing the second page: %v", err)
	}

	if got := titlesOf(first); len(got) != 2 || got[0] != "Alpha" || got[1] != "Bravo" {
		t.Errorf("first page = %v, want Alpha then Bravo", got)
	}
	if got := titlesOf(second); len(got) != 1 || got[0] != "Charlie" {
		t.Errorf("second page = %v, want Charlie alone", got)
	}
	if total != 3 {
		t.Errorf("total = %d, want the three matching items", total)
	}
}

func TestContentStoreListNarrowedHonoursTheStatusItIsGiven(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareFilterable(t, pool)
	holding(t, store, "Draft", author, content.Values{"price": float64(10)})
	live := holding(t, store, "Published", author, content.Values{"price": float64(10)})
	at := time.Now().UTC()
	live.Status = content.StatusPublished
	live.PublishedAt = &at
	if _, err := store.Update(t.Context(), live, live.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("publishing: %v", err)
	}

	rows, total, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, Status: content.StatusPublished, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 1, PerPage: 20, Fields: map[string]any{"price": float64(10)},
	})

	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if got := titlesOf(rows); len(got) != 1 || got[0] != "Published" {
		t.Errorf("titles = %v, want only the published item", got)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
}

func TestContentStoreListNarrowedHonoursTheSearchItIsGiven(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareFilterable(t, pool)
	holding(t, store, "Winter sale", author, content.Values{"price": float64(10)})
	holding(t, store, "Summer sale", author, content.Values{"price": float64(10)})

	rows, total, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, Search: "Winter", OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 1, PerPage: 20, Fields: map[string]any{"price": float64(10)},
	})

	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if got := titlesOf(rows); len(got) != 1 || got[0] != "Winter sale" {
		t.Errorf("titles = %v, want only the searched item", got)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
}

func TestContentStoreReportsANarrowedListingItCannotRead(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareFilterable(t, pool)
	holding(t, store, "Match", author, content.Values{"price": float64(10)})
	sabotage(t, pool, "ALTER TABLE core.content DROP COLUMN excerpt CASCADE")

	narrowed := content.Filter{
		Type: content.TypePost, Page: 1, PerPage: 20, Fields: map[string]any{"price": float64(10)},
	}
	if _, _, err := store.List(t.Context(), narrowed); err == nil {
		t.Error("List() narrowed by fields error = nil, want the unreadable listing reported")
	}
}
