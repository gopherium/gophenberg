// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// fieldStanding returns the stored field carrying the identity, however deep it stands, failing the test otherwise.
func fieldStanding(t *testing.T, registry *content.Registry, id int) content.Field {
	t.Helper()
	groups, err := registry.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups() error = %v, want nil", err)
	}
	for _, g := range groups {
		if held, found := fieldBelow(g.Fields, id); found {
			return held
		}
	}
	t.Fatalf("no stored field carries the identity %d", id)
	return content.Field{}
}

// fieldBelow returns the field carrying the identity among the fields or inside them.
func fieldBelow(fields []content.Field, id int) (content.Field, bool) {
	for _, f := range fields {
		if f.ID == id {
			return f, true
		}
		if held, found := fieldBelow(f.Fields, id); found {
			return held, true
		}
	}
	return content.Field{}, false
}

// holdsKey reports whether the fields hold one carrying the key at their own level.
func holdsKey(fields []content.Field, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

// topFieldsOf returns the fields the group holds at its top.
func topFieldsOf(t *testing.T, registry *content.Registry, groupID int) []content.Field {
	t.Helper()
	groups, err := registry.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups() error = %v, want nil", err)
	}
	for _, g := range groups {
		if g.ID == groupID {
			return g.Fields
		}
	}
	t.Fatalf("no stored group carries the identity %d", groupID)
	return nil
}

// topFieldIn returns the field the group holds at its top under the key, failing the test when it holds none.
func topFieldIn(t *testing.T, registry *content.Registry, groupID int, key string) content.Field {
	t.Helper()
	for _, f := range topFieldsOf(t, registry, groupID) {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("the group %d holds no field %q at its top", groupID, key)
	return content.Field{}
}

// subtitleIn declares the subtitle text field at the top of the group and returns it.
func subtitleIn(t *testing.T, registry *content.Registry, groupID int) content.Field {
	t.Helper()
	created, err := registry.CreateFieldInGroup(t.Context(), groupID, groupedTextField(t))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(subtitle) error = %v, want nil", err)
	}
	return created
}

// textInside declares a text field inside the container and returns it.
func textInside(t *testing.T, registry *content.Registry, parentID int, key string) content.Field {
	t.Helper()
	created, err := registry.CreateSubField(t.Context(), parentID, content.Field{
		Key: key, Label: "Inside", Kind: content.FieldKindText,
	})
	if err != nil {
		t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
	}
	return created
}

func TestMoveFieldCarriesAFieldIntoAContainer(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	details := sectionIn(t, registry, held.ID)
	subtitle := subtitleIn(t, registry, held.ID)

	moved, err := registry.MoveField(t.Context(), subtitle.ID, held.ID, details.ID)

	if err != nil {
		t.Fatalf("MoveField() error = %v, want the field carried inside the section", err)
	}
	if moved.ID != subtitle.ID || moved.ParentID != details.ID {
		t.Errorf("MoveField() = %+v, want the same row standing under %d", moved, details.ID)
	}
	if !holdsKey(fieldStanding(t, registry, details.ID).Fields, "subtitle") {
		t.Errorf("the section holds no subtitle, want the field inside it")
	}
	if holdsKey(topFieldsOf(t, registry, held.ID), "subtitle") {
		t.Errorf("the group still holds subtitle at its top, want it gone from there")
	}
}

func TestMoveFieldCarriesAFieldOutToTheTopOfItsGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	details := sectionIn(t, registry, held.ID)
	note := textInside(t, registry, details.ID, "note")

	moved, err := registry.MoveField(t.Context(), note.ID, held.ID, 0)

	if err != nil {
		t.Fatalf("MoveField() error = %v, want the field carried to the top", err)
	}
	if moved.ID != note.ID || moved.ParentID != 0 {
		t.Errorf("MoveField() = %+v, want the same row standing at the top", moved)
	}
	if !holdsKey(topFieldsOf(t, registry, held.ID), "note") {
		t.Errorf("the group holds no note at its top, want the field there")
	}
	if holdsKey(fieldStanding(t, registry, details.ID).Fields, "note") {
		t.Errorf("the section still holds note, want it gone from there")
	}
}

func TestMoveFieldLeavesAFieldStandingWhereItAlreadyIs(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	held := groupNaming(t, registry, "Article details", namingPost())
	subtitle := subtitleIn(t, registry, held.ID)
	store.moveErr = errStoreDown

	moved, err := registry.MoveField(t.Context(), subtitle.ID, held.ID, 0)

	if err != nil || moved.ID != subtitle.ID {
		t.Errorf("MoveField() = %+v, %v, want the field handed back with the store untouched", moved, err)
	}
}

