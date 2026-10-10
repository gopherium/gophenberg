// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"reflect"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// fieldMoveValueCases are the cases over the values and relation index rows a field move sweeps or keeps.
var fieldMoveValueCases = []Case{
	{"MovingAFieldIntoAContainerSweepsTheValuesItHeld", movingAFieldIntoAContainerSweepsTheValuesItHeld},
	{"MovingAFieldOutOfAContainerSweepsItFromInsideIt", movingAFieldOutOfAContainerSweepsItFromInsideIt},
	{"MovingAFieldBetweenGroupTopsKeepsItsValues", movingAFieldBetweenGroupTopsKeepsItsValues},
	{
		"MovingAFieldBetweenTopsKeepsTheValuesOnContentBothGroupsReach",
		movingAFieldBetweenTopsKeepsTheValuesOnContentBothGroupsReach,
	},
	{
		"MovingAFieldBetweenTopsSweepsTheValuesOnContentTheNewGroupMisses",
		movingAFieldBetweenTopsSweepsTheValuesOnContentTheNewGroupMisses,
	},
	{
		"MovingAFieldBetweenTopsSparesTheValueAnotherGroupServesWhereItLeaves",
		movingAFieldBetweenTopsSparesTheValueAnotherGroupServesWhereItLeaves,
	},
	{
		"MovingAFieldIntoARestingGroupOnTheSameContentKeepsItsValuesForTheWake",
		movingAFieldIntoARestingGroupOnTheSameContentKeepsItsValuesForTheWake,
	},
	{
		"MovingAShadowedRelationOffContentDropsOnlyItsOwnIndexRows",
		movingAShadowedRelationOffContentDropsOnlyItsOwnIndexRows,
	},
	{
		"MovingARelationBetweenTopsSweepsItsIndexRowsWhereTheNewGroupMisses",
		movingARelationBetweenTopsSweepsItsIndexRowsWhereTheNewGroupMisses,
	},
	{"MovingARelationSweepsItsIndexRows", movingARelationSweepsItsIndexRows},
	{"MovingASectionSweepsTheRelationsInsideIt", movingASectionSweepsTheRelationsInsideIt},
	{
		"MovingAShadowedFieldKeepsTheValuesTheServedFieldHolds",
		movingAShadowedFieldKeepsTheValuesTheServedFieldHolds,
	},
	{
		"MovingAFieldOfARestingGroupSweepsTheValuesNoGroupServes",
		movingAFieldOfARestingGroupSweepsTheValuesNoGroupServes,
	},
	{
		"MovingARestingGroupsSubFieldOutSweepsTheValueTheServedSectionLacks",
		movingARestingGroupsSubFieldOutSweepsTheValueTheServedSectionLacks,
	},
	{
		"MovingAFieldOutOfAContainerItsGroupServesSweepsItWhereARivalHoldsTheContainer",
		movingAFieldOutOfAContainerItsGroupServesSweepsItWhereARivalHoldsTheContainer,
	},
	{
		"MovingAFieldItsGroupServesIntoAContainerSweepsItWhereARivalHoldsTheKey",
		movingAFieldItsGroupServesIntoAContainerSweepsItWhereARivalHoldsTheKey,
	},
	{"MovingALayoutBetweenFlexiblesTakesItsRowsAway", movingALayoutBetweenFlexiblesTakesItsRowsAway},
}

// snapshotsHolding counts the revision and the author's autosave of the item that still hold the key.
func snapshotsHolding(
	t *testing.T, store content.Store, item content.Content, revision content.Revision, author uuid.UUID, key string,
) int {
	t.Helper()
	held := 0
	for _, values := range []content.Values{
		RevisionValuesOf(t, store, revision), AutosaveValuesOf(t, store, item, author),
	} {
		if _, kept := values[key]; kept {
			held++
		}
	}
	return held
}

// pointersThrough returns how many published items point at the target through the field.
func pointersThrough(t *testing.T, store content.Store, target uuid.UUID, field int) int {
	t.Helper()
	_, total, err := store.PointingAt(t.Context(), target, field, 1, 20)
	if err != nil {
		t.Fatalf("PointingAt() error = %v, want nil", err)
	}
	return total
}

