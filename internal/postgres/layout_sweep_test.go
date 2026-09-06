// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// declareFlexible stores a flexible field named features at the top of the car type and returns it.
func declareFlexible(t *testing.T, store *postgres.TypeStore) content.Field {
	t.Helper()
	built, err := content.NewField(
		content.Field{Key: "features", Label: "features", Kind: content.FieldKindFlexible})
	if err != nil {
		t.Fatalf("NewField(flexible) error = %v, want nil", err)
	}
	built.TypeKey = "car"
	stored, err := store.CreateField(t.Context(), built)
	if err != nil {
		t.Fatalf("CreateField(flexible) error = %v, want nil", err)
	}
	return stored
}

// declareLayout stores a layout under the flexible and returns it.
func declareLayout(t *testing.T, store *postgres.TypeStore, parentID int, key string) content.Field {
	t.Helper()
	built, err := content.NewSubField(
		content.Field{Key: key, Label: key, Kind: content.FieldKindLayout}, content.FieldKindFlexible)
	if err != nil {
		t.Fatalf("NewSubField(layout %s) error = %v, want nil", key, err)
	}
	stored, err := store.CreateSubField(t.Context(), parentID, built)
	if err != nil {
		t.Fatalf("CreateSubField(layout %s) error = %v, want nil", key, err)
	}
	return stored
}

// declareUnder stores a text sub field under the parent.
func declareUnder(t *testing.T, store *postgres.TypeStore, parentID int, key string) content.Field {
	t.Helper()
	stored, err := store.CreateSubField(t.Context(), parentID, fieldOn(t, "", key, content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
	}
	return stored
}

func TestDeletingASubFieldSweepsItOnlyFromItsOwnLayout(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	features := declareFlexible(t, store)
	hero := declareLayout(t, store, features.ID, "hero")
	quote := declareLayout(t, store, features.ID, "quote")
	declareUnder(t, store, hero.ID, "title")
	dropped := declareUnder(t, store, hero.ID, "caption")
	declareUnder(t, store, quote.ID, "caption")
	plantTyped(t, pool, author, "car", "one",
		`{"features": [{"hero": {"title": "A", "caption": "drop"}}, `+
			`{"quote": {"caption": "keep"}}, {"hero": {"title": "C", "caption": "drop too"}}]}`)

	if err := store.DeleteSubField(t.Context(), dropped.ID); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := `{"features": [{"hero": {"title": "A"}}, {"quote": {"caption": "keep"}}, {"hero": {"title": "C"}}]}`
	if held := valuesHeld(t, pool); held != want {
		t.Errorf("stored values = %s, want the caption swept from the hero rows alone", held)
	}
	if held := revisionValuesHeld(t, pool, "car"); held != want {
		t.Errorf("revision values = %s, want the same sweep in the revision", held)
	}
}

func TestDeletingALayoutTakesItsRowsAway(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	features := declareFlexible(t, store)
	hero := declareLayout(t, store, features.ID, "hero")
	quote := declareLayout(t, store, features.ID, "quote")
	declareUnder(t, store, hero.ID, "title")
	declareUnder(t, store, quote.ID, "title")
	plantTyped(t, pool, author, "car", "one",
		`{"features": [{"hero": {"title": "A"}}, {"quote": {"title": "B"}}, {"hero": {"title": "C"}}]}`)

	if err := store.DeleteSubField(t.Context(), hero.ID); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := `{"features": [{"quote": {"title": "B"}}]}`
	if held := valuesHeld(t, pool); held != want {
		t.Errorf("stored values = %s, want the hero rows gone rather than emptied", held)
	}
	if held := revisionValuesHeld(t, pool, "car"); held != want {
		t.Errorf("revision values = %s, want the hero rows gone there too", held)
	}
}

func TestDeletingALayoutTakesTheFieldsItHeld(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	features := declareFlexible(t, store)
	hero := declareLayout(t, store, features.ID, "hero")
	declareUnder(t, store, hero.ID, "title")

	if err := store.DeleteSubField(t.Context(), hero.ID); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	held, err := store.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	for _, listed := range held {
		if listed.Key != "car" {
			continue
		}
		if len(listed.Fields) != 1 || len(listed.Fields[0].Fields) != 0 {
			t.Errorf("fields = %v, want the flexible left holding no layout", listed.Fields)
		}
	}
}

func TestDeletingALayoutInsideARepeaterTakesItsRowsAway(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	team := declareRepeater(t, store, "team")
	features, err := store.CreateSubField(t.Context(), team.ID, mustFlexible(t, "features"))
	if err != nil {
		t.Fatalf("CreateSubField(flexible) error = %v, want nil", err)
	}
	hero := declareLayout(t, store, features.ID, "hero")
	quote := declareLayout(t, store, features.ID, "quote")
	declareUnder(t, store, hero.ID, "title")
	declareUnder(t, store, quote.ID, "title")
	plantTyped(t, pool, author, "car", "one",
		`{"team": [{"features": [{"hero": {"title": "A"}}, {"quote": {"title": "B"}}]}]}`)

	if err := store.DeleteSubField(t.Context(), hero.ID); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := `{"team": [{"features": [{"quote": {"title": "B"}}]}]}`
	if held := valuesHeld(t, pool); held != want {
		t.Errorf("stored values = %s, want the hero rows gone inside every repeater row", held)
	}
}

func TestDeletingALayoutReportsASweepItCannotRun(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	features := declareFlexible(t, store)
	hero := declareLayout(t, store, features.ID, "hero")
	declareUnder(t, store, hero.ID, "title")
	plantTyped(t, pool, author, "car", "one", `{"features": [{"hero": {"title": "A"}}]}`)
	sabotage(t, pool, "DROP FUNCTION core.strip_layout(jsonb, text [])")

	err := store.DeleteSubField(t.Context(), hero.ID)

	if err == nil {
		t.Error("DeleteSubField() error = nil, want the sweep it cannot run reported")
	}
}

// mustFlexible returns a flexible sub field ready to stand inside a container.
func mustFlexible(t *testing.T, key string) content.Field {
	t.Helper()
	built, err := content.NewSubField(
		content.Field{Key: key, Label: key, Kind: content.FieldKindFlexible}, content.FieldKindRepeater)
	if err != nil {
		t.Fatalf("NewSubField(flexible %s) error = %v, want nil", key, err)
	}
	return built
}
