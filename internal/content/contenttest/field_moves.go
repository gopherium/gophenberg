// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// fieldMoveCases are the cases over moving a field into, out of and between containers and group tops.
var fieldMoveCases = []Case{
	{"MovingAFieldIntoAContainerReparentsIt", movingAFieldIntoAContainerReparentsIt},
	{"MovingAContainerRecountsTheDepthOfEverythingBelowIt", movingAContainerRecountsTheDepthOfEverythingBelowIt},
	{"MovingAFieldOutOfAContainerStandsItAfterTheTopFields", movingAFieldOutOfAContainerStandsItAfterTheTopFields},
	{"MovingAFieldOntoAKeyTheContainerHoldsReportsFieldTaken", movingAFieldOntoAKeyTheContainerHoldsReportsFieldTaken},
	{"MovingAFieldOntoAKeyTheTopHoldsReportsFieldTaken", movingAFieldOntoAKeyTheTopHoldsReportsFieldTaken},
	{
		"MovingAFieldOutToAKeyARivalGroupServesReportsFieldTaken",
		movingAFieldOutToAKeyARivalGroupServesReportsFieldTaken,
	},
	{"MovingAContainerIntoItsOwnTreeIsRefusedByTheStore", movingAContainerIntoItsOwnTreeIsRefusedByTheStore},
	{"MovingAFieldReportsAParentTheLandingGroupDoesNotHold", movingAFieldReportsAParentTheLandingGroupDoesNotHold},
}

// chainedAt fails the case unless each named field stands directly inside the one before, the first at the group top.
func chainedAt(t *testing.T, types content.TypeStore, groupID int, ids ...int) {
	t.Helper()
	fields := GroupAt(t, types, groupID).Fields
	for _, id := range ids {
		at := slices.IndexFunc(fields, func(f content.Field) bool { return f.ID == id })
		if at < 0 {
			t.Fatalf("field %d stands nowhere among %+v, want it directly inside the field before it", id, fields)
		}
		fields = fields[at].Fields
	}
}

// movingAFieldIntoAContainerReparentsIt moves a top field into a section and serves it under the section.
func movingAFieldIntoAContainerReparentsIt(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	title := DeclareTypedField(t, s.Types, "car", "title")

	moved, err := s.Types.MoveField(t.Context(), title.ID, specs.GroupID, specs.ID, content.DefaultFieldDepth, nil)

	if err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}
	if moved.ID != title.ID || moved.ParentID != specs.ID || moved.GroupID != specs.GroupID {
		t.Errorf("MoveField() = %+v, want the same row standing under the section", moved)
	}
	top := GroupAt(t, s.Types, specs.GroupID).Fields
	if len(top) != 1 || top[0].ID != specs.ID {
		t.Fatalf("the group holds %+v at its top, want specs alone", top)
	}
	if len(top[0].Fields) != 1 || top[0].Fields[0].ID != title.ID {
		t.Errorf("specs holds %+v, want the title standing under it", top[0].Fields)
	}
}

// movingAContainerRecountsTheDepthOfEverythingBelowIt carries the depth of every field below a moved section.
func movingAContainerRecountsTheDepthOfEverythingBelowIt(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	inner := DeclaredInside(t, s.Types, specs, "inner", content.FieldKindSection)
	leaf := DeclaredInside(t, s.Types, inner, "leaf", content.FieldKindText)
	box := DeclareSection(t, s.Types, "box")

	if _, err := s.Types.MoveField(
		t.Context(), specs.ID, box.GroupID, box.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField(into box) error = %v, want nil", err)
	}
	chainedAt(t, s.Types, box.GroupID, box.ID, specs.ID, inner.ID, leaf.ID)
	probe := FieldOn(t, "", "probe", content.FieldKindText, "")
	if _, err := s.Types.CreateSubField(t.Context(), inner.ID, probe, 2); !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("CreateSubField(under inner, limit 2) error = %v, want %v with inner two levels down",
			err, content.ErrFieldTooDeep)
	}
	if _, err := s.Types.CreateSubField(t.Context(), inner.ID, probe, 3); err != nil {
		t.Errorf("CreateSubField(under inner, limit 3) error = %v, want nil with inner two levels down", err)
	}

	if _, err := s.Types.MoveField(
		t.Context(), specs.ID, box.GroupID, 0, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField(to the top) error = %v, want nil", err)
	}
	chainedAt(t, s.Types, box.GroupID, specs.ID, inner.ID, leaf.ID)
	again := FieldOn(t, "", "again", content.FieldKindText, "")
	if _, err := s.Types.CreateSubField(t.Context(), inner.ID, again, 2); err != nil {
		t.Errorf("CreateSubField(under inner, limit 2) error = %v, want nil with inner one level down", err)
	}
}