// movingAFieldIntoAContainerSweepsTheValuesItHeld sweeps a top field's values from items, revisions and autosaves.
func movingAFieldIntoAContainerSweepsTheValuesItHeld(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	subtitle := DeclareTypedField(t, s.Types, "car", "subtitle")
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"subtitle": "kept words"})
	ParkValues(t, s.Content, one, author, content.Values{"subtitle": "typed words"})

	if _, err := s.Types.MoveField(
		t.Context(), subtitle.ID, specs.GroupID, specs.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, one.ID); len(held) != 0 {
		t.Errorf("stored values = %v, want the subtitle swept from the item", held)
	}
	if held := snapshotsHolding(t, s.Content, one, revision, author, "subtitle"); held != 0 {
		t.Errorf("%d revisions still hold the subtitle, want it swept from revisions and autosaves", held)
	}
}

// movingAFieldOutOfAContainerSweepsItFromInsideIt sweeps a sub field moved to the top from inside its section.
func movingAFieldOutOfAContainerSweepsItFromInsideIt(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	DeclaredInside(t, s.Types, specs, "colour", content.FieldKindText)
	doors := DeclaredInside(t, s.Types, specs, "doors", content.FieldKindText)
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"specs": map[string]any{"colour": "red", "doors": "five"}})

	if _, err := s.Types.MoveField(
		t.Context(), doors.ID, specs.GroupID, 0, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	want := content.Values{"specs": map[string]any{"colour": "red"}}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the doors swept from inside the section", held)
	}
	if held := RevisionValuesOf(t, s.Content, revision); !reflect.DeepEqual(held, want) {
		t.Errorf("revision values = %v, want the doors swept there too", held)
	}
}

// movingAFieldBetweenGroupTopsKeepsItsValues keeps the values of a field moved to another group's top on the type.
func movingAFieldBetweenGroupTopsKeepsItsValues(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	subtitle := DeclareTypedField(t, s.Types, "car", "subtitle")
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"subtitle": "kept words"})
	extras, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	if _, err := s.Types.MoveField(
		t.Context(), subtitle.ID, extras.ID, 0, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	held := ValuesOf(t, s.Content, one.ID)
	if !reflect.DeepEqual(held, content.Values{"subtitle": "kept words"}) {
		t.Errorf("stored values = %v, want the value kept while the path stays the same", held)
	}
}

// movingAFieldBetweenTopsKeepsTheValuesOnContentBothGroupsReach keeps the values where both groups serve the type.
func movingAFieldBetweenTopsKeepsTheValuesOnContentBothGroupsReach(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	title := TitleIn(t, s.Types, GroupOn(t, s.Types, "Car extras", "car").ID)
	notes := GroupOn(t, s.Types, "Car notes", "car")
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"title": "kept words"})

	if _, err := s.Types.MoveField(t.Context(), title.ID, notes.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	want := content.Values{"title": "kept words"}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the title kept on the content both groups reach", held)
	}
	if held := RevisionValuesOf(t, s.Content, revision); !reflect.DeepEqual(held, want) {
		t.Errorf("revision values = %v, want the title kept on the content both groups reach", held)
	}
}

// movingAFieldBetweenTopsSweepsTheValuesOnContentTheNewGroupMisses sweeps the values the landing group misses.
func movingAFieldBetweenTopsSweepsTheValuesOnContentTheNewGroupMisses(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	title := TitleIn(t, s.Types, GroupOn(t, s.Types, "Car extras", "car").ID)
	books := GroupOn(t, s.Types, "Book extras", "book")
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"title": "old words"})

	if _, err := s.Types.MoveField(t.Context(), title.ID, books.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, one.ID); len(held) != 0 {
		t.Errorf("stored values = %v, want the title swept from the content the book group misses", held)
	}
	if held := RevisionValuesOf(t, s.Content, revision); len(held) != 0 {
		t.Errorf("revision values = %v, want the title swept from the content the book group misses", held)
	}
	if top := GroupAt(t, s.Types, books.ID).Fields; len(top) != 1 || top[0].ID != title.ID {
		t.Errorf("the book group holds %+v at its top, want the title standing there alone", top)
	}
}

