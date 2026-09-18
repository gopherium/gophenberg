// SPDX-License-Identifier: Apache-2.0

package definitions_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/definitions"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// authorOn stores a user who may file content and returns the identity the items carry.
func authorOn(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	author := uuid.Must(uuid.NewV7())
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO auth.users (id, email, name, password_hash, disabled, created_at)
		VALUES ($1, $2, 'Maria Perez', 'hash', false, $3)`,
		author, author.String()+"@example.com", time.Now().UTC(),
	); err != nil {
		t.Fatalf("inserting the author: %v", err)
	}
	return author
}

// nestingSite returns the planning site with its recipe type nesting, the content store and an author.
func nestingSite(t *testing.T) (*content.Registry, *postgres.ContentStore, uuid.UUID) {
	t.Helper()
	pool, registry := definedSite(t)
	recipe, err := registry.ByKey(t.Context(), "recipe")
	if err != nil {
		t.Fatalf("ByKey(recipe) error = %v, want nil", err)
	}
	recipe.Hierarchical, recipe.UpdatedAt = true, time.Now().UTC()
	if _, err := registry.Update(t.Context(), recipe); err != nil {
		t.Fatalf("Update(recipe) error = %v, want the type nesting", err)
	}
	return registry, postgres.NewContentStore(pool), authorOn(t, pool)
}

// filed stores an item of the type under the parent and returns it.
func filed(
	t *testing.T, items *postgres.ContentStore, held content.Type, parent *content.Content, title string,
	author uuid.UUID,
) content.Content {
	t.Helper()
	built, err := content.New(held, parent, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	stored, err := items.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return stored
}

// recipeUnderAnother stores a recipe and a second one filed under it.
func recipeUnderAnother(t *testing.T, registry *content.Registry, items *postgres.ContentStore, author uuid.UUID) {
	t.Helper()
	recipe, err := registry.ByKey(t.Context(), "recipe")
	if err != nil {
		t.Fatalf("ByKey(recipe) error = %v, want nil", err)
	}
	bread := filed(t, items, recipe, nil, "Bread", author)
	filed(t, items, recipe, &bread, "Roll", author)
}

// flatteningRecipe marks the envelope's recipe type as flat.
func flatteningRecipe(t *testing.T, envelope definitions.Envelope) {
	t.Helper()
	for i := range envelope.Types {
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].Hierarchical = false
			return
		}
	}
	t.Fatal("the envelope holds no recipe type")
}

// storedNesting reports whether the registry holds the type as nesting.
func storedNesting(t *testing.T, registry *content.Registry, key string) bool {
	t.Helper()
	held, err := registry.ByKey(t.Context(), key)
	if err != nil {
		t.Fatalf("ByKey(%s) error = %v, want nil", key, err)
	}
	return held.Hierarchical
}

// keptNesting reports whether the changes hold the type left nesting.
func keptNesting(changes []definitions.Change, key string) bool {
	for _, held := range changes {
		if held.Subject == "type" && held.Key == key && held.Reason == "nesting_kept" {
			return true
		}
	}
	return false
}

func TestCompareWarnsThatATypeWhoseItemsNestKeepsNesting(t *testing.T) {
	t.Parallel()

	registry, items, author := nestingSite(t)
	recipeUnderAnother(t, registry, items, author)
	envelope := exported(t, registry)
	flatteningRecipe(t, envelope)

	plan := compared(t, registry, envelope)

	warned := false
	for _, held := range plan.Warnings {
		if held.Code == "nesting_kept" && held.Key == "recipe" {
			warned = true
		}
	}
	if !warned {
		t.Errorf("warnings = %+v, want the type kept nesting surfaced", plan.Warnings)
	}
}

func TestCompareWarnsNothingWhenATypeStopsNestingWithNothingNested(t *testing.T) {
	t.Parallel()

	registry, items, author := nestingSite(t)
	recipe, _ := registry.ByKey(t.Context(), "recipe")
	filed(t, items, recipe, nil, "Bread", author)
	envelope := exported(t, registry)
	flatteningRecipe(t, envelope)

	plan := compared(t, registry, envelope)

	if len(plan.Warnings) != 0 {
		t.Errorf("warnings = %+v, want none while nothing sits inside another item", plan.Warnings)
	}
}

func TestCompareReportsNestedItemsItCannotCount(t *testing.T) {
	t.Parallel()

	pool, registry := definedSite(t)
	recipe, err := registry.ByKey(t.Context(), "recipe")
	if err != nil {
		t.Fatalf("ByKey(recipe) error = %v, want nil", err)
	}
	recipe.Hierarchical, recipe.UpdatedAt = true, time.Now().UTC()
	if _, err := registry.Update(t.Context(), recipe); err != nil {
		t.Fatalf("Update(recipe) error = %v, want the type nesting", err)
	}
	envelope := exported(t, registry)
	flatteningRecipe(t, envelope)
	sabotage(t, pool, "ALTER TABLE core.content RENAME COLUMN parent_id TO parent_gone")

	_, err = definitions.Compare(t.Context(), registry, envelope)

	if err == nil {
		t.Error("Compare() error = nil, want the failing count reported")
	}
}

func TestApplyLeavesATypeNestingWhileItsItemsNest(t *testing.T) {
	t.Parallel()

	registry, items, author := nestingSite(t)
	recipeUnderAnother(t, registry, items, author)
	envelope := exported(t, registry)
	flatteningRecipe(t, envelope)
	for i := range envelope.Types {
		if envelope.Types[i].Key == "recipe" {
			envelope.Types[i].SingularLabel = "Dish"
		}
	}

	outcome := applied(t, registry, importing(envelope))

	if !storedNesting(t, registry, "recipe") {
		t.Errorf("the recipe type stopped nesting, want it kept while its items nest")
	}
	if held, _ := registry.ByKey(t.Context(), "recipe"); held.SingularLabel != "Dish" {
		t.Errorf("the recipe type is labeled %q, want the file's label carried anyway", held.SingularLabel)
	}
	if !keptNesting(outcome.Skipped, "recipe") {
		t.Errorf("skipped = %+v, want the kept nesting named there", outcome.Skipped)
	}
}

func TestApplyStopsATypeNestingWhileNoneOfItsItemsNests(t *testing.T) {
	t.Parallel()

	registry, _, _ := nestingSite(t)
	envelope := exported(t, registry)
	flatteningRecipe(t, envelope)

	outcome := applied(t, registry, importing(envelope))

	if storedNesting(t, registry, "recipe") {
		t.Errorf("the recipe type still nests, want the file's flat type stored")
	}
	if len(outcome.Skipped) != 0 {
		t.Errorf("skipped = %+v, want nothing left alone", outcome.Skipped)
	}
}

func TestDeclareTypeKeepsAPluginTypeNestingWhileItsItemsNest(t *testing.T) {
	t.Parallel()

	pool, registrar := declaringPool(t)
	nesting := eventType()
	nesting.Hierarchical = true
	if err := registrar.DeclareType(t.Context(), nesting); err != nil {
		t.Fatalf("DeclareType(nesting) error = %v, want nil", err)
	}
	registry := content.NewRegistry(postgres.NewTypeStore(pool))
	event, err := registry.ByKey(t.Context(), "event")
	if err != nil {
		t.Fatalf("ByKey(event) error = %v, want nil", err)
	}
	items, author := postgres.NewContentStore(pool), authorOn(t, pool)
	gala := filed(t, items, event, nil, "Gala", author)
	filed(t, items, event, &gala, "After party", author)

	err = registrar.DeclareType(t.Context(), eventType())

	if !errors.Is(err, content.ErrNestingInUse) {
		t.Errorf("DeclareType(flat) error = %v, want %v", err, content.ErrNestingInUse)
	}
}
