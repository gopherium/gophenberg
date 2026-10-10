// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"reflect"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// layoutSweepCases are the cases over the rows and fields a deleted layout or layout sub field takes away.
var layoutSweepCases = []Case{
	{"DeletingASubFieldSweepsItOnlyFromItsOwnLayout", deletingASubFieldSweepsItOnlyFromItsOwnLayout},
	{"DeletingALayoutTakesItsRowsAway", deletingALayoutTakesItsRowsAway},
	{"DeletingALayoutTakesTheFieldsItHeld", deletingALayoutTakesTheFieldsItHeld},
	{"DeletingALayoutInsideARepeaterTakesItsRowsAway", deletingALayoutInsideARepeaterTakesItsRowsAway},
}

// deletingASubFieldSweepsItOnlyFromItsOwnLayout sweeps a deleted layout sub field from that layout's rows alone.
func deletingASubFieldSweepsItOnlyFromItsOwnLayout(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	features := DeclareFlexible(t, s.Types)
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	quote := DeclareLayout(t, s.Types, features.ID, "quote")
	DeclareUnder(t, s.Types, hero.ID, "title")
	dropped := DeclareUnder(t, s.Types, hero.ID, "caption")
	DeclareUnder(t, s.Types, quote.ID, "caption")
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"features": []any{
		map[string]any{"hero": map[string]any{"title": "A", "caption": "drop"}},
		map[string]any{"quote": map[string]any{"caption": "keep"}},
		map[string]any{"hero": map[string]any{"title": "C", "caption": "drop too"}},
	}})

	if err := s.Types.DeleteSubField(t.Context(), dropped.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := content.Values{"features": []any{
		map[string]any{"hero": map[string]any{"title": "A"}},
		map[string]any{"quote": map[string]any{"caption": "keep"}},
		map[string]any{"hero": map[string]any{"title": "C"}},
	}}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the caption swept from the hero rows alone", held)
	}
	if held := RevisionValuesOf(t, s.Content, revision); !reflect.DeepEqual(held, want) {
		t.Errorf("revision values = %v, want the same sweep in the revision", held)
	}
}

// deletingALayoutTakesItsRowsAway takes a deleted layout's rows out of the flexible rather than emptying them.
func deletingALayoutTakesItsRowsAway(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	features := DeclareFlexible(t, s.Types)
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	quote := DeclareLayout(t, s.Types, features.ID, "quote")
	DeclareUnder(t, s.Types, hero.ID, "title")
	DeclareUnder(t, s.Types, quote.ID, "title")
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"features": []any{
		map[string]any{"hero": map[string]any{"title": "A"}},
		map[string]any{"quote": map[string]any{"title": "B"}},
		map[string]any{"hero": map[string]any{"title": "C"}},
	}})

	if err := s.Types.DeleteSubField(t.Context(), hero.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := content.Values{"features": []any{map[string]any{"quote": map[string]any{"title": "B"}}}}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the hero rows gone rather than emptied", held)
	}
	if held := RevisionValuesOf(t, s.Content, revision); !reflect.DeepEqual(held, want) {
		t.Errorf("revision values = %v, want the hero rows gone there too", held)
	}
}

// deletingALayoutTakesTheFieldsItHeld leaves the flexible holding no layout once its one layout is deleted.
func deletingALayoutTakesTheFieldsItHeld(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	features := DeclareFlexible(t, s.Types)
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	DeclareUnder(t, s.Types, hero.ID, "title")

	if err := s.Types.DeleteSubField(t.Context(), hero.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	held, err := s.Types.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	listed := slices.IndexFunc(held, func(stored content.Type) bool { return stored.Key == "car" })
	if listed < 0 {
		t.Fatalf("List() = %+v, want the car type listed", held)
	}
	if fields := held[listed].Fields; len(fields) != 1 || len(fields[0].Fields) != 0 {
		t.Errorf("fields = %v, want the flexible left holding no layout", fields)
	}
}

// deletingALayoutInsideARepeaterTakesItsRowsAway takes a deleted layout's rows away inside every repeater row.
func deletingALayoutInsideARepeaterTakesItsRowsAway(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	team := DeclareRepeater(t, s.Types, "team")
	features, err := s.Types.CreateSubField(
		t.Context(), team.ID, MustFlexible(t, "features"), content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("CreateSubField(flexible) error = %v, want nil", err)
	}
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	quote := DeclareLayout(t, s.Types, features.ID, "quote")
	DeclareUnder(t, s.Types, hero.ID, "title")
	DeclareUnder(t, s.Types, quote.ID, "title")
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"team": []any{
		map[string]any{"features": []any{
			map[string]any{"hero": map[string]any{"title": "A"}},
			map[string]any{"quote": map[string]any{"title": "B"}},
		}},
	}})

	if err := s.Types.DeleteSubField(t.Context(), hero.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := content.Values{"team": []any{
		map[string]any{"features": []any{map[string]any{"quote": map[string]any{"title": "B"}}}},
	}}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the hero rows gone inside every repeater row", held)
	}
}