// movingAFieldOutOfAContainerStandsItAfterTheTopFields moves a sub field to the top, after every top field.
func movingAFieldOutOfAContainerStandsItAfterTheTopFields(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	doors := DeclaredInside(t, s.Types, specs, "doors", content.FieldKindText)
	title := DeclareTypedField(t, s.Types, "car", "title")

	moved, err := s.Types.MoveField(t.Context(), doors.ID, specs.GroupID, 0, content.DefaultFieldDepth, nil)

	if err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}
	if moved.ParentID != 0 {
		t.Errorf("MoveField() = %+v, want the field standing at the top", moved)
	}
	top := GroupAt(t, s.Types, specs.GroupID).Fields
	if len(top) != 3 {
		t.Fatalf("the group holds %+v at its top, want specs, title and doors", top)
	}
	if top[0].ID != specs.ID || top[1].ID != title.ID || top[2].ID != doors.ID {
		t.Errorf("the group holds %q, %q, %q at its top, want specs, title, doors", top[0].Key, top[1].Key, top[2].Key)
	}
	if len(top[0].Fields) != 0 {
		t.Errorf("specs holds %+v, want doors moved out of it", top[0].Fields)
	}
}

// movingAFieldOntoAKeyTheContainerHoldsReportsFieldTaken refuses a move into a section already holding the key.
func movingAFieldOntoAKeyTheContainerHoldsReportsFieldTaken(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	DeclaredInside(t, s.Types, specs, "title", content.FieldKindText)
	title := DeclareTypedField(t, s.Types, "car", "title")

	_, err := s.Types.MoveField(t.Context(), title.ID, specs.GroupID, specs.ID, content.DefaultFieldDepth, nil)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

// movingAFieldOntoAKeyTheTopHoldsReportsFieldTaken refuses a move to a group top already holding the key.
func movingAFieldOntoAKeyTheTopHoldsReportsFieldTaken(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	inside := DeclaredInside(t, s.Types, specs, "title", content.FieldKindText)
	DeclareTypedField(t, s.Types, "car", "title")

	_, err := s.Types.MoveField(t.Context(), inside.ID, specs.GroupID, 0, content.DefaultFieldDepth, nil)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

// movingAFieldOutToAKeyARivalGroupServesReportsFieldTaken refuses a move to the top when a rival group serves the key.
func movingAFieldOutToAKeyARivalGroupServesReportsFieldTaken(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	inside := DeclaredInside(t, s.Types, specs, "title", content.FieldKindText)
	extras, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := s.Types.CreateFieldInGroup(
		t.Context(), extras.ID, FieldOn(t, "", "title", content.FieldKindText, ""), nil); err != nil {
		t.Fatalf("CreateFieldInGroup(title) error = %v, want nil", err)
	}

	_, err = s.Types.MoveField(t.Context(), inside.ID, specs.GroupID, 0, content.DefaultFieldDepth, nil)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

// movingAContainerIntoItsOwnTreeIsRefusedByTheStore refuses a section moved into itself or a container inside it.
func movingAContainerIntoItsOwnTreeIsRefusedByTheStore(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	inner := DeclaredInside(t, s.Types, specs, "inner", content.FieldKindSection)

	for name, parent := range map[string]int{"itself": specs.ID, "a container inside it": inner.ID} {
		_, err := s.Types.MoveField(t.Context(), specs.ID, specs.GroupID, parent, content.DefaultFieldDepth, nil)

		if !errors.Is(err, content.ErrFieldInsideItself) {
			t.Errorf("moving into %s: error = %v, want %v", name, err, content.ErrFieldInsideItself)
		}
	}
}

// movingAFieldReportsAParentTheLandingGroupDoesNotHold answers field not found for a parent outside the landing group.
func movingAFieldReportsAParentTheLandingGroupDoesNotHold(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	title := DeclareTypedField(t, s.Types, "car", "title")
	extras, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	_, err = s.Types.MoveField(t.Context(), title.ID, extras.ID, specs.ID, content.DefaultFieldDepth, nil)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}
