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

// unlistingOnceStore refuses the first type read after an update, then recovers.
type unlistingOnceStore struct {
	content.TypeStore
	wrote   bool
	refused bool
}

// Update stores the type and arms the single refused read.
func (s *unlistingOnceStore) Update(ctx context.Context, wanted content.Type) (content.Type, error) {
	stored, err := s.TypeStore.Update(ctx, wanted)
	s.wrote = err == nil
	return stored, err
}

// List refuses one read after a type update, allowing later reads to succeed.
func (s *unlistingOnceStore) List(ctx context.Context) ([]content.Type, error) {
	if s.wrote && !s.refused {
		s.refused = true
		return nil, errTypesUnread
	}
	return s.TypeStore.List(ctx)
}

func TestApplyStopsWhenItCannotReadTheRouteItKeeps(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	store := &unlistingOnceStore{TypeStore: postgres.NewTypeStore(pool)}
	registry := content.NewRegistry(store)
	siteDefined(t, registry)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == content.TypePost {
			envelope.Types[i].SingularLabel = "Entry"
			envelope.Types[i].Default, envelope.Types[i].RouteWord = false, "posts"
		}
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].SingularLabel = "Dish"
			envelope.Types[i].Default, envelope.Types[i].RouteWord = true, ""
		}
	}
	groupNamed(t, envelope, "recipe-details").Title = "Imported recipe details"

	_, err := definitions.Apply(t.Context(), registry, importing(envelope))

	if !errors.Is(err, errTypesUnread) {
		t.Fatalf("Apply() error = %v, want the refused read of the current route reported", err)
	}
	stored := content.NewRegistry(postgres.NewTypeStore(pool))
	post, err := stored.ByKey(t.Context(), content.TypePost)
	if err != nil || post.SingularLabel != "Entry" || !post.Default || post.RouteWord != "" {
		t.Errorf("the post type = %+v, %v, want the earlier label write with the root preserved", post, err)
	}
	recipe, err := stored.ByKey(t.Context(), "recipe")
	if err != nil || recipe.SingularLabel != "Recipe" || recipe.Default || recipe.RouteWord != "recipes" {
		t.Errorf("the recipe type = %+v, %v, want the failed import to leave it unchanged", recipe, err)
	}
	group, found := storedGroup(t, stored, "recipe-details")
	if !found || group.Title != "Recipe details" {
		t.Errorf("the recipe group = %+v, %v, want the later group write left unapplied", group, found)
	}
}

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

func TestApplyReportsATypeDeletedWhileItRuns(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	store := &editingStore{TypeStore: postgres.NewTypeStore(pool)}
	registry := content.NewRegistry(store)
	siteDefined(t, registry)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].Default, envelope.Types[i].RouteWord = true, ""
		}
		if envelope.Types[i].Key == content.TypePost {
			envelope.Types[i].Default, envelope.Types[i].RouteWord = false, "posts"
		}
	}
	store.skip = 1
	store.rival = func(ctx context.Context) {
		if err := registry.Delete(ctx, "recipe"); err != nil {
			t.Errorf("Delete(recipe) error = %v, want nil", err)
		}
	}

	_, err := definitions.Apply(t.Context(), registry, importing(envelope))

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("Apply() error = %v, want the type deleted while the import ran reported as missing", err)
	}
	stored := content.NewRegistry(postgres.NewTypeStore(pool))
	if _, err := stored.ByKey(t.Context(), "recipe"); !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ByKey(recipe) error = %v, want the deleted type left deleted", err)
	}
}
