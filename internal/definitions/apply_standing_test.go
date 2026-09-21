// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
)

// refusedWith reports whether the error carries the code and names the field and the group in its details.
func refusedWith(err error, code, field, group string) bool {
	held, _ := content.CodeOf(err)
	details, _ := content.DetailsOf(err)
	return held == code && details["field"] == field && details["group"] == group
}

// wineRules places a group on the wine type.
func wineRules() content.Rules {
	return content.Rules{{{Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "wine"}}}
}

// wineDefinition returns the wine type as a file declares it.
func wineDefinition() definitions.TypeDefinition {
	return definitions.TypeDefinition{
		Key: "wine", SingularLabel: "Wine", PluralLabel: "Wines", RouteWord: "wines",
		RevisionCap: 20, PageKind: "single", Active: true,
	}
}

// wineSite returns the planning site with a wine type and a group holding a relation pointing at wines.
func wineSite(t *testing.T) *content.Registry {
	t.Helper()
	registry := planningSite(t)
	wine, err := content.NewType("wine", "Wine", "Wines", "wines")
	if err != nil {
		t.Fatalf("NewType(wine) error = %v, want nil", err)
	}
	if _, err := registry.Create(t.Context(), wine); err != nil {
		t.Fatalf("Create(wine) error = %v, want nil", err)
	}
	recipeGroup(t, registry, "recipe-links", "Recipe links", content.Field{
		Key: "wine-pairing", Label: "Wine pairing", Kind: content.FieldKindRelation, RelatesTo: "wine",
	})
	return registry
}

// notingSite returns the planning site with a group on recipes holding a wine note.
func notingSite(t *testing.T) *content.Registry {
	t.Helper()
	registry := planningSite(t)
	recipeGroup(t, registry, "recipe-links", "Recipe links", content.Field{
		Key: "wine-note", Label: "Wine note", Kind: content.FieldKindText,
	})
	return registry
}

// withoutWine returns the file with the wine type left out.
func withoutWine(envelope definitions.Envelope) definitions.Envelope {
	envelope.Types = slices.DeleteFunc(envelope.Types, func(d definitions.TypeDefinition) bool {
		return d.Key == "wine"
	})
	return envelope
}

// rebuiltFile returns the site's export with the links group dropped and its wine note standing in a new group.
func rebuiltFile(t *testing.T, registry *content.Registry) definitions.Envelope {
	t.Helper()
	envelope := exported(t, registry)
	envelope.Groups = slices.DeleteFunc(envelope.Groups, func(g definitions.GroupDefinition) bool {
		return g.Key == "recipe-links"
	})
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "recipe-pairs", Title: "Recipe pairs", Active: true, Location: recipeRules(),
		Fields: []definitions.FieldDefinition{{Key: "wine-note", Label: "Wine note", Kind: "text"}},
	})
	return envelope
}

func TestApplyRefusesADeclaredReaderOfAMoveNobodyConfirmed(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	moved := *declaredIn(t, recipe, "cook-time")
	leftOut(recipe, "cook-time")
	loose := groupNamed(t, envelope, "loose-ends")
	loose.Title = "Odds and ends"
	loose.Fields = append(loose.Fields, moved, definitions.FieldDefinition{
		Key: "serving", Label: "Serving", Kind: "text", Settings: readingWhenSet("cook-time"),
	})

	_, err := definitions.Apply(t.Context(), registry, importing(envelope))

	if !refusedAsRead(err, "cook-time", "serving") {
		t.Errorf("Apply() error = %v, want the held back cook time named as read by serving", err)
	}
	stored, _ := storedGroup(t, registry, "loose-ends")
	if stored.Title != "Loose ends" || len(stored.Fields) != 0 {
		t.Errorf("the loose ends group = %+v, want the refused import to have written nothing", stored)
	}
}

func TestApplyRefusesADeclaredReaderInsideASectionOfAMoveNobodyConfirmed(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	section := declaredIn(t, groupNamed(t, envelope, "recipe-details"), "steps")
	moved := section.Fields
	section.Fields = nil
	loose := groupNamed(t, envelope, "loose-ends")
	loose.Fields = append(loose.Fields, definitions.FieldDefinition{
		Key: "steps", Label: "Steps", Kind: "section",
		Fields: append(moved, definitions.FieldDefinition{
			Key: "tip", Label: "Tip", Kind: "text", Settings: readingWhenSet("note"),
		}),
	})

	_, err := definitions.Apply(t.Context(), registry, importing(envelope))

	if !refusedAsRead(err, "note", "tip") {
		t.Errorf("Apply() error = %v, want the held back note named as read by the tip beside it", err)
	}
	if _, found := storedField(t, registry, "loose-ends", "steps"); found {
		t.Errorf("the loose ends group holds a steps section, want the refused import to have written nothing")
	}
}

