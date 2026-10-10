// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// contractStampCases are the cases over the stamps a store writes and the edits holding a stamp it left behind.
var contractStampCases = []Case{
	{"AFieldEditHoldingTheStampFromBeforeAMoveConflicts", aFieldEditHoldingTheStampFromBeforeAMoveConflicts},
	{"AFieldEditHoldingTheStampFromBeforeAnAdoptionConflicts", aFieldEditHoldingTheStampFromBeforeAnAdoptionConflicts},
	{"AdoptGroupClearsTheOriginOfItsSubFields", adoptGroupClearsTheOriginOfItsSubFields},
	{"AnItemEditHoldingTheStampFromBeforeACarryConflicts", anItemEditHoldingTheStampFromBeforeACarryConflicts},
	{"CreateGroupAndUpdateGroupStampTheGroup", createGroupAndUpdateGroupStampTheGroup},
	{"UpdateFieldInGroupStoresTheStampTheEditSends", updateFieldInGroupStoresTheStampTheEditSends},
	{"AMoveLeavesTheStampsOfTheFieldsInsideTheMovedOne", aMoveLeavesTheStampsOfTheFieldsInsideTheMovedOne},
	{"AGroupEditLeavesTheStampsOfItsFieldsAlone", aGroupEditLeavesTheStampsOfItsFieldsAlone},
	{"ANestedItemEditHoldingTheStampFromBeforeACarryConflicts", aNestedItemEditHoldingTheStampFromBeforeACarryConflicts},
	{"AHandOverStampsTheDemotedTypeAndItsItemsWithTheEditStamp", aHandOverStampsTheDemotedTypeAndItsItemsWithTheEditStamp},
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

// createGroupAndUpdateGroupStampTheGroup stamps a new group as created now and moves only its update stamp on an edit.
func createGroupAndUpdateGroupStampTheGroup(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	created := GroupOn(t, s.Types, "Extras", "car")
	if created.CreatedAt.IsZero() || !created.UpdatedAt.Equal(created.CreatedAt) {
		t.Fatalf("CreateGroup() stamps = %v and %v, want both set at creation", created.CreatedAt, created.UpdatedAt)
	}
	edited := created
	edited.Title = "Extras renamed"
	edited.CreatedAt = created.CreatedAt.Add(time.Hour)

	updated, err := s.Types.UpdateGroup(t.Context(), edited, nil, nil)

	if err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}
	for what, held := range map[string]content.Group{
		"UpdateGroup()": updated, "the stored group": GroupAt(t, s.Types, created.ID),
	} {
		if !held.CreatedAt.Equal(created.CreatedAt) {
			t.Errorf("%s creation stamp = %v, want %v kept", what, held.CreatedAt, created.CreatedAt)
		}
		if !held.UpdatedAt.After(created.UpdatedAt) {
			t.Errorf("%s update stamp = %v, want it moved past %v", what, held.UpdatedAt, created.UpdatedAt)
		}
	}
}

// updateFieldInGroupStoresTheStampTheEditSends turns away a later edit holding the stamp from before the first one.
func updateFieldInGroupStoresTheStampTheEditSends(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	title := DeclareTypedField(t, s.Types, "car", "title")

	if _, err := s.Types.UpdateFieldInGroup(
		t.Context(), title.GroupID, relabelled(title), title.UpdatedAt, nil,
	); err != nil {
		t.Fatalf("UpdateFieldInGroup() error = %v, want nil", err)
	}

	served := storedField(t, s.Types, title.GroupID, title.ID)
	staleThenServed(t, "UpdateFieldInGroup() after an edit", title.UpdatedAt, served.UpdatedAt,
		topFieldEdit(t, s.Types, title.GroupID, served))
}