// movingAFieldBetweenTopsSparesTheValueAnotherGroupServesWhereItLeaves keeps a value another group still serves.
func movingAFieldBetweenTopsSparesTheValueAnotherGroupServesWhereItLeaves(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	TitleIn(t, s.Types, GroupOn(t, s.Types, "Car facts", "car").ID)
	title := TitleIn(t, s.Types, GroupOn(t, s.Types, "Car extras", "car").ID)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	books := GroupOn(t, s.Types, "Book extras", "book")
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"title": "served words"})

	if _, err := s.Types.MoveField(t.Context(), title.ID, books.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, content.Values{"title": "served words"}) {
		t.Errorf("stored values = %v, want the title kept where the car facts group still serves it", held)
	}
}

// movingAFieldIntoARestingGroupOnTheSameContentKeepsItsValuesForTheWake keeps the values for a resting group.
func movingAFieldIntoARestingGroupOnTheSameContentKeepsItsValuesForTheWake(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	title := TitleIn(t, s.Types, GroupOn(t, s.Types, "Car extras", "car").ID)
	GroupOn(t, s.Types, "Car notes", "car")
	notes := Rested(t, s.Types, "Car notes")
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"title": "kept words"})

	if _, err := s.Types.MoveField(t.Context(), title.ID, notes.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	want := content.Values{"title": "kept words"}
	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, want) {
		t.Errorf("stored values = %v, want the title kept for the resting group to serve once woken", held)
	}
	if held := RevisionValuesOf(t, s.Content, revision); !reflect.DeepEqual(held, want) {
		t.Errorf("revision values = %v, want the title kept for the resting group to serve once woken", held)
	}
}

// movingAShadowedRelationOffContentDropsOnlyItsOwnIndexRows drops the moved relation's rows and no other.
func movingAShadowedRelationOffContentDropsOnlyItsOwnIndexRows(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "book")
	maker := FieldOn(t, "", "maker", content.FieldKindRelation, "book")
	facts := GroupOn(t, s.Types, "Car facts", "car")
	serving, err := s.Types.CreateFieldInGroup(t.Context(), facts.ID, maker, nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(facts maker) error = %v, want nil", err)
	}
	extras := GroupOn(t, s.Types, "Car extras", "car")
	shadowed, err := s.Types.CreateFieldInGroup(t.Context(), extras.ID, maker, nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(extras maker) error = %v, want nil", err)
	}
	StoreType(t, s.Types, "car")
	books := GroupOn(t, s.Types, "Book extras", "book")
	pointed, _ := TypedHolding(t, s.Types, s.Content, "book", "Pointed", author, content.Values{})
	pointing, _ := TypedHolding(t, s.Types, s.Content, "car", "Pointing", author,
		content.Values{"maker": []any{pointed.ID.String()}})
	PublishItem(t, s.Content, pointing)
	if held := pointersThrough(t, s.Content, pointed.ID, shadowed.ID); held != 1 {
		t.Fatalf("the shadowed relation indexes %d rows before the move, want the published item's row", held)
	}

	if _, err := s.Types.MoveField(t.Context(), shadowed.ID, books.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	values := ValuesOf(t, s.Content, pointing.ID)
	if _, kept := values["maker"]; !kept {
		t.Errorf("stored values = %v, want the relation kept where the car facts group serves it", values)
	}
	if held := pointersThrough(t, s.Content, pointed.ID, serving.ID); held != 1 {
		t.Errorf("the serving relation indexes %d rows, want its own row kept", held)
	}
	if held := pointersThrough(t, s.Content, pointed.ID, shadowed.ID); held != 0 {
		t.Errorf("the moved relation indexes %d rows on content it left, want none", held)
	}
}

// movingARelationBetweenTopsSweepsItsIndexRowsWhereTheNewGroupMisses drops the rows the landing group misses.
func movingARelationBetweenTopsSweepsItsIndexRowsWhereTheNewGroupMisses(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	cars := GroupOn(t, s.Types, "Car extras", "car")
	maker, err := s.Types.CreateFieldInGroup(
		t.Context(), cars.ID, FieldOn(t, "", "maker", content.FieldKindRelation, "car"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(maker) error = %v, want nil", err)
	}
	books := GroupOn(t, s.Types, "Book extras", "book")
	two, _ := TypedHolding(t, s.Types, s.Content, "car", "Two", author, content.Values{})
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"maker": []any{two.ID.String()}})
	PublishItem(t, s.Content, one)
	if held := pointersThrough(t, s.Content, two.ID, maker.ID); held != 1 {
		t.Fatalf("the relation indexes %d rows before the move, want the published item's row", held)
	}

	if _, err := s.Types.MoveField(t.Context(), maker.ID, books.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := pointersThrough(t, s.Content, two.ID, maker.ID); held != 0 {
		t.Errorf("%d relation rows survive the move, want the index swept where the book group misses", held)
	}
}

