// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"reflect"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// groupValueCases are the cases over the values a group write sweeps from items, revisions and autosaves or keeps.
var groupValueCases = []Case{
	{"DeleteGroupSweepsAValueLeftOnATypeItStoppedMatching", deleteGroupSweepsAValueLeftOnATypeItStoppedMatching},
	{
		"DeleteFieldsOfGroupSweepsAValueLeftOnATypeItStoppedMatching",
		deleteFieldsOfGroupSweepsAValueLeftOnATypeItStoppedMatching,
	},
	{"DeleteGroupSparesAValueAnotherGroupStillServes", deleteGroupSparesAValueAnotherGroupStillServes},
	{"DeleteFieldInGroupSweepsTheValuesOfItsMatchedTypes", deleteFieldInGroupSweepsTheValuesOfItsMatchedTypes},
	{
		"DeletingARestingGroupsFieldKeepsTheValueTheServedFieldHolds",
		deletingARestingGroupsFieldKeepsTheValueTheServedFieldHolds,
	},
	{
		"DeletingAShadowedGroupsFieldKeepsTheValueTheServingGroupHolds",
		deletingAShadowedGroupsFieldKeepsTheValueTheServingGroupHolds,
	},
	{"DeletingARestingGroupsFieldSweepsTheValuesNoGroupServes", deletingARestingGroupsFieldSweepsTheValuesNoGroupServes},
	{"MoveFieldCarriesTheFieldAndKeepsItsValues", moveFieldCarriesTheFieldAndKeepsItsValues},
	{"DeleteGroupTakesItsFieldsAndValuesInOneSweep", deleteGroupTakesItsFieldsAndValuesInOneSweep},
	{"ReorderingTheTopLeavesASubFieldSharingAKeyWhereItStands", reorderingTheTopLeavesASubFieldSharingAKeyWhereItStands},
}

// deleteGroupSweepsAValueLeftOnATypeItStoppedMatching sweeps the value a deleted group left on a type it moved off.
func deleteGroupSweepsAValueLeftOnATypeItStoppedMatching(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	group, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := s.Types.CreateFieldInGroup(
		t.Context(), group.ID, FieldOn(t, "", "subtitle", content.FieldKindText, ""),
		nil,
	); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	left, _ := TypedHolding(t, s.Types, s.Content, "car", "Left Behind", author,
		content.Values{"subtitle": "must go"})
	group.Location = LocationOf("book")
	if _, err := s.Types.UpdateGroup(t.Context(), group, nil, nil); err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}

	if err := s.Types.DeleteGroup(t.Context(), group.ID, nil); err != nil {
		t.Fatalf("DeleteGroup() error = %v, want nil", err)
	}

	held := ValuesOf(t, s.Content, left.ID)
	if _, kept := held["subtitle"]; kept {
		t.Errorf("fields = %v, want the value gone with the group that declared it", held)
	}
}

// deleteFieldsOfGroupSweepsAValueLeftOnATypeItStoppedMatching sweeps only the named field's value off a former type.
func deleteFieldsOfGroupSweepsAValueLeftOnATypeItStoppedMatching(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	group, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	for _, key := range []string{"subtitle", "footnote"} {
		if _, err := s.Types.CreateFieldInGroup(
			t.Context(), group.ID, FieldOn(t, "", key, content.FieldKindText, ""),
			nil,
		); err != nil {
			t.Fatalf("CreateFieldInGroup(%s) error = %v, want nil", key, err)
		}
	}
	left, _ := TypedHolding(t, s.Types, s.Content, "car", "Left Behind", author,
		content.Values{"subtitle": "must go", "footnote": "stays"})
	group.Location = LocationOf("book")
	if _, err := s.Types.UpdateGroup(t.Context(), group, nil, nil); err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}

	if err := s.Types.DeleteFieldsOfGroup(t.Context(), group.ID, []string{"subtitle"}, nil); err != nil {
		t.Fatalf("DeleteFieldsOfGroup() error = %v, want nil", err)
	}

	held := ValuesOf(t, s.Content, left.ID)
	if held["subtitle"] == "must go" || held["footnote"] != "stays" {
		t.Errorf("fields = %v, want the named field's value gone and the other one left", held)
	}
	if id := GroupHolding(t, s.Types, "footnote"); id != group.ID {
		t.Errorf("the footnote stands in the group %d, want the group kept with it", id)
	}
}

