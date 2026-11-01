// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// declaredType returns a valid declaration under the key, plural label and route word, the root when root is set.
func declaredType(key, plural, word string, root bool) TypeDefinition {
	return TypeDefinition{
		Key: key, SingularLabel: "Item", PluralLabel: plural, RouteWord: word,
		PageKind: "single", Default: root, Active: true,
	}
}

func TestImportChecksRouteWordsInTheOrderApplyWrites(t *testing.T) {
	t.Parallel()

	root := content.Type{Key: "post", Default: true}
	recipe := content.Type{Key: "recipe", RouteWord: "recipes"}
	book := content.Type{Key: "book", RouteWord: "books"}
	derivedRecipes := content.Details{"type": "chapter", "name": "Recipes", "word": "recipes"}
	const stated = "stated_route_word_taken"
	for _, test := range []struct {
		name     string
		stored   []content.Type
		declared []TypeDefinition
		taken    bool
		code     string
		details  content.Details
	}{
		{
			name:   "stored type moves onto a word a new type derived",
			stored: []content.Type{root, book},
			declared: []TypeDefinition{
				declaredType("chapter", "Recipes", "", true), declaredType("book", "Books", "recipes", false),
			},
			taken: true, code: "derived_route_word_taken", details: derivedRecipes,
		},
		{
			name:     "kept root asks for a word another type holds",
			stored:   []content.Type{root, recipe},
			declared: []TypeDefinition{declaredType("post", "Posts", "recipes", true)},
			taken:    true, code: stated, details: content.Details{"type": "post", "name": "Posts", "word": "recipes"},
		},
		{
			name:     "type handed the root derives no word from its label",
			stored:   []content.Type{root, recipe, book},
			declared: []TypeDefinition{declaredType("recipe", "Books", "", true)},
		},
		{
			name:   "kept root holds its own word against a new type",
			stored: []content.Type{{Key: "post", Default: true, RouteWord: "blog"}},
			declared: []TypeDefinition{
				declaredType("post", "Posts", "blog", true), declaredType("book", "Books", "blog", false),
			},
			taken: true, code: stated, details: content.Details{"type": "book", "name": "Books", "word": "blog"},
		},
		{
			name:   "new type claims a word before its holder moves away",
			stored: []content.Type{root, recipe},
			declared: []TypeDefinition{
				declaredType("chapter", "Recipes", "", true), declaredType("recipe", "Recipes", "meals", false),
			},
			taken: true, code: "derived_route_word_taken", details: derivedRecipes,
		},
		{
			name:   "new type claims a word after its holder moved away",
			stored: []content.Type{root, recipe},
			declared: []TypeDefinition{
				declaredType("recipe", "Recipes", "meals", false), declaredType("chapter", "Recipes", "", true),
			},
		},
		{
			name:   "type handed the root keeps its word against a later new type",
			stored: []content.Type{root, recipe},
			declared: []TypeDefinition{
				declaredType("recipe", "Recipes", "", true), declaredType("chapter", "Chapters", "recipes", false),
			},
			taken: true, code: stated, details: content.Details{"type": "chapter", "name": "Chapters", "word": "recipes"},
		},
		{
			name:   "type handed the root leaves the word it asked for to a later new type",
			stored: []content.Type{root, recipe},
			declared: []TypeDefinition{
				declaredType("recipe", "Recipes", "meals", true), declaredType("chapter", "Chapters", "meals", false),
			},
		},
		{
			name:     "new type derives the word of an active type the file leaves out",
			stored:   []content.Type{root, {Key: "recipe", RouteWord: "recipes", Active: true}},
			declared: []TypeDefinition{declaredType("chapter", "Recipes", "", true)},
			taken:    true, code: "derived_route_word_taken", details: derivedRecipes,
		},
		{
			name:     "new type states a stored word",
			stored:   []content.Type{root, recipe},
			declared: []TypeDefinition{declaredType("zine", "Zines", "recipes", false)},
			taken:    true, code: stated, details: content.Details{"type": "zine", "name": "Zines", "word": "recipes"},
		},
		{
			name:     "stored type states a stored word",
			stored:   []content.Type{root, recipe, book},
			declared: []TypeDefinition{declaredType("book", "Books", "recipes", false)},
			taken:    true, code: stated, details: content.Details{"type": "book", "name": "Books", "word": "recipes"},
		},
		{
			name:   "second new type states the word of the first",
			stored: []content.Type{root},
			declared: []TypeDefinition{
				declaredType("zine", "Zines", " zines ", false), declaredType(" book ", " Books ", "zines", false),
			},
			taken: true, code: stated, details: content.Details{"type": "book", "name": "Books", "word": "zines"},
		},
		{
			name: "padded key shares its word with the same trimmed key",
			declared: []TypeDefinition{
				declaredType(" chapter ", "Books", "books", false), declaredType("chapter", "Books", "books", false),
			},
		},
		{
			name:     "padded copy of a stored key keeps its word",
			stored:   []content.Type{root, recipe},
			declared: []TypeDefinition{declaredType(" recipe ", "Recipes", "recipes", false)},
		},
		{
			name:   "padded copy of a stored key derives over its own word",
			stored: []content.Type{root, recipe, book},
			declared: []TypeDefinition{
				declaredType(" recipe ", "Recipes", "", true), declaredType("book", "Books", "recipes", false),
			},
			taken: true, code: "derived_route_word_taken",
			details: content.Details{"type": "recipe", "name": "Recipes", "word": "recipes"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := validateTypes(test.declared, test.stored)
			if !test.taken {
				if err != nil {
					t.Fatalf("validateTypes() error = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, content.ErrRouteWordTaken) {
				t.Fatalf("validateTypes() error = %v, want %v", err, content.ErrRouteWordTaken)
			}
			if code, _ := content.CodeOf(err); code != test.code {
				t.Errorf("code = %q, want %q", code, test.code)
			}
			if details, _ := content.DetailsOf(err); !reflect.DeepEqual(details, test.details) {
				t.Errorf("details = %v, want %v", details, test.details)
			}
		})
	}
}
