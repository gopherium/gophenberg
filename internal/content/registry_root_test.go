// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestDerivedRouteWordReservedNamesTheTypeAndTheWord(t *testing.T) {
	t.Parallel()

	err := content.DerivedRouteWordReserved("media", "3D Files", "media")

	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Errorf("error = %v, want it to carry %v", err, content.ErrRouteWordReserved)
	}
	if code, _ := content.CodeOf(err); code != "derived_route_word_reserved" {
		t.Errorf("code = %q, want derived_route_word_reserved", code)
	}
	held, _ := content.DetailsOf(err)
	if held["type"] != "media" || held["name"] != "3D Files" || held["word"] != "media" {
		t.Errorf("details = %v, want the type media, the name 3D Files and the word media named", held)
	}
	if !strings.Contains(err.Error(), `"3D Files" would answer under "media"`) {
		t.Errorf("message = %q, want it to name the type and the word", err.Error())
	}
}

func TestDerivedRouteWordTakenNamesTheTypeAndTheWord(t *testing.T) {
	t.Parallel()

	err := content.DerivedRouteWordTaken("chapter", "Recipes", "recipes")

	if !errors.Is(err, content.ErrRouteWordTaken) {
		t.Errorf("error = %v, want it to carry %v", err, content.ErrRouteWordTaken)
	}
	if code, _ := content.CodeOf(err); code != "derived_route_word_taken" {
		t.Errorf("code = %q, want derived_route_word_taken", code)
	}
	held, _ := content.DetailsOf(err)
	if held["type"] != "chapter" || held["name"] != "Recipes" || held["word"] != "recipes" {
		t.Errorf("details = %v, want the type chapter, the name Recipes and the word recipes named", held)
	}
	if want := `content: route word taken: "Recipes" would answer under "recipes"`; err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
}

func TestStatedRouteWordTakenNamesTheTypeAndTheWord(t *testing.T) {
	t.Parallel()

	err := content.StatedRouteWordTaken("chapter", "Chapters", "recipes")

	if !errors.Is(err, content.ErrRouteWordTaken) {
		t.Errorf("error = %v, want it to carry %v", err, content.ErrRouteWordTaken)
	}
	if code, _ := content.CodeOf(err); code != "stated_route_word_taken" {
		t.Errorf("code = %q, want stated_route_word_taken", code)
	}
	held, _ := content.DetailsOf(err)
	if held["type"] != "chapter" || held["name"] != "Chapters" || held["word"] != "recipes" {
		t.Errorf("details = %v, want the type chapter, the name Chapters and the word recipes named", held)
	}
	if want := `content: route word taken: "Chapters" would answer under "recipes"`; err.Error() != want {
		t.Errorf("message = %q, want %q", err.Error(), want)
	}
}

func TestRegistryRefusesANewTypeClaimingTheRoot(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	registry := content.NewRegistry(store)
	claimingRoot := carType(t)
	claimingRoot.RouteWord, claimingRoot.Default = "", true

	_, err := registry.Create(t.Context(), claimingRoot)

	if !errors.Is(err, content.ErrRootTaken) {
		t.Fatalf("Create() error = %v, want %v", err, content.ErrRootTaken)
	}
	if errors.Is(err, content.ErrRouteWordTaken) {
		t.Errorf("Create() error = %v, want the root conflict rather than the named word conflict", err)
	}
	if code, named := content.CodeOf(err); named {
		t.Errorf("Create() error code = %q, want the bare conflict the registry raises on a stored type", code)
	}
	if len(store.types) != 1 {
		t.Errorf("the store holds %d types, want the claim refused before it was written", len(store.types))
	}
}
