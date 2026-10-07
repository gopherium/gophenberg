// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// filterCases are the cases over narrowing a listing by the values its items hold.
var filterCases = []Case{
	{"ListNarrowsByEachFilterableKind", listNarrowsByEachFilterableKind},
	{"ListLeavesAnUnfilteredListingWhole", listLeavesAnUnfilteredListingWhole},
	{"ListCarriesTheValuesOfEveryRow", listCarriesTheValuesOfEveryRow},
	{"ListAnswersNothingWhenNoItemHoldsTheTerm", listAnswersNothingWhenNoItemHoldsTheTerm},
	{"ListNarrowedKeepsTheOrderAndThePage", listNarrowedKeepsTheOrderAndThePage},
	{"ListNarrowedHonoursTheStatusItIsGiven", listNarrowedHonoursTheStatusItIsGiven},
	{"ListNarrowedHonoursTheSearchItIsGiven", listNarrowedHonoursTheSearchItIsGiven},
}

// declareFilterable declares one field of every kind a filter reads on the post type.
func declareFilterable(t *testing.T, types content.TypeStore) {
	t.Helper()
	DeclareFields(t, types,
		PostField("note", content.FieldKindText),
		PostField("price", content.FieldKindNumber),
		PostField("on-sale", content.FieldKindBoolean),
		PostField("since", content.FieldKindDate),
		choiceField("colour", false),
		choiceField("tags", true),
	)
}

// choiceField returns a choice field offering red, blue and warm, holding several when asked.
func choiceField(key string, multiple bool) content.Field {
	field := PostField(key, content.FieldKindChoice)
	field.Settings = map[string]any{
		content.SettingChoices: []any{
			map[string]any{"value": "red", "label": "Red"},
			map[string]any{"value": "blue", "label": "Blue"},
			map[string]any{"value": "warm", "label": "Warm"},
		},
		content.SettingMultiple: multiple,
	}
	return field
}

// listedUnder returns the titles and the total the filter answers, narrowed by the terms.
func listedUnder(t *testing.T, store content.Store, terms map[string]any) ([]string, int) {
	t.Helper()
	rows, total, err := store.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 1, PerPage: 20, Fields: terms,
	})
	if err != nil {
		t.Fatalf("List(%v): %v", terms, err)
	}
	return TitlesOf(rows), total
}

// listNarrowsByEachFilterableKind keeps only the item holding the term, for every kind a filter reads.
func listNarrowsByEachFilterableKind(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	declareFilterable(t, s.Types)
	Holding(t, s.Content, "Match", author, content.Values{
		"note": "half price", "price": float64(10), "on-sale": true,
		"since": "2026-09-05", "colour": "red", "tags": []any{"red", "warm"},
	})
	Holding(t, s.Content, "Miss", author, content.Values{
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

			rows, total, err := s.Content.List(t.Context(), content.Filter{
				Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
				Page: 1, PerPage: 20, Fields: terms,
			})

			if err != nil {
				t.Fatalf("List(%v): %v", terms, err)
			}
			if titles := TitlesOf(rows); len(titles) != 1 || titles[0] != "Match" {
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

// listLeavesAnUnfilteredListingWhole lists every item when no term narrows it.
func listLeavesAnUnfilteredListingWhole(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	declareFilterable(t, s.Types)
	Holding(t, s.Content, "Match", author, content.Values{"price": float64(10)})
	Holding(t, s.Content, "Miss", author, content.Values{"price": float64(20)})

	titles, total := listedUnder(t, s.Content, nil)

	if len(titles) != 2 || total != 2 {
		t.Errorf("titles = %v with total %d, want both items", titles, total)
	}
}

// listCarriesTheValuesOfEveryRow carries the field values onto each listed row.
func listCarriesTheValuesOfEveryRow(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	declareFilterable(t, s.Types)
	Holding(t, s.Content, "Match", author, content.Values{"note": "half price"})

	rows, _, err := s.Content.List(t.Context(), content.Filter{
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

// listAnswersNothingWhenNoItemHoldsTheTerm answers an empty page when no item holds the term.
func listAnswersNothingWhenNoItemHoldsTheTerm(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	declareFilterable(t, s.Types)
	Holding(t, s.Content, "Match", author, content.Values{"price": float64(10)})

	titles, total := listedUnder(t, s.Content, map[string]any{"price": float64(99)})

	if len(titles) != 0 || total != 0 {
		t.Errorf("titles = %v with total %d, want an empty page", titles, total)
	}
}

// listNarrowedKeepsTheOrderAndThePage sorts and pages a narrowed listing under the total of its matches.
func listNarrowedKeepsTheOrderAndThePage(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	declareFilterable(t, s.Types)
	for _, title := range []string{"Charlie", "Alpha", "Bravo"} {
		Holding(t, s.Content, title, author, content.Values{"price": float64(10)})
	}
	Holding(t, s.Content, "Delta", author, content.Values{"price": float64(20)})

	first, total, err := s.Content.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 1, PerPage: 2, Fields: map[string]any{"price": float64(10)},
	})
	if err != nil {
		t.Fatalf("listing the first page: %v", err)
	}
	second, _, err := s.Content.List(t.Context(), content.Filter{
		Type: content.TypePost, OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 2, PerPage: 2, Fields: map[string]any{"price": float64(10)},
	})
	if err != nil {
		t.Fatalf("listing the second page: %v", err)
	}

	if got := TitlesOf(first); len(got) != 2 || got[0] != "Alpha" || got[1] != "Bravo" {
		t.Errorf("first page = %v, want Alpha then Bravo", got)
	}
	if got := TitlesOf(second); len(got) != 1 || got[0] != "Charlie" {
		t.Errorf("second page = %v, want Charlie alone", got)
	}
	if total != 3 {
		t.Errorf("total = %d, want the three matching items", total)
	}
}

// listNarrowedHonoursTheStatusItIsGiven keeps only the narrowed items in the status the filter names.
func listNarrowedHonoursTheStatusItIsGiven(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	declareFilterable(t, s.Types)
	Holding(t, s.Content, "Draft", author, content.Values{"price": float64(10)})
	live := Holding(t, s.Content, "Published", author, content.Values{"price": float64(10)})
	at := time.Now().UTC()
	live.Status = content.StatusPublished
	live.PublishedAt = &at
	if _, err := s.Content.Update(t.Context(), live, live.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("publishing: %v", err)
	}

	rows, total, err := s.Content.List(t.Context(), content.Filter{
		Type: content.TypePost, Statuses: []content.Status{content.StatusPublished},
		OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 1, PerPage: 20, Fields: map[string]any{"price": float64(10)},
	})

	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if got := TitlesOf(rows); len(got) != 1 || got[0] != "Published" {
		t.Errorf("titles = %v, want only the published item", got)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
}

// listNarrowedHonoursTheSearchItIsGiven keeps only the narrowed items the search matches.
func listNarrowedHonoursTheSearchItIsGiven(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	declareFilterable(t, s.Types)
	Holding(t, s.Content, "Winter sale", author, content.Values{"price": float64(10)})
	Holding(t, s.Content, "Summer sale", author, content.Values{"price": float64(10)})

	rows, total, err := s.Content.List(t.Context(), content.Filter{
		Type: content.TypePost, Search: "Winter", OrderBy: content.OrderByTitle, Order: content.OrderAsc,
		Page: 1, PerPage: 20, Fields: map[string]any{"price": float64(10)},
	})

	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	if got := TitlesOf(rows); len(got) != 1 || got[0] != "Winter sale" {
		t.Errorf("titles = %v, want only the searched item", got)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
}
