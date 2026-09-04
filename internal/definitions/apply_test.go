// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// applied performs the import against the registry, failing the test when it is refused.
func applied(t *testing.T, registry *content.Registry, asked definitions.Import) definitions.Outcome {
	t.Helper()
	outcome, err := definitions.Apply(t.Context(), registry, asked)
	if err != nil {
		t.Fatalf("Apply() error = %v, want nil", err)
	}
	return outcome
}

// importing returns the envelope as an import confirming nothing.
func importing(envelope definitions.Envelope) definitions.Import {
	return definitions.Import{Envelope: envelope}
}

// storedGroup returns the stored group carrying the key, reporting false when none does.
func storedGroup(t *testing.T, registry *content.Registry, key string) (content.Group, bool) {
	t.Helper()
	groups, err := registry.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups() error = %v, want nil", err)
	}
	for _, held := range groups {
		if held.Key == key {
			return held, true
		}
	}
	return content.Group{}, false
}

// storedField returns the field the group holds under the key, reporting false when it holds none.
func storedField(t *testing.T, registry *content.Registry, group, key string) (content.Field, bool) {
	t.Helper()
	held, found := storedGroup(t, registry, group)
	if !found {
		return content.Field{}, false
	}
	for _, f := range held.Fields {
		if f.Key == key {
			return f, true
		}
	}
	return content.Field{}, false
}

// named reports whether the changes hold one for the subject and key.
func named(changes []definitions.Change, subject, key string) bool {
	for _, held := range changes {
		if held.Subject == subject && held.Key == key {
			return true
		}
	}
	return false
}

func TestApplyChangesNothingForTheSitesOwnExport(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)

	outcome := applied(t, registry, importing(envelope))

	if len(outcome.Applied) != 0 || len(outcome.Skipped) != 0 {
		t.Fatalf("outcome = %+v, want an export of the site to change nothing", outcome)
	}
	if plan := compared(t, registry, exported(t, registry)); len(plan.Changes) != 0 {
		t.Errorf("plan after applying = %+v, want the site standing as it was", plan.Changes)
	}
}

func TestApplyAddsTheTypeGroupAndFieldsAFileBrings(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	envelope.Types = append(envelope.Types, definitions.TypeDefinition{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
		RevisionCap: 20, PageKind: "single", Active: true,
	})
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "event-details", Title: "Event details", Active: true,
		Location: content.Rules{{{
			Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "event",
		}}},
		Fields: []definitions.FieldDefinition{
			{Key: "venue", Label: "Venue", Kind: "text"},
			{Key: "schedule", Label: "Schedule", Kind: "section", Fields: []definitions.FieldDefinition{
				{Key: "doors", Label: "Doors", Kind: "date"},
			}},
		},
	})

	outcome := applied(t, registry, importing(envelope))

	if len(outcome.Skipped) != 0 {
		t.Errorf("skipped = %+v, want nothing left undone", outcome.Skipped)
	}
	if _, err := registry.ByKey(t.Context(), "event"); err != nil {
		t.Errorf("ByKey(event) error = %v, want the type the file brought", err)
	}
	if _, found := storedField(t, registry, "event-details", "venue"); !found {
		t.Errorf("the venue field is missing, want it stored in the event group")
	}
	section, found := storedField(t, registry, "event-details", "schedule")
	if !found || len(section.Fields) != 1 || section.Fields[0].Key != "doors" {
		t.Errorf("the schedule section = %+v, %v, want it holding the doors field", section, found)
	}
}

func TestApplyLeavesBehindADeleteNobodyConfirmed(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	envelope.Groups = envelope.Groups[:0]

	outcome := applied(t, registry, importing(envelope))

	if !named(outcome.Skipped, "group", "recipe-details") {
		t.Errorf("skipped = %+v, want the unconfirmed delete named there", outcome.Skipped)
	}
	if _, found := storedGroup(t, registry, "recipe-details"); !found {
		t.Errorf("the recipe group is gone, want an import to take nothing away unasked")
	}
}

func TestApplyTakesAwayWhatTheAdminConfirmed(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	envelope.Groups = envelope.Groups[:0]

	outcome := applied(t, registry, definitions.Import{
		Envelope: envelope,
		Confirm: []definitions.Confirmed{
			{Subject: "group", Key: "recipe-details"},
			{Subject: "group", Key: "loose-ends"},
		},
	})

	if len(outcome.Skipped) != 0 {
		t.Errorf("skipped = %+v, want both confirmed deletes done", outcome.Skipped)
	}
	if _, found := storedGroup(t, registry, "recipe-details"); found {
		t.Errorf("the recipe group stands, want the confirmed delete to have taken it")
	}
}