// deleteGroupSparesAValueAnotherGroupStillServes keeps the value a second group still serves on the item's type.
func deleteGroupSparesAValueAnotherGroupStillServes(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	holding := func(title, typeKey string) content.Group {
		t.Helper()
		group, err := s.Types.CreateGroup(t.Context(), content.Group{Title: title, Location: LocationOf(typeKey)})
		if err != nil {
			t.Fatalf("CreateGroup(%q) error = %v, want nil", title, err)
		}
		if _, err := s.Types.CreateFieldInGroup(
			t.Context(), group.ID, FieldOn(t, "", "subtitle", content.FieldKindText, ""),
			nil,
		); err != nil {
			t.Fatalf("CreateFieldInGroup(%q) error = %v, want nil", title, err)
		}
		return group
	}
	cars := holding("Car extras", "car")
	holding("Book extras", "book")
	kept, _ := TypedHolding(t, s.Types, s.Content, "book", "Kept", author, content.Values{"subtitle": "must remain"})

	if err := s.Types.DeleteGroup(t.Context(), cars.ID, nil); err != nil {
		t.Fatalf("DeleteGroup() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, kept.ID); held["subtitle"] != "must remain" {
		t.Errorf("fields = %v, want the value the other group still serves left alone", held)
	}
}

// deleteFieldInGroupSweepsTheValuesOfItsMatchedTypes sweeps the field's values from its own type and no other.
func deleteFieldInGroupSweepsTheValuesOfItsMatchedTypes(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	declared := DeclareTypedField(t, s.Types, "car", "subtitle")
	DeclareTypedField(t, s.Types, "book", "subtitle")
	car, _ := TypedHolding(t, s.Types, s.Content, "car", "One Car", author, content.Values{"subtitle": "car words"})
	book, _ := TypedHolding(t, s.Types, s.Content, "book", "One Book", author,
		content.Values{"subtitle": "book words"})

	if err := s.Types.DeleteFieldInGroup(t.Context(), declared.GroupID, "subtitle", nil); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, car.ID); len(held) != 0 {
		t.Errorf("car fields = %v, want the deleted field's value swept", held)
	}
	if held := ValuesOf(t, s.Content, book.ID); !reflect.DeepEqual(held, content.Values{"subtitle": "book words"}) {
		t.Errorf("book fields = %v, want another type's same named field untouched", held)
	}
}

// deletingARestingGroupsFieldKeepsTheValueTheServedFieldHolds keeps the value an active group still serves everywhere.
func deletingARestingGroupsFieldKeepsTheValueTheServedFieldHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	DeclareTypedField(t, s.Types, "car", "title")
	if _, err := s.Types.CreateGroup(
		t.Context(), content.Group{Title: "Shadow", Location: LocationOf("car")},
	); err != nil {
		t.Fatalf("CreateGroup(Shadow) error = %v, want nil", err)
	}
	idle := Rested(t, s.Types, "Shadow")
	if _, err := s.Types.CreateFieldInGroup(
		t.Context(), idle.ID, FieldOn(t, "", "title", content.FieldKindText, ""), nil); err != nil {
		t.Fatalf("CreateFieldInGroup(shadow title) error = %v, want nil", err)
	}
	one, revision := TypedHolding(t, s.Types, s.Content, "car", "One", author,
		content.Values{"title": "served words"})
	ParkValues(t, s.Content, one, author, content.Values{"title": "typed words"})

	if err := s.Types.DeleteFieldInGroup(t.Context(), idle.ID, "title", nil); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, one.ID); !reflect.DeepEqual(held, content.Values{"title": "served words"}) {
		t.Errorf("stored values = %v, want the served field's value kept", held)
	}
	held := 0
	for _, values := range []content.Values{
		RevisionValuesOf(t, s.Content, revision), AutosaveValuesOf(t, s.Content, one, author),
	} {
		if _, kept := values["title"]; kept {
			held++
		}
	}
	if held != 2 {
		t.Errorf("%d revisions hold the title, want the revision and the autosave both kept", held)
	}
}

// deletingAShadowedGroupsFieldKeepsTheValueTheServingGroupHolds keeps the section another group serves on the type.
func deletingAShadowedGroupsFieldKeepsTheValueTheServingGroupHolds(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	everywhere := ServingEverything(t, s.Types)
	specs, err := s.Types.CreateFieldInGroup(t.Context(), everywhere.ID, SectionOn(t, "specs"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	DeclaredInside(t, s.Types, specs, "color", content.FieldKindText)
	trucks := RivalOnTruck(t, s.Types, "specs")
	one, _ := TypedHolding(t, s.Types, s.Content, "truck", "One", author,
		content.Values{"specs": map[string]any{"color": "red"}})

	if err := s.Types.DeleteFieldInGroup(t.Context(), trucks.ID, "specs", nil); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	held := ValuesOf(t, s.Content, one.ID)
	if !reflect.DeepEqual(held, content.Values{"specs": map[string]any{"color": "red"}}) {
		t.Errorf("stored values = %v, want the specs the serving group holds kept", held)
	}
}

// deletingARestingGroupsFieldSweepsTheValuesNoGroupServes sweeps the value of a resting group's field nobody serves.
func deletingARestingGroupsFieldSweepsTheValuesNoGroupServes(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	resting, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Resting", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Resting) error = %v, want nil", err)
	}
	if _, err := s.Types.CreateFieldInGroup(
		t.Context(), resting.ID, FieldOn(t, "", "title", content.FieldKindText, ""), nil); err != nil {
		t.Fatalf("CreateFieldInGroup(title) error = %v, want nil", err)
	}
	one, _ := TypedHolding(t, s.Types, s.Content, "car", "One", author, content.Values{"title": "old words"})
	Rested(t, s.Types, "Resting")
	if held := ValuesOf(t, s.Content, one.ID); held["title"] != "old words" {
		t.Fatalf("stored values = %v after resting the group, want the title kept until the delete", held)
	}

	if err := s.Types.DeleteFieldInGroup(t.Context(), resting.ID, "title", nil); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, one.ID); len(held) != 0 {
		t.Errorf("stored values = %v, want the title swept since no group serves it", held)
	}
}

