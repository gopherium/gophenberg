// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"reflect"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// recipeRules places a group on the recipe type.
func recipeRules() content.Rules {
	return content.Rules{{{Source: content.ScreenContentType, Operator: content.OperatorIs, Value: "recipe"}}}
}

// siteDefined stores the site's recipe type, its details group with a text field and a section, and a ruleless group.
func siteDefined(t *testing.T, registry *content.Registry) {
	t.Helper()
	recipe, err := content.NewType("recipe", "Recipe", "Recipes", "recipes")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	if _, err := registry.Create(t.Context(), recipe); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	group, err := registry.CreateGroup(t.Context(), content.Group{Title: "Recipe details", Location: recipeRules()})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := registry.CreateFieldInGroup(t.Context(), group.ID, content.Field{
		Key: "cook-time", Label: "Cook time", Kind: content.FieldKindText, Required: true,
		Settings: map[string]any{content.SettingPlaceholder: "Minutes"},
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(cook-time) error = %v, want nil", err)
	}
	section, err := registry.CreateFieldInGroup(t.Context(), group.ID, content.Field{
		Key: "steps", Label: "Steps", Kind: content.FieldKindSection,
	})
	if err != nil {
		t.Fatalf("CreateFieldInGroup(steps) error = %v, want nil", err)
	}
	if _, err := registry.CreateSubField(t.Context(), section.ID, content.Field{
		Key: "note", Label: "Note", Kind: content.FieldKindText,
	}); err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	if _, err := registry.CreateGroup(t.Context(), content.Group{Title: "Loose ends"}); err != nil {
		t.Fatalf("CreateGroup(Loose ends) error = %v, want nil", err)
	}
}

// typeKeyed returns the exported type under the key, and whether one is there.
func typeKeyed(types []definitions.TypeDefinition, key string) (definitions.TypeDefinition, bool) {
	for _, t := range types {
		if t.Key == key {
			return t, true
		}
	}
	return definitions.TypeDefinition{}, false
}

func TestExportCarriesWhatTheSiteDefinedAndLeavesPluginRowsOut(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	declared(t, registrar)
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	siteDefined(t, registry)

	envelope, err := definitions.Export(t.Context(), registry)

	if err != nil {
		t.Fatalf("Export() error = %v, want nil", err)
	}
	if envelope.Format != definitions.Format {
		t.Errorf("format = %q, want %q", envelope.Format, definitions.Format)
	}
	if _, held := typeKeyed(envelope.Types, "event"); held {
		t.Errorf("types = %+v, want the plugin declared event type left out", envelope.Types)
	}
	recipe, held := typeKeyed(envelope.Types, "recipe")
	if !held || recipe.SingularLabel != "Recipe" || recipe.RouteWord != "recipes" || !recipe.Active {
		t.Errorf("recipe = %+v, %v, want the site's type as stored", recipe, held)
	}
	if len(envelope.Groups) != 2 {
		t.Fatalf("groups = %+v, want only the site's two groups", envelope.Groups)
	}
	if loose := envelope.Groups[1]; loose.Key != "loose-ends" || loose.Location == nil || len(loose.Location) != 0 {
		t.Errorf("loose ends = %+v, want the ruleless group with an empty location, never a missing one", loose)
	}
	group := envelope.Groups[0]
	if group.Key != "recipe-details" || group.Title != "Recipe details" || !group.Active {
		t.Errorf("group = %+v, want the site's group as stored", group)
	}
	if !group.Location.Equal(recipeRules()) {
		t.Errorf("location = %+v, want the rules the group was created with", group.Location)
	}
	want := []definitions.FieldDefinition{
		{Key: "cook-time", Label: "Cook time", Kind: "text", Required: true,
			Settings: map[string]any{content.SettingPlaceholder: "Minutes"}},
		{Key: "steps", Label: "Steps", Kind: "section", Fields: []definitions.FieldDefinition{
			{Key: "note", Label: "Note", Kind: "text"},
		}},
	}
	if !reflect.DeepEqual(group.Fields, want) {
		t.Errorf("fields = %+v, want %+v", group.Fields, want)
	}
}

func TestExportReportsARegistryItCannotRead(t *testing.T) {
	t.Parallel()

	for name, statement := range map[string]string{
		"types behind an unreadable table":  "ALTER TABLE core.content_types RENAME COLUMN singular_label TO retired",
		"groups behind an unreadable table": "ALTER TABLE core.field_groups RENAME COLUMN title TO retired",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			pool, _ := declaringPool(t)
			sabotage(t, pool, statement)

			_, err := definitions.Export(t.Context(), content.NewRegistry(postgres.NewTypeStore(pool)))

			if err == nil {
				t.Errorf("%s: error = nil, want the unreadable store reported", name)
			}
		})
	}
}
