// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// planningSite returns a registry over a migrated database holding the site's recipe definitions.
func planningSite(t *testing.T) *content.Registry {
	t.Helper()
	pool, _ := declaringPool(t)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	siteDefined(t, registry)
	return registry
}

// exported returns the envelope the site would download right now.
func exported(t *testing.T, registry *content.Registry) definitions.Envelope {
	t.Helper()
	envelope, err := definitions.Export(t.Context(), registry)
	if err != nil {
		t.Fatalf("Export() error = %v, want nil", err)
	}
	return envelope
}

// compared returns the plan the envelope makes against the registry, failing the test when it is refused.
func compared(t *testing.T, registry *content.Registry, envelope definitions.Envelope) definitions.Plan {
	t.Helper()
	plan, err := definitions.Compare(t.Context(), registry, envelope)
	if err != nil {
		t.Fatalf("Compare() error = %v, want nil", err)
	}
	return plan
}

// groupNamed returns the envelope's group carrying the key, failing the test when it holds none.
func groupNamed(t *testing.T, envelope definitions.Envelope, key string) *definitions.GroupDefinition {
	t.Helper()
	for i := range envelope.Groups {
		if envelope.Groups[i].Key == key {
			return &envelope.Groups[i]
		}
	}
	t.Fatalf("the envelope holds no group %q", key)
	return nil
}

// changeFor returns the plan's change for the subject and key, reporting false when it plans none.
func changeFor(plan definitions.Plan, subject, key string) (definitions.Change, bool) {
	for _, held := range plan.Changes {
		if held.Subject == subject && held.Key == key {
			return held, true
		}
	}
	return definitions.Change{}, false
}

// actionsFor returns every action the plan holds for the subject and key, in plan order.
func actionsFor(plan definitions.Plan, subject, key string) []string {
	actions := make([]string, 0, len(plan.Changes))
	for _, held := range plan.Changes {
		if held.Subject == subject && held.Key == key {
			actions = append(actions, held.Action)
		}
	}
	return actions
}

func TestCompareFindsNothingToDoForTheSitesOwnExport(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)

	plan := compared(t, registry, exported(t, registry))

	if len(plan.Changes) != 0 || len(plan.Warnings) != 0 {
		t.Errorf("plan = %+v, want an export of the site to ask for nothing", plan)
	}
}

func TestComparePlansWhatAFileAddsToTheSite(t *testing.T) {
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

	plan := compared(t, registry, envelope)

	held, planned := changeFor(plan, "type", "event")
	if !planned || held.Action != "create" {
		t.Errorf("the event type = %+v, %v, want a create", held, planned)
	}
	group, planned := changeFor(plan, "group", "event-details")
	if !planned || group.Action != "create" {
		t.Errorf("the event group = %+v, %v, want a create", group, planned)
	}
	field, planned := changeFor(plan, "field", "venue")
	if !planned || field.Action != "create" || field.Group != "event-details" {
		t.Errorf("the venue field = %+v, %v, want a create inside the event group", field, planned)
	}
}

func TestComparePlansWhatAFileCarriesOverTheSite(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	group := groupNamed(t, envelope, "recipe-details")
	group.Title = "Recipe facts"
	group.Fields[0].Label = "Time in the oven"

	plan := compared(t, registry, envelope)

	held, planned := changeFor(plan, "group", "recipe-details")
	if !planned || held.Action != "update" {
		t.Errorf("the recipe group = %+v, %v, want an update", held, planned)
	}
	field, planned := changeFor(plan, "field", "cook-time")
	if !planned || field.Action != "update" {
		t.Errorf("the cook time field = %+v, %v, want an update", field, planned)
	}
}

func TestComparePlansWhatAFileTakesAwayAsDestructive(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	envelope.Groups = envelope.Groups[:0]

	plan := compared(t, registry, envelope)

	held, planned := changeFor(plan, "group", "recipe-details")
	if !planned || held.Action != "delete" || held.Reason != "removed" {
		t.Errorf("the recipe group = %+v, %v, want a delete because the file dropped it", held, planned)
	}
	if loose, planned := changeFor(plan, "group", "loose-ends"); !planned || loose.Action != "delete" {
		t.Errorf("the ruleless group = %+v, %v, want a delete too", loose, planned)
	}
}

func TestComparePlansAChangedKindAsTakingTheFieldAwayAndAddingItBack(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	groupNamed(t, envelope, "recipe-details").Fields[0].Kind = "number"

	plan := compared(t, registry, envelope)

	if actions := actionsFor(plan, "field", "cook-time"); len(actions) != 2 ||
		actions[0] != "delete" || actions[1] != "create" {
		t.Fatalf("the cook time field actions = %v, want a delete then a create", actions)
	}
	held, _ := changeFor(plan, "field", "cook-time")
	if held.Reason != "kind_changed" {
		t.Errorf("reason = %q, want kind_changed", held.Reason)
	}
}

