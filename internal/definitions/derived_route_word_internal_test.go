// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestImportRefusesADerivedRouteWordAnotherTypeHolds(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		key      string
		label    string
		word     string
		stored   []content.Type
		declared []TypeDefinition
	}{
		{
			name: "site type", key: "chapter", label: "Recipes", word: "recipes",
			stored: []content.Type{{Key: "recipe", RouteWord: "recipes"}},
		},
		{
			name: "plugin type", key: "chapter", label: "Recipes", word: "recipes",
			stored: []content.Type{{Key: "recipe", RouteWord: "recipes", Origin: "recipes"}},
		},
		{
			name: "trimmed label", key: " chapter ", label: " Recipes ", word: "recipes",
			stored: []content.Type{{Key: "recipe", RouteWord: "recipes"}},
		},
		{
			name: "key fallback", key: "chapter", label: "Страницы", word: "chapter",
			stored: []content.Type{{Key: "other", RouteWord: "chapter"}},
		},
		{
			name: "later declaration", key: "chapter", label: "Recipes", word: "recipes",
			declared: []TypeDefinition{{
				Key: "recipe", SingularLabel: "Recipe", PluralLabel: "Recipes",
				RouteWord: "recipes", PageKind: "single", Active: true,
			}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			declaration := TypeDefinition{
				Key: test.key, SingularLabel: "Chapter", PluralLabel: test.label,
				PageKind: "single", Default: true, Active: true,
			}
			declarations := append([]TypeDefinition{declaration}, test.declared...)
			err := validateTypes(declarations, test.stored)
			if !errors.Is(err, content.ErrRouteWordTaken) {
				t.Errorf("validateTypes() error = %v, want %v", err, content.ErrRouteWordTaken)
			}
			if code, _ := content.CodeOf(err); code != "derived_route_word_taken" {
				t.Errorf("code = %q, want derived_route_word_taken", code)
			}
			wanted := importedAs(declaration, false)
			details, _ := content.DetailsOf(err)
			if details["type"] != wanted.Key || details["name"] != wanted.PluralLabel || details["word"] != test.word {
				t.Errorf("details = %v, want type %q, name %q, word %q", details, wanted.Key, wanted.PluralLabel, test.word)
			}
		})
	}
}

func TestImportAllowsADerivedWordTheKeptRootDoesNotUse(t *testing.T) {
	t.Parallel()
	stored := []content.Type{{Key: "post", Default: true}}
	declared := []TypeDefinition{
		{Key: "post", SingularLabel: "Post", PluralLabel: "Posts", RouteWord: "posts", PageKind: "single", Active: true},
		{Key: "chapter", SingularLabel: "Chapter", PluralLabel: "Posts", PageKind: "single", Default: true, Active: true},
	}
	if err := validateTypes(declared, stored); err != nil {
		t.Errorf("validateTypes() error = %v, want the site's root kept without claiming posts", err)
	}
}
