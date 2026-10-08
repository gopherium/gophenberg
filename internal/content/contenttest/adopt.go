// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// adoptCases are the cases over the site taking a plugin's type or group over as its own.
var adoptCases = []Case{
	{
		"AdoptGroupTakesThePluginsGroupAndItsFieldsOverAsTheSites",
		adoptGroupTakesThePluginsGroupAndItsFieldsOverAsTheSites,
	},
	{"AdoptTypeTakesThePluginsTypeOverAsTheSites", adoptTypeTakesThePluginsTypeOverAsTheSites},
	{"AdoptReportsADefinitionTheSiteDoesNotHold", adoptReportsADefinitionTheSiteDoesNotHold},
}

// declaredSite returns a registry over the type store holding one plugin's type, group and field.
func declaredSite(t *testing.T, types content.TypeStore) *content.Registry {
	t.Helper()
	registry := content.NewRegistry(types)
	DeclaredInto(t, registry)
	return registry
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

// adoptGroupTakesThePluginsGroupAndItsFieldsOverAsTheSites clears the origin of the group and its fields.
func adoptGroupTakesThePluginsGroupAndItsFieldsOverAsTheSites(t *testing.T, s Stores) {
	registry := declaredSite(t, s.Types)

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

// adoptTypeTakesThePluginsTypeOverAsTheSites clears the origin of the type.
func adoptTypeTakesThePluginsTypeOverAsTheSites(t *testing.T, s Stores) {
	registry := declaredSite(t, s.Types)

	if err := registry.AdoptType(t.Context(), "event"); err != nil {
		t.Fatalf("AdoptType() error = %v, want nil", err)
	}

	held, err := registry.ByKey(t.Context(), "event")
	if err != nil || held.Origin != "" {
		t.Errorf("the event type = %+v, %v, want the site owning it", held, err)
	}
}

// adoptReportsADefinitionTheSiteDoesNotHold reports a type or a group the site holds under no such key.
func adoptReportsADefinitionTheSiteDoesNotHold(t *testing.T, s Stores) {
	registry := declaredSite(t, s.Types)

	if err := registry.AdoptType(t.Context(), "nowhere"); !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("AdoptType(nowhere) error = %v, want %v", err, content.ErrTypeNotFound)
	}
	if err := registry.AdoptGroup(t.Context(), "nowhere"); !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("AdoptGroup(nowhere) error = %v, want %v", err, content.ErrGroupNotFound)
	}
}
