// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// fieldRow is where a stored field row stands: its group, its parent and its depth.
type fieldRow struct {
	group  int
	parent *int
	depth  int
}

// rowOf reads where the field row stands, straight from the table.
func rowOf(t *testing.T, pool *pgxpool.Pool, id int) fieldRow {
	t.Helper()
	var held fieldRow
	if err := pool.QueryRow(t.Context(),
		`SELECT group_id, parent_field_id, depth FROM core.content_fields WHERE id = $1`, id,
	).Scan(&held.group, &held.parent, &held.depth); err != nil {
		t.Fatalf("reading the row of field %d: %v, want nil", id, err)
	}
	return held
}

// standsUnder asserts the row sits in the group under the parent at the depth, zero parent meaning the top.
func standsUnder(t *testing.T, pool *pgxpool.Pool, id, group, parent, depth int) {
	t.Helper()
	held := rowOf(t, pool, id)
	at := 0
	if held.parent != nil {
		at = *held.parent
	}
	if held.group != group || at != parent || held.depth != depth {
		t.Errorf("field %d stands in group %d under %d at depth %d, want group %d under %d at depth %d",
			id, held.group, at, held.depth, group, parent, depth)
	}
}

// plantedID returns the identity of the planted row carrying the slug.
func plantedID(t *testing.T, pool *pgxpool.Pool, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(t.Context(),
		`SELECT id FROM core.content WHERE slug = $1`, slug).Scan(&id); err != nil {
		t.Fatalf("reading the planted row %q: %v, want nil", slug, err)
	}
	return id
}

// plantRelation stores one index row pointing from the planted row through the field at the other.
func plantRelation(t *testing.T, pool *pgxpool.Pool, from, to uuid.UUID, field int) {
	t.Helper()
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO core.content_relations (from_id, field_id, to_id, position, sort_at, visible)
		VALUES ($1, $2, $3, 1, now(), true)`, from, field, to); err != nil {
		t.Fatalf("planting the relation row: %v, want nil", err)
	}
}

// relationRowsOf counts the index rows the field holds.
func relationRowsOf(t *testing.T, pool *pgxpool.Pool, field int) int {
	t.Helper()
	var held int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM core.content_relations WHERE field_id = $1`, field).Scan(&held); err != nil {
		t.Fatalf("counting the relation rows of field %d: %v, want nil", field, err)
	}
	return held
}

// groupOn, titleIn, restingTwinOf and relationInside are the shared fixtures these tests call.
var (
	groupOn        = contenttest.GroupOn
	titleIn        = contenttest.TitleIn
	restingTwinOf  = contenttest.RestingTwinOf
	relationInside = contenttest.RelationInside
)

