// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/sdk"
)

// shownWhen returns the declared rule showing a field when the source holds the value.
func shownWhen(source, operator, value string) [][]sdk.Rule {
	return [][]sdk.Rule{{{Source: source, Operator: operator, Value: value}}}
}

// readingGroup returns the event group whose venue field is read by a note declared before it.
func readingGroup() sdk.GroupDeclaration {
	group := eventGroup()
	group.Fields = []sdk.FieldDeclaration{
		{Key: "note", Label: "Note", Kind: "text", Conditions: shownWhen("venue", content.OperatorNotEmpty, "")},
		{Key: "venue", Label: "Venue", Kind: "text", Required: true},
	}
	return group
}

// declaringEvents returns a registrar holding the event type and the registry reading what it declares.
func declaringEvents(t *testing.T) (*definitions.Registrar, *content.Registry) {
	t.Helper()
	pool, registrar := declaringPool(t)
	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}
	return registrar, content.NewRegistry(postgres.NewTypeStore(pool))
}

// conditioned returns a declared text field shown only while the source holds a value.
func conditioned(key, source string) definitions.FieldDefinition {
	return definitions.FieldDefinition{
		Key: key, Label: "A Reader", Kind: "text",
		Settings: map[string]any{content.SettingConditions: content.ConditionsSetting(
			content.Rules{{{Source: source, Operator: content.OperatorNotEmpty, Value: ""}}},
		)},
	}
}

// fieldNamed returns the stored field of the group carrying the key, failing the test when it holds none.
func fieldNamed(t *testing.T, held content.Group, key string) content.Field {
	t.Helper()
	for _, f := range held.Fields {
		if f.Key == key {
			return f
		}
	}
	t.Fatalf("the group holds no field %q", key)
	return content.Field{}
}

func TestAPluginMayDeclareAReaderBeforeItsSource(t *testing.T) {
	t.Parallel()

	registrar, registry := declaringEvents(t)

	err := registrar.DeclareGroup(t.Context(), readingGroup())

	if err != nil {
		t.Fatalf("DeclareGroup() error = %v, want the declaration order tolerated", err)
	}
	held := heldGroup(t, registry, "event-details")
	if rules := content.ConditionsOf(fieldNamed(t, held, "note")); len(rules) != 1 {
		t.Errorf("ConditionsOf(note) = %v, want the declared condition stored", rules)
	}
}

func TestAPluginDeclaringTwiceWritesNothingTheSecondTime(t *testing.T) {
	t.Parallel()

	registrar, registry := declaringEvents(t)
	if err := registrar.DeclareGroup(t.Context(), readingGroup()); err != nil {
		t.Fatalf("the first declaration: %v, want nil", err)
	}
	first := fieldNamed(t, heldGroup(t, registry, "event-details"), "note")

	if err := registrar.DeclareGroup(t.Context(), readingGroup()); err != nil {
		t.Fatalf("the second declaration: %v, want nil", err)
	}

	again := fieldNamed(t, heldGroup(t, registry, "event-details"), "note")
	if !again.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("UpdatedAt moved from %v to %v, want the second boot writing nothing", first.UpdatedAt, again.UpdatedAt)
	}
}

func TestAPluginDeclaresTheListFlag(t *testing.T) {
	t.Parallel()

	registrar, registry := declaringEvents(t)
	group := eventGroup()
	group.Fields[0].Listed = true

	if err := registrar.DeclareGroup(t.Context(), group); err != nil {
		t.Fatalf("DeclareGroup() error = %v, want nil", err)
	}

	if !content.Listed(fieldNamed(t, heldGroup(t, registry, "event-details"), "venue")) {
		t.Error("Listed() = false, want the declared flag stored")
	}
}

func TestAPluginDeclaringALoopIsRefused(t *testing.T) {
	t.Parallel()

	registrar, _ := declaringEvents(t)
	group := readingGroup()
	group.Fields[1].Conditions = shownWhen("note", content.OperatorNotEmpty, "")

	err := registrar.DeclareGroup(t.Context(), group)

	if !errors.Is(err, content.ErrRuleCycle) {
		t.Errorf("DeclareGroup() error = %v, want %v", err, content.ErrRuleCycle)
	}
}

