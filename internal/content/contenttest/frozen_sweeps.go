// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// frozenSweepCases are the cases over the values a delete or a move sweeps from types the group no longer reaches.
var frozenSweepCases = []Case{
	{
		"DeletingASubFieldSweepsAValueLeftOnATypeItsGroupMovedOff",
		deletingASubFieldSweepsAValueLeftOnATypeItsGroupMovedOff,
	},
	{
		"DeletingALayoutSweepsItsRowsFromATypeItsGroupMovedOff",
		deletingALayoutSweepsItsRowsFromATypeItsGroupMovedOff,
	},
	{
		"DeletingASubFieldSweepsAValueLeftOnATurnedOffType",
		deletingASubFieldSweepsAValueLeftOnATurnedOffType,
	},
	{
		"MovingAFieldOutOfAContainerSweepsAValueLeftOnATypeItsGroupMovedOff",
		movingAFieldOutOfAContainerSweepsAValueLeftOnATypeItsGroupMovedOff,
	},
	{
		"MovingAFieldIntoAContainerSweepsAValueLeftOnATypeItsGroupMovedOff",
		movingAFieldIntoAContainerSweepsAValueLeftOnATypeItsGroupMovedOff,
	},
	{
		"MovingAFieldBetweenTopsSweepsAValueLeftOnATypeItsGroupMovedOff",
		movingAFieldBetweenTopsSweepsAValueLeftOnATypeItsGroupMovedOff,
	},
	{
		"MovingALayoutBetweenFlexiblesSweepsItsRowsFromATypeItsGroupMovedOff",
		movingALayoutBetweenFlexiblesSweepsItsRowsFromATypeItsGroupMovedOff,
	},
}

// frozenDoors is a car item holding a colour and doors inside the specs of a group that moved to books.
type frozenDoors struct {
	extras   content.Group
	specs    content.Field
	doors    content.Field
	left     content.Content
	revision content.Revision
}

// frozenDoorsOnCar stores both specs values on a car item, then moves their group to books and checks them frozen.
func frozenDoorsOnCar(t *testing.T, s Stores) frozenDoors {
	t.Helper()
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	held := frozenDoors{extras: GroupOn(t, s.Types, "Extras", "car")}
	held.specs = specsIn(t, s.Types, held.extras.ID)
	DeclaredInside(t, s.Types, held.specs, "colour", content.FieldKindText)
	held.doors = DeclaredInside(t, s.Types, held.specs, "doors", content.FieldKindText)
	held.left, held.revision = TypedHolding(t, s.Types, s.Content, "car", "Left Behind", author, bothSpecs())
	movedToBooks(t, s.Types, held.extras)
	expectHeld(t, t.Fatalf, s.Content, held.left, held.revision, bothSpecs(),
		"both values frozen on the car once the group moved off it")
	return held
}

// bothSpecs returns the colour and the doors inside the specs of a frozen car item.
func bothSpecs() content.Values {
	return content.Values{"specs": map[string]any{"colour": "red", "doors": "five"}}
}

// colourAlone returns the specs of a frozen car item once its doors are swept.
func colourAlone() content.Values {
	return content.Values{"specs": map[string]any{"colour": "red"}}
}

// specsIn stores a section keyed specs at the top of the group and returns it.
func specsIn(t *testing.T, types content.TypeStore, groupID int) content.Field {
	t.Helper()
	stored, err := types.CreateFieldInGroup(t.Context(), groupID, SectionOn(t, "specs"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	return stored
}

// flexibleIn stores a flexible field under the key at the top of the group and returns it.
func flexibleIn(t *testing.T, types content.TypeStore, groupID int, key string) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{Key: key, Label: key, Kind: content.FieldKindFlexible})
	if err != nil {
		t.Fatalf("NewField(flexible %s) error = %v, want nil", key, err)
	}
	stored, err := types.CreateFieldInGroup(t.Context(), groupID, built, nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(flexible %s) error = %v, want nil", key, err)
	}
	return stored
}

// movedToBooks places the group on the book type alone, leaving the values it stored on other types frozen.
func movedToBooks(t *testing.T, types content.TypeStore, group content.Group) {
	t.Helper()
	group.Location = LocationOf("book")
	if _, err := types.UpdateGroup(t.Context(), group, nil, nil); err != nil {
		t.Fatalf("UpdateGroup(%s) error = %v, want nil", group.Title, err)
	}
}

// heroAndQuote returns a flexible value holding one hero row and one quote row.
func heroAndQuote() content.Values {
	return content.Values{"features": []any{
		map[string]any{"hero": map[string]any{"title": "A"}},
		map[string]any{"quote": map[string]any{"title": "B"}},
	}}
}

// quoteAlone returns the flexible value left once the hero row is taken away.
func quoteAlone() content.Values {
	return content.Values{"features": []any{map[string]any{"quote": map[string]any{"title": "B"}}}}
}

// deletingASubFieldSweepsAValueLeftOnATypeItsGroupMovedOff sweeps a sub field's value off a type its group left.
func deletingASubFieldSweepsAValueLeftOnATypeItsGroupMovedOff(t *testing.T, s Stores) {
	held := frozenDoorsOnCar(t, s)

	if err := s.Types.DeleteSubField(t.Context(), held.doors.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, held.left, held.revision, colourAlone(),
		"the frozen doors swept with the sub field that declared them")
}

