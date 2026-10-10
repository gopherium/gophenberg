// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// fieldSweepCases are the cases over the values a sub field delete sweeps from items and revisions or leaves.
var fieldSweepCases = []Case{
	{"DeletingASubFieldSweepsItsValuesInsideASection", deletingASubFieldSweepsItsValuesInsideASection},
	{
		"DeletingARestingGroupsSubFieldKeepsTheValueTheServedFieldHolds",
		deletingARestingGroupsSubFieldKeepsTheValueTheServedFieldHolds,
	},
	{
		"DeletingARestingGroupsSubFieldSweepsTheValueTheServedSectionLacks",
		deletingARestingGroupsSubFieldSweepsTheValueTheServedSectionLacks,
	},
	{"DeletingASubFieldSweepsItsValuesFromEveryRow", deletingASubFieldSweepsItsValuesFromEveryRow},
	{
		"DeletingASubFieldLeavesTheContainerKeyHoldingAWord",
		deletingASubFieldLeavesTheContainerKeyHoldingAWord,
	},
	{
		"DeletingASubFieldLeavesAnItemLackingTheContainerKey",
		deletingASubFieldLeavesAnItemLackingTheContainerKey,
	},
	{
		"DeletingASubFieldLeavesASectionLackingTheInnerKey",
		deletingASubFieldLeavesASectionLackingTheInnerKey,
	},
	{"DeletingASubFieldLeavesRowsThatAreNotObjects", deletingASubFieldLeavesRowsThatAreNotObjects},
	{"DeletingAContainerSweepsEverythingInsideIt", deletingAContainerSweepsEverythingInsideIt},
	{"DeletingASubFieldReportsOneThatIsGone", deletingASubFieldReportsOneThatIsGone},
}

// deletingASubFieldSweepsItsValuesInsideASection sweeps a deleted sub field from inside its section everywhere.
func deletingASubFieldSweepsItsValuesInsideASection(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	if _, err := s.Types.CreateSubField(
		t.Context(), specs.ID, FieldOn(t, "", "colour", content.FieldKindText, ""), content.DefaultFieldDepth,
	); err != nil {
		t.Fatalf("declaring colour: %v, want nil", err)
	}
	dropped, err := s.Types.CreateSubField(
		t.Context(), specs.ID, FieldOn(t, "", "doors", content.FieldKindText, ""), content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("declaring doors: %v, want nil", err)
	}
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"specs": map[string]any{"colour": "red", "doors": "five"}})

	if err := s.Types.DeleteSubField(t.Context(), dropped.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := content.Values{"specs": map[string]any{"colour": "red"}}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the doors swept from inside the section", held)
	}
	if held := RevisionValuesOf(t, s.Content, revision); !reflect.DeepEqual(held, want) {
		t.Errorf("revision values = %v, want the doors swept there too", held)
	}
}

// deletingARestingGroupsSubFieldKeepsTheValueTheServedFieldHolds keeps the value the served section still holds.
func deletingARestingGroupsSubFieldKeepsTheValueTheServedFieldHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	DeclaredInside(t, s.Types, specs, "color", content.FieldKindText)
	twin := RestingTwinOf(t, s.Types, "specs")
	dropped := DeclaredInside(t, s.Types, twin, "color", content.FieldKindText)
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"specs": map[string]any{"color": "red"}})

	if err := s.Types.DeleteSubField(t.Context(), dropped.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := content.Values{"specs": map[string]any{"color": "red"}}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the color the served section holds kept", held)
	}
	if held := RevisionValuesOf(t, s.Content, revision); !reflect.DeepEqual(held, want) {
		t.Errorf("revision values = %v, want the color kept there too", held)
	}
}

// deletingARestingGroupsSubFieldSweepsTheValueTheServedSectionLacks sweeps the value the served section lacks.
func deletingARestingGroupsSubFieldSweepsTheValueTheServedSectionLacks(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	DeclaredInside(t, s.Types, specs, "size", content.FieldKindText)
	twin := RestingTwinOf(t, s.Types, "specs")
	dropped := DeclaredInside(t, s.Types, twin, "color", content.FieldKindText)
	stored := content.Values{"specs": map[string]any{"color": "red", "size": "big"}}
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author, stored)
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, stored) {
		t.Fatalf("stored values = %v before the delete, want the color and the size stored", held)
	}

	if err := s.Types.DeleteSubField(t.Context(), dropped.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := content.Values{"specs": map[string]any{"size": "big"}}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the color swept since the served section lacks it", held)
	}
	if held := RevisionValuesOf(t, s.Content, revision); !reflect.DeepEqual(held, want) {
		t.Errorf("revision values = %v, want the color swept there too", held)
	}
}