// moveFieldCarriesTheFieldAndKeepsItsValues moves a field into another group on the type and keeps its values.
func moveFieldCarriesTheFieldAndKeepsItsValues(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	subtitle := DeclareTypedField(t, s.Types, "car", "subtitle")
	car, _ := TypedHolding(t, s.Types, s.Content, "car", "One Car", author, content.Values{"subtitle": "kept words"})
	extras, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	moved, err := s.Types.MoveField(t.Context(), subtitle.ID, extras.ID, 0, content.DefaultFieldDepth, nil)

	if err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}
	if moved.GroupID != extras.ID {
		t.Errorf("GroupID = %d, want the field carried into %d", moved.GroupID, extras.ID)
	}
	if held := ValuesOf(t, s.Content, car.ID); !reflect.DeepEqual(held, content.Values{"subtitle": "kept words"}) {
		t.Errorf("fields = %v, want the value kept through the move", held)
	}
	served, err := s.Types.ByKey(t.Context(), "car")
	if err != nil || len(served.Fields) != 1 || served.Fields[0].Key != "subtitle" {
		t.Errorf("ByKey().Fields = %v, %v, want the field still served on the type", served.Fields, err)
	}
}

// deleteGroupTakesItsFieldsAndValuesInOneSweep removes the group, its fields and every value they held.
func deleteGroupTakesItsFieldsAndValuesInOneSweep(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	StoreType(t, s.Types, "car")
	DeclareTypedField(t, s.Types, "car", "subtitle")
	DeclareTypedField(t, s.Types, "car", "mileage")
	car, _ := TypedHolding(t, s.Types, s.Content, "car", "One Car", author,
		content.Values{"subtitle": "kept words", "mileage": "9"})
	groups, err := s.Types.ListGroups(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("ListGroups() = %v, %v, want the one raised group", groups, err)
	}

	if err := s.Types.DeleteGroup(t.Context(), groups[0].ID, nil); err != nil {
		t.Fatalf("DeleteGroup() error = %v, want nil", err)
	}

	if held := ValuesOf(t, s.Content, car.ID); len(held) != 0 {
		t.Errorf("fields = %v, want every value of the group swept", held)
	}
	if listed, err := s.Types.ListGroups(t.Context()); err != nil || len(listed) != 0 {
		t.Errorf("ListGroups() = %v, %v, want no group left", listed, err)
	}
	held, err := s.Types.ByKey(t.Context(), "car")
	if err != nil || len(held.Fields) != 0 {
		t.Errorf("ByKey() = %v, %v, want the type left fieldless", held.Fields, err)
	}
}

// reorderingTheTopLeavesASubFieldSharingAKeyWhereItStands keeps a sub field in place when a top key it shares moves.
func reorderingTheTopLeavesASubFieldSharingAKeyWhereItStands(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	title := DeclareTypedField(t, s.Types, "car", "title")
	for _, key := range []string{"colour", "size", "title"} {
		if _, err := s.Types.CreateSubField(
			t.Context(), specs.ID, FieldOn(t, "", key, content.FieldKindText, ""), content.DefaultFieldDepth); err != nil {
			t.Fatalf("declaring %s inside specs: %v, want nil", key, err)
		}
	}
	standing := []string{"colour", "size", "title"}
	if held := subKeysOf(t, s.Types, specs); !slices.Equal(held, standing) {
		t.Fatalf("the sub fields stand as %v, want %v so a reorder could move the title", held, standing)
	}

	if err := s.Types.ReorderFieldsInGroup(
		t.Context(), title.GroupID, []string{"title", "specs"}); err != nil {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want nil", err)
	}

	if held := subKeysOf(t, s.Types, specs); !slices.Equal(held, standing) {
		t.Errorf("the sub fields stand as %v, want them left as %v by a reorder of the top", held, standing)
	}
}

// subKeysOf returns the keys of the sub fields the stored container holds, in the order its group serves them.
func subKeysOf(t *testing.T, types content.TypeStore, container content.Field) []string {
	t.Helper()
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	for _, g := range groups {
		for _, f := range g.Fields {
			if f.ID != container.ID {
				continue
			}
			keys := make([]string, len(f.Fields))
			for i, sub := range f.Fields {
				keys[i] = sub.Key
			}
			return keys
		}
	}
	t.Fatalf("no stored group holds the container %q", container.Key)
	return nil
}
