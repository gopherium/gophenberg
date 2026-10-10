// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"context"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// stampKept fails the test when the type the registry holds under the held one's key carries another stamp.
func stampKept(t *testing.T, registry *content.Registry, held content.Type) {
	t.Helper()
	after, err := registry.ByKey(t.Context(), held.Key)
	if err != nil || !after.UpdatedAt.Equal(held.UpdatedAt) {
		t.Errorf("%s UpdatedAt = %v, %v, want the stored %v kept", held.Key, after.UpdatedAt, err, held.UpdatedAt)
	}
}

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

func TestApplyKeepsTheStampOfATypeItLeavesNesting(t *testing.T) {
	t.Parallel()

	registry, items, author := nestingSite(t)
	recipeUnderAnother(t, registry, items, author)
	envelope := exported(t, registry)
	flatteningRecipe(t, envelope)
	held, err := registry.ByKey(t.Context(), "recipe")
	if err != nil {
		t.Fatalf("ByKey(recipe) error = %v, want nil", err)
	}

	applied(t, registry, importing(envelope))

	stampKept(t, registry, held)
}

func TestApplyKeepsTheStampOfATypeWhoseLabelOnlyGainsSpaces(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	relabeling(t, envelope, "recipe", " Recipe ")
	held, err := registry.ByKey(t.Context(), "recipe")
	if err != nil {
		t.Fatalf("ByKey(recipe) error = %v, want nil", err)
	}

	applied(t, registry, importing(envelope))

	stampKept(t, registry, held)
}

func TestApplyKeepsTheStampOfAnEditMadeWhileItRunsThatMatchesTheFile(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	store := &editingStore{TypeStore: postgres.NewTypeStore(pool)}
	registry := content.NewRegistry(store)
	siteDefined(t, registry)
	envelope := exported(t, registry)
	relabeling(t, envelope, "recipe", "Dish")
	var edited content.Type
	store.skip = 1
	store.rival = func(ctx context.Context) {
		recipe, err := registry.ByKey(ctx, "recipe")
		if err != nil {
			t.Errorf("ByKey(recipe) error = %v, want nil", err)
			return
		}
		if recipe.SingularLabel == "Dish" {
			t.Error("the recipe type already carries Dish, want the edit made before the import writes it")
		}
		recipe.SingularLabel, recipe.UpdatedAt = "Dish", postgresNow()
		if edited, err = registry.Update(ctx, recipe); err != nil {
			t.Errorf("Update(recipe) error = %v, want nil", err)
		}
	}

	outcome := applied(t, registry, importing(envelope))

	if edited.UpdatedAt.IsZero() {
		t.Fatal("the edit never landed, want it made after the plan")
	}
	if !named(outcome.Applied, "type", "recipe") {
		t.Fatalf("applied = %+v, want the recipe change planned before the edit", outcome.Applied)
	}
	stampKept(t, registry, edited)
}
