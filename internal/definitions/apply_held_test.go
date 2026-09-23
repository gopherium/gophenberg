// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/definitions"
)

// leftUndoneAt reports whether the changes hold the one for the key inside the group under the action.
func leftUndoneAt(changes []definitions.Change, action, subject, group, key string) bool {
	return slices.ContainsFunc(changes, func(held definitions.Change) bool {
		return held.Action == action && held.Subject == subject && held.Group == group && held.Key == key
	})
}

// sectionMovedFile returns the site's export with the steps section moved into a new group on recipes.
func sectionMovedFile(t *testing.T, envelope definitions.Envelope) definitions.Envelope {
	t.Helper()
	recipe := groupNamed(t, envelope, "recipe-details")
	moved := *declaredIn(t, recipe, "steps")
	leftOut(recipe, "steps")
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "recipe-facts", Title: "Recipe facts", Active: true, Location: recipeRules(),
		Fields: []definitions.FieldDefinition{moved},
	})
	return envelope
}

func TestApplyLeavesOutANewGroupEveryFieldOfWhichIsHeldBack(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)

	outcome := applied(t, registry, importing(carriedFile(t, registry, recipeRules())))

	if _, found := storedGroup(t, registry, "recipe-facts"); found {
		t.Errorf("the recipe facts group stands, want it left out with the field it was declared for")
	}
	if _, found := storedField(t, registry, "recipe-details", "cook-time"); !found {
		t.Errorf("the cook time left the recipe group, want the unconfirmed move left undone")
	}
	if !leftUndoneAt(outcome.Skipped, "create", "group", "", "recipe-facts") {
		t.Errorf("skipped = %+v, want the group left out named there", outcome.Skipped)
	}
	if !leftUndoneAt(outcome.Skipped, "create", "field", "recipe-facts", "cook-time") {
		t.Errorf("skipped = %+v, want the held back cook time named there", outcome.Skipped)
	}
}

func TestApplyCreatesANewGroupThatAlsoGainsAFreshField(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := carriedFile(t, registry, recipeRules())
	facts := groupNamed(t, envelope, "recipe-facts")
	facts.Fields = append(facts.Fields, definitions.FieldDefinition{Key: "yield", Label: "Yield", Kind: "text"})

	outcome := applied(t, registry, importing(envelope))

	stored, found := storedGroup(t, registry, "recipe-facts")
	if keys := keysOfFields(stored.Fields); !found || !slices.Equal(keys, []string{"yield"}) {
		t.Errorf("the recipe facts group = %v, %v, want it standing with the fresh field alone", keys, found)
	}
	if leftUndoneAt(outcome.Skipped, "create", "group", "", "recipe-facts") {
		t.Errorf("skipped = %+v, want the group that gains a field left out of it", outcome.Skipped)
	}
	if !leftUndoneAt(outcome.Skipped, "create", "field", "recipe-facts", "cook-time") {
		t.Errorf("skipped = %+v, want the held back cook time named there", outcome.Skipped)
	}
}

func TestApplyNamesTheFieldsInsideAHeldBackSection(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)

	outcome := applied(t, registry, importing(sectionMovedFile(t, exported(t, registry))))

	for _, key := range []string{"steps", "steps.note"} {
		if !leftUndoneAt(outcome.Skipped, "create", "field", "recipe-facts", key) {
			t.Errorf("skipped = %+v, want %s named among what the import left undone", outcome.Skipped, key)
		}
	}
	if _, found := storedGroup(t, registry, "recipe-facts"); found {
		t.Errorf("the recipe facts group stands, want it left out with the section it was declared for")
	}
}

func TestApplyNamesTheFieldsInsideAReplacementNobodyConfirmed(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	declaredIn(t, groupNamed(t, envelope, "recipe-details"), "steps").Kind = "repeater"

	outcome := applied(t, registry, importing(envelope))

	for _, held := range []struct{ action, key string }{
		{"delete", "steps"}, {"create", "steps"}, {"create", "steps.note"},
	} {
		if !leftUndoneAt(outcome.Skipped, held.action, "field", "recipe-details", held.key) {
			t.Errorf("skipped = %+v, want the %s of %s named there", outcome.Skipped, held.action, held.key)
		}
	}
}