func TestComparePlansAMovedFieldAsTakingItAwayAndAddingItBack(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	moved := recipe.Fields[0]
	recipe.Fields = recipe.Fields[1:]
	loose := groupNamed(t, envelope, "loose-ends")
	loose.Fields = append(loose.Fields, moved)

	plan := compared(t, registry, envelope)

	held, planned := changeFor(plan, "field", "cook-time")
	if !planned || held.Action != "delete" || held.Reason != "moved" {
		t.Errorf("the cook time field = %+v, %v, want a delete naming the move", held, planned)
	}
}

func TestComparePlansADroppedFieldAsRemovedWhenNoGroupTakesItOn(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	recipe := groupNamed(t, envelope, "recipe-details")
	recipe.Fields = recipe.Fields[1:]

	plan := compared(t, registry, envelope)

	held, planned := changeFor(plan, "field", "cook-time")
	if !planned || held.Action != "delete" || held.Reason != "removed" {
		t.Errorf("the cook time field = %+v, %v, want a delete naming the removal", held, planned)
	}
}

func TestComparePlansAChangedShapeAsTakingTheFieldAwayAndAddingItBack(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	group, err := registry.CreateGroup(t.Context(), content.Group{Title: "Recipe media", Location: recipeRules()})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), group.ID, content.Field{
		Key: "photos", Label: "Photos", Kind: content.FieldKindMedia, Many: true,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	groupNamed(t, envelope, "recipe-media").Fields[0].Many = false

	plan := compared(t, registry, envelope)

	held, planned := changeFor(plan, "field", "photos")
	if !planned || held.Action != "delete" || held.Reason != "shape_changed" {
		t.Errorf("the photos field = %+v, %v, want a delete naming the changed shape", held, planned)
	}
}

func TestComparePlansAGroupUnderANewKeyWithoutCallingItACollision(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	groupNamed(t, envelope, "recipe-details").Key = "recipe-facts"

	plan := compared(t, registry, envelope)

	held, planned := changeFor(plan, "group", "recipe-facts")
	if !planned || held.Action != "create" {
		t.Errorf("the renamed group = %+v, %v, want a create", held, planned)
	}
	gone, planned := changeFor(plan, "group", "recipe-details")
	if !planned || gone.Action != "delete" {
		t.Errorf("the old key = %+v, %v, want a delete beside it", gone, planned)
	}
}

func TestCompareRefusesAGroupWithNoKeyAtAll(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)

	_, err := definitions.Compare(t.Context(), registry, definitions.Envelope{
		Format: definitions.Format,
		Groups: []definitions.GroupDefinition{{Title: "Recipe details"}},
	})

	if !errors.Is(err, content.ErrInvalidGroupKey) {
		t.Errorf("Compare() error = %v, want %v", err, content.ErrInvalidGroupKey)
	}
}

func TestCompareRefusesFieldsNestedDeeperThanTheSiteStores(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	deepest := definitions.FieldDefinition{Key: "note", Label: "Note", Kind: "text"}
	for range content.MaxFieldDepth + 1 {
		deepest = definitions.FieldDefinition{
			Key: "wrap", Label: "Wrap", Kind: "section", Fields: []definitions.FieldDefinition{deepest},
		}
	}

	_, err := definitions.Compare(t.Context(), registry, definitions.Envelope{
		Format: definitions.Format,
		Groups: []definitions.GroupDefinition{{
			Key: "deep-ends", Title: "Deep ends", Fields: []definitions.FieldDefinition{deepest},
		}},
	})

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("Compare() error = %v, want %v", err, content.ErrFieldTooDeep)
	}
}

