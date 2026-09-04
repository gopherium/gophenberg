// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// showingWhen returns the settings map showing a field when the source holds the value.
func showingWhen(source, operator, value string) map[string]any {
	return map[string]any{content.SettingConditions: content.ConditionsSetting(
		content.Rules{{{Source: source, Operator: operator, Value: value}}},
	)}
}

// switchIn declares a boolean field inside the group and returns it.
func switchIn(t *testing.T, registry *content.Registry, groupID int, key string) content.Field {
	t.Helper()
	created, err := registry.CreateFieldInGroup(t.Context(), groupID, content.Field{
		Key: key, Label: "A Switch", Kind: content.FieldKindBoolean,
	})
	if err != nil {
		t.Fatalf("CreateFieldInGroup(%s) error = %v, want nil", key, err)
	}
	return created
}

// readerOf returns a text field shown when the source holds the value.
func readerOf(key, source, operator, value string) content.Field {
	return content.Field{
		Key: key, Label: "A Reader", Kind: content.FieldKindText,
		Settings: showingWhen(source, operator, value),
	}
}

// groupWithSwitch returns a group on posts already holding a boolean field the conditions read.
func groupWithSwitch(t *testing.T, registry *content.Registry) content.Group {
	t.Helper()
	held := groupNaming(t, registry, "Article details", namingPost())
	switchIn(t, registry, held.ID, "on-sale")
	return held
}

func TestCreateFieldInGroupTakesAConditionOnASibling(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)

	created, err := registry.CreateFieldInGroup(t.Context(), held.ID,
		readerOf("sale-note", "on-sale", content.OperatorIs, "true"))

	if err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want the condition accepted", err)
	}
	if rules := content.ConditionsOf(created); len(rules) != 1 || rules[0][0].Source != "on-sale" {
		t.Errorf("ConditionsOf() = %v, want the stored condition naming the sibling", rules)
	}
}

func TestCreateFieldInGroupRefusesAConditionOnNoSibling(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())

	_, err := registry.CreateFieldInGroup(t.Context(), held.ID,
		readerOf("sale-note", "vanished", content.OperatorIs, "true"))

	if !errors.Is(err, content.ErrRuleSourceUnknown) {
		t.Fatalf("CreateFieldInGroup() error = %v, want %v", err, content.ErrRuleSourceUnknown)
	}
}

func TestCreateFieldInGroupRefusesAConditionOnItself(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())

	_, err := registry.CreateFieldInGroup(t.Context(), held.ID,
		readerOf("loop", "loop", content.OperatorNotEmpty, ""))

	if !errors.Is(err, content.ErrRuleCycle) {
		t.Fatalf("CreateFieldInGroup() error = %v, want %v", err, content.ErrRuleCycle)
	}
}

func TestCreateFieldInGroupRefusesAConditionOnAFieldOfAnotherGroup(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	groupWithSwitch(t, registry)
	rival := groupNaming(t, registry, "Extras", namingPost())

	_, err := registry.CreateFieldInGroup(t.Context(), rival.ID,
		readerOf("sale-note", "on-sale", content.OperatorIs, "true"))

	if !errors.Is(err, content.ErrRuleSourceUnknown) {
		t.Errorf("CreateFieldInGroup() error = %v, want a source outside the group refused", err)
	}
}

func TestUpdateFieldInGroupRefusesAConditionClosingALoop(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)
	reader, err := registry.CreateFieldInGroup(t.Context(), held.ID,
		readerOf("sale-note", "on-sale", content.OperatorIs, "true"))
	if err != nil {
		t.Fatalf("declaring the reader: %v, want nil", err)
	}

	_, err = registry.UpdateFieldInGroup(t.Context(), held.ID, content.Field{
		Key: "on-sale", Label: "A Switch", Settings: showingWhen("sale-note", content.OperatorNotEmpty, ""),
	}, reader.UpdatedAt)

	if !errors.Is(err, content.ErrRuleCycle) {
		t.Fatalf("UpdateFieldInGroup() error = %v, want %v", err, content.ErrRuleCycle)
	}
	var refused *content.Error
	if !errors.As(err, &refused) || refused.Held["field"] == nil {
		t.Errorf("details = %v, want the looping field named", err)
	}
}

