// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// subFieldWriteCases are the cases over editing a sub field and reordering the sub fields of a container.
var subFieldWriteCases = []Case{
	{"UpdatingASubFieldCarriesItsLabelRequiredAndSettings", updatingASubFieldCarriesItsLabelRequiredAndSettings},
	{"UpdatingASubFieldRefusesAStaleStamp", updatingASubFieldRefusesAStaleStamp},
	{"UpdatingASubFieldWillNotReachATopLevelField", updatingASubFieldWillNotReachATopLevelField},
	{"UpdatingASubFieldReportsOneThatIsGone", updatingASubFieldReportsOneThatIsGone},
	{"ReorderingInsideAContainerReportsOneThatIsGone", reorderingInsideAContainerReportsOneThatIsGone},
	{"ReorderingInsideAContainerAcceptsOneHoldingNoSubFields", reorderingInsideAContainerAcceptsOneHoldingNoSubFields},
	{"ReorderingInsideAContainerStandsTheSubFieldsAsAsked", reorderingInsideAContainerStandsTheSubFieldsAsAsked},
	{"ReorderingInsideAContainerLeavesAnotherContainerAlone", reorderingInsideAContainerLeavesAnotherContainerAlone},
}

// topFieldOf returns the field standing at the top of its group, failing the case when the group holds it elsewhere.
func topFieldOf(t *testing.T, types content.TypeStore, field content.Field) content.Field {
	t.Helper()
	for _, held := range GroupAt(t, types, field.GroupID).Fields {
		if held.ID == field.ID {
			return held
		}
	}
	t.Fatalf("group %d holds no field %d at its top", field.GroupID, field.ID)
	return content.Field{}
}

// placeOf returns where the field the identity names stands among the fields, counting from one, or zero when absent.
func placeOf(fields []content.Field, id int) int {
	for i, f := range fields {
		if f.ID == id {
			return i + 1
		}
	}
	return 0
}

// updatingASubFieldCarriesItsLabelRequiredAndSettings stores the label, required flag and settings an edit asks for.
func updatingASubFieldCarriesItsLabelRequiredAndSettings(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	sub, err := s.Types.CreateSubField(
		t.Context(), specs.ID, FieldOn(t, "", "title", content.FieldKindText, ""), content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	asked := sub
	asked.Label = "The title"
	asked.Required = true
	asked.Settings = map[string]any{"maxlength": float64(20)}

	held, err := s.Types.UpdateSubField(t.Context(), sub.ID, asked, sub.UpdatedAt)

	if err != nil {
		t.Fatalf("UpdateSubField() error = %v, want nil", err)
	}
	if held.Label != "The title" || !held.Required {
		t.Errorf("stored label %q required %v, want the edit carried", held.Label, held.Required)
	}
	if held.Settings["maxlength"] != float64(20) {
		t.Errorf("stored settings = %v, want maxlength carried", held.Settings)
	}
}

// updatingASubFieldRefusesAStaleStamp answers a conflict for a sub field edit holding a stamp from before the last one.
func updatingASubFieldRefusesAStaleStamp(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	sub, err := s.Types.CreateSubField(
		t.Context(), specs.ID, FieldOn(t, "", "title", content.FieldKindText, ""), content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	stamped := sub
	stamped.UpdatedAt = sub.UpdatedAt.Add(time.Second)
	if _, err := s.Types.UpdateSubField(t.Context(), sub.ID, stamped, sub.UpdatedAt); err != nil {
		t.Fatalf("the first update: %v, want nil", err)
	}

	_, err = s.Types.UpdateSubField(t.Context(), sub.ID, stamped, sub.UpdatedAt)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateSubField() error = %v, want %v", err, content.ErrConflict)
	}
}

// updatingASubFieldWillNotReachATopLevelField answers a conflict for a sub field edit naming a field at a group top.
func updatingASubFieldWillNotReachATopLevelField(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	title := DeclareTypedField(t, s.Types, "car", "title")

	_, err := s.Types.UpdateSubField(t.Context(), title.ID, title, title.UpdatedAt)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateSubField() on a top field: error = %v, want %v", err, content.ErrConflict)
	}
}

