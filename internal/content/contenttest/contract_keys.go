// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// contractKeyCases are the cases over refusing a field key already taken in its group, a rival group or a container.
var contractKeyCases = []Case{
	{"CreateFieldInGroupRefusesAKeyTheGroupHolds", createFieldInGroupRefusesAKeyTheGroupHolds},
	{"CreateFieldInGroupRefusesAKeyARivalGroupServes", createFieldInGroupRefusesAKeyARivalGroupServes},
	{"CreateSubFieldRefusesAKeyItsParentHolds", createSubFieldRefusesAKeyItsParentHolds},
}

// holdsOnly fails the case unless the fields hold the kept field and nothing else.
func holdsOnly(t *testing.T, where string, fields []content.Field, kept content.Field) {
	t.Helper()
	if len(fields) != 1 || fields[0].ID != kept.ID {
		t.Errorf("%s holds %+v, want the first %q alone", where, fields, kept.Key)
	}
}

// createFieldInGroupRefusesAKeyTheGroupHolds answers field taken for a second field under a key the group declares.
func createFieldInGroupRefusesAKeyTheGroupHolds(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	first := DeclareTypedField(t, s.Types, "car", "subtitle")

	_, err := s.Types.CreateFieldInGroup(
		t.Context(), first.GroupID, FieldOn(t, "", "subtitle", content.FieldKindText, ""), nil)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("CreateFieldInGroup() error = %v, want %v", err, content.ErrFieldTaken)
	}
	holdsOnly(t, "the group", GroupAt(t, s.Types, first.GroupID).Fields, first)
}

// createFieldInGroupRefusesAKeyARivalGroupServes answers field taken for a key a rival group serves on a shared type.
func createFieldInGroupRefusesAKeyARivalGroupServes(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	first := DeclareTypedField(t, s.Types, "car", "subtitle")
	everywhere := ServingEverything(t, s.Types)
	books := GroupOn(t, s.Types, "Books", "book")

	_, err := s.Types.CreateFieldInGroup(
		t.Context(), everywhere.ID, FieldOn(t, "", "subtitle", content.FieldKindText, ""), nil)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("CreateFieldInGroup(Everywhere) error = %v, want %v", err, content.ErrFieldTaken)
	}
	if held := GroupAt(t, s.Types, everywhere.ID).Fields; len(held) != 0 {
		t.Errorf("Everywhere holds %+v, want the refused field stored nowhere", held)
	}
	holdsOnly(t, "the car group", GroupAt(t, s.Types, first.GroupID).Fields, first)
	car, err := s.Types.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey(car) error = %v, want nil", err)
	}
	holdsOnly(t, "the car type", car.Fields, first)
	if _, err := s.Types.CreateFieldInGroup(
		t.Context(), books.ID, FieldOn(t, "", "subtitle", content.FieldKindText, ""), nil,
	); err != nil {
		t.Errorf("CreateFieldInGroup(Books) error = %v, want nil with no type shared with the car group", err)
	}
}

// createSubFieldRefusesAKeyItsParentHolds answers field taken for a key the container holds and takes it elsewhere.
func createSubFieldRefusesAKeyItsParentHolds(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	extras := DeclareSection(t, s.Types, "extras")
	first := DeclaredInside(t, s.Types, specs, "title", content.FieldKindText)

	_, err := s.Types.CreateSubField(
		t.Context(), specs.ID, FieldOn(t, "", "title", content.FieldKindText, ""), content.DefaultFieldDepth)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("CreateSubField(under specs) error = %v, want %v", err, content.ErrFieldTaken)
	}
	other, err := s.Types.CreateSubField(
		t.Context(), extras.ID, FieldOn(t, "", "title", content.FieldKindText, ""), content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("CreateSubField(under extras) error = %v, want nil with the key free there", err)
	}
	held := GroupAt(t, s.Types, specs.GroupID)
	holdsOnly(t, "specs", subFieldsOf(held, "specs"), first)
	holdsOnly(t, "extras", subFieldsOf(held, "extras"), other)
}
