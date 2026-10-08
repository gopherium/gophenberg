// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// groupCases are the cases over field groups, the fields they hold and the order a type serves them in.
var groupCases = []Case{
	{"ByKeyServesOneFieldWhenTwoGroupsCarryTheKey", byKeyServesOneFieldWhenTwoGroupsCarryTheKey},
	{"DeleteFieldsOfGroupReportsAGroupThatIsGone", deleteFieldsOfGroupReportsAGroupThatIsGone},
	{"UpdateGroupRefusesOneOfTwoMovesOntoTheSameType", updateGroupRefusesOneOfTwoMovesOntoTheSameType},
	{"ByKeyFlattensTheFieldsOfMatchingGroups", byKeyFlattensTheFieldsOfMatchingGroups},
	{"AGroupThatStopsMatchingStopsServingItsFields", aGroupThatStopsMatchingStopsServingItsFields},
	{"TheAnyRuleServesAGroupOnEveryType", theAnyRuleServesAGroupOnEveryType},
	{"UpdateFieldInGroupStoresTheLabelAndTheRequiredFlag", updateFieldInGroupStoresTheLabelAndTheRequiredFlag},
	{"UpdateFieldInGroupReportsAFieldThatIsGone", updateFieldInGroupReportsAFieldThatIsGone},
	{"UpdateFieldInGroupTurnsAwayAStaleExpectation", updateFieldInGroupTurnsAwayAStaleExpectation},
	{"UpdateFieldInGroupReportsAGroupThatIsGone", updateFieldInGroupReportsAGroupThatIsGone},
	{"DeleteFieldInGroupReportsAGroupThatIsGone", deleteFieldInGroupReportsAGroupThatIsGone},
	{"ReorderFieldsInGroupSettlesTheOrder", reorderFieldsInGroupSettlesTheOrder},
	{"MoveFieldReportsAFieldThatIsGone", moveFieldReportsAFieldThatIsGone},
	{"MoveFieldReportsAGroupThatIsGone", moveFieldReportsAGroupThatIsGone},
	{"DeleteGroupReportsAMissingGroup", deleteGroupReportsAMissingGroup},
	{"ReorderGroupsSettlesTheFlattenedOrder", reorderGroupsSettlesTheFlattenedOrder},
	{"UpdateGroupReportsAGroupThatIsGone", updateGroupReportsAGroupThatIsGone},
	{"CreateFieldInGroupReportsAGroupThatIsGone", createFieldInGroupReportsAGroupThatIsGone},
}

// byKeyServesOneFieldWhenTwoGroupsCarryTheKey serves a key once, from the first of two matching groups carrying it.
func byKeyServesOneFieldWhenTwoGroupsCarryTheKey(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	holding := func(title, away string) content.Group {
		t.Helper()
		group, err := s.Types.CreateGroup(
			t.Context(), content.Group{Title: title, Location: content.Rules{{
				{Source: "content_type", Operator: content.OperatorIsNot, Value: away},
				{Source: "content_type", Operator: content.OperatorIsNot, Value: content.TypePost},
			}}},
		)
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
	first := holding("Not cars", "car")
	holding("Not books", "book")

	StoreType(t, s.Types, "page")
	held, err := s.Types.ByKey(t.Context(), "page")

	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	keys := make([]string, len(held.Fields))
	for i, f := range held.Fields {
		keys[i] = f.Key
	}
	if len(held.Fields) != 1 {
		t.Fatalf("Fields = %v, want the key served once by the first group", keys)
	}
	if held.Fields[0].GroupID != first.ID {
		t.Errorf("GroupID = %d, want the first group %d winning the key", held.Fields[0].GroupID, first.ID)
	}
}

// deleteFieldsOfGroupReportsAGroupThatIsGone answers group not found for removing named fields from a missing group.
func deleteFieldsOfGroupReportsAGroupThatIsGone(t *testing.T, s Stores) {
	err := s.Types.DeleteFieldsOfGroup(t.Context(), 12345, []string{"subtitle"}, nil)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("DeleteFieldsOfGroup() = %v, want %v", err, content.ErrGroupNotFound)
	}
}

// updateGroupRefusesOneOfTwoMovesOntoTheSameType refuses a second group moving a key onto a type serving it already.
func updateGroupRefusesOneOfTwoMovesOntoTheSameType(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	StoreType(t, s.Types, "page")
	moving := func(title, typeKey string) content.Group {
		t.Helper()
		group, err := s.Types.CreateGroup(
			t.Context(), content.Group{Title: title, Location: LocationOf(typeKey)},
		)
		if err != nil {
			t.Fatalf("CreateGroup(%q) error = %v, want nil", title, err)
		}
		if _, err := s.Types.CreateFieldInGroup(
			t.Context(), group.ID, FieldOn(t, "", "subtitle", content.FieldKindText, ""),
			nil,
		); err != nil {
			t.Fatalf("CreateFieldInGroup(%q) error = %v, want nil", title, err)
		}
		group.Location = LocationOf("page")
		return group
	}
	first, second := moving("Cars", "car"), moving("Books", "book")

	if _, err := s.Types.UpdateGroup(t.Context(), first, nil, nil); err != nil {
		t.Fatalf("UpdateGroup(first) error = %v, want nil", err)
	}

	_, err := s.Types.UpdateGroup(t.Context(), second, nil, nil)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Fatalf("UpdateGroup(second) error = %v, want %v", err, content.ErrFieldTaken)
	}
	held, err := s.Types.ByKey(t.Context(), "page")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 1 {
		t.Errorf("Fields = %v, want the key served once", held.Fields)
	}
}

