// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// keysOfFields returns the keys the stored fields stand in, in order.
func keysOfFields(fields []content.Field) []string {
	keys := make([]string, len(fields))
	for i, f := range fields {
		keys[i] = f.Key
	}
	return keys
}

// keysOfGroups returns the keys the stored groups stand in, in order.
func keysOfGroups(t *testing.T, registry *content.Registry) []string {
	t.Helper()
	groups, err := registry.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups() error = %v, want nil", err)
	}
	keys := make([]string, len(groups))
	for i, g := range groups {
		keys[i] = g.Key
	}
	return keys
}

func TestApplyMovesAFieldTheAdminConfirmed(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	moved := recipe.Fields[0]
	recipe.Fields = recipe.Fields[1:]
	loose := groupNamed(t, envelope, "loose-ends")
	loose.Fields = append(loose.Fields, moved)

	applied(t, registry, definitions.Import{
		Envelope: envelope,
		Confirm:  []definitions.Confirmed{{Subject: "field", Key: "cook-time", Group: "recipe-details"}},
	})

	if _, found := storedField(t, registry, "recipe-details", "cook-time"); found {
		t.Errorf("the cook time field stands in the recipe group, want it moved out")
	}
	if _, found := storedField(t, registry, "loose-ends", "cook-time"); !found {
		t.Errorf("the cook time field is missing from the loose ends group, want it moved in")
	}
}

func TestApplyLeavesAMoveNobodyConfirmed(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	moved := recipe.Fields[0]
	recipe.Fields = recipe.Fields[1:]
	loose := groupNamed(t, envelope, "loose-ends")
	loose.Fields = append(loose.Fields, moved)

	outcome := applied(t, registry, importing(envelope))

	if _, found := storedField(t, registry, "recipe-details", "cook-time"); !found {
		t.Errorf("the cook time field left the recipe group, want the unconfirmed move refused")
	}
	if _, found := storedField(t, registry, "loose-ends", "cook-time"); found {
		t.Errorf("the cook time field reached the loose ends group, want the unconfirmed move refused")
	}
	if !named(outcome.Skipped, "field", "cook-time") {
		t.Errorf("skipped = %+v, want the unconfirmed move named there", outcome.Skipped)
	}
}

func TestApplyLeavesAnotherGroupsFieldAloneWhenAMoveIsDeclined(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	loose, found := storedGroup(t, registry, "loose-ends")
	if !found {
		t.Fatalf("the loose ends group is missing from the site")
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), loose.ID, content.Field{
		Key: "cook-time", Label: "Cook time", Kind: content.FieldKindText,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	moved := recipe.Fields[0]
	recipe.Fields = recipe.Fields[1:]
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "recipe-extras", Title: "Recipe extras", Active: true, Location: recipeRules(),
		Fields: []definitions.FieldDefinition{moved},
	})
	groupNamed(t, envelope, "loose-ends").Fields[0].Label = "Timer"

	applied(t, registry, importing(envelope))

	held, found := storedField(t, registry, "loose-ends", "cook-time")
	if !found || held.Label != "Timer" {
		t.Errorf("the loose ends field = %+v, %v, want its own label carried", held, found)
	}
	if _, still := storedField(t, registry, "recipe-details", "cook-time"); !still {
		t.Errorf("the recipe field left its group, want the unconfirmed move refused")
	}
}

func TestApplyStandsGroupsAndFieldsInTheOrderTheFileLists(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	steps, found := storedField(t, registry, "recipe-details", "steps")
	if !found {
		t.Fatalf("the steps section is missing from the site")
	}
	if _, err := registry.CreateSubField(t.Context(), steps.ID, content.Field{
		Key: "timing", Label: "Timing", Kind: content.FieldKindText,
	}); err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	envelope.Groups[0], envelope.Groups[1] = envelope.Groups[1], envelope.Groups[0]
	recipe := groupNamed(t, envelope, "recipe-details")
	recipe.Fields[0], recipe.Fields[1] = recipe.Fields[1], recipe.Fields[0]
	inside := groupNamed(t, envelope, "recipe-details")
	inside.Fields[0].Fields[0], inside.Fields[0].Fields[1] =
		inside.Fields[0].Fields[1], inside.Fields[0].Fields[0]

	applied(t, registry, importing(envelope))

	if held := keysOfGroups(t, registry); held[0] != "loose-ends" {
		t.Errorf("group order = %v, want the file's own order", held)
	}
	group, _ := storedGroup(t, registry, "recipe-details")
	if held := keysOfFields(group.Fields); len(held) != 2 || held[0] != "steps" {
		t.Errorf("field order = %v, want the file's own order", held)
	}
	section, _ := storedField(t, registry, "recipe-details", "steps")
	if held := keysOfFields(section.Fields); len(held) != 2 || held[0] != "timing" {
		t.Errorf("order inside the section = %v, want the file's own order", held)
	}
}

