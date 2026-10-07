// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// holding is the shared fixture under the name these tests use.
var holding = contenttest.Holding

// declareFilterable declares one field of every kind a filter reads on the post type.
func declareFilterable(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	declareField(t, pool, "note", content.FieldKindText)
	declareField(t, pool, "price", content.FieldKindNumber)
	declareField(t, pool, "on-sale", content.FieldKindBoolean)
	declareField(t, pool, "since", content.FieldKindDate)
	declareChoice(t, pool, "colour", false)
	declareChoice(t, pool, "tags", true)
}

// declareChoice declares a choice field offering red, blue and warm, holding several when asked.
func declareChoice(t *testing.T, pool *pgxpool.Pool, key string, multiple bool) {
	t.Helper()
	declared := fieldOn(t, content.TypePost, key, content.FieldKindChoice, "")
	declared.Settings = map[string]any{
		content.SettingChoices: []any{
			map[string]any{"value": "red", "label": "Red"},
			map[string]any{"value": "blue", "label": "Blue"},
			map[string]any{"value": "warm", "label": "Warm"},
		},
		content.SettingMultiple: multiple,
	}
	if _, err := postgres.NewTypeStore(pool).CreateField(t.Context(), declared); err != nil {
		t.Fatalf("declaring the %q field: %v, want nil", key, err)
	}
}

func TestMigrationsIndexTheFieldValuesForContainment(t *testing.T) {
	t.Parallel()

	held := newTestDB(t)

	var definition string
	err := held.QueryRow(
		`SELECT indexdef FROM pg_indexes
		WHERE schemaname = 'core' AND tablename = 'content' AND indexname = 'content_field_values_idx'`,
	).Scan(&definition)

	if err != nil {
		t.Fatalf("querying pg_indexes: %v", err)
	}
	for _, want := range []string{"USING gin", "jsonb_path_ops"} {
		if !strings.Contains(definition, want) {
			t.Errorf("index = %q, want it to carry %q", definition, want)
		}
	}
}

func TestContentStoreReportsANarrowedListingItCannotRead(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	declareFilterable(t, pool)
	holding(t, store, "Match", author, content.Values{"price": float64(10)})
	sabotage(t, pool, "ALTER TABLE core.content DROP COLUMN excerpt CASCADE")

	narrowed := content.Filter{
		Type: content.TypePost, Page: 1, PerPage: 20, Fields: map[string]any{"price": float64(10)},
	}
	if _, _, err := store.List(t.Context(), narrowed); err == nil {
		t.Error("List() narrowed by fields error = nil, want the unreadable listing reported")
	}
}
