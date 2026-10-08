// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// fieldParentCases are the cases over sub fields standing inside their parent, and the parents carrying them.
var fieldParentCases = []Case{
	{"DeletingATopFieldLeavesASubFieldSharingItsKey", deletingATopFieldLeavesASubFieldSharingItsKey},
	{"MovingATopFieldLeavesASubFieldSharingItsKey", movingATopFieldLeavesASubFieldSharingItsKey},
	{"MovingASectionCarriesASubFieldTwoLevelsDown", movingASectionCarriesASubFieldTwoLevelsDown},
	{"DeletingTheGroupASectionLeftKeepsTheSectionWhole", deletingTheGroupASectionLeftKeepsTheSectionWhole},
	{"CreateSubFieldStoresTheKeyTheDomainSettledOn", createSubFieldStoresTheKeyTheDomainSettledOn},
	{"CreatingASubFieldStoresItUnderItsParent", creatingASubFieldStoresItUnderItsParent},
	{"CreatingASubFieldOrdersItAfterItsSiblings", creatingASubFieldOrdersItAfterItsSiblings},
	{"CreatingASubFieldReportsAParentThatIsGone", creatingASubFieldReportsAParentThatIsGone},
	{"CreatingASubFieldRefusesAParentHoldingNone", creatingASubFieldRefusesAParentHoldingNone},
	{
		"CreatingASubFieldRefusesAParentChainPastTheDefaultDepth",
		creatingASubFieldRefusesAParentChainPastTheDefaultDepth,
	},
	{
		"CreatingASubFieldTakesAChainPastTheDefaultWhenTheLimitAllowsIt",
		creatingASubFieldTakesAChainPastTheDefaultWhenTheLimitAllowsIt,
	},
	{"AGroupServesItsSubFieldsInsideTheirParent", aGroupServesItsSubFieldsInsideTheirParent},
}

// keyedWithin counts the fields carrying the key at any depth of the fields.
func keyedWithin(fields []content.Field, key string) int {
	held := 0
	for _, f := range fields {
		if f.Key == key {
			held++
		}
		held += keyedWithin(f.Fields, key)
	}
	return held
}

// deletingATopFieldLeavesASubFieldSharingItsKey removes a top field and keeps the sub field carrying the same key.
func deletingATopFieldLeavesASubFieldSharingItsKey(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	title := DeclareTypedField(t, s.Types, "car", "title")
	DeclaredInside(t, s.Types, specs, "title", content.FieldKindText)

	if err := s.Types.DeleteFieldInGroup(t.Context(), title.GroupID, "title", nil); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	if held := keyedWithin(GroupAt(t, s.Types, specs.GroupID).Fields, "title"); held != 1 {
		t.Errorf("rows holding title = %d, want the sub field left standing", held)
	}
}

