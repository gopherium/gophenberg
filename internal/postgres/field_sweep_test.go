// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// valuesHeld returns the stored values of the planted car row.
func valuesHeld(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var held string
	if err := pool.QueryRow(t.Context(),
		`SELECT fields::text FROM core.content WHERE type = $1`, "car").Scan(&held); err != nil {
		t.Fatalf("reading the stored values: %v, want nil", err)
	}
	return held
}

// revisionValuesHeld returns the stored values of the planted revision of the type.
func revisionValuesHeld(t *testing.T, pool *pgxpool.Pool, typeKey string) string {
	t.Helper()
	var held string
	if err := pool.QueryRow(t.Context(),
		`SELECT r.fields::text FROM core.content_revisions r
		JOIN core.content c ON r.content_id = c.id WHERE c.type = $1`, typeKey).Scan(&held); err != nil {
		t.Fatalf("reading the stored revision values: %v, want nil", err)
	}
	return held
}

func TestDeletingASubFieldSweepsItsValuesInsideASection(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	if _, err := store.CreateSubField(
		t.Context(), specs.ID, fieldOn(t, "", "colour", content.FieldKindText, "")); err != nil {
		t.Fatalf("declaring colour: %v, want nil", err)
	}
	dropped, err := store.CreateSubField(
		t.Context(), specs.ID, fieldOn(t, "", "doors", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("declaring doors: %v, want nil", err)
	}
	plantTyped(t, pool, author, "car", "one", `{"specs": {"colour": "red", "doors": "five"}}`)

	if err := store.DeleteSubField(t.Context(), dropped.ID); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	if held := valuesHeld(t, pool); held != `{"specs": {"colour": "red"}}` {
		t.Errorf("stored values = %s, want the doors swept from inside the section", held)
	}
	if held := revisionValuesHeld(t, pool, "car"); held != `{"specs": {"colour": "red"}}` {
		t.Errorf("revision values = %s, want the doors swept there too", held)
	}
}

func TestDeletingASubFieldSweepsItsValuesFromEveryRow(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	team := declareRepeater(t, store, "team")
	if _, err := store.CreateSubField(
		t.Context(), team.ID, fieldOn(t, "", "name", content.FieldKindText, "")); err != nil {
		t.Fatalf("declaring name: %v, want nil", err)
	}
	dropped, err := store.CreateSubField(
		t.Context(), team.ID, fieldOn(t, "", "role", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("declaring role: %v, want nil", err)
	}
	plantTyped(t, pool, author, "car", "one",
		`{"team": [{"name": "Maria Perez", "role": "lead"}, {"name": "Kip", "role": "smith"}]}`)

	if err := store.DeleteSubField(t.Context(), dropped.ID); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	want := `{"team": [{"name": "Maria Perez"}, {"name": "Kip"}]}`
	if held := valuesHeld(t, pool); held != want {
		t.Errorf("stored values = %s, want the role swept from every row", held)
	}
}

func TestDeletingASubFieldLeavesWhatThePathDoesNotReach(t *testing.T) {
	t.Parallel()

	for name, asked := range map[string]struct {
		kind    content.FieldKind
		planted string
		want    string
	}{
		"the container key holding a word": {
			content.FieldKindSection, `{"specs": "typed"}`, `{"specs": "typed"}`,
		},
		"an item lacking the container key": {
			content.FieldKindSection, `{"colour": "red"}`, `{"colour": "red"}`,
		},
		"a section lacking the inner key": {
			content.FieldKindSection, `{"specs": {"colour": "red"}}`, `{"specs": {"colour": "red"}}`,
		},
		"rows that are not objects": {
			content.FieldKindRepeater, `{"specs": [42, "stray", {"doors": "five"}]}`, `{"specs": [42, "stray", {}]}`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, author, pool := typedStore(t)
			storeType(t, store, "car")
			var parent content.Field
			if asked.kind == content.FieldKindRepeater {
				parent = declareRepeater(t, store, "specs")
			} else {
				parent = declareSection(t, store, "specs")
			}
			dropped, err := store.CreateSubField(
				t.Context(), parent.ID, fieldOn(t, "", "doors", content.FieldKindText, ""))
			if err != nil {
				t.Fatalf("declaring doors: %v, want nil", err)
			}
			plantTyped(t, pool, author, "car", "one", asked.planted)

			if err := store.DeleteSubField(t.Context(), dropped.ID); err != nil {
				t.Fatalf("DeleteSubField() error = %v, want nil", err)
			}

			if held := valuesHeld(t, pool); held != asked.want {
				t.Errorf("stored values = %s, want %s left untouched by the sweep", held, asked.want)
			}
		})
	}
}

func TestDeletingAContainerSweepsEverythingInsideIt(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	team := declareRepeater(t, store, "team")
	inner, err := store.CreateSubField(t.Context(), team.ID, sectionOn(t, "contact"))
	if err != nil {
		t.Fatalf("declaring contact: %v, want nil", err)
	}
	if _, err := store.CreateSubField(
		t.Context(), inner.ID, fieldOn(t, "", "phone", content.FieldKindText, "")); err != nil {
		t.Fatalf("declaring phone: %v, want nil", err)
	}
	plantTyped(t, pool, author, "car", "one",
		`{"team": [{"contact": {"phone": "184467235"}}]}`)

	if err := store.DeleteSubField(t.Context(), inner.ID); err != nil {
		t.Fatalf("DeleteSubField() error = %v, want nil", err)
	}

	if held := valuesHeld(t, pool); held != `{"team": [{}]}` {
		t.Errorf("stored values = %s, want the whole contact swept from the row", held)
	}
}

func TestDeletingASubFieldReportsOneThatIsGone(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")

	err := store.DeleteSubField(t.Context(), 424242)

	if err == nil {
		t.Error("DeleteSubField() error = nil, want the field that is gone reported")
	}
}

func TestDeletingASubFieldReportsWhatItCannotReach(t *testing.T) {
	t.Parallel()

	for name, sabotaged := range map[string]func(*testing.T, *pgxpool.Pool){
		"the groups it cannot read": func(t *testing.T, pool *pgxpool.Pool) {
			sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")
		},
		"the types it cannot match": func(t *testing.T, pool *pgxpool.Pool) {
			sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN key TO retired")
		},
		"the row it cannot remove": func(t *testing.T, pool *pgxpool.Pool) {
			raiseOn(t, pool, "core.content_fields", "DELETE")
		},
		"the values it cannot sweep": func(t *testing.T, pool *pgxpool.Pool) {
			raiseOn(t, pool, "core.content", "UPDATE")
		},
		"the revisions it cannot sweep": func(t *testing.T, pool *pgxpool.Pool) {
			raiseOn(t, pool, "core.content_revisions", "UPDATE")
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, author, pool := typedStore(t)
			storeType(t, store, "car")
			specs := declareSection(t, store, "specs")
			dropped, err := store.CreateSubField(
				t.Context(), specs.ID, fieldOn(t, "", "doors", content.FieldKindText, ""))
			if err != nil {
				t.Fatalf("declaring doors: %v, want nil", err)
			}
			plantTyped(t, pool, author, "car", "one", `{"specs": {"doors": "five"}}`)
			sabotaged(t, pool)

			if err := store.DeleteSubField(t.Context(), dropped.ID); err == nil {
				t.Error("DeleteSubField() error = nil, want the failure reported")
			}
		})
	}
}

// declareRepeater stores a repeater at the top of the car type and returns it.
func declareRepeater(t *testing.T, store *postgres.TypeStore, key string) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: "car", Key: key, Label: key, Kind: content.FieldKindRepeater,
	})
	if err != nil {
		t.Fatalf("NewField(repeater %s) error = %v, want nil", key, err)
	}
	stored, err := store.CreateField(t.Context(), built)
	if err != nil {
		t.Fatalf("CreateField(repeater %s) error = %v, want nil", key, err)
	}
	return stored
}
