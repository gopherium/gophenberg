// SPDX-License-Identifier: Apache-2.0

package definitions

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// derivedWordTaken is the code an answer carries when a new default type derives a held word.
const derivedWordTaken = "derived_route_word_taken"

// statedWordTaken is the code an answer carries when a type states a held word.
const statedWordTaken = "stated_route_word_taken"

// wordlessRoot is the stored root answering under no word.
var wordlessRoot = content.Type{Key: "post", Default: true}

// recipeType is a stored type holding the word recipes.
var recipeType = content.Type{Key: "recipe", RouteWord: "recipes"}

// bookType is a stored type holding the word books.
var bookType = content.Type{Key: "book", RouteWord: "books"}

// chapterDerivesRecipes is what an answer names when the new type chapter derives the word recipes.
var chapterDerivesRecipes = content.Details{"type": "chapter", "name": "Recipes", "word": "recipes"}

// wordCase is one file the route word checks judge and the answer it gets when its word is taken.
type wordCase struct {
	name     string
	stored   []content.Type
	declared []TypeDefinition
	taken    bool
	code     string
	details  content.Details
}

// declaredType returns a valid declaration under the key, plural label and route word, the root when root is set.
func declaredType(key, plural, word string, root bool) TypeDefinition {
	return TypeDefinition{
		Key: key, SingularLabel: "Item", PluralLabel: plural, RouteWord: word,
		PageKind: "single", Default: root, Active: true,
	}
}

// checksWords fails each case whose file does not get the answer the case names.
func checksWords(t *testing.T, cases []wordCase) {
	t.Helper()
	for _, test := range cases {
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

func TestImportChecksRouteWordsInTheOrderApplyWrites(t *testing.T) {
	t.Parallel()

	checksWords(t, []wordCase{
		{
			name:   "stored type moves onto a word a new type derived",
			stored: []content.Type{wordlessRoot, bookType},
			declared: []TypeDefinition{
				declaredType("chapter", "Recipes", "", true), declaredType("book", "Books", "recipes", false),
			},
			taken: true, code: derivedWordTaken, details: chapterDerivesRecipes,
		},
		{
			name:   "new type claims a word before its holder moves away",
			stored: []content.Type{wordlessRoot, recipeType},
			declared: []TypeDefinition{
				declaredType("chapter", "Recipes", "", true), declaredType("recipe", "Recipes", "meals", false),
			},
			taken: true, code: derivedWordTaken, details: chapterDerivesRecipes,
		},
		{
			name:   "new type claims a word after its holder moved away",
			stored: []content.Type{wordlessRoot, recipeType},
			declared: []TypeDefinition{
				declaredType("recipe", "Recipes", "meals", false), declaredType("chapter", "Recipes", "", true),
			},
		},
		{
			name:     "new type derives the word of an active type the file leaves out",
			stored:   []content.Type{wordlessRoot, {Key: "recipe", RouteWord: "recipes", Active: true}},
			declared: []TypeDefinition{declaredType("chapter", "Recipes", "", true)},
			taken:    true, code: derivedWordTaken, details: chapterDerivesRecipes,
		},
		{
			name:     "new type states a stored word",
			stored:   []content.Type{wordlessRoot, recipeType},
			declared: []TypeDefinition{declaredType("zine", "Zines", "recipes", false)},
			taken:    true, code: statedWordTaken,
			details: content.Details{"type": "zine", "name": "Zines", "word": "recipes"},
		},
		{
			name:     "stored type states a stored word",
			stored:   []content.Type{wordlessRoot, recipeType, bookType},
			declared: []TypeDefinition{declaredType("book", "Books", "recipes", false)},
			taken:    true, code: statedWordTaken,
			details: content.Details{"type": "book", "name": "Books", "word": "recipes"},
		},
		{
			name:   "second new type states the word of the first",
			stored: []content.Type{wordlessRoot},
			declared: []TypeDefinition{
				declaredType("zine", "Zines", " zines ", false), declaredType(" book ", " Books ", "zines", false),
			},
			taken: true, code: statedWordTaken, details: content.Details{"type": "book", "name": "Books", "word": "zines"},
		},
	})
}

func TestImportChecksRouteWordsWithRootChangesLeftAsApplyLeavesThem(t *testing.T) {
	t.Parallel()

	checksWords(t, []wordCase{
		{
			name:     "kept root asks for a word another type holds",
			stored:   []content.Type{wordlessRoot, recipeType},
			declared: []TypeDefinition{declaredType("post", "Posts", "recipes", true)},
			taken:    true, code: statedWordTaken,
			details: content.Details{"type": "post", "name": "Posts", "word": "recipes"},
		},
		{
			name:     "type handed the root derives no word from its label",
			stored:   []content.Type{wordlessRoot, recipeType, bookType},
			declared: []TypeDefinition{declaredType("recipe", "Books", "", true)},
		},
		{
			name:   "kept root holds its own word against a new type",
			stored: []content.Type{{Key: "post", Default: true, RouteWord: "blog"}},
			declared: []TypeDefinition{
				declaredType("post", "Posts", "blog", true), declaredType("book", "Books", "blog", false),
			},
			taken: true, code: statedWordTaken, details: content.Details{"type": "book", "name": "Books", "word": "blog"},
		},
		{
			name:   "type handed the root keeps its word against a later new type",
			stored: []content.Type{wordlessRoot, recipeType},
			declared: []TypeDefinition{
				declaredType("recipe", "Recipes", "", true), declaredType("chapter", "Chapters", "recipes", false),
			},
			taken: true, code: statedWordTaken,
			details: content.Details{"type": "chapter", "name": "Chapters", "word": "recipes"},
		},
		{
			name:   "type handed the root leaves the word it asked for to a later new type",
			stored: []content.Type{wordlessRoot, recipeType},
			declared: []TypeDefinition{
				declaredType("recipe", "Recipes", "meals", true), declaredType("chapter", "Chapters", "meals", false),
			},
		},
	})
}

func TestImportChecksThePaddedKeyAWordIsHeldUnder(t *testing.T) {
	t.Parallel()

	checksWords(t, []wordCase{
		{
			name: "padded key shares its word with the same trimmed key",
			declared: []TypeDefinition{
				declaredType(" chapter ", "Books", "books", false), declaredType("chapter", "Books", "books", false),
			},
		},
		{
			name:     "padded copy of a stored key keeps its word",
			stored:   []content.Type{wordlessRoot, recipeType},
			declared: []TypeDefinition{declaredType(" recipe ", "Recipes", "recipes", false)},
		},
		{
			name:   "padded copy of a stored key derives over its own word",
			stored: []content.Type{wordlessRoot, recipeType, bookType},
			declared: []TypeDefinition{
				declaredType(" recipe ", "Recipes", "", true), declaredType("book", "Books", "recipes", false),
			},
			taken: true, code: derivedWordTaken,
			details: content.Details{"type": "recipe", "name": "Recipes", "word": "recipes"},
		},
	})
}