func TestCompareRefusesTwoGroupsMeetingOnOneTypeOverOneFieldKey(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	shared := []definitions.FieldDefinition{{Key: "cook-time", Label: "Cook time", Kind: "text"}}

	_, err := definitions.Compare(t.Context(), registry, definitions.Envelope{
		Format: definitions.Format,
		Types:  exported(t, registry).Types,
		Groups: []definitions.GroupDefinition{
			{Key: "recipe-details", Title: "Recipe details", Active: true, Location: recipeRules(), Fields: shared},
			{Key: "recipe-extras", Title: "Recipe extras", Active: true, Location: recipeRules(), Fields: shared},
		},
	})

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("Compare() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

func TestComparePlansAFieldInsideASectionUnderItsDottedPath(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	groupNamed(t, envelope, "recipe-details").Fields[1].Fields[0].Label = "Remark"

	plan := compared(t, registry, envelope)

	held, planned := changeFor(plan, "field", "steps.note")
	if !planned || held.Action != "update" || held.Group != "recipe-details" {
		t.Errorf("the note field = %+v, %v, want an update under its dotted path", held, planned)
	}
}

func TestCompareWarnsWhenAFileMovesTheRootOrTheRouteWord(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].RouteWord = "dishes"
		}
	}

	plan := compared(t, registry, envelope)

	warned := false
	for _, held := range plan.Warnings {
		if held.Code == "route_word_changed" && held.Key == "recipe" {
			warned = true
		}
	}
	if !warned {
		t.Errorf("warnings = %+v, want the changed route word surfaced", plan.Warnings)
	}
}

func TestCompareWarnsWhenAFileHandsTheRootToAnotherType(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].Default, envelope.Types[i].RouteWord = true, ""
		}
		if envelope.Types[i].Key == content.TypePost {
			envelope.Types[i].Default, envelope.Types[i].RouteWord = false, "posts"
		}
	}

	plan := compared(t, registry, envelope)

	warned := false
	for _, held := range plan.Warnings {
		if held.Code == "root_moved" && held.Key == "recipe" {
			warned = true
		}
	}
	if !warned {
		t.Errorf("warnings = %+v, want the moved root surfaced", plan.Warnings)
	}
}

func TestCompareRefusesAFileNamingTwoTypesAsTheRoot(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].Default, envelope.Types[i].RouteWord = true, ""
		}
	}

	_, err := definitions.Compare(t.Context(), registry, envelope)

	if !errors.Is(err, content.ErrRootTaken) {
		t.Errorf("Compare() error = %v, want %v", err, content.ErrRootTaken)
	}
}

func TestCompareRefusesAnEnvelopeUnderAFormatItCannotRead(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)

	_, err := definitions.Compare(t.Context(), registry, definitions.Envelope{Format: "tomorrow"})

	if !errors.Is(err, definitions.ErrFormatUnreadable) {
		t.Errorf("Compare() error = %v, want %v", err, definitions.ErrFormatUnreadable)
	}
}

func TestCompareRefusesAFileTouchingWhatAPluginDeclared(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	envelope := definitions.Envelope{
		Format: definitions.Format,
		Types: []definitions.TypeDefinition{{
			Key: "event", SingularLabel: "Gathering", PluralLabel: "Gatherings", RouteWord: "events",
			PageKind: "single", Active: true,
		}},
	}

	_, err := definitions.Compare(t.Context(), registry, envelope)

	if !errors.Is(err, content.ErrDefinitionReadOnly) {
		t.Fatalf("Compare() error = %v, want %v", err, content.ErrDefinitionReadOnly)
	}
	held, _ := content.DetailsOf(err)
	if held["origin"] != "events" {
		t.Errorf("origin = %v, want the plugin that declared the type", held["origin"])
	}
}

func TestCompareRefusesAFileTouchingAGroupAPluginDeclared(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))

	_, err := definitions.Compare(t.Context(), registry, definitions.Envelope{
		Format: definitions.Format,
		Groups: []definitions.GroupDefinition{{Key: "event-details", Title: "Event details"}},
	})

	if !errors.Is(err, content.ErrDefinitionReadOnly) {
		t.Fatalf("Compare() error = %v, want %v", err, content.ErrDefinitionReadOnly)
	}
	held, _ := content.DetailsOf(err)
	if held["origin"] != "events" {
		t.Errorf("origin = %v, want the plugin that declared the group", held["origin"])
	}
}

func TestCompareLeavesWhatAPluginDeclaredOutOfTheDeletes(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))

	plan := compared(t, registry, definitions.Envelope{Format: definitions.Format})

	for _, held := range plan.Changes {
		if held.Key == "event" || held.Key == "event-details" || held.Key == "venue" {
			t.Errorf("plan holds %+v, want the plugin's own definitions left alone", held)
		}
	}
}

