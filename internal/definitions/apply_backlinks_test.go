// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
)

func TestApplyStoresABacklinksTheFileListsBeforeItsRelation(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	envelope.Groups = append(envelope.Groups,
		definitions.GroupDefinition{
			Key: "recipe-backlinks", Title: "Recipe backlinks", Active: true, Location: recipeRules(),
			Fields: []definitions.FieldDefinition{{
				Key: "linked-from", Label: "Linked from", Kind: "backlinks",
				Settings: map[string]any{"source_group": "recipe-links", "source_field": []any{"pairs-with"}},
			}},
		},
		definitions.GroupDefinition{
			Key: "recipe-links", Title: "Recipe links", Active: true, Location: recipeRules(),
			Fields: []definitions.FieldDefinition{{
				Key: "pairs-with", Label: "Pairs with", Kind: "relation", RelatesTo: "recipe",
			}},
		},
	)

	applied(t, registry, importing(envelope))

	if _, found := storedField(t, registry, "recipe-backlinks", "linked-from"); !found {
		t.Errorf("the backlinks group holds no linked from field, want it stored after its relation")
	}
}

func TestApplyPointsABacklinksAtARelationTheFileMovesLater(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	links := groupNamed(t, envelope, "recipe-links")
	moved := *declaredIn(t, links, "pairs-with")
	leftOut(links, "pairs-with")
	declaredIn(t, groupNamed(t, envelope, "recipe-backlinks"), "linked-from").Settings = map[string]any{
		content.SettingSourceGroup: "recipe-pairs", content.SettingSourceField: []any{"pairs-with"},
	}
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "recipe-pairs", Title: "Recipe pairs", Active: true, Location: recipeRules(),
		Fields: []definitions.FieldDefinition{moved},
	})

	applied(t, registry, confirmingFields(envelope, "recipe-links", "pairs-with"))

	reader, _ := storedField(t, registry, "recipe-backlinks", "linked-from")
	if got := content.SourceGroupOf(reader); got != "recipe-pairs" {
		t.Errorf("the backlinks reads the group %q, want the one the relation moved to", got)
	}
}