// movingARelationSweepsItsIndexRows drops the index rows of a relation moved into a section.
func movingARelationSweepsItsIndexRows(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	maker, err := s.Types.CreateFieldInGroup(t.Context(), FieldsGroupOf(t, s.Types, "car").ID,
		FieldOn(t, "car", "maker", content.FieldKindRelation, "car"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(maker) error = %v, want nil", err)
	}
	two, _ := TypedHolding(t, s.Types, s.Content, "car", "Two", author, content.Values{})
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"maker": []any{two.ID.String()}})
	PublishItem(t, s.Content, one)
	if held := pointersThrough(t, s.Content, two.ID, maker.ID); held != 1 {
		t.Fatalf("the relation indexes %d rows before the move, want the published item's row", held)
	}

	if _, err := s.Types.MoveField(
		t.Context(), maker.ID, specs.GroupID, specs.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := pointersThrough(t, s.Content, two.ID, maker.ID); held != 0 {
		t.Errorf("%d relation rows survive the move, want the index swept with the values", held)
	}
}

// movingASectionSweepsTheRelationsInsideIt drops the index rows of a relation inside a moved section.
func movingASectionSweepsTheRelationsInsideIt(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	maker := RelationInside(t, s.Types, specs, "maker")
	box := DeclareSection(t, s.Types, "box")
	two, _ := TypedHolding(t, s.Types, s.Content, "car", "Two", author, content.Values{})
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"specs": map[string]any{"maker": []any{two.ID.String()}}})
	PublishItem(t, s.Content, one)
	if held := pointersThrough(t, s.Content, two.ID, maker.ID); held != 1 {
		t.Fatalf("the relation inside the section indexes %d rows before the move, want the published item's row",
			held)
	}

	if _, err := s.Types.MoveField(
		t.Context(), specs.ID, box.GroupID, box.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := pointersThrough(t, s.Content, two.ID, maker.ID); held != 0 {
		t.Errorf("%d relation rows survive the move, want the rows of the relation inside the section swept", held)
	}
}

// movingAShadowedFieldKeepsTheValuesTheServedFieldHolds keeps the values of the field an active group serves.
func movingAShadowedFieldKeepsTheValuesTheServedFieldHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	DeclareTypedField(t, s.Types, "car", "title")
	shadow, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Shadow", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Shadow) error = %v, want nil", err)
	}
	idle := Rested(t, s.Types, "Shadow")
	specs, err := s.Types.CreateFieldInGroup(t.Context(), idle.ID, SectionOn(t, "specs"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	shadowed, err := s.Types.CreateFieldInGroup(
		t.Context(), shadow.ID, FieldOn(t, "", "title", content.FieldKindText, ""), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(shadow title) error = %v, want nil", err)
	}
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"title": "served words"})
	ParkValues(t, s.Content, one, author, content.Values{"title": "typed words"})

	if _, err := s.Types.MoveField(
		t.Context(), shadowed.ID, idle.ID, specs.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, content.Values{"title": "served words"}) {
		t.Errorf("stored values = %v, want the served field's value kept", held)
	}
	if held := snapshotsHolding(t, s.Content, one, revision, author, "title"); held != 2 {
		t.Errorf("%d revisions hold the title, want the revision and the autosave both kept", held)
	}
}

// movingAFieldOfARestingGroupSweepsTheValuesNoGroupServes sweeps the values of a resting group's moved field.
func movingAFieldOfARestingGroupSweepsTheValuesNoGroupServes(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	resting, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Resting", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Resting) error = %v, want nil", err)
	}
	specs, err := s.Types.CreateFieldInGroup(t.Context(), resting.ID, SectionOn(t, "specs"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	title, err := s.Types.CreateFieldInGroup(
		t.Context(), resting.ID, FieldOn(t, "", "title", content.FieldKindText, ""), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(title) error = %v, want nil", err)
	}
	Rested(t, s.Types, "Resting")
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"title": "old words"})
	if held := ValuesOf(t, s.Content, one.ID); held["title"] != "old words" {
		t.Fatalf("stored values = %v before the move, want the title stored", held)
	}

	if _, err := s.Types.MoveField(
		t.Context(), title.ID, resting.ID, specs.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, one.ID); len(held) != 0 {
		t.Errorf("stored values = %v, want the title swept since no group serves it", held)
	}
}