func TestMovingAFieldIntoAContainerStoresItsRowOneLevelDown(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	title := declareTypedField(t, store, "car", "title")

	if _, err := store.MoveField(t.Context(), title.ID, specs.GroupID, specs.ID, deepEnough, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	standsUnder(t, pool, title.ID, specs.GroupID, specs.ID, 1)
}

func TestMovingAContainerStoresTheDepthOfEveryRowBelowIt(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	inner := declaredInside(t, store, specs, "inner", content.FieldKindSection)
	leaf := declaredInside(t, store, inner, "leaf", content.FieldKindText)
	box := declareSection(t, store, "box")

	if _, err := store.MoveField(t.Context(), specs.ID, box.GroupID, box.ID, deepEnough, nil); err != nil {
		t.Fatalf("MoveField(into box) error = %v, want nil", err)
	}
	standsUnder(t, pool, specs.ID, box.GroupID, box.ID, 1)
	standsUnder(t, pool, inner.ID, box.GroupID, specs.ID, 2)
	standsUnder(t, pool, leaf.ID, box.GroupID, inner.ID, 3)

	if _, err := store.MoveField(t.Context(), specs.ID, box.GroupID, 0, deepEnough, nil); err != nil {
		t.Fatalf("MoveField(to the top) error = %v, want nil", err)
	}
	standsUnder(t, pool, specs.ID, box.GroupID, 0, 0)
	standsUnder(t, pool, inner.ID, box.GroupID, specs.ID, 1)
	standsUnder(t, pool, leaf.ID, box.GroupID, inner.ID, 2)
}

func TestMovingAFieldOutOfAContainerStoresItsRowAfterTheTopFields(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	doors := declaredInside(t, store, specs, "doors", content.FieldKindText)

	if _, err := store.MoveField(t.Context(), doors.ID, specs.GroupID, 0, deepEnough, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held, after := positionOf(t, pool, doors.ID), positionOf(t, pool, specs.ID); held != after+1 {
		t.Errorf("the moved field stands at position %d, want it right after the section at %d", held, after)
	}
	standsUnder(t, pool, doors.ID, specs.GroupID, 0, 0)
}

func TestMovingAFieldBetweenTopsStoresItsRowAtTheTopOfTheNewGroup(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	title := titleIn(t, store, groupOn(t, store, "Car extras", "car").ID)
	books := groupOn(t, store, "Book extras", "book")
	plantTyped(t, pool, author, "car", "one", `{"title": "old words"}`)

	if _, err := store.MoveField(t.Context(), title.ID, books.ID, 0, deepEnough, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	standsUnder(t, pool, title.ID, books.ID, 0, 0)
}

func TestMovingAShadowedRelationOffContentDeletesOnlyItsOwnIndexRows(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "book")
	maker := fieldOn(t, "", "maker", content.FieldKindRelation, "book")
	serving, err := store.CreateFieldInGroup(t.Context(), groupOn(t, store, "Car facts", "car").ID, maker, nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(facts maker) error = %v, want nil", err)
	}
	shadowed, err := store.CreateFieldInGroup(t.Context(), groupOn(t, store, "Car extras", "car").ID, maker, nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(extras maker) error = %v, want nil", err)
	}
	storeType(t, store, "car")
	books := groupOn(t, store, "Book extras", "book")
	plantTyped(t, pool, author, "car", "pointing", `{"maker": ["00000000-0000-0000-0000-000000000000"]}`)
	plantTyped(t, pool, author, "book", "pointed", `{}`)
	for _, id := range []int{serving.ID, shadowed.ID} {
		plantRelation(t, pool, plantedID(t, pool, "pointing"), plantedID(t, pool, "pointed"), id)
	}

	if _, err := store.MoveField(t.Context(), shadowed.ID, books.ID, 0, deepEnough, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := relationRowsOf(t, pool, serving.ID); held != 1 {
		t.Errorf("the serving relation indexes %d rows, want its own row kept", held)
	}
	if held := relationRowsOf(t, pool, shadowed.ID); held != 0 {
		t.Errorf("the moved relation indexes %d rows on content it left, want none", held)
	}
}

func TestMovingARelationBetweenTopsDeletesItsIndexRowsWhereTheNewGroupMisses(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	cars := groupOn(t, store, "Car extras", "car")
	maker, err := store.CreateFieldInGroup(
		t.Context(), cars.ID, fieldOn(t, "", "maker", content.FieldKindRelation, "car"), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(maker) error = %v, want nil", err)
	}
	books := groupOn(t, store, "Book extras", "book")
	plantTyped(t, pool, author, "car", "one", `{"maker": ["00000000-0000-0000-0000-000000000000"]}`)
	plantTyped(t, pool, author, "car", "two", `{}`)
	plantRelation(t, pool, plantedID(t, pool, "one"), plantedID(t, pool, "two"), maker.ID)

	if _, err := store.MoveField(t.Context(), maker.ID, books.ID, 0, deepEnough, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := relationRowsOf(t, pool, maker.ID); held != 0 {
		t.Errorf("%d relation rows survive the move, want the index swept where the book group misses", held)
	}
}

func TestMovingARelationDeletesItsIndexRows(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	maker, err := store.CreateField(t.Context(), fieldOn(t, "car", "maker", content.FieldKindRelation, "car"))
	if err != nil {
		t.Fatalf("CreateField(maker) error = %v, want nil", err)
	}
	plantTyped(t, pool, author, "car", "one", `{"maker": ["00000000-0000-0000-0000-000000000000"]}`)
	plantTyped(t, pool, author, "car", "two", `{}`)
	plantRelation(t, pool, plantedID(t, pool, "one"), plantedID(t, pool, "two"), maker.ID)

	if _, err := store.MoveField(t.Context(), maker.ID, specs.GroupID, specs.ID, deepEnough, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := relationRowsOf(t, pool, maker.ID); held != 0 {
		t.Errorf("%d relation rows survive the move, want the index swept with the values", held)
	}
}

func TestMovingASectionDeletesTheIndexRowsOfTheRelationsInsideIt(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	maker := relationInside(t, store, specs, "maker")
	box := declareSection(t, store, "box")
	plantTyped(t, pool, author, "car", "one", `{"specs": {"maker": []}}`)
	plantTyped(t, pool, author, "car", "two", `{}`)
	plantRelation(t, pool, plantedID(t, pool, "one"), plantedID(t, pool, "two"), maker.ID)

	if _, err := store.MoveField(t.Context(), specs.ID, box.GroupID, box.ID, deepEnough, nil); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := relationRowsOf(t, pool, maker.ID); held != 0 {
		t.Errorf("%d relation rows survive the move, want the rows of the relation inside the section swept", held)
	}
}

func TestRecountingAPlantedCycleEndsWithEveryFieldAtItsFirstDepth(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	inner := declaredInside(t, store, specs, "inner", content.FieldKindSection)
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content_fields SET parent_field_id = $1 WHERE id = $2`, inner.ID, specs.ID); err != nil {
		t.Fatalf("planting the cycle: %v, want nil", err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	err := db.New(pool).RecountContentFieldDepth(ctx, db.RecountContentFieldDepthParams{
		ToGroup: int32(specs.GroupID), Depth: 0, ID: int32(specs.ID),
	})

	if err != nil {
		t.Fatalf("RecountContentFieldDepth() error = %v, want nil", err)
	}
	for id, want := range map[int]int{specs.ID: 0, inner.ID: 1} {
		if held := rowOf(t, pool, id); held.depth != want {
			t.Errorf("field %d depth = %d, want %d", id, held.depth, want)
		}
	}
}

func TestMovingAFieldReportsALockItCannotTake(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	title := declareTypedField(t, store, "car", "title")
	holdFieldGroupsLock(t, pool)
	timed := lockTimedStore(t, pool)

	_, err := timed.MoveField(t.Context(), title.ID, specs.GroupID, specs.ID, deepEnough, nil)

	if err == nil || !strings.Contains(err.Error(), "lock field groups") {
		t.Errorf("MoveField() error = %v, want the held lock reported", err)
	}
}

func TestMovingAFieldReportsWhatItCannotRead(t *testing.T) {
	t.Parallel()

	for name, broken := range map[string]func(t *testing.T, pool *pgxpool.Pool){
		"the groups it cannot read": func(t *testing.T, pool *pgxpool.Pool) {
			sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")
		},
		"the types it cannot read": func(t *testing.T, pool *pgxpool.Pool) {
			sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN key TO retired")
		},
		"the depth it cannot recount": func(t *testing.T, pool *pgxpool.Pool) {
			plantRaiseFunction(t, pool)
			sabotage(t, pool, "CREATE TRIGGER sabotage BEFORE UPDATE ON core.content_fields "+
				"FOR EACH ROW WHEN (NEW.depth IS DISTINCT FROM OLD.depth) EXECUTE FUNCTION sabotage_raise()")
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, _, pool := typedStore(t)
			storeType(t, store, "car")
			specs := declareSection(t, store, "specs")
			title := declareTypedField(t, store, "car", "title")
			broken(t, pool)

			_, err := store.MoveField(t.Context(), title.ID, specs.GroupID, specs.ID, deepEnough, nil)

			if err == nil {
				t.Errorf("MoveField() error = nil, want %s reported", name)
			}
		})
	}
}

func TestMovingAFieldReportsWhatItCannotWrite(t *testing.T) {
	t.Parallel()

	for name, table := range map[string]struct{ table, operation string }{
		"the row it cannot reparent":       {"core.content_fields", "UPDATE"},
		"the values it cannot sweep":       {"core.content", "UPDATE"},
		"the relation rows it cannot drop": {"core.content_relations", "DELETE"},
		"the revisions it cannot sweep":    {"core.content_revisions", "UPDATE"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store, author, pool := typedStore(t)
			storeType(t, store, "car")
			specs := declareSection(t, store, "specs")
			maker, err := store.CreateField(t.Context(), fieldOn(t, "car", "maker", content.FieldKindRelation, "car"))
			if err != nil {
				t.Fatalf("CreateField(maker) error = %v, want nil", err)
			}
			plantTyped(t, pool, author, "car", "one", `{"maker": []}`)
			raiseOn(t, pool, table.table, table.operation)

			_, err = store.MoveField(t.Context(), maker.ID, specs.GroupID, specs.ID, deepEnough, nil)

			if err == nil {
				t.Errorf("MoveField() error = nil, want %s reported", name)
			}
		})
	}
}