func TestApplyRestsAGroupTheFileBringsAlreadyResting(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "quiet-corner", Title: "Quiet corner", Active: false,
	})

	applied(t, registry, importing(envelope))

	held, found := storedGroup(t, registry, "quiet-corner")
	if !found || held.Active {
		t.Errorf("the quiet corner group = %+v, %v, want it stored and resting", held, found)
	}
}

func TestApplyStandsANewDefaultTypeBesideTheRootTheSiteHas(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == content.TypePost {
			envelope.Types[i].Default, envelope.Types[i].RouteWord = false, "posts"
		}
	}
	envelope.Types = append(envelope.Types, definitions.TypeDefinition{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events",
		PageKind: "single", Default: true, Active: true,
	})

	outcome := applied(t, registry, importing(envelope))

	if !named(outcome.Skipped, "type", "event") {
		t.Errorf("skipped = %+v, want the root left where the site has it", outcome.Skipped)
	}
	held, err := registry.ByKey(t.Context(), "event")
	if err != nil || held.Default || held.RouteWord != "events" {
		t.Errorf("the event type = %+v, %v, want it standing beside the root under its own word", held, err)
	}
	post, err := registry.ByKey(t.Context(), content.TypePost)
	if err != nil || !post.Default {
		t.Errorf("the post type = %+v, %v, want it still holding the root", post, err)
	}
}

func TestApplyTakesAwayATypeTheAdminConfirmed(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	envelope.Groups = envelope.Groups[:0]
	kept := make([]definitions.TypeDefinition, 0, len(envelope.Types))
	for _, held := range envelope.Types {
		if held.Key != "recipe" {
			kept = append(kept, held)
		}
	}
	envelope.Types = kept

	applied(t, registry, definitions.Import{
		Envelope: envelope,
		Confirm: []definitions.Confirmed{
			{Subject: "type", Key: "recipe"},
			{Subject: "group", Key: "recipe-details"},
			{Subject: "group", Key: "loose-ends"},
		},
	})

	if _, err := registry.ByKey(t.Context(), "recipe"); err == nil {
		t.Errorf("ByKey(recipe) found the type, want the confirmed delete to have taken it")
	}
}

func TestApplyAddsAndTakesAwayAFieldInsideAStoredSection(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	group := groupNamed(t, envelope, "recipe-details")
	group.Fields[1].Fields = append(group.Fields[1].Fields, definitions.FieldDefinition{
		Key: "timing", Label: "Timing", Kind: "text",
	})

	applied(t, registry, importing(envelope))

	stored, _ := storedField(t, registry, "recipe-details", "steps")
	if len(stored.Fields) != 2 {
		t.Fatalf("the steps section holds %v, want the field the file added inside it", keysOfFields(stored.Fields))
	}
	gone := exported(t, registry)
	groupNamed(t, gone, "recipe-details").Fields[1].Fields = nil

	applied(t, registry, definitions.Import{
		Envelope: gone,
		Confirm: []definitions.Confirmed{
			{Subject: "field", Key: "steps.note", Group: "recipe-details"},
			{Subject: "field", Key: "steps.timing", Group: "recipe-details"},
		},
	})

	emptied, _ := storedField(t, registry, "recipe-details", "steps")
	if len(emptied.Fields) != 0 {
		t.Errorf("the steps section holds %v, want both confirmed deletes done", keysOfFields(emptied.Fields))
	}
}

func TestApplyReportsAStoreItCannotWrite(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]struct {
		table     string
		operation string
		condition string
	}{
		"a group it cannot store": {"core.field_groups", "INSERT", "true"},
		"a group it cannot carry": {"core.field_groups", "UPDATE", "true"},
		"a field it cannot store": {"core.content_fields", "INSERT", "true"},
		"a type it cannot store":  {"core.content_types", "INSERT", "true"},
		"a type it cannot carry":  {"core.content_types", "UPDATE", "true"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, _ := declaringPool(t)
			registry := content.NewRegistry(postgres.NewTypeStore(pool))
			siteDefined(t, registry)
			envelope := exported(t, registry)
			envelope.Types = append(envelope.Types, definitions.TypeDefinition{
				Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
				PageKind: "single", Active: true,
			})
			envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
				Key: "event-details", Title: "Event details", Active: true,
				Fields: []definitions.FieldDefinition{{Key: "venue", Label: "Venue", Kind: "text"}},
			})
			groupNamed(t, envelope, "recipe-details").Title = "Recipe facts"
			for i := range envelope.Types {
				if envelope.Types[i].Key == "recipe" {
					envelope.Types[i].SingularLabel = "Dish"
				}
			}
			raiseOn(t, pool, held.table, held.operation, held.condition)

			_, err := definitions.Apply(t.Context(), content.NewRegistry(postgres.NewTypeStore(pool)),
				importing(envelope))

			if err == nil {
				t.Errorf("%s: Apply() error = nil, want the refused write reported", name)
			}
		})
	}
}
