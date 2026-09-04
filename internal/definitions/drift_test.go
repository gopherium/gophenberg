// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// heldAmong reports whether the list names the subject and key.
func heldAmong(list []definitions.Held, subject, key string) bool {
	for _, one := range list {
		if one.Subject == subject && one.Key == key {
			return true
		}
	}
	return false
}

// strayAmong reports whether the list names a stray of the subject and key.
func strayAmong(list []definitions.Stray, subject, key string) bool {
	for _, one := range list {
		if one.Subject == subject && one.Key == key {
			return true
		}
	}
	return false
}

func TestRegistrarNamesWhatItDeclaredAndHolds(t *testing.T) {
	t.Parallel()

	_, registrar := declaringPool(t)
	declared(t, registrar)

	held := registrar.Declared()

	if !heldAmong(held, "type", "event") || !heldAmong(held, "group", "event-details") {
		t.Errorf("Declared() = %+v, want the type and the group the plugin holds", held)
	}
	if len(registrar.Skipped()) != 0 {
		t.Errorf("Skipped() = %+v, want nothing skipped", registrar.Skipped())
	}
}

func TestRegistrarNamesWhatAnotherOwnerAlreadyHeld(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	site, err := content.NewType("event", "Meetup", "Meetups", "meetups")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	if _, err := registry.Create(t.Context(), site); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if err := registrar.DeclareType(t.Context(), eventType()); err != nil {
		t.Fatalf("DeclareType() error = %v, want nil", err)
	}

	if !heldAmong(registrar.Skipped(), "type", "event") {
		t.Errorf("Skipped() = %+v, want the key the site already held", registrar.Skipped())
	}
	if heldAmong(registrar.Declared(), "type", "event") {
		t.Errorf("Declared() = %+v, want the skipped key left out", registrar.Declared())
	}
}

func TestAdriftNamesNothingWhenEveryPluginRowIsStillDeclared(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))

	drift, err := definitions.Adrift(t.Context(), registry, definitions.Walked{
		"events": {Declared: registrar.Declared()},
	})

	if err != nil {
		t.Fatalf("Adrift() error = %v, want nil", err)
	}
	if len(drift.Orphans) != 0 || len(drift.Collisions) != 0 {
		t.Errorf("drift = %+v, want nothing standing apart", drift)
	}
}

func TestAdriftNamesTheRowsNoPluginDeclaresAnyMore(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))

	drift, err := definitions.Adrift(t.Context(), registry, definitions.Walked{})

	if err != nil {
		t.Fatalf("Adrift() error = %v, want nil", err)
	}
	if !strayAmong(drift.Orphans, "type", "event") {
		t.Errorf("orphans = %+v, want the type the plugin no longer declares", drift.Orphans)
	}
	if !strayAmong(drift.Orphans, "group", "event-details") {
		t.Errorf("orphans = %+v, want the group the plugin no longer declares", drift.Orphans)
	}
	for _, one := range drift.Orphans {
		if one.Origin != "events" {
			t.Errorf("orphan = %+v, want the plugin it belonged to named", one)
		}
	}
}

func TestAdriftNamesTheKeysAPluginCouldNotClaim(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	site, err := content.NewType("event", "Meetup", "Meetups", "meetups")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	if _, err := registry.Create(t.Context(), site); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	drift, err := definitions.Adrift(t.Context(), registry, definitions.Walked{
		"events": {Skipped: []definitions.Held{{Subject: "type", Key: "event"}}},
	})

	if err != nil {
		t.Fatalf("Adrift() error = %v, want nil", err)
	}
	if !strayAmong(drift.Collisions, "type", "event") {
		t.Fatalf("collisions = %+v, want the key the plugin could not claim", drift.Collisions)
	}
	if drift.Collisions[0].Origin != "events" || drift.Collisions[0].Label != "Meetup" {
		t.Errorf("collision = %+v, want the rival plugin and the site's own label named", drift.Collisions[0])
	}
}

func TestAdriftNamesTheGroupKeyAPluginCouldNotClaim(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	if _, err := registry.CreateGroup(t.Context(), content.Group{Key: "extras", Title: "Extras"}); err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	drift, err := definitions.Adrift(t.Context(), registry, definitions.Walked{
		"events": {Skipped: []definitions.Held{{Subject: "group", Key: "extras"}}},
	})

	if err != nil {
		t.Fatalf("Adrift() error = %v, want nil", err)
	}
	if len(drift.Collisions) != 1 || drift.Collisions[0].Label != "Extras" {
		t.Errorf("collisions = %+v, want the site's own group named", drift.Collisions)
	}
}

func TestAdoptTakesEitherKindOfDefinitionOver(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))

	if err := definitions.Adopt(t.Context(), registry,
		definitions.Held{Subject: "type", Key: "event"}); err != nil {
		t.Fatalf("Adopt(type) error = %v, want nil", err)
	}
	if err := definitions.Adopt(t.Context(), registry,
		definitions.Held{Subject: "group", Key: "event-details"}); err != nil {
		t.Fatalf("Adopt(group) error = %v, want nil", err)
	}

	drift, err := definitions.Adrift(t.Context(), registry, definitions.Walked{})
	if err != nil {
		t.Fatalf("Adrift() error = %v, want nil", err)
	}
	if len(drift.Orphans) != 0 {
		t.Errorf("orphans = %+v, want the site owning what it took over", drift.Orphans)
	}
}

func TestDeclareTypeReportsAStoreThatWillNotHoldTheType(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	raiseOn(t, pool, "core.content_types", "INSERT", "true")

	if err := registrar.DeclareType(t.Context(), eventType()); err == nil {
		t.Errorf("DeclareType() error = nil, want the refused write reported")
	}
}

func TestAdoptRefusesASubjectNoDefinitionAnswersTo(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))

	err := definitions.Adopt(t.Context(), registry,
		definitions.Held{Subject: "banana", Key: "event-details"})

	if !errors.Is(err, definitions.ErrSubjectUnknown) {
		t.Fatalf("Adopt(banana) error = %v, want %v", err, definitions.ErrSubjectUnknown)
	}
	drift, err := definitions.Adrift(t.Context(), registry, definitions.Walked{})
	if err != nil {
		t.Fatalf("Adrift() error = %v, want nil", err)
	}
	if !strayAmong(drift.Orphans, "group", "event-details") {
		t.Errorf("orphans = %+v, want the group left where it was", drift.Orphans)
	}
}

func TestAdriftReportsARegistryItCannotRead(t *testing.T) {
	t.Parallel()

	for name, statement := range map[string]string{
		"groups behind an unreadable table": "ALTER TABLE core.field_groups RENAME COLUMN title TO retired",
		"types behind an unreadable table":  "ALTER TABLE core.content_types RENAME COLUMN singular_label TO retired",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, _ := declaringPool(t)
			sabotage(t, pool, statement)

			_, err := definitions.Adrift(t.Context(), content.NewRegistry(postgres.NewTypeStore(pool)),
				definitions.Walked{})

			if err == nil {
				t.Errorf("%s: error = nil, want the unreadable store reported", name)
			}
		})
	}
}
