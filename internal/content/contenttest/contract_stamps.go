// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// contractStampCases are the cases over refusing an edit holding a stamp a move, an adoption or a carry left behind.
var contractStampCases = []Case{
	{"AFieldEditHoldingTheStampFromBeforeAMoveConflicts", aFieldEditHoldingTheStampFromBeforeAMoveConflicts},
	{"AFieldEditHoldingTheStampFromBeforeAnAdoptionConflicts", aFieldEditHoldingTheStampFromBeforeAnAdoptionConflicts},
	{"AdoptGroupClearsTheOriginOfItsSubFields", adoptGroupClearsTheOriginOfItsSubFields},
	{"AnItemEditHoldingTheStampFromBeforeACarryConflicts", anItemEditHoldingTheStampFromBeforeACarryConflicts},
}

// fieldNamed returns the field the identity names among the fields or anywhere below them, and whether one does.
func fieldNamed(fields []content.Field, id int) (content.Field, bool) {
	for _, f := range fields {
		if f.ID == id {
			return f, true
		}
		if held, found := fieldNamed(f.Fields, id); found {
			return held, true
		}
	}
	return content.Field{}, false
}

// storedField returns the field the identity names at any depth of the group, as the store serves it now.
func storedField(t *testing.T, types content.TypeStore, groupID, id int) content.Field {
	t.Helper()
	if held, found := fieldNamed(GroupAt(t, types, groupID).Fields, id); found {
		return held
	}
	t.Fatalf("group %d holds no field %d at any depth", groupID, id)
	return content.Field{}
}

// relabelled returns the field under a new label, stamped now.
func relabelled(f content.Field) content.Field {
	f.Label = "Edited " + f.Key
	f.UpdatedAt = time.Now().UTC()
	return f
}

// subFieldEdit returns an edit relabelling the sub field while holding the expected stamp.
func subFieldEdit(t *testing.T, types content.TypeStore, f content.Field) func(time.Time) error {
	return func(expected time.Time) error {
		_, err := types.UpdateSubField(t.Context(), f.ID, relabelled(f), expected)
		return err
	}
}

// topFieldEdit returns an edit relabelling the field at the top of the group while holding the expected stamp.
func topFieldEdit(t *testing.T, types content.TypeStore, groupID int, f content.Field) func(time.Time) error {
	return func(expected time.Time) error {
		_, err := types.UpdateFieldInGroup(t.Context(), groupID, relabelled(f), expected, nil)
		return err
	}
}

// staleThenServed fails the case unless the edit conflicts holding the stale stamp and lands holding the served one.
func staleThenServed(t *testing.T, what string, stale, served time.Time, edit func(time.Time) error) {
	t.Helper()
	if err := edit(stale); !errors.Is(err, content.ErrConflict) {
		t.Errorf("%s holding the stamp from before: error = %v, want %v", what, err, content.ErrConflict)
	}
	if err := edit(served); err != nil {
		t.Errorf("%s holding the stamp the store serves now: error = %v, want nil", what, err)
	}
}

// aFieldEditHoldingTheStampFromBeforeAMoveConflicts turns away an edit holding the stamp a field had before it moved.
func aFieldEditHoldingTheStampFromBeforeAMoveConflicts(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	title := DeclareTypedField(t, s.Types, "car", "title")

	if _, err := s.Types.MoveField(
		t.Context(), title.ID, specs.GroupID, specs.ID, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField(into specs) error = %v, want nil", err)
	}

	inside := storedField(t, s.Types, specs.GroupID, title.ID)
	staleThenServed(t, "UpdateSubField() after the move into specs", title.UpdatedAt, inside.UpdatedAt,
		subFieldEdit(t, s.Types, title))
	edited := storedField(t, s.Types, specs.GroupID, title.ID)
	if _, err := s.Types.MoveField(
		t.Context(), title.ID, specs.GroupID, 0, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField(to the top) error = %v, want nil", err)
	}
	top := storedField(t, s.Types, specs.GroupID, title.ID)
	staleThenServed(t, "UpdateFieldInGroup() after the move to the top", edited.UpdatedAt, top.UpdatedAt,
		topFieldEdit(t, s.Types, specs.GroupID, title))
}