// aMoveLeavesTheStampsOfTheFieldsInsideTheMovedOne lets an edit of a sub field land after its container moved.
func aMoveLeavesTheStampsOfTheFieldsInsideTheMovedOne(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	doors := DeclareUnder(t, s.Types, specs.ID, "doors")
	extras := GroupOn(t, s.Types, "Extras", "car")
	before := storedField(t, s.Types, specs.GroupID, doors.ID)

	if _, err := s.Types.MoveField(
		t.Context(), specs.ID, extras.ID, 0, content.DefaultFieldDepth, nil,
	); err != nil {
		t.Fatalf("MoveField(specs to extras) error = %v, want nil", err)
	}

	if _, err := s.Types.UpdateSubField(t.Context(), doors.ID, relabelled(doors), before.UpdatedAt); err != nil {
		t.Errorf("UpdateSubField() holding the stamp from before its container moved: error = %v, want nil", err)
	}
}

// aGroupEditLeavesTheStampsOfItsFieldsAlone lets a field edit land after its group was renamed.
func aGroupEditLeavesTheStampsOfItsFieldsAlone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	title := DeclareTypedField(t, s.Types, "car", "title")
	before := storedField(t, s.Types, title.GroupID, title.ID)
	renamed := GroupAt(t, s.Types, title.GroupID)
	renamed.Title = "Car details"

	if _, err := s.Types.UpdateGroup(t.Context(), renamed, nil, nil); err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}

	if _, err := s.Types.UpdateFieldInGroup(
		t.Context(), title.GroupID, relabelled(title), before.UpdatedAt, nil,
	); err != nil {
		t.Errorf("UpdateFieldInGroup() holding the stamp from before the group edit: error = %v, want nil", err)
	}
}

// aNestedItemEditHoldingTheStampFromBeforeACarryConflicts turns away an edit of a nested item a carry stamped.
func aNestedItemEditHoldingTheStampFromBeforeACarryConflicts(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	about := MustNest(t, s.Content, nil, "About", author)
	team := MustNest(t, s.Content, &about, "Team", author)
	pages, err := s.Types.ByKey(t.Context(), "page")
	if err != nil {
		t.Fatalf("ByKey(page) error = %v, want nil", err)
	}
	pages.RouteWord, pages.UpdatedAt = "docs", time.Now().UTC()
	if _, err := s.Types.Update(t.Context(), pages); err != nil {
		t.Fatalf("Update(page) error = %v, want nil", err)
	}
	carried, err := s.Content.ByID(t.Context(), team.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}

	_, err = s.Content.Update(t.Context(), EditTitle(carried, "Crew"), team.UpdatedAt, nil, 0)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("Update() of the nested page holding the stamp from before the carry: error = %v, want %v",
			err, content.ErrConflict)
	}
	if _, err := s.Content.Update(t.Context(), EditTitle(carried, "Crew"), carried.UpdatedAt, nil, 0); err != nil {
		t.Errorf("Update() of the nested page holding the stamp the store serves now: error = %v, want nil", err)
	}
}

// aHandOverStampsTheDemotedTypeAndItsItemsWithTheEditStamp gives the demoted type and each carried item the edit stamp.
func aHandOverStampsTheDemotedTypeAndItsItemsWithTheEditStamp(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	post := StoreItem(t, s.Content, PostType(), nil, "Hello World", author)
	about := MustNest(t, s.Content, nil, "About", author)
	at := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	promoted := PageType()
	promoted.Default, promoted.UpdatedAt = true, at

	if _, err := s.Types.Update(t.Context(), promoted); err != nil {
		t.Fatalf("Update() handing over the root error = %v, want nil", err)
	}

	demoted, err := s.Types.ByKey(t.Context(), content.TypePost)
	if err != nil || !demoted.UpdatedAt.Equal(at) {
		t.Errorf("the demoted post type stamp = %v, %v, want the edit stamp %v", demoted.UpdatedAt, err, at)
	}
	for _, item := range []content.Content{post, about} {
		carried, err := s.Content.ByID(t.Context(), item.ID)
		if err != nil || !carried.UpdatedAt.Equal(at) {
			t.Errorf("the carried %q stamp = %v, %v, want the edit stamp %v", item.Title, carried.UpdatedAt, err, at)
		}
	}
}