func TestApplyReplacesAFieldWhoseKindChanged(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	groupNamed(t, envelope, "recipe-details").Fields[0].Kind = "number"

	applied(t, registry, definitions.Import{
		Envelope: envelope,
		Confirm:  []definitions.Confirmed{{Subject: "field", Key: "cook-time", Group: "recipe-details"}},
	})

	held, found := storedField(t, registry, "recipe-details", "cook-time")
	if !found || held.Kind != content.FieldKindNumber {
		t.Errorf("the cook time field = %+v, %v, want it standing under its new kind", held, found)
	}
}

func TestApplyLeavesAReplacementNobodyConfirmed(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	groupNamed(t, envelope, "recipe-details").Fields[0].Kind = "number"

	outcome := applied(t, registry, importing(envelope))

	held, found := storedField(t, registry, "recipe-details", "cook-time")
	if !found || held.Kind != content.FieldKindText {
		t.Errorf("the cook time field = %+v, %v, want the stored kind kept", held, found)
	}
	if !named(outcome.Skipped, "field", "cook-time") {
		t.Errorf("skipped = %+v, want the unconfirmed replacement named there", outcome.Skipped)
	}
}

func TestApplyCarriesALabelOntoAStoredField(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	group := groupNamed(t, envelope, "recipe-details")
	group.Title = "Recipe facts"
	group.Fields[0].Label = "Time in the oven"
	group.Fields[1].Fields[0].Label = "Remark"

	applied(t, registry, importing(envelope))

	held, found := storedGroup(t, registry, "recipe-details")
	if !found || held.Title != "Recipe facts" {
		t.Errorf("the recipe group = %+v, %v, want the carried title", held, found)
	}
	field, found := storedField(t, registry, "recipe-details", "cook-time")
	if !found || field.Label != "Time in the oven" {
		t.Errorf("the cook time field = %+v, %v, want the carried label", field, found)
	}
	section, _ := storedField(t, registry, "recipe-details", "steps")
	if len(section.Fields) != 1 || section.Fields[0].Label != "Remark" {
		t.Errorf("the steps section = %+v, want the carried label on the field inside it", section.Fields)
	}
}

func TestApplyRunsTwiceAndAsksForNothingTheSecondTime(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	envelope.Types = append(envelope.Types, definitions.TypeDefinition{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
		RevisionCap: 20, PageKind: "single", Active: true,
	})
	envelope.Groups = append(envelope.Groups, definitions.GroupDefinition{
		Key: "event-details", Title: "Event details", Active: true,
		Location: content.Rules{{{
			Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "event",
		}}},
		Fields: []definitions.FieldDefinition{{Key: "venue", Label: "Venue", Kind: "text"}},
	})

	applied(t, registry, importing(envelope))
	outcome := applied(t, registry, importing(envelope))

	if len(outcome.Applied) != 0 {
		t.Errorf("the second run applied %+v, want a file already stored to ask for nothing", outcome.Applied)
	}
}

func TestApplyLeavesTheRootWhereTheSiteHasIt(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].Default, envelope.Types[i].RouteWord = true, ""
			envelope.Types[i].SingularLabel = "Dish"
		}
		if envelope.Types[i].Key == content.TypePost {
			envelope.Types[i].Default, envelope.Types[i].RouteWord = false, "posts"
		}
	}

	outcome := applied(t, registry, importing(envelope))

	if !named(outcome.Skipped, "type", "recipe") {
		t.Errorf("skipped = %+v, want the root move left for the types screen", outcome.Skipped)
	}
	post, err := registry.ByKey(t.Context(), content.TypePost)
	if err != nil || !post.Default {
		t.Errorf("the post type = %+v, %v, want it still holding the root", post, err)
	}
	recipe, err := registry.ByKey(t.Context(), "recipe")
	if err != nil || recipe.SingularLabel != "Dish" {
		t.Errorf("the recipe type = %+v, %v, want its label carried even so", recipe, err)
	}
}

func TestApplyRefusesAFileTouchingWhatAPluginDeclared(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))

	_, err := definitions.Apply(t.Context(), registry, importing(definitions.Envelope{
		Format: definitions.Format,
		Groups: []definitions.GroupDefinition{{Key: "event-details", Title: "Event details"}},
	}))

	if !errors.Is(err, content.ErrDefinitionReadOnly) {
		t.Errorf("Apply() error = %v, want %v", err, content.ErrDefinitionReadOnly)
	}
}