func TestMoveFieldRefusesADestinationInsideTheMovedField(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	details := sectionIn(t, registry, held.ID)
	address, err := registry.CreateSubField(t.Context(), details.ID, sectionKeyed("address"))
	if err != nil {
		t.Fatalf("CreateSubField(address) error = %v, want nil", err)
	}

	for name, parent := range map[string]int{"itself": details.ID, "a container inside it": address.ID} {
		_, err := registry.MoveField(t.Context(), details.ID, held.ID, parent)

		if !errors.Is(err, content.ErrFieldInsideItself) || codeOf(err) != "field_moves_inside_itself" {
			t.Errorf("moving into %s: error = %v, want %v", name, err, content.ErrFieldInsideItself)
		}
	}
}

func TestMoveFieldRefusesAContainerAlreadyHoldingTheKey(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	details := sectionIn(t, registry, held.ID)
	textInside(t, registry, details.ID, "subtitle")
	subtitle := subtitleIn(t, registry, held.ID)

	_, err := registry.MoveField(t.Context(), subtitle.ID, held.ID, details.ID)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

func TestMoveFieldRefusesATopAlreadyHoldingTheKey(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	details := sectionIn(t, registry, held.ID)
	inside := textInside(t, registry, details.ID, "subtitle")
	subtitleIn(t, registry, held.ID)

	_, err := registry.MoveField(t.Context(), inside.ID, held.ID, 0)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

func TestMoveFieldRefusesAFieldMovedDeeperThanTheSiteAllows(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore()).WithFieldDepth(1)
	held := groupNaming(t, registry, "Article details", namingPost())
	details := sectionIn(t, registry, held.ID)
	address, err := registry.CreateFieldInGroup(t.Context(), held.ID, sectionKeyed("address"))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(address) error = %v, want nil", err)
	}
	textInside(t, registry, address.ID, "street")

	_, err = registry.MoveField(t.Context(), address.ID, held.ID, details.ID)

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTooDeep)
	}
}

func TestMoveFieldRefusesAKindTheDestinationCannotHold(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	_, makers := readableSource(t, registry)
	details := sectionIn(t, registry, makers.ID)
	linked := backlinksIn(t, registry, makers.ID)
	subtitle := subtitleIn(t, registry, makers.ID)

	for name, asked := range map[string]struct {
		field, parent int
		code          string
	}{
		"a backlinks into a section": {linked.ID, details.ID, "field_backlinks_inside"},
		"a text into a text":         {subtitle.ID, linked.ID, "field_parent_holds_none"},
	} {
		_, err := registry.MoveField(t.Context(), asked.field, makers.ID, asked.parent)

		if !errors.Is(err, content.ErrFieldShape) || codeOf(err) != asked.code {
			t.Errorf("moving %s: error = %v, want %s", name, err, asked.code)
		}
	}
}

func TestMoveFieldRefusesALayoutAtTheTopOfAGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	flexible, err := registry.CreateFieldInGroup(t.Context(), held.ID, content.Field{
		Key: "blocks", Label: "Blocks", Kind: content.FieldKindFlexible,
	})
	if err != nil {
		t.Fatalf("CreateFieldInGroup(flexible) error = %v, want nil", err)
	}
	layout, err := registry.CreateSubField(t.Context(), flexible.ID, content.Field{
		Key: "quote", Label: "Quote", Kind: content.FieldKindLayout,
	})
	if err != nil {
		t.Fatalf("CreateSubField(layout) error = %v, want nil", err)
	}

	_, err = registry.MoveField(t.Context(), layout.ID, held.ID, 0)

	if !errors.Is(err, content.ErrFieldShape) || codeOf(err) != "field_layout_alone" {
		t.Errorf("MoveField() error = %v, want field_layout_alone", err)
	}
}

func TestMoveFieldRefusesAFieldARowSiblingReads(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	details := sectionIn(t, registry, held.ID)
	paid, err := registry.CreateSubField(t.Context(), details.ID, content.Field{
		Key: "paid", Label: "Paid", Kind: content.FieldKindBoolean,
	})
	if err != nil {
		t.Fatalf("CreateSubField(paid) error = %v, want nil", err)
	}
	if _, err := registry.CreateSubField(t.Context(), details.ID,
		readerOf("fee", "paid", content.OperatorIs, "true")); err != nil {
		t.Fatalf("CreateSubField(fee) error = %v, want nil", err)
	}

	_, err = registry.MoveField(t.Context(), paid.ID, held.ID, 0)

	if !errors.Is(err, content.ErrFieldReferenced) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldReferenced)
	}
}

