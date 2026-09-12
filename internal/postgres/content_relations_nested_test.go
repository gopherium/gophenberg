// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// rowsPointing declares a repeater on posts holding a relation that points at categories.
func rowsPointing(t *testing.T, pool *pgxpool.Pool) {
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
	if _, err := types.CreateSubField(t.Context(), stored.ID, inside); err != nil {
		t.Fatalf("declaring the relation inside the rows: %v, want nil", err)
	}
}

// rowsFiledUnder returns the rows value pointing at every target through the relation inside it.
func rowsFiledUnder(targets ...uuid.UUID) content.Values {
	rows := make([]any, len(targets))
	for i, target := range targets {
		rows[i] = map[string]any{"filed": []any{target.String()}}
	}
	return content.Values{"team": rows}
}

// pointedAtBy returns how many published items point at the target through any field.
func pointedAtBy(t *testing.T, store *postgres.ContentStore, target uuid.UUID) int {
	t.Helper()
	_, total, err := store.RelatedTo(t.Context(), target, 1, 20)
	if err != nil {
		t.Fatalf("reading what points at the target: %v, want nil", err)
	}
	return total
}

func TestContentStoreIndexesTheTargetsARowPointsAt(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	rowsPointing(t, pool)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	guides := publishItem(t, store, storedCategory(t, store, "Guides", author))
	post := mustCreate(t, store, "Filed", author)
	post.Fields = rowsFiledUnder(news.ID, guides.ID)
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()

	updated, err := store.Update(t.Context(), post, version, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want the rows stored", err)
	}
	publishItem(t, store, updated)
	if held := pointedAtBy(t, store, news.ID); held != 1 {
		t.Errorf("%d items point at News, want the one pointing through a row", held)
	}
	if held := pointedAtBy(t, store, guides.ID); held != 1 {
		t.Errorf("%d items point at Guides, want the one pointing through a row", held)
	}
}

func TestContentStoreIndexesTheTargetsAFreshItemPointsAt(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	rowsPointing(t, pool)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	built := mustPost(t, "Filed at once", author)
	built.Fields = rowsFiledUnder(news.ID)

	created, err := store.Create(t.Context(), built)

	if err != nil {
		t.Fatalf("Create() error = %v, want the rows stored", err)
	}
	publishItem(t, store, created)
	if held := pointedAtBy(t, store, news.ID); held != 1 {
		t.Errorf("%d items point at News, want the one that pointed as it was created", held)
	}
}

func TestContentStoreForgetsTheTargetsARowNoLongerPointsAt(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	rowsPointing(t, pool)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	post := mustCreate(t, store, "Filed", author)
	post.Fields = rowsFiledUnder(news.ID)
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()
	pointing, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("Update() error = %v, want the rows stored", err)
	}

	pointing.Fields = content.Values{"team": []any{}}
	version = pointing.UpdatedAt
	pointing.UpdatedAt = time.Now().UTC()
	if _, err := store.Update(t.Context(), pointing, version, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want the rows cleared", err)
	}

	if held := pointedAtBy(t, store, news.ID); held != 0 {
		t.Errorf("%d items point at News, want none once the rows were cleared", held)
	}
}

func TestContentStoreReportsAValueNoRelationHolds(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	post := mustCreate(t, store, "Filed", author)
	post.Fields = content.Values{"categories": "news"}
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()

	_, err := store.Update(t.Context(), post, version, nil, 0)

	if err == nil {
		t.Fatal("Update() error = nil, want a relation value of the wrong shape reported")
	}
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

func TestContentStoreRefusesATargetOfTheWrongTypeInsideARow(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	rowsPointing(t, pool)
	other := mustCreate(t, store, "Another post", author)
	post := mustCreate(t, store, "Filed", author)
	post.Fields = rowsFiledUnder(other.ID)
	version := post.UpdatedAt
	post.UpdatedAt = time.Now().UTC()

	_, err := store.Update(t.Context(), post, version, nil, 0)

	if err == nil {
		t.Fatal("Update() error = nil, want a target of the wrong type refused inside a row")
	}
}
