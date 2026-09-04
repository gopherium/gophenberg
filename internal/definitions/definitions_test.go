// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
	"github.com/gopherium/gophenberg/sdk"
)

// declaringRegistry returns a registry over a migrated database and a registrar declaring for the events plugin.
func declaringRegistry(t *testing.T) (*content.Registry, *definitions.Registrar) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	cfg := pgtestdb.Custom(t, testdb.Config(), testdb.Migrator())
	pool, err := pgxpool.New(t.Context(), cfg.URL())
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	t.Cleanup(pool.Close)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	return registry, definitions.New(registry, "events")
}

// eventType is the content type the events plugin declares.
func eventType() sdk.TypeDeclaration {
	return sdk.TypeDeclaration{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
		Revisions: true, RevisionCap: 20, PageKind: "single",
	}
}

// eventGroup is the field group the events plugin declares, holding a text field and a section with one sub field.
func eventGroup() sdk.GroupDeclaration {
	return sdk.GroupDeclaration{
		Key:      "event-details",
		Title:    "Event details",
		Location: [][]sdk.Rule{{{Source: "content_type", Operator: "==", Value: "event"}}},
		Fields: []sdk.FieldDeclaration{
			{Key: "venue", Label: "Venue", Kind: "text", Required: true},
			{Key: "schedule", Label: "Schedule", Kind: "section", Fields: []sdk.FieldDeclaration{
				{Key: "starts-at", Label: "Starts at", Kind: "date"},
			}},
		},
	}
}

// heldGroup returns the stored group carrying the key.
func heldGroup(t *testing.T, registry *content.Registry, key string) content.Group {
	t.Helper()
	groups, err := registry.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups() error = %v, want nil", err)
	}
	for _, g := range groups {
		if g.Key == key {
			return g
		}
	}
	t.Fatalf("Groups() = %+v, want one keyed %q", groups, key)
	return content.Group{}
}

func TestDeclareTypeStoresTheTypeUnderThePluginsOrigin(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)

	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}

	held, err := registry.ByKey(t.Context(), "event")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want the declared type", err)
	}
	if held.Origin != "events" || held.Default || !held.Active || held.RevisionCap != 20 {
		t.Errorf("held = %+v, want an open events type that is never the default", held)
	}
}

func TestDeclareTypeCarriesLabelsAndRefusesARouteWordChange(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)
	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}
	relabeled := eventType()
	relabeled.PluralLabel = "Happenings"
	moved := eventType()
	moved.RouteWord = "happenings"

	if err := registrar.DeclareType(t.Context(), relabeled); err != nil {
		t.Fatalf("DeclareType(relabeled) error = %v, want the label carried", err)
	}
	drift := registrar.DeclareType(t.Context(), moved)

	held, err := registry.ByKey(t.Context(), "event")
	if err != nil || held.PluralLabel != "Happenings" || held.RouteWord != "events" {
		t.Errorf("held = %+v, %v, want the new label and the old route word", held, err)
	}
	if !errors.Is(drift, definitions.ErrRouteWordChanged) {
		t.Errorf("DeclareType(moved) error = %v, want %v", drift, definitions.ErrRouteWordChanged)
	}
}

func TestDeclareTypeSkipsAKeyTheSiteHolds(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)
	site, err := content.NewType("event", "Meetup", "Meetups", "meetups")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	if _, err := registry.Create(t.Context(), site); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	err = registrar.DeclareType(t.Context(), eventType())

	if err != nil {
		t.Fatalf("DeclareType() error = %v, want a collision skipped rather than refused", err)
	}
	if skipped := registrar.Skipped(); !slices.Equal(skipped, []definitions.Held{{Subject: "type", Key: "event"}}) {
		t.Errorf("Skipped() = %v, want the event key recorded", skipped)
	}
	held, _ := registry.ByKey(t.Context(), "event")
	if held.PluralLabel != "Meetups" || held.Origin != "" {
		t.Errorf("held = %+v, want the site's type left as it was", held)
	}
}

