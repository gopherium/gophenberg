// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

func TestImportRefusesANewRootTypeWhoseLabelGivesAReservedWord(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		label string
		word  string
	}{
		{label: "Page", word: "page"},
		{label: "Media", word: "media"},
	} {
		t.Run(test.label, func(t *testing.T) {
			t.Parallel()
			registry := planningSite(t)
			envelope := exported(t, registry)
			for i := range envelope.Types {
				if envelope.Types[i].Key == content.TypePost {
					envelope.Types[i].Default, envelope.Types[i].RouteWord = false, "posts"
				}
			}
			envelope.Types = append(envelope.Types, definitions.TypeDefinition{
				Key: "chapter", SingularLabel: "Chapter", PluralLabel: test.label,
				PageKind: "single", Default: true, Active: true,
			})
			envelope.Groups = envelope.Groups[:0]
			asked := definitions.Import{
				Envelope: envelope,
				Confirm:  []definitions.Confirmed{{Subject: "group", Key: "recipe-details"}},
			}

			_, compareErr := definitions.Compare(t.Context(), registry, envelope)
			_, applyErr := definitions.Apply(t.Context(), registry, asked)

			if !errors.Is(compareErr, content.ErrRouteWordReserved) {
				t.Errorf("Compare() error = %v, want %v", compareErr, content.ErrRouteWordReserved)
			}
			if !errors.Is(applyErr, content.ErrRouteWordReserved) {
				t.Errorf("Apply() error = %v, want %v", applyErr, content.ErrRouteWordReserved)
			}
			for call, err := range map[string]error{"Compare": compareErr, "Apply": applyErr} {
				namesTheDerivedWord(t, call, err, "derived_route_word_reserved", test.label, test.word)
			}
			if _, found := storedGroup(t, registry, "recipe-details"); !found {
				t.Errorf("the recipe group is gone, want the refused import to have taken nothing")
			}
		})
	}
}

// namesTheDerivedWord fails the case unless the error carries the code and names the chapter type, label and word.
func namesTheDerivedWord(t *testing.T, call string, err error, want, label, word string) {
	t.Helper()
	if code, _ := content.CodeOf(err); code != want {
		t.Errorf("%s() code = %q, want %q", call, code, want)
	}
	if held, _ := content.DetailsOf(err); held["type"] != "chapter" || held["name"] != label || held["word"] != word {
		t.Errorf("%s() details = %v, want the type chapter named %q and the word %q", call, held, label, word)
	}
}

func TestApplyCanEditALegacyPageTypeWithoutMovingIt(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	store := postgres.NewTypeStore(pool)
	legacy, err := content.NewType("chapter", "Chapter", "Chapters", "page")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), legacy); err != nil {
		t.Fatalf("storing legacy type: %v", err)
	}
	registry := content.NewRegistry(store)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == "chapter" {
			envelope.Types[i].PluralLabel = "Site chapters"
		}
	}
	compared(t, registry, envelope)
	applied(t, registry, importing(envelope))
	held, err := registry.ByKey(t.Context(), "chapter")
	if err != nil {
		t.Fatal(err)
	}
	if held.RouteWord != "page" || held.PluralLabel != "Site chapters" {
		t.Errorf("imported type = %+v, want the labels edited at the existing route", held)
	}
}

func TestDeclareTypeCanEditALegacyPageTypeWithoutMovingIt(t *testing.T) {
	t.Parallel()

	pool, _ := declaringPool(t)
	store := postgres.NewTypeStore(pool)
	legacy, err := content.NewType("event", "Event", "Events", "page")
	if err != nil {
		t.Fatal(err)
	}
	legacy.Origin = "events"
	if _, err := store.Create(t.Context(), legacy); err != nil {
		t.Fatalf("storing legacy plugin type: %v", err)
	}
	registry := content.NewRegistry(store)
	declared := eventType()
	declared.RouteWord, declared.Description = "page", "Updated events"
	if err := definitions.New(registry, "events").DeclareType(t.Context(), declared); err != nil {
		t.Fatalf("DeclareType() error = %v, want the existing plugin type updated", err)
	}
	held, err := registry.ByKey(t.Context(), "event")
	if err != nil {
		t.Fatal(err)
	}
	if held.RouteWord != "page" || held.Description != "Updated events" {
		t.Errorf("declared type = %+v, want the plugin's changes at the existing route", held)
	}
}