func TestApplyRefusesADeclaredBacklinksReadingAMoveNobodyConfirmed(t *testing.T) {
	t.Parallel()

	registry := pairingSite(t)
	envelope := exported(t, registry)
	links := groupNamed(t, envelope, "recipe-links")
	moved := *declaredIn(t, links, "pairs-with")
	leftOut(links, "pairs-with")
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "recipe-pairs", Title: "Recipe pairs", Active: true, Location: recipeRules(),
		Fields: []definitions.FieldDefinition{moved, {
			Key: "paired-from", Label: "Paired from", Kind: "backlinks",
			Settings: map[string]any{
				content.SettingSourceGroup: "recipe-pairs", content.SettingSourceField: []any{"pairs-with"},
			},
		}},
	})

	_, err := definitions.Apply(t.Context(), registry, importing(envelope))

	if !refusedAsRead(err, "pairs-with", "paired-from") {
		t.Errorf("Apply() error = %v, want the held back relation named as read by the backlinks", err)
	}
	if _, found := storedGroup(t, registry, "recipe-pairs"); found {
		t.Errorf("the recipe pairs group stands, want the refused import to have written nothing")
	}
}

func TestApplyRefusesATakenTypeARelationItKeepsStillPointsAt(t *testing.T) {
	t.Parallel()

	registry := wineSite(t)
	envelope := withoutWine(exported(t, registry))
	groupNamed(t, envelope, "recipe-details").Title = "Recipe facts"

	_, err := definitions.Apply(t.Context(), registry, definitions.Import{
		Envelope: envelope, Confirm: []definitions.Confirmed{{Subject: "type", Key: "wine"}},
	})

	if !refusedWith(err, "type_targeted", "wine-pairing", "Recipe links") {
		t.Errorf("Apply() error = %v, want the wine type kept while the wine pairing points at it", err)
	}
	stored, _ := storedGroup(t, registry, "recipe-details")
	if stored.Title != "Recipe details" {
		t.Errorf("the recipe group is titled %q, want the refused import to have written nothing", stored.Title)
	}
}

func TestApplyRefusesATakenTypeARelationInAKeptGroupStillPointsAt(t *testing.T) {
	t.Parallel()

	registry := wineSite(t)
	envelope := withoutWine(exported(t, registry))
	envelope.Groups = slices.DeleteFunc(envelope.Groups, func(g definitions.GroupDefinition) bool {
		return g.Key == "recipe-links"
	})
	groupNamed(t, envelope, "recipe-details").Title = "Recipe facts"

	_, err := definitions.Apply(t.Context(), registry, definitions.Import{
		Envelope: envelope, Confirm: []definitions.Confirmed{{Subject: "type", Key: "wine"}},
	})

	if !refusedWith(err, "type_targeted", "wine-pairing", "Recipe links") {
		t.Errorf("Apply() error = %v, want the wine type kept while the unremoved group points at it", err)
	}
	stored, _ := storedGroup(t, registry, "recipe-details")
	if stored.Title != "Recipe details" {
		t.Errorf("the recipe group is titled %q, want the refused import to have written nothing", stored.Title)
	}
}

func TestApplyMovesABacklinksGroupToTheTypeItsRelationNowPointsAt(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	envelope.Types = append(envelope.Types, wineDefinition())
	groupNamed(t, envelope, "recipe-backlinks").Location = wineRules()
	declaredIn(t, groupNamed(t, envelope, "recipe-links"), "pairs-with").RelatesTo = "wine"

	applied(t, registry, confirmingFields(envelope, "recipe-links", "pairs-with"))

	moved, _ := storedGroup(t, registry, "recipe-backlinks")
	if !moved.Location.Equal(wineRules()) {
		t.Errorf("the backlinks group is placed by %v, want it moved to wines", moved.Location)
	}
	relation, _ := storedField(t, registry, "recipe-links", "pairs-with")
	if relation.RelatesTo != "wine" {
		t.Errorf("pairs-with points at %q, want the file's wine target standing", relation.RelatesTo)
	}
}

