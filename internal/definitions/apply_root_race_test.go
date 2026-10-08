// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"context"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// TestApplyKeepsARouteWordEditedWhileItRuns checks that preserving the site's root keeps its latest route word.
func TestApplyKeepsARouteWordEditedWhileItRuns(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	store := &editingStore{TypeStore: postgres.NewTypeStore(pool)}
	registry := content.NewRegistry(store)
	siteDefined(t, registry)
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
	store.rival = func(ctx context.Context) {
		recipe, err := registry.ByKey(ctx, "recipe")
		if err != nil {
			t.Errorf("ByKey(recipe) error = %v, want nil", err)
			return
		}
		recipe.RouteWord = "dishes"
		if _, err := registry.Update(ctx, recipe); err != nil {
			t.Errorf("Update(recipe) error = %v, want nil", err)
		}
	}

	outcome := applied(t, registry, importing(envelope))

	recipe, err := registry.ByKey(t.Context(), "recipe")
	if err != nil || recipe.SingularLabel != "Dish" {
		t.Fatalf("the recipe type = %+v, %v, want the imported label carried", recipe, err)
	}
	if recipe.RouteWord != "dishes" || recipe.Default {
		t.Errorf("the recipe type = %+v, want the route word edited during import kept beside the root", recipe)
	}
	post, err := registry.ByKey(t.Context(), content.TypePost)
	if err != nil || !post.Default {
		t.Errorf("the post type = %+v, %v, want it still holding the root", post, err)
	}
	if !named(outcome.Skipped, "type", "recipe") {
		t.Errorf("skipped = %+v, want the root move reported as skipped", outcome.Skipped)
	}
}
