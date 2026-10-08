// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// groupRecheckCases are the cases over the group writes that store nothing once the check they were handed refuses.
var groupRecheckCases = []Case{
	{"UpdateGroupStoresNothingItsCheckRefuses", updateGroupStoresNothingItsCheckRefuses},
	{"DeleteGroupStoresNothingItsCheckRefuses", deleteGroupStoresNothingItsCheckRefuses},
	{"DeleteFieldsOfGroupStoresNothingItsCheckRefuses", deleteFieldsOfGroupStoresNothingItsCheckRefuses},
	{"CreateFieldInGroupStoresNothingItsCheckRefuses", createFieldInGroupStoresNothingItsCheckRefuses},
	{"UpdateFieldInGroupStoresNothingItsCheckRefuses", updateFieldInGroupStoresNothingItsCheckRefuses},
	{"DeleteFieldInGroupStoresNothingItsCheckRefuses", deleteFieldInGroupStoresNothingItsCheckRefuses},
	{"DeleteSubFieldStoresNothingItsCheckRefuses", deleteSubFieldStoresNothingItsCheckRefuses},
	{"MoveFieldStoresNothingItsCheckRefuses", moveFieldStoresNothingItsCheckRefuses},
}

// errJudged stands for what a caller's check raises on fields that changed meanwhile.
var errJudged = errors.New("the fields changed meanwhile")

// judgedGroup holds a car group with a title at its top and a street inside an address section.
type judgedGroup struct {
	group                  content.Group
	title, address, street content.Field
}

// judgedOn stores the car type and the judged group, and returns the group.
func judgedOn(t *testing.T, types content.TypeStore) judgedGroup {
	t.Helper()
	StoreType(t, types, "car")
	group := GroupOn(t, types, "Details", "car")
	title := TitleIn(t, types, group.ID)
	address, err := types.CreateFieldInGroup(
		t.Context(), group.ID, FieldOn(t, "", "address", content.FieldKindSection, ""), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(address) error = %v, want nil", err)
	}
	street := DeclaredInside(t, types, address, "street", content.FieldKindText)
	return judgedGroup{group: group, title: title, address: address, street: street}
}

// storedGroups returns every group the store holds, with its fields.
func storedGroups(t *testing.T, types content.TypeStore) []content.Group {
	t.Helper()
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	return groups
}

// refusing returns a check that expects the stored details group and car type, then refuses the write.
func refusing(t *testing.T) content.Recheck {
	return func(groups []content.Group, types []content.Type) error {
		if !slices.ContainsFunc(groups, func(g content.Group) bool { return g.Key == "details" && len(g.Fields) == 2 }) {
			t.Error("the check was handed no details group holding its two fields")
		}
		if !slices.ContainsFunc(types, func(held content.Type) bool { return held.Key == "car" }) {
			t.Error("the check was handed no car type")
		}
		return errJudged
	}
}

// refusedWrite runs the write on the judged group with a refusing check and asserts it stored nothing.
func refusedWrite(t *testing.T, s Stores, write func(at judgedGroup, check content.Recheck) error) {
	t.Helper()
	at := judgedOn(t, s.Types)
	before := storedGroups(t, s.Types)

	err := write(at, refusing(t))

	if !errors.Is(err, errJudged) || err.Error() != errJudged.Error() {
		t.Errorf("error = %v, want %v handed back as the check raised it", err, errJudged)
	}
	if after := storedGroups(t, s.Types); !reflect.DeepEqual(after, before) {
		t.Errorf("the groups read %+v, want them left as %+v", after, before)
	}
}

// updateGroupStoresNothingItsCheckRefuses leaves the groups as they stood when its check refuses the edit.
func updateGroupStoresNothingItsCheckRefuses(t *testing.T, s Stores) {
	refusedWrite(t, s, func(at judgedGroup, check content.Recheck) error {
		at.group.Title = "Renamed"
		_, err := s.Types.UpdateGroup(t.Context(), at.group, nil, check)
		return err
	})
}

// deleteGroupStoresNothingItsCheckRefuses leaves the groups as they stood when its check refuses the removal.
func deleteGroupStoresNothingItsCheckRefuses(t *testing.T, s Stores) {
	refusedWrite(t, s, func(at judgedGroup, check content.Recheck) error {
		return s.Types.DeleteGroup(t.Context(), at.group.ID, check)
	})
}

// deleteFieldsOfGroupStoresNothingItsCheckRefuses leaves the groups as they stood when its check refuses the removal.
func deleteFieldsOfGroupStoresNothingItsCheckRefuses(t *testing.T, s Stores) {
	refusedWrite(t, s, func(at judgedGroup, check content.Recheck) error {
		return s.Types.DeleteFieldsOfGroup(t.Context(), at.group.ID, []string{"title"}, check)
	})
}

// createFieldInGroupStoresNothingItsCheckRefuses leaves the groups as they stood when its check refuses the field.
func createFieldInGroupStoresNothingItsCheckRefuses(t *testing.T, s Stores) {
	refusedWrite(t, s, func(at judgedGroup, check content.Recheck) error {
		subtitle := content.Field{Key: "subtitle", Label: "Subtitle", Kind: content.FieldKindText}
		_, err := s.Types.CreateFieldInGroup(t.Context(), at.group.ID, subtitle, check)
		return err
	})
}

// updateFieldInGroupStoresNothingItsCheckRefuses leaves the groups as they stood when its check refuses the edit.
func updateFieldInGroupStoresNothingItsCheckRefuses(t *testing.T, s Stores) {
	refusedWrite(t, s, func(at judgedGroup, check content.Recheck) error {
		edited := at.title
		edited.Label = "Renamed"
		_, err := s.Types.UpdateFieldInGroup(t.Context(), at.group.ID, edited, at.title.UpdatedAt, check)
		return err
	})
}

// deleteFieldInGroupStoresNothingItsCheckRefuses leaves the groups as they stood when its check refuses the removal.
func deleteFieldInGroupStoresNothingItsCheckRefuses(t *testing.T, s Stores) {
	refusedWrite(t, s, func(at judgedGroup, check content.Recheck) error {
		return s.Types.DeleteFieldInGroup(t.Context(), at.group.ID, "title", check)
	})
}

// deleteSubFieldStoresNothingItsCheckRefuses leaves the groups as they stood when its check refuses the removal.
func deleteSubFieldStoresNothingItsCheckRefuses(t *testing.T, s Stores) {
	refusedWrite(t, s, func(at judgedGroup, check content.Recheck) error {
		return s.Types.DeleteSubField(t.Context(), at.street.ID, check)
	})
}

// moveFieldStoresNothingItsCheckRefuses leaves the groups as they stood when its check refuses the move.
func moveFieldStoresNothingItsCheckRefuses(t *testing.T, s Stores) {
	refusedWrite(t, s, func(at judgedGroup, check content.Recheck) error {
		_, err := s.Types.MoveField(
			t.Context(), at.title.ID, at.group.ID, at.address.ID, content.DefaultFieldDepth, check,
		)
		return err
	})
}