func TestUpdateFieldInGroupTakesAConditionOnASibling(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)
	reader, err := registry.CreateFieldInGroup(t.Context(), held.ID, content.Field{
		Key: "sale-note", Label: "A Reader", Kind: content.FieldKindText,
	})
	if err != nil {
		t.Fatalf("declaring the reader: %v, want nil", err)
	}

	updated, err := registry.UpdateFieldInGroup(t.Context(), held.ID, content.Field{
		Key: "sale-note", Label: "A Reader", Settings: showingWhen("on-sale", content.OperatorIs, "true"),
	}, reader.UpdatedAt)

	if err != nil {
		t.Fatalf("UpdateFieldInGroup() error = %v, want the condition accepted", err)
	}
	if rules := content.ConditionsOf(updated); len(rules) != 1 {
		t.Errorf("ConditionsOf() = %v, want the condition stored", rules)
	}
}

func TestDeleteFieldInGroupRefusesAFieldASiblingReads(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)
	if _, err := registry.CreateFieldInGroup(t.Context(), held.ID,
		readerOf("sale-note", "on-sale", content.OperatorIs, "true")); err != nil {
		t.Fatalf("declaring the reader: %v, want nil", err)
	}

	err := registry.DeleteFieldInGroup(t.Context(), held.ID, "on-sale")

	if !errors.Is(err, content.ErrFieldReferenced) {
		t.Fatalf("DeleteFieldInGroup() error = %v, want %v", err, content.ErrFieldReferenced)
	}
	var refused *content.Error
	if !errors.As(err, &refused) || refused.Held["by"] != "sale-note" {
		t.Errorf("details = %v, want the sibling reading it named", err)
	}
}

func TestDeleteFieldInGroupTakesAFieldNobodyReads(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)

	if err := registry.DeleteFieldInGroup(t.Context(), held.ID, "on-sale"); err != nil {
		t.Errorf("DeleteFieldInGroup() error = %v, want the field taken away", err)
	}
}

func TestDeleteFieldInGroupReportsAKeyTheGroupNeverHeld(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())

	err := registry.DeleteFieldInGroup(t.Context(), held.ID, "vanished")

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("DeleteFieldInGroup() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestMoveFieldReportsAKeyTheGroupNeverHeld(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	landing := groupNaming(t, registry, "Extras", namingPost())

	_, err := registry.MoveField(t.Context(), held.ID, "vanished", landing.ID)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestMoveFieldRefusesAFieldASiblingReads(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)
	if _, err := registry.CreateFieldInGroup(t.Context(), held.ID,
		readerOf("sale-note", "on-sale", content.OperatorIs, "true")); err != nil {
		t.Fatalf("declaring the reader: %v, want nil", err)
	}
	landing := groupNaming(t, registry, "Extras", namingPost())

	_, err := registry.MoveField(t.Context(), held.ID, "on-sale", landing.ID)

	if !errors.Is(err, content.ErrFieldReferenced) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldReferenced)
	}
}

func TestMoveFieldRefusesAConditionTheLandingGroupCannotAnswer(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)
	if _, err := registry.CreateFieldInGroup(t.Context(), held.ID,
		readerOf("sale-note", "on-sale", content.OperatorIs, "true")); err != nil {
		t.Fatalf("declaring the reader: %v, want nil", err)
	}
	landing := groupNaming(t, registry, "Extras", namingPost())

	_, err := registry.MoveField(t.Context(), held.ID, "sale-note", landing.ID)

	if !errors.Is(err, content.ErrRuleSourceUnknown) {
		t.Errorf("MoveField() error = %v, want the condition refused against the landing group", err)
	}
}

