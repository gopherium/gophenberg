// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// declaredSite returns a registry over a migrated database holding one plugin's type, group and field.
func declaredSite(t *testing.T) *content.Registry {
	t.Helper()
	registry := content.NewRegistry(newTypeStore(t))
	declaredInto(t, registry)
	return registry
}

// declaredInto stores one plugin's type, group and field through the registry.
func declaredInto(t *testing.T, registry *content.Registry) {
	t.Helper()
	ctx := content.Declaring(t.Context(), "events")
	event, err := content.NewType("event", "Event", "Events", "events")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	event.Origin = "events"
	if _, err := registry.Create(ctx, event); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	group, err := registry.CreateGroup(ctx, content.Group{
		Key: "event-details", Title: "Event details", Origin: "events",
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(ctx, group.ID, content.Field{
		Key: "venue", Label: "Venue", Kind: content.FieldKindText, Origin: "events",
	}); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
}

// groupKeyed returns the stored group carrying the key, failing the test when the site holds none.
func groupKeyed(t *testing.T, registry *content.Registry, key string) content.Group {
	t.Helper()
	groups, err := registry.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups() error = %v, want nil", err)
	}
	for _, held := range groups {
		if held.Key == key {
			return held
		}
	}
	t.Fatalf("the site holds no group %q", key)
	return content.Group{}
}

func TestAdoptGroupTakesThePluginsGroupAndItsFieldsOverAsTheSites(t *testing.T) {
	t.Parallel()

	registry := declaredSite(t)

	if err := registry.AdoptGroup(t.Context(), "event-details"); err != nil {
		t.Fatalf("AdoptGroup() error = %v, want nil", err)
	}

	held := groupKeyed(t, registry, "event-details")
	if held.Origin != "" {
		t.Errorf("the group names %q as its origin, want the site owning it", held.Origin)
	}
	if len(held.Fields) != 1 || held.Fields[0].Origin != "" {
		t.Errorf("fields = %+v, want the fields taken over with the group", held.Fields)
	}
	if _, err := registry.UpdateGroup(t.Context(), content.Group{
		ID: held.ID, Key: held.Key, Title: "Event facts", Location: held.Location, Active: held.Active,
	}); err != nil {
		t.Errorf("UpdateGroup() error = %v, want the adopted group open to the site", err)
	}
}

func TestAdoptTypeTakesThePluginsTypeOverAsTheSites(t *testing.T) {
	t.Parallel()

	registry := declaredSite(t)

	if err := registry.AdoptType(t.Context(), "event"); err != nil {
		t.Fatalf("AdoptType() error = %v, want nil", err)
	}

	held, err := registry.ByKey(t.Context(), "event")
	if err != nil || held.Origin != "" {
		t.Errorf("the event type = %+v, %v, want the site owning it", held, err)
	}
}

func TestAdoptReportsAStoreThatWillNotTakeItOver(t *testing.T) {
	t.Parallel()

	for name, held := range map[string]struct {
		table string
		adopt func(t *testing.T, registry *content.Registry) error
	}{
		"a type it cannot carry": {
			"core.content_types",
			func(t *testing.T, registry *content.Registry) error {
				return registry.AdoptType(t.Context(), "event")
			},
		},
		"a group it cannot carry": {
			"core.field_groups",
			func(t *testing.T, registry *content.Registry) error {
				return registry.AdoptGroup(t.Context(), "event-details")
			},
		},
		"the fields inside a group it cannot carry": {
			"core.content_fields",
			func(t *testing.T, registry *content.Registry) error {
				return registry.AdoptGroup(t.Context(), "event-details")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, _, pool := newContentStoreWithPool(t)
			registry := content.NewRegistry(postgres.NewTypeStore(pool))
			declaredInto(t, registry)
			raiseOn(t, pool, held.table, "UPDATE")

			if err := held.adopt(t, registry); err == nil {
				t.Errorf("%s: error = nil, want the refused write reported", name)
			}
		})
	}
}

func TestAdoptReportsADefinitionTheSiteDoesNotHold(t *testing.T) {
	t.Parallel()

	registry := declaredSite(t)

	if err := registry.AdoptType(t.Context(), "nowhere"); !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("AdoptType(nowhere) error = %v, want %v", err, content.ErrTypeNotFound)
	}
	if err := registry.AdoptGroup(t.Context(), "nowhere"); !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("AdoptGroup(nowhere) error = %v, want %v", err, content.ErrGroupNotFound)
	}
}