// movingATopFieldLeavesASubFieldSharingItsKey moves a top field away and leaves the sub field sharing its key in place.
func movingATopFieldLeavesASubFieldSharingItsKey(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	title := DeclareTypedField(t, s.Types, "car", "title")
	sub := DeclaredInside(t, s.Types, specs, "title", content.FieldKindText)
	landing := GroupOn(t, s.Types, "Elsewhere", "car")

	if _, err := s.Types.MoveField(t.Context(), title.ID, landing.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if moved := GroupAt(t, s.Types, landing.ID); len(moved.Fields) != 1 || moved.Fields[0].ID != title.ID {
		t.Fatalf("Elsewhere holds %+v, want the top title moved into it", moved.Fields)
	}
	kept := GroupAt(t, s.Types, specs.GroupID)
	if len(kept.Fields) != 1 || len(kept.Fields[0].Fields) != 1 || kept.Fields[0].Fields[0].ID != sub.ID {
		t.Fatalf("the group holds %+v, want specs alone holding the title sub field", kept.Fields)
	}
	if parked := kept.Fields[0].Fields[0].GroupID; parked != specs.GroupID {
		t.Errorf("the sub field sits in group %d, want it left in %d", parked, specs.GroupID)
	}
}

// movingASectionCarriesASubFieldTwoLevelsDown moves a section into another group with every field below it.
func movingASectionCarriesASubFieldTwoLevelsDown(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	source := GroupOn(t, s.Types, "Details", "car")
	section, err := s.Types.CreateFieldInGroup(
		t.Context(), source.ID, FieldOn(t, "", "author", content.FieldKindSection, ""), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(author) error = %v, want nil", err)
	}
	rows := DeclaredInside(t, s.Types, section, "rows", content.FieldKindRepeater)
	title := DeclaredInside(t, s.Types, rows, "title", content.FieldKindText)
	landing := GroupOn(t, s.Types, "Elsewhere", "car")

	if _, err := s.Types.MoveField(t.Context(), section.ID, landing.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField(author) error = %v, want nil", err)
	}

	if left := GroupAt(t, s.Types, source.ID); len(left.Fields) != 0 {
		t.Errorf("Details holds %+v, want author moved out with everything below it", left.Fields)
	}
	carried := GroupAt(t, s.Types, landing.ID)
	if len(carried.Fields) != 1 || carried.Fields[0].ID != section.ID {
		t.Fatalf("Elsewhere holds %+v, want author carried into it", carried.Fields)
	}
	inside := carried.Fields[0].Fields
	if len(inside) != 1 || inside[0].ID != rows.ID {
		t.Fatalf("author holds %+v, want rows carried with its section", inside)
	}
	if inside[0].GroupID != landing.ID {
		t.Errorf("the sub field sits in group %d, want it carried into %d with its section",
			inside[0].GroupID, landing.ID)
	}
	deep := inside[0].Fields
	if len(deep) != 1 || deep[0].ID != title.ID {
		t.Fatalf("rows holds %+v, want title carried two levels down", deep)
	}
	if deep[0].GroupID != landing.ID {
		t.Errorf("the field two levels down sits in group %d, want it carried into %d", deep[0].GroupID, landing.ID)
	}
}

// movedSection holds a section declared in one group and moved into another, with the sub field it carries.
type movedSection struct {
	source, landing content.Group
	section, sub    content.Field
}

// moveSection declares a section holding one text sub field in a group and moves it into a second group.
func moveSection(t *testing.T, types content.TypeStore) movedSection {
	t.Helper()
	source := GroupOn(t, types, "Details", "car")
	section, err := types.CreateFieldInGroup(
		t.Context(), source.ID, FieldOn(t, "", "author", content.FieldKindSection, ""), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(author) error = %v, want nil", err)
	}
	sub := DeclaredInside(t, types, section, "name", content.FieldKindText)
	landing := GroupOn(t, types, "Elsewhere", "car")
	if _, err := types.MoveField(t.Context(), section.ID, landing.ID, 0, content.DefaultFieldDepth, nil); err != nil {
		t.Fatalf("MoveField(author) error = %v, want nil", err)
	}
	return movedSection{source: source, landing: landing, section: section, sub: sub}
}

// deletingTheGroupASectionLeftKeepsTheSectionWhole removes the group a section moved out of and keeps its sub field.
func deletingTheGroupASectionLeftKeepsTheSectionWhole(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	moved := moveSection(t, s.Types)

	if err := s.Types.DeleteGroup(t.Context(), moved.source.ID, nil); err != nil {
		t.Fatalf("DeleteGroup() error = %v, want the group the section left gone", err)
	}

	if held := GroupHolding(t, s.Types, "author"); held != moved.landing.ID {
		t.Errorf("author is declared by group %d, want %d", held, moved.landing.ID)
	}
	landing := GroupAt(t, s.Types, moved.landing.ID)
	if len(landing.Fields) != 1 || landing.Fields[0].ID != moved.section.ID {
		t.Fatalf("Elsewhere holds %+v, want author alone", landing.Fields)
	}
	inside := landing.Fields[0].Fields
	if kept := len(inside); kept != 1 {
		t.Fatalf("the section holds %d sub fields after the delete, want its one kept", kept)
	}
	if inside[0].ID != moved.sub.ID {
		t.Errorf("the section holds %q, want its name sub field kept", inside[0].Key)
	}
}

// createSubFieldStoresTheKeyTheDomainSettledOn stores a sub field under the key and label trimmed by the domain.
func createSubFieldStoresTheKeyTheDomainSettledOn(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")

	created, err := s.Types.CreateSubField(t.Context(), specs.ID, content.Field{
		Key: "  title  ", Label: "  Title  ", Kind: content.FieldKindText,
	}, content.DefaultFieldDepth)

	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	if created.Key != "title" || created.Label != "Title" {
		t.Errorf("stored key %q label %q, want them trimmed to title and Title", created.Key, created.Label)
	}
}

// creatingASubFieldStoresItUnderItsParent stores a sub field naming its parent and standing in the parent's group.
func creatingASubFieldStoresItUnderItsParent(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")

	held, err := s.Types.CreateSubField(
		t.Context(), specs.ID, FieldOn(t, "", "title", content.FieldKindText, ""), content.DefaultFieldDepth)

	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	if held.ParentID != specs.ID {
		t.Errorf("ParentID = %d, want %d", held.ParentID, specs.ID)
	}
	if held.GroupID != specs.GroupID {
		t.Errorf("GroupID = %d, want the parent's group %d", held.GroupID, specs.GroupID)
	}
}

// creatingASubFieldOrdersItAfterItsSiblings serves the sub fields of a parent in the order they were declared.
func creatingASubFieldOrdersItAfterItsSiblings(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	other := DeclareSection(t, s.Types, "extras")
	if _, err := s.Types.CreateSubField(
		t.Context(), other.ID, FieldOn(t, "", "away", content.FieldKindText, ""), content.DefaultFieldDepth); err != nil {
		t.Fatalf("declaring a sibling elsewhere: %v, want nil", err)
	}

	for _, key := range []string{"title", "colour"} {
		if _, err := s.Types.CreateSubField(
			t.Context(), specs.ID, FieldOn(t, "", key, content.FieldKindText, ""), content.DefaultFieldDepth); err != nil {
			t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
		}
	}

	groups, err := s.Types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := GroupOf(groups, specs.GroupID)
	if !found {
		t.Fatalf("the group %d is not served", specs.GroupID)
	}
	inside := subFieldsOf(held, "specs")
	if len(inside) != 2 {
		t.Fatalf("specs holds %d sub fields, want the two declared inside it alone", len(inside))
	}
	if inside[0].Key != "title" || inside[1].Key != "colour" {
		t.Errorf("specs holds %q then %q, want them in the order they were declared",
			inside[0].Key, inside[1].Key)
	}
}

// subFieldsOf returns the sub fields the group's named field holds.
func subFieldsOf(held content.Group, key string) []content.Field {
	for _, f := range held.Fields {
		if f.Key == key {
			return f.Fields
		}
	}
	return nil
}

// creatingASubFieldReportsAParentThatIsGone answers field not found for a sub field under a parent nothing holds.
func creatingASubFieldReportsAParentThatIsGone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")

	_, err := s.Types.CreateSubField(
		t.Context(), 424242, FieldOn(t, "", "title", content.FieldKindText, ""), content.DefaultFieldDepth)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("CreateSubField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

// creatingASubFieldRefusesAParentHoldingNone refuses a sub field under a parent whose kind holds no fields.
func creatingASubFieldRefusesAParentHoldingNone(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	plain := DeclareTypedField(t, s.Types, "car", "subtitle")

	_, err := s.Types.CreateSubField(
		t.Context(), plain.ID, FieldOn(t, "", "title", content.FieldKindText, ""), content.DefaultFieldDepth)

	if !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("CreateSubField() error = %v, want %v", err, content.ErrFieldShape)
	}
}

// creatingASubFieldRefusesAParentChainPastTheDefaultDepth refuses a sub field one container past the default depth.
func creatingASubFieldRefusesAParentChainPastTheDefaultDepth(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	registry := content.NewRegistry(s.Types)
	deepest := SectionChainOnCar(t, s.Types, registry, content.DefaultFieldDepth)

	_, err := registry.CreateSubField(t.Context(), deepest.ID, FieldOn(t, "", "title", content.FieldKindText, ""))

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("CreateSubField() one container too deep error = %v, want %v", err, content.ErrFieldTooDeep)
	}
	_, err = s.Types.CreateSubField(
		t.Context(), deepest.ID, FieldOn(t, "", "title", content.FieldKindText, ""), content.DefaultFieldDepth)
	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("the store's CreateSubField() one container too deep error = %v, want %v",
			err, content.ErrFieldTooDeep)
	}
}