func TestCompareRefusesAnEnvelopeTheSiteCouldNotStore(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]struct {
		envelope definitions.Envelope
		want     error
	}{
		"a type key no site may carry": {
			definitions.Envelope{Format: definitions.Format, Types: []definitions.TypeDefinition{{
				Key: "Recipe Box", SingularLabel: "R", PluralLabel: "Rs", RouteWord: "rs", PageKind: "single",
			}}},
			content.ErrInvalidKey,
		},
		"a group key no site may carry": {
			definitions.Envelope{Format: definitions.Format, Groups: []definitions.GroupDefinition{{
				Key: "Recipe Details", Title: "Recipe details",
			}}},
			content.ErrInvalidGroupKey,
		},
		"a group with no title": {
			definitions.Envelope{Format: definitions.Format, Groups: []definitions.GroupDefinition{{
				Key: "recipe-details", Title: "  ",
			}}},
			content.ErrInvalidGroupTitle,
		},
		"a field kind nothing holds": {
			definitions.Envelope{Format: definitions.Format, Groups: []definitions.GroupDefinition{{
				Key: "recipe-details", Title: "Recipe details",
				Fields: []definitions.FieldDefinition{{Key: "cook-time", Label: "Cook time", Kind: "sonnet"}},
			}}},
			content.ErrInvalidFieldKind,
		},
		"a sub field under a kind that holds none": {
			definitions.Envelope{Format: definitions.Format, Groups: []definitions.GroupDefinition{{
				Key: "recipe-details", Title: "Recipe details",
				Fields: []definitions.FieldDefinition{{
					Key: "cook-time", Label: "Cook time", Kind: "text",
					Fields: []definitions.FieldDefinition{{Key: "note", Label: "Note", Kind: "text"}},
				}},
			}}},
			content.ErrFieldShape,
		},
		"a setting the kind does not take": {
			definitions.Envelope{Format: definitions.Format, Groups: []definitions.GroupDefinition{{
				Key: "recipe-details", Title: "Recipe details",
				Fields: []definitions.FieldDefinition{{
					Key: "cook-time", Label: "Cook time", Kind: "text",
					Settings: map[string]any{"surprise": true},
				}},
			}}},
			content.ErrSettingUnknown,
		},
		"a rule source nothing declares": {
			definitions.Envelope{Format: definitions.Format, Groups: []definitions.GroupDefinition{{
				Key: "recipe-details", Title: "Recipe details",
				Location: content.Rules{{{Source: "phase-of-the-moon", Operator: "==", Value: "waxing"}}},
			}}},
			content.ErrRuleSourceUnknown,
		},
		"the same type key twice": {
			definitions.Envelope{Format: definitions.Format, Types: []definitions.TypeDefinition{
				{Key: "recipe", SingularLabel: "R", PluralLabel: "Rs", RouteWord: "rs", PageKind: "single"},
				{Key: "recipe", SingularLabel: "R", PluralLabel: "Rs", RouteWord: "rs", PageKind: "single"},
			}},
			content.ErrTypeTaken,
		},
		"the same group key twice": {
			definitions.Envelope{Format: definitions.Format, Groups: []definitions.GroupDefinition{
				{Key: "recipe-details", Title: "Recipe details"},
				{Key: "recipe-details", Title: "Recipe details"},
			}},
			content.ErrGroupKeyTaken,
		},
		"the same field key twice under one group": {
			definitions.Envelope{Format: definitions.Format, Groups: []definitions.GroupDefinition{{
				Key: "recipe-details", Title: "Recipe details",
				Fields: []definitions.FieldDefinition{
					{Key: "cook-time", Label: "Cook time", Kind: "text"},
					{Key: "cook-time", Label: "Cook time", Kind: "text"},
				},
			}}},
			content.ErrFieldTaken,
		},
		"a relation naming a type nothing holds": {
			definitions.Envelope{Format: definitions.Format, Groups: []definitions.GroupDefinition{{
				Key: "recipe-details", Title: "Recipe details",
				Fields: []definitions.FieldDefinition{{
					Key: "pairs-with", Label: "Pairs with", Kind: "relation", RelatesTo: "wine", Many: true,
				}},
			}}},
			content.ErrTargetUnknown,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			registry := planningSite(t)

			_, err := definitions.Compare(t.Context(), registry, held.envelope)

			if !errors.Is(err, held.want) {
				t.Errorf("%s: Compare() error = %v, want %v", name, err, held.want)
			}
		})
	}
}

func TestCompareReportsARegistryItCannotRead(t *testing.T) {
	t.Parallel()

	for name, statement := range map[string]string{
		"types behind an unreadable table":  "ALTER TABLE core.content_types RENAME COLUMN singular_label TO retired",
		"groups behind an unreadable table": "ALTER TABLE core.field_groups RENAME COLUMN title TO retired",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, _ := declaringPool(t)
			sabotage(t, pool, statement)
			registry := content.NewRegistry(postgres.NewTypeStore(pool))

			_, err := definitions.Compare(context.Background(), registry, definitions.Envelope{Format: definitions.Format})

			if err == nil {
				t.Errorf("%s: error = nil, want the unreadable store reported", name)
			}
		})
	}
}