// deletingASubFieldSweepsItsValuesFromEveryRow sweeps a deleted sub field of a repeater from every row.
func deletingASubFieldSweepsItsValuesFromEveryRow(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	team := DeclareRepeater(t, s.Types, "team")
	if _, err := s.Types.CreateSubField(
		t.Context(), team.ID, FieldOn(t, "", "name", content.FieldKindText, ""), content.DefaultFieldDepth,
	); err != nil {
		t.Fatalf("declaring name: %v, want nil", err)
	}
	dropped, err := s.Types.CreateSubField(
		t.Context(), team.ID, FieldOn(t, "", "role", content.FieldKindText, ""), content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("declaring role: %v, want nil", err)
	}
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"team": []any{
		map[string]any{"name": "Maria Perez", "role": "lead"}, map[string]any{"name": "Kip", "role": "smith"},
	}})

	if err := s.Types.DeleteSubField(t.Context(), dropped.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := content.Values{"team": []any{map[string]any{"name": "Maria Perez"}, map[string]any{"name": "Kip"}}}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the role swept from every row", held)
	}
}

// deletingASubFieldLeavesTheContainerKeyHoldingAWord leaves a word standing where the section should be.
func deletingASubFieldLeavesTheContainerKeyHoldingAWord(t *testing.T, s Stores) {
	leavesWhatThePathDoesNotReach(t, s, content.FieldKindSection,
		content.Values{"specs": "typed"}, content.Values{"specs": "typed"})
}

// deletingASubFieldLeavesAnItemLackingTheContainerKey leaves an item that holds no section untouched.
func deletingASubFieldLeavesAnItemLackingTheContainerKey(t *testing.T, s Stores) {
	leavesWhatThePathDoesNotReach(t, s, content.FieldKindSection,
		content.Values{"colour": "red"}, content.Values{"colour": "red"})
}

// deletingASubFieldLeavesASectionLackingTheInnerKey leaves a section that never held the deleted key untouched.
func deletingASubFieldLeavesASectionLackingTheInnerKey(t *testing.T, s Stores) {
	leavesWhatThePathDoesNotReach(t, s, content.FieldKindSection,
		content.Values{"specs": map[string]any{"colour": "red"}},
		content.Values{"specs": map[string]any{"colour": "red"}})
}

// deletingASubFieldLeavesRowsThatAreNotObjects leaves the repeater rows that are not objects as they stand.
func deletingASubFieldLeavesRowsThatAreNotObjects(t *testing.T, s Stores) {
	leavesWhatThePathDoesNotReach(t, s, content.FieldKindRepeater,
		content.Values{"specs": []any{float64(42), "stray", map[string]any{"doors": "five"}}},
		content.Values{"specs": []any{float64(42), "stray", map[string]any{}}})
}

// leavesWhatThePathDoesNotReach deletes doors from a car container of the kind and asserts the values become want.
func leavesWhatThePathDoesNotReach(t *testing.T, s Stores, kind content.FieldKind, planted, want content.Values) {
	t.Helper()
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	DeclareTypedField(t, s.Types, "car", "colour")
	var parent content.Field
	if kind == content.FieldKindRepeater {
		parent = DeclareRepeater(t, s.Types, "specs")
	} else {
		parent = DeclareSection(t, s.Types, "specs")
	}
	dropped, err := s.Types.CreateSubField(
		t.Context(), parent.ID, FieldOn(t, "", "doors", content.FieldKindText, ""), content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("declaring doors: %v, want nil", err)
	}
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author, planted)
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, planted) {
		t.Fatalf("stored values = %v before the delete, want %v stored", held, planted)
	}

	if err := s.Types.DeleteSubField(t.Context(), dropped.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want %v left untouched by the sweep", held, want)
	}
}

// deletingAContainerSweepsEverythingInsideIt sweeps a deleted container and all it held from every row.
func deletingAContainerSweepsEverythingInsideIt(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	team := DeclareRepeater(t, s.Types, "team")
	inner, err := s.Types.CreateSubField(t.Context(), team.ID, SectionOn(t, "contact"), content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("declaring contact: %v, want nil", err)
	}
	if _, err := s.Types.CreateSubField(
		t.Context(), inner.ID, FieldOn(t, "", "phone", content.FieldKindText, ""), content.DefaultFieldDepth,
	); err != nil {
		t.Fatalf("declaring phone: %v, want nil", err)
	}
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"team": []any{map[string]any{"contact": map[string]any{"phone": "184467235"}}}})

	if err := s.Types.DeleteSubField(t.Context(), inner.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := content.Values{"team": []any{map[string]any{}}}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the whole contact swept from the row", held)
	}
}

// deletingASubFieldReportsOneThatIsGone answers field not found for a sub field that does not exist.
func deletingASubFieldReportsOneThatIsGone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")

	err := s.Types.DeleteSubField(t.Context(), 424242, nil)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("DeleteSubField() error = %v, want the field that is gone reported as %v", err, content.ErrFieldNotFound)
	}
}