func TestDeclareGroupStoresTheGroupAndItsFieldTree(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)
	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}

	if err := registrar.DeclareGroup(t.Context(), eventGroup()); err != nil {
		t.Fatalf("DeclareGroup() error = %v, want nil", err)
	}

	held := heldGroup(t, registry, "event-details")
	if held.Origin != "events" || held.Title != "Event details" || len(held.Fields) != 2 {
		t.Fatalf("held = %+v, want the events group with two fields", held)
	}
	venue, schedule := held.Fields[0], held.Fields[1]
	if venue.Origin != "events" || !venue.Required || schedule.Kind != content.FieldKindSection {
		t.Errorf("fields = %+v, want a required venue and a schedule section under the events origin", held.Fields)
	}
	if len(schedule.Fields) != 1 || schedule.Fields[0].Key != "starts-at" || schedule.Fields[0].Origin != "events" {
		t.Errorf("schedule holds %+v, want the starts-at sub field under the events origin", schedule.Fields)
	}
}

func TestDeclareGroupCarriesChangesAndKeepsWhatIsNoLongerDeclared(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)
	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}
	if err := registrar.DeclareGroup(t.Context(), eventGroup()); err != nil {
		t.Fatalf("DeclareGroup(first) error = %v, want nil", err)
	}
	changed := eventGroup()
	changed.Title = "Event info"
	changed.Fields = []sdk.FieldDeclaration{{Key: "venue", Label: "Where", Kind: "text"}}

	if err := registrar.DeclareGroup(t.Context(), changed); err != nil {
		t.Fatalf("DeclareGroup(changed) error = %v, want nil", err)
	}

	groups, err := registry.Groups(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("Groups() = %d groups, %v, want the one group declared twice", len(groups), err)
	}
	held := groups[0]
	if held.Title != "Event info" || held.Fields[0].Label != "Where" || held.Fields[0].Required {
		t.Errorf("held = %+v, want the title and the venue label carried and required dropped", held)
	}
	if len(held.Fields) != 2 || held.Fields[1].Key != "schedule" {
		t.Errorf("fields = %+v, want the schedule the plugin stopped declaring kept", held.Fields)
	}
}

func TestDeclareTypeLeavesAnUnchangedTypeAlone(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)
	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType(first) error = %v, want nil", err)
	}
	first, err := registry.ByKey(t.Context(), "event")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}

	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType(again) error = %v, want nil", err)
	}

	again, err := registry.ByKey(t.Context(), "event")
	if err != nil || !again.UpdatedAt.Equal(first.UpdatedAt) {
		t.Errorf("UpdatedAt moved from %v to %v (%v), want an unchanged type left untouched",
			first.UpdatedAt, again.UpdatedAt, err)
	}
}

func TestDeclareGroupCarriesASubFieldAndItsSettings(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)
	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}
	if err := registrar.DeclareGroup(t.Context(), eventGroup()); err != nil {
		t.Fatalf("DeclareGroup(first) error = %v, want nil", err)
	}
	changed := eventGroup()
	changed.Fields[0].Settings = map[string]any{"placeholder": "Where it happens"}
	changed.Fields[1].Fields[0].Label = "Doors open"

	if err := registrar.DeclareGroup(t.Context(), changed); err != nil {
		t.Fatalf("DeclareGroup(changed) error = %v, want nil", err)
	}
	if err := registrar.DeclareGroup(t.Context(), changed); err != nil {
		t.Fatalf("DeclareGroup(same again) error = %v, want nil", err)
	}

	held := heldGroup(t, registry, "event-details")
	if held.Fields[0].Settings["placeholder"] != "Where it happens" {
		t.Errorf("venue settings = %v, want the placeholder carried", held.Fields[0].Settings)
	}
	if len(held.Fields[1].Fields) != 1 || held.Fields[1].Fields[0].Label != "Doors open" {
		t.Errorf("schedule holds %+v, want the sub field label carried", held.Fields[1].Fields)
	}
}

