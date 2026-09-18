// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
)

// readingWhenSet returns the settings showing a field once the source holds a value.
func readingWhenSet(source string) map[string]any {
	return map[string]any{content.SettingConditions: content.ConditionsSetting(
		content.Rules{{{Source: source, Operator: content.OperatorNotEmpty, Value: ""}}},
	)}
}

// readingSite returns the planning site with a serving field shown once the cook time holds a value.
func readingSite(t *testing.T) *content.Registry {
	t.Helper()
	registry := planningSite(t)
	recipe, _ := storedGroup(t, registry, "recipe-details")
	if _, err := registry.CreateFieldInGroup(t.Context(), recipe.ID, content.Field{
		Key: "serving", Label: "Serving", Kind: content.FieldKindText, Settings: readingWhenSet("cook-time"),
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(serving) error = %v, want nil", err)
	}
	return registry
}

// linkingSite returns the planning site with a relation between recipes and a backlinks reading it, in two groups.
func linkingSite(t *testing.T) *content.Registry {
	t.Helper()
	registry := planningSite(t)
	links, err := registry.CreateGroup(t.Context(), content.Group{
		Key: "recipe-links", Title: "Recipe links", Location: recipeRules(), Active: true,
	})
	if err != nil {
		t.Fatalf("CreateGroup(recipe-links) error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), links.ID, content.Field{
		Key: "pairs-with", Label: "Pairs with", Kind: content.FieldKindRelation, RelatesTo: "recipe",
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(pairs-with) error = %v, want nil", err)
	}
	backlinks, err := registry.CreateGroup(t.Context(), content.Group{
		Key: "recipe-backlinks", Title: "Recipe backlinks", Location: recipeRules(), Active: true,
	})
	if err != nil {
		t.Fatalf("CreateGroup(recipe-backlinks) error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), backlinks.ID, content.Field{
		Key: "linked-from", Label: "Linked from", Kind: content.FieldKindBacklinks,
		Settings: map[string]any{
			content.SettingSourceGroup: "recipe-links", content.SettingSourceField: []any{"pairs-with"},
		},
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(linked-from) error = %v, want nil", err)
	}
	return registry
}

// leftOut takes the named fields out of the envelope's group.
func leftOut(group *definitions.GroupDefinition, keys ...string) {
	group.Fields = slices.DeleteFunc(group.Fields, func(f definitions.FieldDefinition) bool {
		return slices.Contains(keys, f.Key)
	})
}

// declaredIn returns the envelope's field the group holds under the key, failing the test when it holds none.
func declaredIn(t *testing.T, group *definitions.GroupDefinition, key string) *definitions.FieldDefinition {
	t.Helper()
	for i := range group.Fields {
		if group.Fields[i].Key == key {
			return &group.Fields[i]
		}
	}
	t.Fatalf("the envelope's group %q holds no field %q", group.Key, key)
	return nil
}

// confirmingFields returns the import confirming the loss of every dotted path the group names.
func confirmingFields(envelope definitions.Envelope, group string, paths ...string) definitions.Import {
	asked := definitions.Import{Envelope: envelope}
	for _, path := range paths {
		asked.Confirm = append(asked.Confirm, definitions.Confirmed{Subject: "field", Key: path, Group: group})
	}
	return asked
}

// refusedAsRead reports whether the error refuses taking the field away while the named one reads it.
func refusedAsRead(err error, field, by string) bool {
	var refused *content.Error
	return errors.Is(err, content.ErrFieldReferenced) && errors.As(err, &refused) &&
		refused.Held["field"] == field && refused.Held["by"] == by
}

func TestApplyTakesAwayAFieldAndTheSiblingReadingIt(t *testing.T) {
	t.Parallel()

	registry := readingSite(t)
	envelope := exported(t, registry)
	leftOut(groupNamed(t, envelope, "recipe-details"), "cook-time", "serving")

	applied(t, registry, confirmingFields(envelope, "recipe-details", "cook-time", "serving"))

	stored, _ := storedGroup(t, registry, "recipe-details")
	if keys := keysOfFields(stored.Fields); !slices.Equal(keys, []string{"steps"}) {
		t.Errorf("recipe-details holds %v, want the field and its reader both taken away", keys)
	}
}

func TestApplyRefusesAKeptReaderBeforeWritingAnything(t *testing.T) {
	t.Parallel()

	registry := readingSite(t)
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	recipe.Title = "Recipe notes"
	leftOut(recipe, "cook-time", "serving")

	_, err := definitions.Apply(t.Context(), registry, confirmingFields(envelope, "recipe-details", "cook-time"))

	if !refusedAsRead(err, "cook-time", "serving") {
		t.Errorf("Apply() error = %v, want cook-time kept while serving still reads it", err)
	}
	stored, _ := storedGroup(t, registry, "recipe-details")
	if stored.Title != "Recipe details" {
		t.Errorf("the recipe group is titled %q, want the refused import to have written nothing", stored.Title)
	}
}

func TestApplyRefusesAReaderWhoseMoveWasDeclined(t *testing.T) {
	t.Parallel()

	registry := readingSite(t)
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	recipe.Title = "Recipe notes"
	moved := *declaredIn(t, recipe, "serving")
	moved.Settings = nil
	leftOut(recipe, "cook-time", "serving")
	loose := groupNamed(t, envelope, "loose-ends")
	loose.Fields = append(loose.Fields, moved)

	_, err := definitions.Apply(t.Context(), registry, confirmingFields(envelope, "recipe-details", "cook-time"))

	if !refusedAsRead(err, "cook-time", "serving") {
		t.Errorf("Apply() error = %v, want cook-time kept while the unmoved serving reads it", err)
	}
	stored, _ := storedGroup(t, registry, "recipe-details")
	if stored.Title != "Recipe details" {
		t.Errorf("the recipe group is titled %q, want the refused import to have written nothing", stored.Title)
	}
}

func TestApplyMovesAFieldItsSiblingStopsReading(t *testing.T) {
	t.Parallel()

	registry := readingSite(t)
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	moved := *declaredIn(t, recipe, "cook-time")
	leftOut(recipe, "cook-time")
	declaredIn(t, recipe, "serving").Settings = nil
	loose := groupNamed(t, envelope, "loose-ends")
	loose.Fields = append(loose.Fields, moved)

	applied(t, registry, confirmingFields(envelope, "recipe-details", "cook-time"))

	if _, found := storedField(t, registry, "loose-ends", "cook-time"); !found {
		t.Errorf("the loose ends group holds no cook time field, want it moved in")
	}
	reader, _ := storedField(t, registry, "recipe-details", "serving")
	if rules := content.ConditionsOf(reader); len(rules) != 0 {
		t.Errorf("ConditionsOf(serving) = %v, want the rule the file cleared gone", rules)
	}
}

func TestApplyReplacesAFieldItsSiblingKeepsReading(t *testing.T) {
	t.Parallel()

	registry := readingSite(t)
	envelope := exported(t, registry)
	declaredIn(t, groupNamed(t, envelope, "recipe-details"), "cook-time").Kind = "number"

	applied(t, registry, confirmingFields(envelope, "recipe-details", "cook-time"))

	held, _ := storedField(t, registry, "recipe-details", "cook-time")
	if held.Kind != content.FieldKindNumber {
		t.Errorf("the cook time field is a %q, want the file's number standing", held.Kind)
	}
	reader, _ := storedField(t, registry, "recipe-details", "serving")
	if rules := content.ConditionsOf(reader); len(rules) != 1 {
		t.Errorf("ConditionsOf(serving) = %v, want the rule reading the replaced field kept", rules)
	}
}

func TestApplyTakesAwayASubFieldAndTheRowSiblingReadingIt(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	steps, _ := storedField(t, registry, "recipe-details", "steps")
	if _, err := registry.CreateSubField(t.Context(), steps.ID, content.Field{
		Key: "tip", Label: "Tip", Kind: content.FieldKindText, Settings: readingWhenSet("note"),
	}); err != nil {
		t.Fatalf("CreateSubField(tip) error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	declaredIn(t, groupNamed(t, envelope, "recipe-details"), "steps").Fields = nil

	applied(t, registry, confirmingFields(envelope, "recipe-details", "steps.note", "steps.tip"))

	emptied, _ := storedField(t, registry, "recipe-details", "steps")
	if len(emptied.Fields) != 0 {
		t.Errorf("the steps section holds %v, want the sub field and its reader both taken away",
			keysOfFields(emptied.Fields))
	}
}

func TestApplyReplacesASubFieldItsRowSiblingKeepsReading(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	steps, _ := storedField(t, registry, "recipe-details", "steps")
	if _, err := registry.CreateSubField(t.Context(), steps.ID, content.Field{
		Key: "tip", Label: "Tip", Kind: content.FieldKindText, Settings: readingWhenSet("note"),
	}); err != nil {
		t.Fatalf("CreateSubField(tip) error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	section := declaredIn(t, groupNamed(t, envelope, "recipe-details"), "steps")
	for i := range section.Fields {
		if section.Fields[i].Key == "note" {
			section.Fields[i].Kind = "number"
		}
	}

	applied(t, registry, confirmingFields(envelope, "recipe-details", "steps.note"))

	held, _ := storedField(t, registry, "recipe-details", "steps")
	kinds := make([]content.FieldKind, 0, len(held.Fields))
	for _, f := range held.Fields {
		kinds = append(kinds, f.Kind)
	}
	if !slices.Contains(kinds, content.FieldKindNumber) || len(kinds) != 2 {
		t.Errorf("the steps section holds kinds %v, want the number note beside the tip reading it", kinds)
	}
}

func TestApplyRefusesABacklinksWhoseGroupRemovalWasDeclined(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	links := groupNamed(t, envelope, "recipe-links")
	links.Title = "Recipe pairings"
	leftOut(links, "pairs-with")
	envelope.Groups = slices.DeleteFunc(envelope.Groups, func(g definitions.GroupDefinition) bool {
		return g.Key == "recipe-backlinks"
	})

	_, err := definitions.Apply(t.Context(), registry, confirmingFields(envelope, "recipe-links", "pairs-with"))

	if !refusedAsRead(err, "pairs-with", "linked-from") {
		t.Errorf("Apply() error = %v, want the relation kept while the unremoved group's backlinks reads it", err)
	}
	stored, _ := storedGroup(t, registry, "recipe-links")
	if stored.Title != "Recipe links" {
		t.Errorf("the links group is titled %q, want the refused import to have written nothing", stored.Title)
	}
}

func TestApplyTakesAwayARelationGroupAndTheBacklinksGroupReadingIt(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	envelope.Groups = slices.DeleteFunc(envelope.Groups, func(g definitions.GroupDefinition) bool {
		return g.Key == "recipe-links" || g.Key == "recipe-backlinks"
	})

	applied(t, registry, definitions.Import{
		Envelope: envelope,
		Confirm: []definitions.Confirmed{
			{Subject: "group", Key: "recipe-links"},
			{Subject: "group", Key: "recipe-backlinks"},
		},
	})

	if keys := keysOfGroups(t, registry); !slices.Equal(keys, []string{"recipe-details", "loose-ends"}) {
		t.Errorf("the site holds the groups %v, want both linking groups taken away", keys)
	}
}

func TestApplyRefusesAKeptBacklinksBeforeWritingAnything(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	envelope.Groups = slices.DeleteFunc(envelope.Groups, func(g definitions.GroupDefinition) bool {
		return g.Key == "recipe-links"
	})
	backlinks := groupNamed(t, envelope, "recipe-backlinks")
	backlinks.Title = "Recipe mentions"
	leftOut(backlinks, "linked-from")

	_, err := definitions.Apply(t.Context(), registry, definitions.Import{
		Envelope: envelope,
		Confirm:  []definitions.Confirmed{{Subject: "group", Key: "recipe-links"}},
	})

	if !refusedAsRead(err, "pairs-with", "linked-from") {
		t.Errorf("Apply() error = %v, want the relation kept while the backlinks reads it", err)
	}
	stored, _ := storedGroup(t, registry, "recipe-backlinks")
	if stored.Title != "Recipe backlinks" {
		t.Errorf("the backlinks group is titled %q, want the refused import to have written nothing", stored.Title)
	}
}

func TestApplyRefusesABacklinksLeftReadingARelationTakenFromASection(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	steps, _ := storedField(t, registry, "recipe-details", "steps")
	if _, err := registry.CreateSubField(t.Context(), steps.ID, content.Field{
		Key: "pairs-with", Label: "Pairs with", Kind: content.FieldKindRelation, RelatesTo: "recipe",
	}); err != nil {
		t.Fatalf("CreateSubField(pairs-with) error = %v, want nil", err)
	}
	loose, _ := storedGroup(t, registry, "loose-ends")
	loose.Location = recipeRules()
	if _, err := registry.UpdateGroup(t.Context(), loose); err != nil {
		t.Fatalf("UpdateGroup(loose-ends) error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), loose.ID, content.Field{
		Key: "linked-from", Label: "Linked from", Kind: content.FieldKindBacklinks,
		Settings: map[string]any{
			content.SettingSourceGroup: "recipe-details", content.SettingSourceField: []any{"steps", "pairs-with"},
		},
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(linked-from) error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	section := declaredIn(t, groupNamed(t, envelope, "recipe-details"), "steps")
	section.Fields = slices.DeleteFunc(section.Fields, func(f definitions.FieldDefinition) bool {
		return f.Key == "pairs-with"
	})
	leftOut(groupNamed(t, envelope, "loose-ends"), "linked-from")

	_, err := definitions.Apply(t.Context(), registry,
		confirmingFields(envelope, "recipe-details", "steps.pairs-with"))

	if !refusedAsRead(err, "pairs-with", "linked-from") {
		t.Errorf("Apply() error = %v, want the relation kept while the backlinks reads it", err)
	}
}