// aFieldEditHoldingTheStampFromBeforeAnAdoptionConflicts turns away an edit holding a stamp from before the adoption.
func aFieldEditHoldingTheStampFromBeforeAnAdoptionConflicts(t *testing.T, s Stores) {
	registry := content.NewRegistry(s.Types)
	section, sub := DeclaredSectionInto(t, registry)
	sectionBefore := storedField(t, s.Types, section.GroupID, section.ID)
	subBefore := storedField(t, s.Types, section.GroupID, sub.ID)

	if err := registry.AdoptGroup(t.Context(), GroupAt(t, s.Types, section.GroupID).Key); err != nil {
		t.Fatalf("AdoptGroup() error = %v, want nil", err)
	}

	adopted := storedField(t, s.Types, section.GroupID, section.ID)
	staleThenServed(t, "UpdateFieldInGroup() on the adopted section", sectionBefore.UpdatedAt, adopted.UpdatedAt,
		topFieldEdit(t, s.Types, section.GroupID, section))
	inside := storedField(t, s.Types, section.GroupID, sub.ID)
	staleThenServed(t, "UpdateSubField() on the adopted sub field", subBefore.UpdatedAt, inside.UpdatedAt,
		subFieldEdit(t, s.Types, sub))
}

// adoptGroupClearsTheOriginOfItsSubFields takes the fields inside a plugin's container over with its group.
func adoptGroupClearsTheOriginOfItsSubFields(t *testing.T, s Stores) {
	registry := content.NewRegistry(s.Types)
	section, sub := DeclaredSectionInto(t, registry)
	if declared := storedField(t, s.Types, section.GroupID, sub.ID); declared.Origin != "events" {
		t.Fatalf("the sub field names %q as its origin, want the plugin declaring it", declared.Origin)
	}

	if err := registry.AdoptGroup(t.Context(), GroupAt(t, s.Types, section.GroupID).Key); err != nil {
		t.Fatalf("AdoptGroup() error = %v, want nil", err)
	}

	inside := subFieldsOf(GroupAt(t, s.Types, section.GroupID), section.Key)
	if len(inside) != 1 {
		t.Fatalf("the section holds %d sub fields, want the one declared inside it", len(inside))
	}
	if inside[0].ID != sub.ID || inside[0].Origin != "" {
		t.Errorf("the sub field = %+v, want it taken over with its group", inside[0])
	}
}

// anItemEditHoldingTheStampFromBeforeACarryConflicts turns away an item edit holding a stamp from before a carry.
func anItemEditHoldingTheStampFromBeforeACarryConflicts(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	cars, err := s.Types.Create(t.Context(), CarType(t))
	if err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	item := StoreItem(t, s.Content, cars, nil, "Ford Focus", author)
	cars.RouteWord, cars.UpdatedAt = "autos", time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), cars); err != nil {
		t.Fatalf("Update(car) error = %v, want nil", err)
	}

	carried, err := s.Content.ByID(t.Context(), item.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	_, err = s.Content.Update(t.Context(), EditTitle(carried, "Ford Fiesta"), item.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("Update() holding the stamp from before the carry: error = %v, want %v", err, content.ErrConflict)
	}
	if got := AddressOf(t, s.Content, item.ID); got != "autos/ford-focus" {
		t.Errorf("address = %q, want the carried address kept", got)
	}
	if _, err := s.Content.Update(
		t.Context(), EditTitle(carried, "Ford Fiesta"), carried.UpdatedAt, nil, 0,
	); err != nil {
		t.Errorf("Update() holding the stamp the store serves now: error = %v, want nil", err)
	}
}