func TestApplyMovesABacklinksGroupAndPointsItAtARelationOnItsNewType(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	envelope.Types = append(envelope.Types, wineDefinition())
	backlinks := groupNamed(t, envelope, "recipe-backlinks")
	backlinks.Location = wineRules()
	declaredIn(t, backlinks, "linked-from").Settings = map[string]any{
		content.SettingSourceGroup: "wine-links", content.SettingSourceField: []any{"goes-with"},
	}
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "wine-links", Title: "Wine links", Active: true, Location: recipeRules(),
		Fields: []definitions.FieldDefinition{{
			Key: "goes-with", Label: "Goes with", Kind: "relation", RelatesTo: "wine",
		}},
	})

	applied(t, registry, importing(envelope))

	reader, _ := storedField(t, registry, "recipe-backlinks", "linked-from")
	if got := content.SourceGroupOf(reader); got != "wine-links" {
		t.Errorf("the backlinks reads the group %q, want the wine links it was pointed at", got)
	}
}

func TestApplyMovesABacklinksGroupWhileTakingItsBacklinksAway(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	envelope.Types = append(envelope.Types, wineDefinition())
	backlinks := groupNamed(t, envelope, "recipe-backlinks")
	backlinks.Location = wineRules()
	leftOut(backlinks, "linked-from")

	applied(t, registry, confirmingFields(envelope, "recipe-backlinks", "linked-from"))

	moved, _ := storedGroup(t, registry, "recipe-backlinks")
	if !moved.Location.Equal(wineRules()) || len(moved.Fields) != 0 {
		t.Errorf("the backlinks group = %+v, want it moved to wines with the backlinks taken away", moved)
	}
}

func TestApplyKeepsABacklinksWhoseRemovalNobodyConfirmedWhileRetitlingItsGroup(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	backlinks := groupNamed(t, envelope, "recipe-backlinks")
	backlinks.Title = "Recipe mentions"
	leftOut(backlinks, "linked-from")

	outcome := applied(t, registry, importing(envelope))

	stored, _ := storedGroup(t, registry, "recipe-backlinks")
	if stored.Title != "Recipe mentions" || len(stored.Fields) != 1 {
		t.Errorf("the backlinks group = %+v, want it retitled with the unconfirmed removal left undone", stored)
	}
	if !named(outcome.Skipped, "field", "linked-from") {
		t.Errorf("skipped = %+v, want the unconfirmed removal named there", outcome.Skipped)
	}
}

func TestApplyRefusesABacklinksGroupMovedAheadOfAReplacementNobodyConfirmed(t *testing.T) {
	t.Parallel()

	registry := linkingSite(t)
	envelope := exported(t, registry)
	envelope.Types = append(envelope.Types, wineDefinition())
	groupNamed(t, envelope, "recipe-backlinks").Location = wineRules()
	declaredIn(t, groupNamed(t, envelope, "recipe-links"), "pairs-with").RelatesTo = "wine"

	_, err := definitions.Apply(t.Context(), registry, importing(envelope))

	if code, _ := content.CodeOf(err); code != "backlinks_source_elsewhere" {
		t.Errorf("Apply() error = %v, want the backlinks kept on the type its relation points at", err)
	}
	if _, err := registry.ByKey(t.Context(), "wine"); err == nil {
		t.Errorf("ByKey(wine) found the type, want the refused import to have written nothing")
	}
}

func TestApplyStandsAFieldInANewGroupWhileTakingItsOldGroupAway(t *testing.T) {
	t.Parallel()

	registry := notingSite(t)

	outcome := applied(t, registry, confirmingGroup(rebuiltFile(t, registry), "recipe-links"))

	if _, found := storedField(t, registry, "recipe-pairs", "wine-note"); !found {
		t.Errorf("the recipe pairs group holds no wine note, want it standing once the old group went")
	}
	if _, found := storedGroup(t, registry, "recipe-links"); found {
		t.Errorf("the recipe links group stands, want the confirmed delete to have taken it")
	}
	if !named(outcome.Applied, "group", "recipe-links") {
		t.Errorf("applied = %+v, want the group removal named there", outcome.Applied)
	}
}

func TestApplyRefusesAFieldAKeptGroupStillHoldsBeforeWritingAnything(t *testing.T) {
	t.Parallel()

	registry := notingSite(t)

	_, err := definitions.Apply(t.Context(), registry, importing(rebuiltFile(t, registry)))

	if !refusedWith(err, "field_taken", "wine-note", "Recipe links") {
		t.Errorf("Apply() error = %v, want the wine note refused while the unremoved group holds it", err)
	}
	if _, found := storedGroup(t, registry, "recipe-pairs"); found {
		t.Errorf("the recipe pairs group stands, want the refused import to have written nothing")
	}
}
