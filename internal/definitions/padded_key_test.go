// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// spaced returns the key with a space on each side.
func spaced(key string) string {
	return " " + key + " "
}

// zine returns a new type declaration under the key, the root when root is set.
func zine(key string, root bool) definitions.TypeDefinition {
	return definitions.TypeDefinition{
		Key: key, SingularLabel: "Zine", PluralLabel: "Zines", RouteWord: "zines",
		PageKind: "single", Default: root, Active: true,
	}
}

// spacedRecipe writes the key of the file's recipe type with spaces around it.
func spacedRecipe(_ *testing.T, envelope *definitions.Envelope) {
	for i := range envelope.Types {
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].Key = spaced("recipe")
		}
	}
}

// spacedCopy adds a second copy of the file's recipe type under the key with spaces around it.
func spacedCopy(_ *testing.T, envelope *definitions.Envelope) {
	for _, d := range envelope.Types {
		if d.Key == "recipe" {
			d.Key = spaced("recipe")
			envelope.Types = append(envelope.Types, d)
			return
		}
	}
}

// spacedField writes the key of a field in the recipe group with spaces around it.
func spacedField(t *testing.T, envelope *definitions.Envelope) {
	t.Helper()
	field := declaredIn(t, groupNamed(t, *envelope, "recipe-details"), "cook-time")
	field.Key = spaced("cook-time")
}

// spacedSubField writes the key of the field inside the steps section with spaces around it.
func spacedSubField(t *testing.T, envelope *definitions.Envelope) {
	t.Helper()
	section := declaredIn(t, groupNamed(t, *envelope, "recipe-details"), "steps")
	section.Fields[0].Key = spaced("note")
}

// spacedFieldCopy adds a second copy of a field in the recipe group under the key with spaces around it.
func spacedFieldCopy(t *testing.T, envelope *definitions.Envelope) {
	t.Helper()
	group := groupNamed(t, *envelope, "recipe-details")
	twin := *declaredIn(t, group, "cook-time")
	twin.Key = spaced("cook-time")
	group.Fields = append(group.Fields, twin)
}

// spacedGroup writes the key of the recipe group with spaces around it.
func spacedGroup(t *testing.T, envelope *definitions.Envelope) {
	t.Helper()
	group := groupNamed(t, *envelope, "recipe-details")
	group.Key = spaced("recipe-details")
}

// spacedKey is one way a file writes a key with spaces around it and the error the import answers.
type spacedKey struct {
	name   string
	change func(t *testing.T, envelope *definitions.Envelope)
	want   error
}

// spacedKeys returns every way a file can write a key with spaces around it.
func spacedKeys() []spacedKey {
	return []spacedKey{
		{name: "type the site holds", change: spacedRecipe, want: content.ErrInvalidKey},
		{name: "second copy of a type", change: spacedCopy, want: content.ErrInvalidKey},
		{
			name: "new type",
			change: func(_ *testing.T, envelope *definitions.Envelope) {
				envelope.Types = append(envelope.Types, zine(spaced("zine"), false))
			},
			want: content.ErrInvalidKey,
		},
		{
			name: "new default type",
			change: func(_ *testing.T, envelope *definitions.Envelope) {
				envelope.Types = append(envelope.Types, zine(spaced("zine"), true))
			},
			want: content.ErrInvalidKey,
		},
		{name: "field", change: spacedField, want: content.ErrInvalidFieldKey},
		{name: "field inside a section", change: spacedSubField, want: content.ErrInvalidFieldKey},
		{name: "second copy of a field", change: spacedFieldCopy, want: content.ErrInvalidFieldKey},
		{name: "group", change: spacedGroup, want: content.ErrInvalidGroupKey},
	}
}

func TestCompareRefusesAKeyWithSpacesAroundIt(t *testing.T) {
	t.Parallel()

	for _, test := range spacedKeys() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			registry := planningSite(t)
			envelope := exported(t, registry)
			test.change(t, &envelope)

			_, err := definitions.Compare(t.Context(), registry, envelope)

			if !errors.Is(err, test.want) {
				t.Errorf("Compare() error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestCompareRefusesASpacedKeyOfATypeAPluginDeclared(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))

	_, err := definitions.Compare(t.Context(), registry, definitions.Envelope{
		Format: definitions.Format,
		Types:  []definitions.TypeDefinition{zine(spaced("event"), false)},
	})

	if !errors.Is(err, content.ErrInvalidKey) {
		t.Errorf("Compare() error = %v, want %v", err, content.ErrInvalidKey)
	}
}

func TestApplyKeepsConfirmedGroupsWhenAKeyHasSpacesAroundIt(t *testing.T) {
	t.Parallel()

	for _, test := range spacedKeys() {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			registry := planningSite(t)
			envelope := exported(t, registry)
			envelope.Groups = slices.DeleteFunc(envelope.Groups, func(g definitions.GroupDefinition) bool {
				return g.Key == "loose-ends"
			})
			test.change(t, &envelope)

			_, err := definitions.Apply(t.Context(), registry, definitions.Import{
				Envelope: envelope,
				Confirm:  []definitions.Confirmed{{Subject: "group", Key: "loose-ends"}},
			})

			if !errors.Is(err, test.want) {
				t.Errorf("Apply() error = %v, want %v", err, test.want)
			}
			if _, found := storedGroup(t, registry, "loose-ends"); !found {
				t.Errorf("the group the file gave up is gone, want it kept while the import stops")
			}
		})
	}
}
