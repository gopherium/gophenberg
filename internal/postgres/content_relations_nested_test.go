// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// rowsFiledUnder and pointedAtBy are the shared row fixtures under the names these tests use.
var (
	rowsFiledUnder = contenttest.RowsFiledUnder
	pointedAtBy    = contenttest.PointedAtBy
)

// rowsPointing declares a repeater on posts holding a relation that points at categories, and returns it.
func rowsPointing(t *testing.T, pool *pgxpool.Pool) content.Field {
	t.Helper()
	types := postgres.NewTypeStore(pool)
	rows, err := content.NewField(content.Field{
		TypeKey: "post", Key: "team", Label: "Team", Kind: content.FieldKindRepeater,
	})
	if err != nil {
		t.Fatalf("NewField(team) error = %v, want nil", err)
	}
	stored, err := types.CreateField(t.Context(), rows)
	if err != nil {
		t.Fatalf("declaring the repeater: %v, want nil", err)
	}
	inside, err := content.NewSubField(content.Field{
		Key: "filed", Label: "Filed", Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	}, content.FieldKindRepeater)
	if err != nil {
		t.Fatalf("NewSubField(filed) error = %v, want nil", err)
	}
	declared, err := types.CreateSubField(t.Context(), stored.ID, inside, deepEnough)
	if err != nil {
		t.Fatalf("declaring the relation inside the rows: %v, want nil", err)
	}
	return declared
}

func TestContentStoreReportsAFreshItemItCannotStore(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	raiseOn(t, pool, "core.content", "INSERT")
	built := mustPost(t, "Filed at once", author)
	built.Fields = content.Values{"categories": namedTargets([]uuid.UUID{news.ID})}

	_, err := store.Create(t.Context(), built)

	if err == nil {
		t.Error("Create() error = nil, want the failing write reported")
	}
}
