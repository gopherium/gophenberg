// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// rootOn gives the site's root the route word and lets its items nest, returning it as stored.
func rootOn(t *testing.T, registry *content.Registry, word string) content.Type {
	t.Helper()
	root, err := registry.ByKey(t.Context(), content.TypePost)
	if err != nil {
		t.Fatalf("ByKey(post) error = %v, want nil", err)
	}
	root.RouteWord, root.Hierarchical = word, true
	stored, err := registry.Update(t.Context(), root)
	if err != nil {
		t.Fatalf("Update(post) error = %v, want nil", err)
	}
	return stored
}

// nested files an item under a parent of the same type.
func nested(t *testing.T, pool *pgxpool.Pool, held content.Type, parent, child string) {
	t.Helper()
	items, author := postgres.NewContentStore(pool), authorOn(t, pool)
	top := filed(t, items, held, nil, parent, author)
	filed(t, items, held, &top, child, author)
}

// wordTooLongToIndex returns a route word the unique index on route words cannot hold.
func wordTooLongToIndex() string {
	var word strings.Builder
	word.WriteString("a")
	for i := range 250 {
		fmt.Fprintf(&word, "%x", sha256.Sum256([]byte{byte(i)}))
	}
	return word.String()
}

// movesWord returns the site's export with the type under the key on the route word and no groups.
func movesWord(t *testing.T, registry *content.Registry, key, word string) definitions.Envelope {
	t.Helper()
	envelope := exported(t, registry)
	for i := range envelope.Types {
		if envelope.Types[i].Key == key {
			envelope.Types[i].RouteWord = word
		}
	}
	envelope.Groups = nil
	return envelope
}

func TestApplyKeepsConfirmedGroupsWhenATypeCannotTakeItsWord(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		key  string
		word string
		site func(t *testing.T, pool *pgxpool.Pool, registry *content.Registry)
		want error
	}{
		{
			name: "address another type's item holds", key: "recipe", word: "meals",
			site: func(t *testing.T, pool *pgxpool.Pool, registry *content.Registry) {
				t.Helper()
				nested(t, pool, rootOn(t, registry, ""), "Meals", "Soup")
				recipe, err := registry.ByKey(t.Context(), "recipe")
				if err != nil {
					t.Fatalf("ByKey(recipe) error = %v, want nil", err)
				}
				filed(t, postgres.NewContentStore(pool), recipe, nil, "Soup", authorOn(t, pool))
			},
			want: content.ErrSlugTaken,
		},
		{
			name: "address a listing keeps", key: content.TypePost, word: "",
			site: func(t *testing.T, pool *pgxpool.Pool, registry *content.Registry) {
				t.Helper()
				nested(t, pool, rootOn(t, registry, "home"), "Page", "2")
			},
			want: content.ErrReservedAddress,
		},
		{
			name: "word too long to index", key: "recipe", word: wordTooLongToIndex(),
			site: func(*testing.T, *pgxpool.Pool, *content.Registry) {},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pool, registry := definedSite(t)
			test.site(t, pool, registry)
			envelope := movesWord(t, registry, test.key, test.word)
			compared(t, registry, envelope)

			_, err := definitions.Apply(t.Context(), registry, definitions.Import{
				Envelope: envelope,
				Confirm:  []definitions.Confirmed{{Subject: "group", Key: "recipe-details"}},
			})

			if err == nil || (test.want != nil && !errors.Is(err, test.want)) {
				t.Errorf("Apply() error = %v, want %v", err, test.want)
			}
			if _, found := storedGroup(t, registry, "recipe-details"); !found {
				t.Errorf("the recipe group is gone, want it kept while no type took its word")
			}
		})
	}
}

func TestApplyLandsAGroupWhereAConfirmedGroupLeft(t *testing.T) {
	t.Parallel()

	registry := planningSite(t)
	side, err := registry.CreateGroup(t.Context(), content.Group{
		Title: "Side notes",
		Location: content.Rules{{{
			Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.TypePost,
		}}},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), side.ID, content.Field{
		Key: "cook-time", Label: "Cook time", Kind: content.FieldKindText,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	envelope := exported(t, registry)
	groupNamed(t, envelope, side.Key).Location = recipeRules()
	envelope.Groups = slices.DeleteFunc(envelope.Groups, func(g definitions.GroupDefinition) bool {
		return g.Key == "recipe-details"
	})

	applied(t, registry, definitions.Import{
		Envelope: envelope,
		Confirm:  []definitions.Confirmed{{Subject: "group", Key: "recipe-details"}},
	})

	if _, found := storedGroup(t, registry, "recipe-details"); found {
		t.Errorf("the recipe group stands, want it gone once the import finished")
	}
}