func TestASubFieldConditionReadsItsOwnRow(t *testing.T) {
	t.Parallel()

	registrar, _ := declaringEvents(t)
	group := eventGroup()
	group.Fields[1].Fields = []sdk.FieldDeclaration{
		{Key: "doors", Label: "Doors", Kind: "text", Conditions: shownWhen("starts-at", content.OperatorNotEmpty, "")},
		{Key: "starts-at", Label: "Starts at", Kind: "date"},
	}

	if err := registrar.DeclareGroup(t.Context(), group); err != nil {
		t.Errorf("DeclareGroup() error = %v, want the row's own order tolerated", err)
	}
}

func TestAnEnvelopeWhoseReaderPrecedesItsSourceImports(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	group := groupNamed(t, envelope, "recipe-details")
	group.Fields = append([]definitions.FieldDefinition{{
		Key: "note", Label: "Note", Kind: "text",
		Settings: map[string]any{content.SettingConditions: content.ConditionsSetting(
			content.Rules{{{Source: "cook-time", Operator: content.OperatorNotEmpty, Value: ""}}},
		)},
	}}, group.Fields...)

	applied(t, registry, importing(envelope))

	held, found := storedGroup(t, registry, "recipe-details")
	if !found {
		t.Fatal("the site holds no recipe-details group")
	}
	if rules := content.ConditionsOf(fieldNamed(t, held, "note")); len(rules) != 1 {
		t.Errorf("ConditionsOf(note) = %v, want the imported condition stored", rules)
	}
}

func TestAnEnvelopeImportedTwiceAsksForNothingTheSecondTime(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	group := groupNamed(t, envelope, "recipe-details")
	group.Fields = append([]definitions.FieldDefinition{{
		Key: "note", Label: "Note", Kind: "text",
		Settings: map[string]any{content.SettingConditions: content.ConditionsSetting(
			content.Rules{{{Source: "cook-time", Operator: content.OperatorNotEmpty, Value: ""}}},
		)},
	}}, group.Fields...)
	applied(t, registry, importing(envelope))

	outcome := applied(t, registry, importing(envelope))

	if len(outcome.Applied) != 0 {
		t.Errorf("Applied = %+v, want the second import changing nothing", outcome.Applied)
	}
}

func TestAnEnvelopeCarriesAConditionOntoASubField(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	steps := &groupNamed(t, envelope, "recipe-details").Fields[1]
	source := steps.Fields[0].Key
	steps.Fields = append([]definitions.FieldDefinition{{
		Key: "timing", Label: "Timing", Kind: "text",
		Settings: map[string]any{content.SettingConditions: content.ConditionsSetting(
			content.Rules{{{Source: source, Operator: content.OperatorNotEmpty, Value: ""}}},
		)},
	}}, steps.Fields...)

	applied(t, registry, importing(envelope))

	held, found := storedGroup(t, registry, "recipe-details")
	if !found {
		t.Fatal("the site holds no recipe-details group")
	}
	inside := fieldNamed(t, content.Group{Fields: fieldNamed(t, held, "steps").Fields}, "timing")
	if rules := content.ConditionsOf(inside); len(rules) != 1 || rules[0][0].Source != source {
		t.Errorf("ConditionsOf(timing) = %v, want the condition naming its row sibling", rules)
	}
}

func TestAnEnvelopeNamingNoSiblingIsRefused(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	group := groupNamed(t, envelope, "recipe-details")
	group.Fields = append(group.Fields, definitions.FieldDefinition{
		Key: "note", Label: "Note", Kind: "text",
		Settings: map[string]any{content.SettingConditions: content.ConditionsSetting(
			content.Rules{{{Source: "vanished", Operator: content.OperatorNotEmpty, Value: ""}}},
		)},
	})

	_, err := definitions.Compare(t.Context(), registry, envelope)

	if !errors.Is(err, content.ErrRuleSourceUnknown) {
		t.Errorf("Compare() error = %v, want %v", err, content.ErrRuleSourceUnknown)
	}
}

func TestAnEnvelopeClosingALoopIsRefused(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	group := groupNamed(t, envelope, "recipe-details")
	group.Fields = append(group.Fields, definitions.FieldDefinition{
		Key: "note", Label: "Note", Kind: "text",
		Settings: map[string]any{content.SettingConditions: content.ConditionsSetting(
			content.Rules{{{Source: "note", Operator: content.OperatorNotEmpty, Value: ""}}},
		)},
	})

	_, err := definitions.Compare(t.Context(), registry, envelope)

	if !errors.Is(err, content.ErrRuleCycle) {
		t.Errorf("Compare() error = %v, want %v", err, content.ErrRuleCycle)
	}
}
