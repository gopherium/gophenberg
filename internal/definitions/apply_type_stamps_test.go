// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestApplyStampsATypeItCreates(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	envelope.Types = append(envelope.Types, definitions.TypeDefinition{
		Key: "event", SingularLabel: "Event", PluralLabel: "Events", RouteWord: "events",
		RevisionCap: 20, PageKind: "single", Active: true,
	})
	before := postgresNow()

	applied(t, registry, importing(envelope))

	event, err := registry.ByKey(t.Context(), "event")
	if err != nil {
		t.Fatalf("ByKey(event) error = %v, want the type the file brought", err)
	}
	if event.CreatedAt.Before(before) || !event.UpdatedAt.Equal(event.CreatedAt) {
		t.Errorf("event stamps = %v and %v, want one pair stamped at or after %v",
			event.CreatedAt, event.UpdatedAt, before)
	}
}

func TestApplyStampsATypeItUpdatesAndKeepsItsCreationTime(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	held, err := registry.ByKey(t.Context(), "recipe")
	if err != nil {
		t.Fatalf("ByKey(recipe) error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	relabeling(t, envelope, "recipe", "Dish")
	before := postgresNow()

	applied(t, registry, importing(envelope))

	recipe, err := registry.ByKey(t.Context(), "recipe")
	if err != nil || recipe.SingularLabel != "Dish" {
		t.Fatalf("the recipe type = %+v, %v, want the imported label carried", recipe, err)
	}
	if recipe.UpdatedAt.Before(before) {
		t.Errorf("UpdatedAt = %v, want it stamped at or after %v", recipe.UpdatedAt, before)
	}
	if !recipe.CreatedAt.Equal(held.CreatedAt) {
		t.Errorf("CreatedAt = %v, want the stored %v kept", recipe.CreatedAt, held.CreatedAt)
	}
}

func TestApplyStampsTheItemsItMovesToANewRouteWord(t *testing.T) {
	t.Parallel()

	pool, registry := definedSite(t)
	items := postgres.NewContentStore(pool)
	recipe, err := registry.ByKey(t.Context(), "recipe")
	if err != nil {
		t.Fatalf("ByKey(recipe) error = %v, want nil", err)
	}
	bread := filed(t, items, recipe, nil, "Bread", authorOn(t, pool))
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].RouteWord = "dishes"
		}
	}
	before := postgresNow()

	applied(t, registry, importing(envelope))

	moved, err := items.ByID(t.Context(), bread.ID)
	if err != nil || moved.Path != "dishes/bread" {
		t.Fatalf("the bread item = %+v, %v, want it moved under the new route word", moved, err)
	}
	if moved.UpdatedAt.Before(before) {
		t.Errorf("UpdatedAt = %v, want it stamped at or after %v", moved.UpdatedAt, before)
	}
}