// creatingASubFieldTakesAChainPastTheDefaultWhenTheLimitAllowsIt stores a sub field deeper under a raised limit.
func creatingASubFieldTakesAChainPastTheDefaultWhenTheLimitAllowsIt(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	registry := content.NewRegistry(s.Types).WithFieldDepth(content.DefaultFieldDepth + 1)
	deepest := SectionChainOnCar(t, s.Types, registry, content.DefaultFieldDepth)

	title, err := registry.CreateSubField(t.Context(), deepest.ID, FieldOn(t, "", "title", content.FieldKindText, ""))

	if err != nil {
		t.Fatalf("CreateSubField() within a raised limit error = %v, want nil", err)
	}
	held, found := fieldNumbered(GroupAt(t, s.Types, deepest.GroupID).Fields, deepest.ID)
	if !found {
		t.Fatalf("no group lists the deepest section %d", deepest.ID)
	}
	if len(held.Fields) != 1 || held.Fields[0].ID != title.ID {
		t.Errorf("the deepest section holds %+v, want title standing under it", held.Fields)
	}
}

// fieldNumbered returns the field carrying the identity at any depth of the fields, and whether one does.
func fieldNumbered(fields []content.Field, id int) (content.Field, bool) {
	for _, f := range fields {
		if f.ID == id {
			return f, true
		}
		if held, found := fieldNumbered(f.Fields, id); found {
			return held, true
		}
	}
	return content.Field{}, false
}

// aGroupServesItsSubFieldsInsideTheirParent lists a sub field inside its parent rather than at the top of the group.
func aGroupServesItsSubFieldsInsideTheirParent(t *testing.T, s Stores) {
	StoreType(t, s.Types, "car")
	specs := DeclareSection(t, s.Types, "specs")
	DeclaredInside(t, s.Types, specs, "title", content.FieldKindText)

	groups, err := s.Types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}

	held, found := GroupOf(groups, specs.GroupID)
	if !found {
		t.Fatalf("the group %d is not served", specs.GroupID)
	}
	if len(held.Fields) != 1 {
		t.Fatalf("the group serves %d fields, want the sub field held inside its parent", len(held.Fields))
	}
	if len(held.Fields[0].Fields) != 1 || held.Fields[0].Fields[0].Key != "title" {
		t.Errorf("specs holds %+v, want the title sub field", held.Fields[0].Fields)
	}
}