// byKeyFlattensTheFieldsOfMatchingGroups serves the fields of every matching group in group order, each with its type.
func byKeyFlattensTheFieldsOfMatchingGroups(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	DeclareTypedField(t, s.Types, "car", "subtitle")
	extras, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := s.Types.CreateFieldInGroup(
		t.Context(), extras.ID, FieldOn(t, "", "trim", content.FieldKindText, ""),
		nil,
	); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}

	held, err := s.Types.ByKey(t.Context(), "car")

	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 2 || held.Fields[0].Key != "subtitle" || held.Fields[1].Key != "trim" {
		t.Fatalf("Fields = %v, want both groups flattened in group order", held.Fields)
	}
	if held.Fields[1].TypeKey != "car" {
		t.Errorf("TypeKey = %q, want the flattened field carrying its type", held.Fields[1].TypeKey)
	}
}

// aGroupThatStopsMatchingStopsServingItsFields serves nothing from an inactive group while it is still listed whole.
func aGroupThatStopsMatchingStopsServingItsFields(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	DeclareTypedField(t, s.Types, "car", "subtitle")
	groups, err := s.Types.ListGroups(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("ListGroups() = %v, %v, want the one raised group", groups, err)
	}
	idle := groups[0]
	idle.Active = false

	if _, err := s.Types.UpdateGroup(t.Context(), idle, nil, nil); err != nil {
		t.Fatalf("UpdateGroup() error = %v, want nil", err)
	}

	held, err := s.Types.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 0 {
		t.Errorf("Fields = %v, want an inactive group serving nothing", held.Fields)
	}
	listed, err := s.Types.ListGroups(t.Context())
	if err != nil || len(listed) != 1 || len(listed[0].Fields) != 1 {
		t.Errorf("ListGroups() = %v, %v, want the inactive group still listed whole", listed, err)
	}
}

