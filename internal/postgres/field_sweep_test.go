// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
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

// declareRepeater is the shared fixture these tests call.
var declareRepeater = contenttest.DeclareRepeater

func TestDeletingASubFieldReportsWhatItCannotReach(t *testing.T) {
	t.Parallel()

	for name, sabotaged := range map[string]func(*testing.T, *pgxpool.Pool){
		"the groups it cannot read": func(t *testing.T, pool *pgxpool.Pool) {
			sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")
		},
		"the types it cannot read": func(t *testing.T, pool *pgxpool.Pool) {
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
				t.Context(), specs.ID, fieldOn(t, "", "doors", content.FieldKindText, ""), deepEnough)
			if err != nil {
				t.Fatalf("declaring doors: %v, want nil", err)
			}
			plantTyped(t, pool, author, "car", "one", `{"specs": {"doors": "five"}}`)
			sabotaged(t, pool)

			if err := store.DeleteSubField(t.Context(), dropped.ID, nil); err == nil {
				t.Error("DeleteSubField() error = nil, want the failure reported")
			}
		})
	}
}
