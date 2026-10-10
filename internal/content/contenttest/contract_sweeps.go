// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// contractSweepCases are the cases over the values a deleted group or layout sweeps and the ones it leaves alone.
var contractSweepCases = []Case{
	{"DeleteGroupSweepsAValueOnlyARestingGroupStillHolds", deleteGroupSweepsAValueOnlyARestingGroupStillHolds},
	{"DeleteGroupSweepsAKeyTheOtherGroupOnTheTypeLacks", deleteGroupSweepsAKeyTheOtherGroupOnTheTypeLacks},
	{"DeleteGroupSweepsAValueAGroupOnAnotherTypeServes", deleteGroupSweepsAValueAGroupOnAnotherTypeServes},
	{"DeletingALayoutLeavesAnItemWithoutTheFlexibleAlone", deletingALayoutLeavesAnItemWithoutTheFlexibleAlone},
	{"DeletingALayoutLeavesAFlexibleHoldingAnObjectAlone", deletingALayoutLeavesAFlexibleHoldingAnObjectAlone},
	{"DeletingALayoutKeepsANumberRowAndAStrayTextRow", deletingALayoutKeepsANumberRowAndAStrayTextRow},
}

// textIn stores a text field under the key at the top of the group.
func textIn(t *testing.T, types content.TypeStore, groupID int, key string) {
	t.Helper()
	if _, err := types.CreateFieldInGroup(
		t.Context(), groupID, FieldOn(t, "", key, content.FieldKindText, ""), nil,
	); err != nil {
		t.Fatalf("CreateFieldInGroup(%s) error = %v, want nil", key, err)
	}
}

// holdsExactly reports whether the values are the wanted ones, an empty want matching nil and an empty map alike.
func holdsExactly(held, want content.Values) bool {
	if len(want) == 0 {
		return len(held) == 0
	}
	return reflect.DeepEqual(held, want)
}

// expectHeld reports through the report unless the item and its revision both hold exactly the wanted values.
func expectHeld(
	t *testing.T, report func(string, ...any), store content.Store, item content.Content, revision content.Revision,
	want content.Values, why string,
) {
	t.Helper()
	if held := ValuesOf(t, store, item.ID); !holdsExactly(held, want) {
		report("stored values of %q = %v, want %v with %s", item.Title, held, want, why)
	}
	if held := RevisionValuesOf(t, store, revision); !holdsExactly(held, want) {
		report("revision values of %q = %v, want %v with %s", item.Title, held, want, why)
	}
}

