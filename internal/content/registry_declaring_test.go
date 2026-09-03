// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestRegistryLetsAPluginChangeWhatItDeclared(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	group := pluginGroup(t, store)
	renamed := group
	renamed.Title = "Event info"
	ctx := content.Declaring(t.Context(), "events")

	updated, err := registry.UpdateGroup(ctx, renamed)

	if err != nil {
		t.Fatalf("UpdateGroup() error = %v, want the declaring plugin allowed", err)
	}
	if updated.Title != "Event info" || updated.Origin != "events" {
		t.Errorf("updated = %+v, want the new title under the same origin", updated)
	}
	if _, err := registry.CreateFieldInGroup(ctx, group.ID, content.Field{
		Key: "seats", Label: "Seats", Kind: content.FieldKindNumber,
	}); err != nil {
		t.Errorf("CreateFieldInGroup() error = %v, want the declaring plugin allowed", err)
	}
}

func TestRegistryKeepsOnePluginOutOfAnothersDeclarations(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	group := pluginGroup(t, store)
	renamed := group
	renamed.Title = "Taken over"
	ctx := content.Declaring(t.Context(), "tickets")

	_, err := registry.UpdateGroup(ctx, renamed)

	if !errors.Is(err, content.ErrDefinitionReadOnly) {
		t.Errorf("UpdateGroup() error = %v, want %v", err, content.ErrDefinitionReadOnly)
	}
}

func TestRegistryKeepsAPluginOutOfTheSitesDefinitions(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newGroupingStore())
	site := groupNaming(t, registry, "Extras", namingPost())
	renamed := site
	renamed.Title = "Claimed"
	ctx := content.Declaring(t.Context(), "events")

	_, err := registry.UpdateGroup(ctx, renamed)

	if !errors.Is(err, content.ErrDefinitionReadOnly) {
		t.Errorf("UpdateGroup() error = %v, want a plugin kept off the site's own group", err)
	}
}