// movingARestingGroupsSubFieldOutSweepsTheValueTheServedSectionLacks sweeps what the served section lacks.
func movingARestingGroupsSubFieldOutSweepsTheValueTheServedSectionLacks(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	DeclaredInside(t, s.Types, specs, "size", content.FieldKindText)
	twin := RestingTwinOf(t, s.Types, "specs")
	color := DeclaredInside(t, s.Types, twin, "color", content.FieldKindText)
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"specs": map[string]any{"color": "red", "size": "big"}})
	stored := ValuesOf(t, s.Content, one.ID)
	if !reflect.DeepEqual(stored, content.Values{"specs": map[string]any{"color": "red", "size": "big"}}) {
		t.Fatalf("stored values = %v before the move, want the color and the size stored", stored)
	}

	if _, err := s.Types.MoveField(
		t.Context(), color.ID, twin.GroupID, 0, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	held := ValuesOf(t, s.Content, one.ID)
	if !reflect.DeepEqual(held, content.Values{"specs": map[string]any{"size": "big"}}) {
		t.Errorf("stored values = %v, want the color swept since the served section lacks it", held)
	}
}

// movingAFieldOutOfAContainerItsGroupServesSweepsItWhereARivalHoldsTheContainer sweeps the moved sub field.
func movingAFieldOutOfAContainerItsGroupServesSweepsItWhereARivalHoldsTheContainer(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	everywhere := ServingEverything(t, s.Types)
	specs, err := s.Types.CreateFieldInGroup(t.Context(), everywhere.ID, SectionOn(t, "specs"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	color := DeclaredInside(t, s.Types, specs, "color", content.FieldKindText)
	RivalOnTruck(t, s.Types, "specs")
	one, _ := TypedHolding(t, s.Types, s.Content, "truck", "One", author,
		content.Values{"specs": map[string]any{"color": "red"}})

	if _, err := s.Types.MoveField(
		t.Context(), color.ID, everywhere.ID, 0, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	held := ValuesOf(t, s.Content, one.ID)
	if inside, standing := held["specs"].(map[string]any); len(held) != 1 || !standing || len(inside) != 0 {
		t.Errorf("stored values = %v, want the color swept from the specs the moving group serves", held)
	}
}

// movingAFieldItsGroupServesIntoAContainerSweepsItWhereARivalHoldsTheKey sweeps the field a rival never takes.
func movingAFieldItsGroupServesIntoAContainerSweepsItWhereARivalHoldsTheKey(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	everywhere := ServingEverything(t, s.Types)
	specs, err := s.Types.CreateFieldInGroup(t.Context(), everywhere.ID, SectionOn(t, "specs"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	title, err := s.Types.CreateFieldInGroup(
		t.Context(), everywhere.ID, FieldOn(t, "", "title", content.FieldKindText, ""), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(title) error = %v, want nil", err)
	}
	RivalOnTruck(t, s.Types, "title")
	one, _ := TypedHolding(t, s.Types, s.Content, "truck", "One", author, content.Values{"title": "served words"})

	if _, err := s.Types.MoveField(
		t.Context(), title.ID, everywhere.ID, specs.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, one.ID); len(held) != 0 {
		t.Errorf("stored values = %v, want the title swept, the rival section never taking it over", held)
	}
}

// movingALayoutBetweenFlexiblesTakesItsRowsAway takes a moved layout's rows out of the flexible it left.
func movingALayoutBetweenFlexiblesTakesItsRowsAway(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	features := DeclareFlexible(t, s.Types)
	blocks := flexibleIn(t, s.Types, features.GroupID, "blocks")
	hero := DeclareLayout(t, s.Types, features.ID, "hero")
	quote := DeclareLayout(t, s.Types, features.ID, "quote")
	DeclareUnder(t, s.Types, hero.ID, "title")
	DeclareUnder(t, s.Types, quote.ID, "title")
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author, heroAndQuote())

	if _, err := s.Types.MoveField(
		t.Context(), hero.ID, features.GroupID, blocks.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	expectHeld(t, t.Errorf, s.Content, one, revision, quoteAlone(), "the hero rows gone rather than emptied")
}