// deleteGroupSweepsAValueOnlyARestingGroupStillHolds sweeps a deleted group's value that a resting group declares.
func deleteGroupSweepsAValueOnlyARestingGroupStillHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	extras := GroupOn(t, s.Types, "Extras", "car")
	textIn(t, s.Types, extras.ID, "subtitle")
	GroupOn(t, s.Types, "Shadow", "car")
	textIn(t, s.Types, Rested(t, s.Types, "Shadow").ID, "subtitle")
	car, revision := TypedHolding(t, s.Types, s.Content, "car", "One Car", author,
		content.Values{"subtitle": "must go"})
	expectHeld(t, t.Fatalf, s.Content, car, revision, content.Values{"subtitle": "must go"},
		"the subtitle stored before the delete")

	if err := s.Types.DeleteGroup(t.Context(), extras.ID, nil); err != nil {
		t.Fatalf("DeleteGroup() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, car, revision, nil, "the subtitle swept since a resting group serves nothing")
}

// deleteGroupSweepsAKeyTheOtherGroupOnTheTypeLacks sweeps the deleted group's key though another group serves the type.
func deleteGroupSweepsAKeyTheOtherGroupOnTheTypeLacks(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	extras := GroupOn(t, s.Types, "Extras", "car")
	textIn(t, s.Types, extras.ID, "subtitle")
	textIn(t, s.Types, GroupOn(t, s.Types, "Others", "car").ID, "footnote")
	car, revision := TypedHolding(t, s.Types, s.Content, "car", "One Car", author,
		content.Values{"subtitle": "must go", "footnote": "stays"})
	expectHeld(t, t.Fatalf, s.Content, car, revision, content.Values{"subtitle": "must go", "footnote": "stays"},
		"both values stored before the delete")

	if err := s.Types.DeleteGroup(t.Context(), extras.ID, nil); err != nil {
		t.Fatalf("DeleteGroup() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, car, revision, content.Values{"footnote": "stays"},
		"the subtitle swept since the other group serves only the footnote")
}

// deleteGroupSweepsAValueAGroupOnAnotherTypeServes sweeps the value off the group's type and spares the other type.
func deleteGroupSweepsAValueAGroupOnAnotherTypeServes(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	extras := GroupOn(t, s.Types, "Extras", "car")
	textIn(t, s.Types, extras.ID, "subtitle")
	textIn(t, s.Types, GroupOn(t, s.Types, "Book extras", "book").ID, "subtitle")
	car, carRevision := TypedHolding(t, s.Types, s.Content, "car", "One Car", author,
		content.Values{"subtitle": "car words"})
	book, bookRevision := TypedHolding(t, s.Types, s.Content, "book", "One Book", author,
		content.Values{"subtitle": "book words"})
	expectHeld(t, t.Fatalf, s.Content, car, carRevision, content.Values{"subtitle": "car words"},
		"the subtitle stored before the delete")

	if err := s.Types.DeleteGroup(t.Context(), extras.ID, nil); err != nil {
		t.Fatalf("DeleteGroup() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, car, carRevision, nil,
		"the subtitle swept since the book group serves no car")
	expectHeld(t, t.Errorf, s.Content, book, bookRevision, content.Values{"subtitle": "book words"},
		"the subtitle kept where the book group serves it")
}

// deletingALayoutLeavesAnItemWithoutTheFlexibleAlone writes nothing into an item holding no flexible value.
func deletingALayoutLeavesAnItemWithoutTheFlexibleAlone(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	features := DeclareFlexible(t, s.Types)
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	quote := DeclareLayout(t, s.Types, features.ID, "quote")
	DeclareUnder(t, s.Types, hero.ID, "title")
	DeclareUnder(t, s.Types, quote.ID, "title")
	DeclareTypedField(t, s.Types, "car", "colour")
	bare, bareRevision := TypedHolding(t, s.Types, s.Content, "car", "Bare", author, content.Values{"colour": "red"})
	full, fullRevision := TypedHolding(t, s.Types, s.Content, "car", "Full", author, content.Values{"features": []any{
		map[string]any{"hero": map[string]any{"title": "A"}},
		map[string]any{"quote": map[string]any{"title": "B"}},
	}})
	expectHeld(t, t.Fatalf, s.Content, full, fullRevision, content.Values{"features": []any{
		map[string]any{"hero": map[string]any{"title": "A"}},
		map[string]any{"quote": map[string]any{"title": "B"}},
	}}, "both rows stored before the delete")

	if err := s.Types.DeleteSubField(t.Context(), hero.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, bare, bareRevision, content.Values{"colour": "red"},
		"no flexible written into an item that holds none")
	expectHeld(t, t.Errorf, s.Content, full, fullRevision, content.Values{"features": []any{
		map[string]any{"quote": map[string]any{"title": "B"}},
	}}, "the hero row swept from the item holding it")
}

// deletingALayoutLeavesAFlexibleHoldingAnObjectAlone keeps a flexible value that is an object rather than rows.
func deletingALayoutLeavesAFlexibleHoldingAnObjectAlone(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	features := DeclareFlexible(t, s.Types)
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	DeclareUnder(t, s.Types, hero.ID, "title")
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{
		"features": map[string]any{"hero": map[string]any{"title": "A"}},
	})

	if err := s.Types.DeleteSubField(t.Context(), hero.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, one, revision, content.Values{
		"features": map[string]any{"hero": map[string]any{"title": "A"}},
	}, "the object kept as it stands since it holds no rows")
}

// deletingALayoutKeepsANumberRowAndAStrayTextRow takes the layout's row away and keeps the other two rows in place.
func deletingALayoutKeepsANumberRowAndAStrayTextRow(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	features := DeclareFlexible(t, s.Types)
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	DeclareUnder(t, s.Types, hero.ID, "title")
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"features": []any{
		float64(42), "stray", map[string]any{"hero": map[string]any{"title": "A"}},
	}})
	expectHeld(t, t.Fatalf, s.Content, one, revision, content.Values{"features": []any{
		float64(42), "stray", map[string]any{"hero": map[string]any{"title": "A"}},
	}}, "every row stored before the delete")

	if err := s.Types.DeleteSubField(t.Context(), hero.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, one, revision, content.Values{"features": []any{float64(42), "stray"}},
		"the hero row gone and the number and text rows kept in place")
}