func TestMoveFieldRefusesAConditionTheNewSiblingsCannotAnswer(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)
	reader, err := registry.CreateFieldInGroup(t.Context(), held.ID,
		readerOf("sale-note", "on-sale", content.OperatorIs, "true"))
	if err != nil {
		t.Fatalf("declaring the reader: %v, want nil", err)
	}
	details := sectionIn(t, registry, held.ID)

	_, err = registry.MoveField(t.Context(), reader.ID, held.ID, details.ID)

	if !errors.Is(err, content.ErrRuleSourceUnknown) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrRuleSourceUnknown)
	}
}

// nestedSource declares a relation inside a section of the cars group and a backlinks on posts reading it.
func nestedSource(t *testing.T, registry *content.Registry) (content.Group, content.Field, content.Field) {
	t.Helper()
	if _, err := registry.Create(t.Context(), content.Type{
		Key: "car", SingularLabel: "Car", PluralLabel: "Cars", Active: true,
		PageKind: content.PageKindSingle, RouteWord: "cars",
	}); err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	cars := groupKeyed(t, registry, "cars", "Cars", namingType("car"))
	details := sectionIn(t, registry, cars.ID)
	maker, err := registry.CreateSubField(t.Context(), details.ID, content.Field{
		Key: "maker", Label: "Maker", Kind: content.FieldKindRelation, RelatesTo: content.TypePost,
	})
	if err != nil {
		t.Fatalf("CreateSubField(maker) error = %v, want nil", err)
	}
	makers := groupNaming(t, registry, "Makers", namingPost())
	if _, err := registry.CreateFieldInGroup(t.Context(), makers.ID, backlinksField(map[string]any{
		content.SettingSourceGroup: "cars", content.SettingSourceField: []any{"details", "maker"},
	})); err != nil {
		t.Fatalf("CreateFieldInGroup(backlinks) error = %v, want nil", err)
	}
	return cars, details, maker
}

func TestMoveFieldRefusesARelationABacklinksReadsThrough(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	cars, details, maker := nestedSource(t, registry)
	spare := groupNaming(t, registry, "Spare", namingType("car"))
	box, err := registry.CreateFieldInGroup(t.Context(), spare.ID, sectionKeyed("box"))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(box) error = %v, want nil", err)
	}

	for name, asked := range map[string]struct{ field, group, parent int }{
		"the relation to the top":          {maker.ID, cars.ID, 0},
		"the section holding it elsewhere": {details.ID, spare.ID, box.ID},
	} {
		_, err := registry.MoveField(t.Context(), asked.field, asked.group, asked.parent)

		if !errors.Is(err, content.ErrFieldReferenced) {
			t.Errorf("moving %s: error = %v, want %v", name, err, content.ErrFieldReferenced)
		}
	}
}

func TestMoveFieldKeepsAPluginsFieldsAndContainersWhereThePluginPutThem(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	plugin := pluginGroup(t, store)
	site := groupNaming(t, registry, "Extras", namingPost())
	details := sectionIn(t, registry, site.ID)
	subtitle := subtitleIn(t, registry, site.ID)
	venue := plugin.Fields[0]

	for name, asked := range map[string]struct{ field, group, parent int }{
		"the plugin's field into a site container": {venue.ID, site.ID, details.ID},
		"a site field into the plugin's field":     {subtitle.ID, plugin.ID, venue.ID},
	} {
		_, err := registry.MoveField(t.Context(), asked.field, asked.group, asked.parent)

		if !errors.Is(err, content.ErrDefinitionReadOnly) {
			t.Errorf("moving %s: error = %v, want %v", name, err, content.ErrDefinitionReadOnly)
		}
	}
}

func TestMoveFieldReportsAParentTheLandingGroupDoesNotHold(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	details := sectionIn(t, registry, held.ID)
	other := groupNaming(t, registry, "Extras", namingPost())
	subtitle := subtitleIn(t, registry, held.ID)

	_, err := registry.MoveField(t.Context(), subtitle.ID, other.ID, details.ID)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}
