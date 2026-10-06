// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"context"
	"sync"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// editingStore lands a rival edit once, the first time the groups are read after it is armed.
type editingStore struct {
	content.TypeStore
	once  sync.Once
	rival func(context.Context)
}

// ListGroups runs the armed rival edit before the first read of the groups, then reads them.
func (s *editingStore) ListGroups(ctx context.Context) ([]content.Group, error) {
	if s.rival != nil {
		s.once.Do(func() { s.rival(ctx) })
	}
	return s.TypeStore.ListGroups(ctx)
}

func TestApplyKeepsADescriptionEditedWhileItRuns(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	store := &editingStore{TypeStore: postgres.NewTypeStore(pool)}
	registry := content.NewRegistry(store)
	siteDefined(t, registry)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		envelope.Types[i].Description = nil
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].PluralLabel = "Dishes"
		}
	}
	store.rival = func(ctx context.Context) {
		recipe, err := registry.ByKey(ctx, "recipe")
		if err != nil {
			t.Errorf("ByKey(recipe) error = %v, want nil", err)
			return
		}
		recipe.Description = "Edited while importing."
		if _, err := registry.Update(ctx, recipe); err != nil {
			t.Errorf("Update(recipe) error = %v, want nil", err)
		}
	}

	applied(t, registry, importing(envelope))

	recipe, err := registry.ByKey(t.Context(), "recipe")
	if err != nil || recipe.PluralLabel != "Dishes" {
		t.Fatalf("the recipe type = %+v, %v, want the new plural label carried", recipe, err)
	}
	if recipe.Description != "Edited while importing." {
		t.Errorf("Description = %q, want the edit made while the import ran kept", recipe.Description)
	}
}