// deletingALayoutSweepsItsRowsFromATypeItsGroupMovedOff takes a deleted layout's rows off a type its group left.
func deletingALayoutSweepsItsRowsFromATypeItsGroupMovedOff(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	extras := GroupOn(t, s.Types, "Extras", "car")
	features := flexibleIn(t, s.Types, extras.ID, "features")
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	quote := DeclareLayout(t, s.Types, features.ID, "quote")
	DeclareUnder(t, s.Types, hero.ID, "title")
	DeclareUnder(t, s.Types, quote.ID, "title")
	left, revision := TypedHolding(t, s.Types, s.Content, "car", "Left Behind", author, heroAndQuote())
	movedToBooks(t, s.Types, extras)
	expectHeld(t, t.Fatalf, s.Content, left, revision, heroAndQuote(),
		"both rows frozen on the car once the group moved off it")

	if err := s.Types.DeleteSubField(t.Context(), hero.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, left, revision, quoteAlone(), "the frozen hero row taken away with its layout")
}

// deletingASubFieldSweepsAValueLeftOnATurnedOffType sweeps a frozen value off a type that is turned off.
func deletingASubFieldSweepsAValueLeftOnATurnedOffType(t *testing.T, s Stores) {
	held := frozenDoorsOnCar(t, s)
	car, err := s.Types.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey(car) error = %v, want nil", err)
	}
	car.Active = false
	car.UpdatedAt = time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), car); err != nil {
		t.Fatalf("turning the car type off: %v, want nil", err)
	}

	if err := s.Types.DeleteSubField(t.Context(), held.doors.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, held.left, held.revision, colourAlone(),
		"the frozen doors swept from the turned off car as well")
}

// movingAFieldOutOfAContainerSweepsAValueLeftOnATypeItsGroupMovedOff sweeps the old place on a type its group left.
func movingAFieldOutOfAContainerSweepsAValueLeftOnATypeItsGroupMovedOff(t *testing.T, s Stores) {
	held := frozenDoorsOnCar(t, s)

	if _, err := s.Types.MoveField(
		t.Context(), held.doors.ID, held.extras.ID, 0, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, held.left, held.revision, colourAlone(),
		"the frozen doors swept from the place the move left")
}

// movingAFieldIntoAContainerSweepsAValueLeftOnATypeItsGroupMovedOff sweeps a top value on a type its group left.
func movingAFieldIntoAContainerSweepsAValueLeftOnATypeItsGroupMovedOff(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	extras := GroupOn(t, s.Types, "Extras", "car")
	specs := specsIn(t, s.Types, extras.ID)
	title := TitleIn(t, s.Types, extras.ID)
	left, revision := TypedHolding(t, s.Types, s.Content, "car", "Left Behind", author,
		content.Values{"title": "old words"})
	movedToBooks(t, s.Types, extras)
	expectHeld(t, t.Fatalf, s.Content, left, revision, content.Values{"title": "old words"},
		"the title frozen on the car once the group moved off it")

	if _, err := s.Types.MoveField(
		t.Context(), title.ID, extras.ID, specs.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, left, revision, nil, "the frozen title swept from the top the move left")
}

// movingAFieldBetweenTopsSweepsAValueLeftOnATypeItsGroupMovedOff sweeps a type neither group reaches any more.
func movingAFieldBetweenTopsSweepsAValueLeftOnATypeItsGroupMovedOff(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	extras := GroupOn(t, s.Types, "Extras", "car")
	title := TitleIn(t, s.Types, extras.ID)
	notes := GroupOn(t, s.Types, "Book notes", "book")
	left, revision := TypedHolding(t, s.Types, s.Content, "car", "Left Behind", author,
		content.Values{"title": "old words"})
	movedToBooks(t, s.Types, extras)
	expectHeld(t, t.Fatalf, s.Content, left, revision, content.Values{"title": "old words"},
		"the title frozen on the car once the group moved off it")

	if _, err := s.Types.MoveField(t.Context(), title.ID, notes.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, left, revision, nil,
		"the frozen title swept from the car the landing group misses")
}

// movingALayoutBetweenFlexiblesSweepsItsRowsFromATypeItsGroupMovedOff takes a moved layout's rows off a left type.
func movingALayoutBetweenFlexiblesSweepsItsRowsFromATypeItsGroupMovedOff(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	extras := GroupOn(t, s.Types, "Extras", "car")
	features := flexibleIn(t, s.Types, extras.ID, "features")
	blocks := flexibleIn(t, s.Types, extras.ID, "blocks")
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	quote := DeclareLayout(t, s.Types, features.ID, "quote")
	DeclareUnder(t, s.Types, hero.ID, "title")
	DeclareUnder(t, s.Types, quote.ID, "title")
	left, revision := TypedHolding(t, s.Types, s.Content, "car", "Left Behind", author, heroAndQuote())
	movedToBooks(t, s.Types, extras)
	expectHeld(t, t.Fatalf, s.Content, left, revision, heroAndQuote(),
		"both rows frozen on the car once the group moved off it")

	if _, err := s.Types.MoveField(
		t.Context(), hero.ID, extras.ID, blocks.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, left, revision, quoteAlone(), "the frozen hero row taken away by the move")
}