// theAnyRuleServesAGroupOnEveryType serves the field of a group under the any rule on every stored type.
func theAnyRuleServesAGroupOnEveryType(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	StoreType(t, s.Types, "book")
	shared, err := s.Types.CreateGroup(t.Context(), content.Group{
		Title:    "Everywhere",
		Location: content.Rules{{{Source: "content_type", Operator: content.OperatorIs, Value: content.AnyContentType}}},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := s.Types.CreateFieldInGroup(
		t.Context(), shared.ID, FieldOn(t, "", "footer", content.FieldKindText, ""),
		nil,
	); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}

	for _, typeKey := range []string{"car", "book"} {
		held, err := s.Types.ByKey(t.Context(), typeKey)
		if err != nil {
			t.Fatalf("ByKey(%s) error = %v, want nil", typeKey, err)
		}
		if len(held.Fields) != 1 || held.Fields[0].Key != "footer" {
			t.Errorf("ByKey(%s).Fields = %v, want the any group's field served", typeKey, held.Fields)
		}
	}
}

// updateFieldInGroupStoresTheLabelAndTheRequiredFlag stores the new label and the required flag of a group field.
func updateFieldInGroupStoresTheLabelAndTheRequiredFlag(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	declared := DeclareTypedField(t, s.Types, "car", "subtitle")

	updated, err := s.Types.UpdateFieldInGroup(t.Context(), declared.GroupID, content.Field{
		Key: "subtitle", Label: "Renamed", Required: true, UpdatedAt: declared.UpdatedAt,
	}, declared.UpdatedAt, nil)

	if err != nil {
		t.Fatalf("UpdateFieldInGroup() error = %v, want nil", err)
	}
	if updated.Label != "Renamed" || !updated.Required {
		t.Errorf("updated = %+v, want the new label and the required flag", updated)
	}
}

// updateFieldInGroupReportsAFieldThatIsGone answers field not found for an edit to a key the group does not hold.
func updateFieldInGroupReportsAFieldThatIsGone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	declared := DeclareTypedField(t, s.Types, "car", "subtitle")

	_, err := s.Types.UpdateFieldInGroup(t.Context(), declared.GroupID,
		content.Field{Key: "absent", Label: "Absent"}, declared.UpdatedAt, nil)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("UpdateFieldInGroup() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

// updateFieldInGroupTurnsAwayAStaleExpectation answers a conflict for an edit expecting an older stamp.
func updateFieldInGroupTurnsAwayAStaleExpectation(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	declared := DeclareTypedField(t, s.Types, "car", "subtitle")

	_, err := s.Types.UpdateFieldInGroup(t.Context(), declared.GroupID, content.Field{
		Key: "subtitle", Label: "Renamed", UpdatedAt: time.Now().UTC(),
	}, declared.UpdatedAt.Add(-time.Hour), nil)

	if !errors.Is(err, content.ErrConflict) {
		t.Errorf("UpdateFieldInGroup() error = %v, want %v", err, content.ErrConflict)
	}
}

// updateFieldInGroupReportsAGroupThatIsGone answers group not found for an edit inside a group nothing holds.
func updateFieldInGroupReportsAGroupThatIsGone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	declared := DeclareTypedField(t, s.Types, "car", "subtitle")

	_, err := s.Types.UpdateFieldInGroup(t.Context(), 424242, content.Field{
		Key: "subtitle", Label: "Renamed", UpdatedAt: declared.UpdatedAt,
	}, declared.UpdatedAt, nil)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("UpdateFieldInGroup() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}

// deleteFieldInGroupReportsAGroupThatIsGone answers group not found for removing one field from a missing group.
func deleteFieldInGroupReportsAGroupThatIsGone(t *testing.T, s Stores) {
	err := s.Types.DeleteFieldInGroup(t.Context(), 4242, "subtitle", nil)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("DeleteFieldInGroup() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}

// reorderFieldsInGroupSettlesTheOrder serves the fields of a group in the order asked.
func reorderFieldsInGroupSettlesTheOrder(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	declared := DeclareTypedField(t, s.Types, "car", "subtitle")
	DeclareTypedField(t, s.Types, "car", "mileage")

	if err := s.Types.ReorderFieldsInGroup(
		t.Context(), declared.GroupID, []string{"mileage", "subtitle"},
	); err != nil {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want nil", err)
	}

	held, err := s.Types.ByKey(t.Context(), "car")
	if err != nil || len(held.Fields) != 2 || held.Fields[0].Key != "mileage" {
		t.Errorf("Fields = %v, %v, want the asked order", held.Fields, err)
	}
}

// moveFieldReportsAFieldThatIsGone answers field not found for a move of a field nothing holds.
func moveFieldReportsAFieldThatIsGone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	DeclareTypedField(t, s.Types, "car", "subtitle")
	groups, err := s.Types.ListGroups(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("ListGroups() = %v, %v, want the one raised group", groups, err)
	}

	_, err = s.Types.MoveField(t.Context(), 4242, groups[0].ID, 0, content.DefaultFieldDepth, nil)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

// moveFieldReportsAGroupThatIsGone answers group not found for a move into a group nothing holds.
func moveFieldReportsAGroupThatIsGone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	subtitle := DeclareTypedField(t, s.Types, "car", "subtitle")

	_, err := s.Types.MoveField(t.Context(), subtitle.ID, 4242, 0, content.DefaultFieldDepth, nil)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}

// deleteGroupReportsAMissingGroup answers group not found for a removal of a group nothing holds.
func deleteGroupReportsAMissingGroup(t *testing.T, s Stores) {
	err := s.Types.DeleteGroup(t.Context(), 12345, nil)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("DeleteGroup() = %v, want %v", err, content.ErrGroupNotFound)
	}
}

// reorderGroupsSettlesTheFlattenedOrder serves the fields of the group moved first ahead of the others.
func reorderGroupsSettlesTheFlattenedOrder(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	DeclareTypedField(t, s.Types, "car", "subtitle")
	extras, err := s.Types.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: LocationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := s.Types.CreateFieldInGroup(
		t.Context(), extras.ID, FieldOn(t, "", "trim", content.FieldKindText, ""),
		nil,
	); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	groups, err := s.Types.ListGroups(t.Context())
	if err != nil || len(groups) != 2 {
		t.Fatalf("ListGroups() = %v, %v, want both groups", groups, err)
	}

	if err := s.Types.ReorderGroups(t.Context(), []int{groups[1].ID, groups[0].ID}); err != nil {
		t.Fatalf("ReorderGroups() error = %v, want nil", err)
	}

	held, err := s.Types.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 2 || held.Fields[0].Key != "trim" {
		t.Errorf("Fields = %v, want the reordered group's field first", held.Fields)
	}
}

// updateGroupReportsAGroupThatIsGone answers group not found for an edit of a group nothing holds.
func updateGroupReportsAGroupThatIsGone(t *testing.T, s Stores) {
	_, err := s.Types.UpdateGroup(t.Context(), content.Group{ID: 4242, Title: "Vanished"}, nil, nil)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("UpdateGroup() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}

// createFieldInGroupReportsAGroupThatIsGone answers group not found for a field declared into a group nothing holds.
func createFieldInGroupReportsAGroupThatIsGone(t *testing.T, s Stores) {
	_, err := s.Types.CreateFieldInGroup(
		t.Context(), 4242, FieldOn(t, "", "orphan", content.FieldKindText, ""),
		nil,
	)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("CreateFieldInGroup() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}
