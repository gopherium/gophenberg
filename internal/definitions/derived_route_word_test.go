// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
)

func TestImportRefusesADerivedWordBeforeRemovingGroups(t *testing.T) {
	t.Parallel()

	book := []definitions.TypeDefinition{{
		Key: "book", SingularLabel: "Book", PluralLabel: "Books", RouteWord: "books",
		PageKind: "single", Active: true,
	}}
	for _, test := range []struct {
		name   string
		label  string
		word   string
		before []definitions.TypeDefinition
		after  []definitions.TypeDefinition
	}{
		{name: "site", label: "Recipes", word: "recipes"},
		{name: "file before", label: "Books", word: "books", before: book},
		{name: "file after", label: "Books", word: "books", after: book},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			registry := planningSite(t)
			before := exported(t, registry)
			envelope := exported(t, registry)
			for i := range envelope.Types {
				if envelope.Types[i].Key == content.TypePost {
					envelope.Types[i].Default, envelope.Types[i].RouteWord = false, "posts"
				}
			}
			derived := definitions.TypeDefinition{
				Key: "chapter", SingularLabel: "Chapter", PluralLabel: test.label,
				PageKind: "single", Default: true, Active: true,
			}
			envelope.Types = slices.Concat(envelope.Types, test.before, []definitions.TypeDefinition{derived}, test.after)
			envelope.Groups = nil
			asked := definitions.Import{
				Envelope: envelope,
				Confirm:  []definitions.Confirmed{{Subject: "group", Key: "recipe-details"}},
			}

			_, compareErr := definitions.Compare(t.Context(), registry, envelope)
			_, applyErr := definitions.Apply(t.Context(), registry, asked)
			for call, err := range map[string]error{"Compare": compareErr, "Apply": applyErr} {
				if !errors.Is(err, content.ErrRouteWordTaken) {
					t.Errorf("%s() error = %v, want %v", call, err, content.ErrRouteWordTaken)
				}
				namesTheDerivedWord(t, call, err, "derived_route_word_taken", test.label, test.word)
			}
			if after := exported(t, registry); !reflect.DeepEqual(before, after) {
				t.Errorf("refused import changed stored definitions: before=%+v after=%+v", before, after)
			}
		})
	}
}

func TestImportCanUseAWordAnEarlierTypeReleases(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	envelope := exported(t, registry)
	for i := range envelope.Types {
		switch envelope.Types[i].Key {
		case content.TypePost:
			envelope.Types[i].Default, envelope.Types[i].RouteWord = false, "posts"
		case "recipe":
			envelope.Types[i].RouteWord = "meals"
		}
	}
	envelope.Types = append(envelope.Types, definitions.TypeDefinition{
		Key: "chapter", SingularLabel: "Chapter", PluralLabel: "Recipes",
		PageKind: "single", Default: true, Active: true,
	})
	compared(t, registry, envelope)
	applied(t, registry, importing(envelope))
	held, err := registry.ByKey(t.Context(), "chapter")
	if err != nil || held.RouteWord != "recipes" || held.Default {
		t.Errorf("new type = %+v, error = %v, want the released word beside the root", held, err)
	}
}