func TestDeclareFieldSkipsAGroupTheSiteHolds(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)
	if _, err := registry.CreateGroup(t.Context(), content.Group{Key: "extras", Title: "Extras"}); err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	seats := sdk.FieldDeclaration{Key: "seats", Label: "Seats", Kind: "number"}

	err := registrar.DeclareField(t.Context(), "extras", seats)

	if err != nil {
		t.Fatalf("DeclareField() error = %v, want the site's group skipped rather than refused", err)
	}
	if skipped := registrar.Skipped(); !slices.Equal(skipped, []definitions.Held{{Subject: "group", Key: "extras"}}) {
		t.Errorf("Skipped() = %v, want the group key recorded", skipped)
	}
	if held := heldGroup(t, registry, "extras"); len(held.Fields) != 0 {
		t.Errorf("fields = %+v, want the site's group left empty", held.Fields)
	}
}

func TestDeclareTypeRefusesAShapeTheRegistryCannotHold(t *testing.T) {
	t.Parallel()

	_, registrar := declaringRegistry(t)
	badKey := eventType()
	badKey.Key = "Not A Key"
	badPage := eventType()
	badPage.PageKind = "poster"

	refused := map[string]sdk.TypeDeclaration{"a malformed key": badKey, "an unknown page kind": badPage}

	for name, declared := range refused {
		if err := registrar.DeclareType(t.Context(), declared); err == nil {
			t.Errorf("DeclareType(%s) error = nil, want the registry's refusal", name)
		}
	}
}

func TestDeclareGroupRefusesAKindChange(t *testing.T) {
	t.Parallel()

	_, registrar := declaringRegistry(t)
	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}
	if err := registrar.DeclareGroup(t.Context(), eventGroup()); err != nil {
		t.Fatalf("DeclareGroup(first) error = %v, want nil", err)
	}
	retyped := eventGroup()
	retyped.Fields[0].Kind = "number"

	err := registrar.DeclareGroup(t.Context(), retyped)

	if !errors.Is(err, definitions.ErrKindChanged) {
		t.Errorf("DeclareGroup(retyped) error = %v, want %v", err, definitions.ErrKindChanged)
	}
}

func TestDeclareGroupSkipsAKeyTheSiteHolds(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)
	if _, err := registry.CreateGroup(t.Context(), content.Group{Key: "event-details", Title: "Mine"}); err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	err := registrar.DeclareGroup(t.Context(), eventGroup())

	if err != nil {
		t.Fatalf("DeclareGroup() error = %v, want a collision skipped rather than refused", err)
	}
	wanted := []definitions.Held{{Subject: "group", Key: "event-details"}}
	if skipped := registrar.Skipped(); !slices.Equal(skipped, wanted) {
		t.Errorf("Skipped() = %v, want the group key recorded", skipped)
	}
	if held := heldGroup(t, registry, "event-details"); held.Title != "Mine" || held.Origin != "" {
		t.Errorf("held = %+v, want the site's group left as it was", held)
	}
}

func TestDeclareFieldAddsAFieldToTheGroupThePluginDeclared(t *testing.T) {
	t.Parallel()

	registry, registrar := declaringRegistry(t)
	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}
	if err := registrar.DeclareGroup(t.Context(), eventGroup()); err != nil {
		t.Fatalf("DeclareGroup() error = %v, want nil", err)
	}

	err := registrar.DeclareField(t.Context(), "event-details", sdk.FieldDeclaration{
		Key: "seats", Label: "Seats", Kind: "number",
	})

	if err != nil {
		t.Fatalf("DeclareField() error = %v, want nil", err)
	}
	held := heldGroup(t, registry, "event-details")
	if len(held.Fields) != 3 || held.Fields[2].Key != "seats" || held.Fields[2].Origin != "events" {
		t.Errorf("fields = %+v, want seats added under the events origin", held.Fields)
	}
}

func TestDeclareFieldReportsAGroupNobodyDeclared(t *testing.T) {
	t.Parallel()

	_, registrar := declaringRegistry(t)

	seats := sdk.FieldDeclaration{Key: "seats", Label: "Seats", Kind: "number"}

	err := registrar.DeclareField(t.Context(), "nowhere", seats)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("DeclareField() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}