// updatingASubFieldReportsOneThatIsGone answers field not found for an edit naming a sub field that was deleted.
func updatingASubFieldReportsOneThatIsGone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	sub, err := s.Types.CreateSubField(
		t.Context(), specs.ID, FieldOn(t, "", "title", content.FieldKindText, ""), content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	if err := s.Types.DeleteSubField(t.Context(), sub.ID, nil); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}
	edited := sub
	edited.UpdatedAt = sub.UpdatedAt.Add(time.Second)

	_, err = s.Types.UpdateSubField(t.Context(), sub.ID, edited, sub.UpdatedAt)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("UpdateSubField() on a deleted sub field: error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

// reorderingInsideAContainerStandsTheSubFieldsAsAsked serves the sub fields of a container in the order asked.
func reorderingInsideAContainerStandsTheSubFieldsAsAsked(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	held := map[string]int{}
	for _, key := range []string{"title", "colour", "trim"} {
		stored, err := s.Types.CreateSubField(
			t.Context(), specs.ID, FieldOn(t, "", key, content.FieldKindText, ""), content.DefaultFieldDepth)
		if err != nil {
			t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
		}
		held[key] = stored.ID
	}

	if err := s.Types.ReorderSubFields(t.Context(), specs.ID, []string{"trim", "title", "colour"}); err != nil {
		t.Fatalf("ReorderSubFields() error = %v, want nil", err)
	}

	inside := topFieldOf(t, s.Types, specs).Fields
	if len(inside) != 3 {
		t.Fatalf("specs holds %d sub fields, want the three declared inside it", len(inside))
	}
	for key, want := range map[string]int{"trim": 1, "title": 2, "colour": 3} {
		if stood := placeOf(inside, held[key]); stood != want {
			t.Errorf("%s sits at %d, want %d", key, stood, want)
		}
	}
}

// reorderingInsideAContainerLeavesAnotherContainerAlone keeps the order of the sub fields another container holds.
func reorderingInsideAContainerLeavesAnotherContainerAlone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	extras := DeclareSection(t, s.Types, "extras")
	for _, key := range []string{"title", "colour"} {
		if _, err := s.Types.CreateSubField(
			t.Context(), specs.ID, FieldOn(t, "", key, content.FieldKindText, ""), content.DefaultFieldDepth); err != nil {
			t.Fatalf("declaring %s inside specs: %v, want nil", key, err)
		}
	}
	for _, key := range []string{"title", "colour"} {
		if _, err := s.Types.CreateSubField(
			t.Context(), extras.ID, FieldOn(t, "", key, content.FieldKindText, ""), content.DefaultFieldDepth); err != nil {
			t.Fatalf("declaring %s inside extras: %v, want nil", key, err)
		}
	}

	if err := s.Types.ReorderSubFields(t.Context(), specs.ID, []string{"colour", "title"}); err != nil {
		t.Fatalf("ReorderSubFields() error = %v, want nil", err)
	}

	away := topFieldOf(t, s.Types, extras).Fields
	if len(away) != 2 {
		t.Fatalf("extras holds %d sub fields, want the two declared inside it", len(away))
	}
	if away[0].Key != "title" || away[1].Key != "colour" {
		t.Errorf("the sub fields elsewhere stand %q then %q, want them left as title then colour",
			away[0].Key, away[1].Key)
	}
}

// reorderingInsideAContainerReportsOneThatIsGone answers field not found for an order inside a field nothing holds.
func reorderingInsideAContainerReportsOneThatIsGone(t *testing.T, s Stores) {
	err := s.Types.ReorderSubFields(t.Context(), 4242, []string{"title"})

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("ReorderSubFields() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

// reorderingInsideAContainerAcceptsOneHoldingNoSubFields takes an empty order for a container holding no sub fields.
func reorderingInsideAContainerAcceptsOneHoldingNoSubFields(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")

	err := s.Types.ReorderSubFields(t.Context(), specs.ID, nil)

	if err != nil {
		t.Errorf("ReorderSubFields() on a container holding no sub fields: error = %v, want nil", err)
	}
}