func TestMoveFieldTakesAFieldCarryingNoConditions(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)
	landing := groupNaming(t, registry, "Extras", namingPost())

	if _, err := registry.MoveField(t.Context(), held.ID, "on-sale", landing.ID); err != nil {
		t.Errorf("MoveField() error = %v, want the move allowed", err)
	}
}

func TestCreateSubFieldTakesAConditionOnARowSibling(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	parent := sectionIn(t, registry, held.ID)
	if _, err := registry.CreateSubField(t.Context(), parent.ID, content.Field{
		Key: "paid", Label: "Paid", Kind: content.FieldKindBoolean,
	}); err != nil {
		t.Fatalf("declaring the sub source: %v, want nil", err)
	}

	_, err := registry.CreateSubField(t.Context(), parent.ID,
		readerOf("fee", "paid", content.OperatorIs, "true"))

	if err != nil {
		t.Errorf("CreateSubField() error = %v, want the condition accepted", err)
	}
}

func TestCreateSubFieldRefusesAConditionOnATopLevelField(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupWithSwitch(t, registry)
	parent := sectionIn(t, registry, held.ID)

	_, err := registry.CreateSubField(t.Context(), parent.ID,
		readerOf("fee", "on-sale", content.OperatorIs, "true"))

	if !errors.Is(err, content.ErrRuleSourceUnknown) {
		t.Errorf("CreateSubField() error = %v, want a source outside the row refused", err)
	}
}

func TestUpdateSubFieldRefusesAConditionOnNoSibling(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	parent := sectionIn(t, registry, held.ID)
	inside, err := registry.CreateSubField(t.Context(), parent.ID, content.Field{
		Key: "fee", Label: "Fee", Kind: content.FieldKindNumber,
	})
	if err != nil {
		t.Fatalf("declaring the sub field: %v, want nil", err)
	}

	_, err = registry.UpdateSubField(t.Context(), inside.ID, content.Field{
		Key: "fee", Label: "Fee", Settings: showingWhen("vanished", content.OperatorIs, "true"),
	}, inside.UpdatedAt)

	if !errors.Is(err, content.ErrRuleSourceUnknown) {
		t.Errorf("UpdateSubField() error = %v, want %v", err, content.ErrRuleSourceUnknown)
	}
}

func TestDeleteSubFieldRefusesAFieldARowSiblingReads(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	parent := sectionIn(t, registry, held.ID)
	source, err := registry.CreateSubField(t.Context(), parent.ID, content.Field{
		Key: "paid", Label: "Paid", Kind: content.FieldKindBoolean,
	})
	if err != nil {
		t.Fatalf("declaring the sub source: %v, want nil", err)
	}
	if _, err := registry.CreateSubField(t.Context(), parent.ID,
		readerOf("fee", "paid", content.OperatorIs, "true")); err != nil {
		t.Fatalf("declaring the sub reader: %v, want nil", err)
	}

	err = registry.DeleteSubField(t.Context(), source.ID)

	if !errors.Is(err, content.ErrFieldReferenced) {
		t.Errorf("DeleteSubField() error = %v, want %v", err, content.ErrFieldReferenced)
	}
}

func TestDeleteSubFieldTakesAFieldNoRowSiblingReads(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	held := groupNaming(t, registry, "Article details", namingPost())
	parent := sectionIn(t, registry, held.ID)
	inside, err := registry.CreateSubField(t.Context(), parent.ID, content.Field{
		Key: "fee", Label: "Fee", Kind: content.FieldKindNumber,
	})
	if err != nil {
		t.Fatalf("declaring the sub field: %v, want nil", err)
	}

	if err := registry.DeleteSubField(t.Context(), inside.ID); err != nil {
		t.Errorf("DeleteSubField() error = %v, want the field taken away", err)
	}
}
