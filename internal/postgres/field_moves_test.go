// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
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

// plantAutosave stores an autosave of the planted car row carrying the raw values.
func plantAutosave(t *testing.T, pool *pgxpool.Pool, author uuid.UUID, slug, values string) {
	t.Helper()
	if _, err := pool.Exec(t.Context(),
		`INSERT INTO core.content_revisions
		(id, content_id, kind, author_id, title, content, excerpt, created_at, fields)
		VALUES ($1, $2, 'autosave', $3, 'Planted', '', '', now(), $4)`,
		uuid.Must(uuid.NewV7()), plantedID(t, pool, slug), author, values); err != nil {
		t.Fatalf("planting the autosave: %v, want nil", err)
	}
}

// relationInside declares a relation pointing at cars inside the container and returns it.
func relationInside(t *testing.T, store *postgres.TypeStore, parent content.Field, key string) content.Field {
	t.Helper()
	built, err := content.NewSubField(content.Field{
		Key: key, Label: key, Kind: content.FieldKindRelation, RelatesTo: "car",
	}, parent.Kind)
	if err != nil {
		t.Fatalf("NewSubField(%s) error = %v, want nil", key, err)
	}
	stored, err := store.CreateSubField(t.Context(), parent.ID, built)
	if err != nil {
		t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
	}
	return stored
}

// revisionsHolding counts the revisions and autosaves of the car rows still carrying the key.
func revisionsHolding(t *testing.T, pool *pgxpool.Pool, key string) int {
	t.Helper()
	var held int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM core.content_revisions r JOIN core.content c ON r.content_id = c.id
		WHERE c.type = 'car' AND r.fields ? $1`, key).Scan(&held); err != nil {
		t.Fatalf("counting the revisions holding %q: %v, want nil", key, err)
	}
	return held
}

// rested turns off the group the title names and returns it.
func rested(t *testing.T, store *postgres.TypeStore, title string) content.Group {
	t.Helper()
	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	for _, held := range groups {
		if held.Title != title {
			continue
		}
		held.Active = false
		idle, err := store.UpdateGroup(t.Context(), held)
		if err != nil {
			t.Fatalf("resting %q: %v, want nil", title, err)
		}
		return idle
	}
	t.Fatalf("no stored group is titled %q", title)
	return content.Group{}
}

// restingTwinOf stores a resting group on car holding a section under the key and returns the section.
func restingTwinOf(t *testing.T, store *postgres.TypeStore, key string) content.Field {
	t.Helper()
	if _, err := store.CreateGroup(t.Context(), content.Group{Title: "Shadow", Location: locationOf("car")}); err != nil {
		t.Fatalf("CreateGroup(Shadow) error = %v, want nil", err)
	}
	idle := rested(t, store, "Shadow")
	twin, err := store.CreateFieldInGroup(t.Context(), idle.ID, sectionOn(t, key))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(shadow %s) error = %v, want nil", key, err)
	}
	return twin
}

// rivalOnTruck stores a truck group holding a section under the key, registers truck and returns the group.
func rivalOnTruck(t *testing.T, store *postgres.TypeStore, key string) content.Group {
	t.Helper()
	trucks, err := store.CreateGroup(t.Context(), content.Group{Title: "Trucks", Location: locationOf("truck")})
	if err != nil {
		t.Fatalf("CreateGroup(Trucks) error = %v, want nil", err)
	}
	if _, err := store.CreateFieldInGroup(t.Context(), trucks.ID, sectionOn(t, key)); err != nil {
		t.Fatalf("CreateFieldInGroup(trucks %s) error = %v, want nil", key, err)
	}
	storeType(t, store, "truck")
	return trucks
}

// servingEverything stores a group matching every type and returns it.
func servingEverything(t *testing.T, store *postgres.TypeStore) content.Group {
	t.Helper()
	everywhere, err := store.CreateGroup(t.Context(),
		content.Group{Title: "Everywhere", Location: locationOf(content.AnyContentType)})
	if err != nil {
		t.Fatalf("CreateGroup(Everywhere) error = %v, want nil", err)
	}
	return everywhere
}

// valuesSlugged returns the stored values of the item carrying the slug.
func valuesSlugged(t *testing.T, pool *pgxpool.Pool, slug string) string {
	t.Helper()
	var held string
	if err := pool.QueryRow(t.Context(),
		`SELECT fields::text FROM core.content WHERE slug = $1`, slug).Scan(&held); err != nil {
		t.Fatalf("reading the values of %s: %v, want nil", slug, err)
	}
	return held
}

func TestMovingAFieldIntoAContainerReparentsItAndRecountsItsDepth(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	title := declareTypedField(t, store, "car", "title")

	moved, err := store.MoveField(t.Context(), title.ID, specs.GroupID, specs.ID)

	if err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}
	if moved.ID != title.ID || moved.ParentID != specs.ID || moved.GroupID != specs.GroupID {
		t.Errorf("MoveField() = %+v, want the same row standing under the section", moved)
	}
	standsUnder(t, pool, title.ID, specs.GroupID, specs.ID, 1)
}

func TestMovingAContainerRecountsTheDepthOfEverythingBelowIt(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	inner := declaredInside(t, store, specs, "inner", content.FieldKindSection)
	leaf := declaredInside(t, store, inner, "leaf", content.FieldKindText)
	box := declareSection(t, store, "box")

	if _, err := store.MoveField(t.Context(), specs.ID, box.GroupID, box.ID); err != nil {
		t.Fatalf("MoveField(into box) error = %v, want nil", err)
	}
	standsUnder(t, pool, specs.ID, box.GroupID, box.ID, 1)
	standsUnder(t, pool, inner.ID, box.GroupID, specs.ID, 2)
	standsUnder(t, pool, leaf.ID, box.GroupID, inner.ID, 3)

	if _, err := store.MoveField(t.Context(), specs.ID, box.GroupID, 0); err != nil {
		t.Fatalf("MoveField(to the top) error = %v, want nil", err)
	}
	standsUnder(t, pool, specs.ID, box.GroupID, 0, 0)
	standsUnder(t, pool, inner.ID, box.GroupID, specs.ID, 1)
	standsUnder(t, pool, leaf.ID, box.GroupID, inner.ID, 2)
}

func TestMovingAFieldOutOfAContainerStandsItAfterTheTopFields(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	doors := declaredInside(t, store, specs, "doors", content.FieldKindText)

	moved, err := store.MoveField(t.Context(), doors.ID, specs.GroupID, 0)

	if err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}
	if moved.ParentID != 0 {
		t.Errorf("MoveField() = %+v, want the field standing at the top", moved)
	}
	if held, after := positionOf(t, pool, doors.ID), positionOf(t, pool, specs.ID); held != after+1 {
		t.Errorf("the moved field stands at position %d, want it right after the section at %d", held, after)
	}
	standsUnder(t, pool, doors.ID, specs.GroupID, 0, 0)
}

func TestMovingAFieldIntoAContainerSweepsTheValuesItHeld(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	subtitle := declareTypedField(t, store, "car", "subtitle")
	plantTyped(t, pool, author, "car", "one", `{"subtitle": "kept words"}`)
	plantAutosave(t, pool, author, "one", `{"subtitle": "typed words"}`)

	if _, err := store.MoveField(t.Context(), subtitle.ID, specs.GroupID, specs.ID); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := valuesHeld(t, pool); held != `{}` {
		t.Errorf("stored values = %s, want the subtitle swept from the item", held)
	}
	if held := revisionsHolding(t, pool, "subtitle"); held != 0 {
		t.Errorf("%d revisions still hold the subtitle, want it swept from revisions and autosaves", held)
	}
}

func TestMovingAFieldOutOfAContainerSweepsItFromInsideIt(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	declaredInside(t, store, specs, "colour", content.FieldKindText)
	doors := declaredInside(t, store, specs, "doors", content.FieldKindText)
	plantTyped(t, pool, author, "car", "one", `{"specs": {"colour": "red", "doors": "five"}}`)

	if _, err := store.MoveField(t.Context(), doors.ID, specs.GroupID, 0); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := valuesHeld(t, pool); held != `{"specs": {"colour": "red"}}` {
		t.Errorf("stored values = %s, want the doors swept from inside the section", held)
	}
	if held := revisionValuesHeld(t, pool); held != `{"specs": {"colour": "red"}}` {
		t.Errorf("revision values = %s, want the doors swept there too", held)
	}
}

func TestMovingAFieldBetweenGroupTopsKeepsItsValues(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	subtitle := declareTypedField(t, store, "car", "subtitle")
	plantTyped(t, pool, author, "car", "one", `{"subtitle": "kept words"}`)
	extras, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	if _, err := store.MoveField(t.Context(), subtitle.ID, extras.ID, 0); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := valuesHeld(t, pool); held != `{"subtitle": "kept words"}` {
		t.Errorf("stored values = %s, want the value kept while the path stays the same", held)
	}
}

func TestMovingARelationSweepsItsIndexRows(t *testing.T) {
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

	if _, err := store.MoveField(t.Context(), maker.ID, specs.GroupID, specs.ID); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := relationRowsOf(t, pool, maker.ID); held != 0 {
		t.Errorf("%d relation rows survive the move, want the index swept with the values", held)
	}
}

func TestMovingASectionSweepsTheRelationsInsideIt(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	maker := relationInside(t, store, specs, "maker")
	box := declareSection(t, store, "box")
	plantTyped(t, pool, author, "car", "one", `{"specs": {"maker": []}}`)
	plantTyped(t, pool, author, "car", "two", `{}`)
	plantRelation(t, pool, plantedID(t, pool, "one"), plantedID(t, pool, "two"), maker.ID)

	if _, err := store.MoveField(t.Context(), specs.ID, box.GroupID, box.ID); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := relationRowsOf(t, pool, maker.ID); held != 0 {
		t.Errorf("%d relation rows survive the move, want the rows of the relation inside the section swept", held)
	}
}

func TestMovingAShadowedFieldKeepsTheValuesTheServedFieldHolds(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "title")
	shadow, err := store.CreateGroup(t.Context(), content.Group{Title: "Shadow", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Shadow) error = %v, want nil", err)
	}
	idle := rested(t, store, "Shadow")
	specs, err := store.CreateFieldInGroup(t.Context(), idle.ID, sectionOn(t, "specs"))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	shadowed, err := store.CreateFieldInGroup(
		t.Context(), shadow.ID, fieldOn(t, "", "title", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(shadow title) error = %v, want nil", err)
	}
	plantTyped(t, pool, author, "car", "one", `{"title": "served words"}`)
	plantAutosave(t, pool, author, "one", `{"title": "typed words"}`)

	if _, err := store.MoveField(t.Context(), shadowed.ID, idle.ID, specs.ID); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := valuesHeld(t, pool); held != `{"title": "served words"}` {
		t.Errorf("stored values = %s, want the served field's value kept", held)
	}
	if held := revisionsHolding(t, pool, "title"); held != 2 {
		t.Errorf("%d revisions hold the title, want the revision and the autosave both kept", held)
	}
}

func TestMovingAFieldOfARestingGroupSweepsTheValuesNoGroupServes(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	resting, err := store.CreateGroup(t.Context(), content.Group{Title: "Resting", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup(Resting) error = %v, want nil", err)
	}
	specs, err := store.CreateFieldInGroup(t.Context(), resting.ID, sectionOn(t, "specs"))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	title, err := store.CreateFieldInGroup(
		t.Context(), resting.ID, fieldOn(t, "", "title", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(title) error = %v, want nil", err)
	}
	rested(t, store, "Resting")
	plantTyped(t, pool, author, "car", "one", `{"title": "old words"}`)

	if _, err := store.MoveField(t.Context(), title.ID, resting.ID, specs.ID); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := valuesHeld(t, pool); held != `{}` {
		t.Errorf("stored values = %s, want the title swept since no group serves it", held)
	}
}

func TestMovingARestingGroupsSubFieldOutSweepsTheValueTheServedSectionLacks(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	declaredInside(t, store, specs, "size", content.FieldKindText)
	twin := restingTwinOf(t, store, "specs")
	color := declaredInside(t, store, twin, "color", content.FieldKindText)
	plantTyped(t, pool, author, "car", "one", `{"specs": {"color": "red", "size": "big"}}`)

	if _, err := store.MoveField(t.Context(), color.ID, twin.GroupID, 0); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := valuesHeld(t, pool); held != `{"specs": {"size": "big"}}` {
		t.Errorf("stored values = %s, want the color swept since the served section lacks it", held)
	}
}

func TestMovingAFieldOutOfAContainerItsGroupServesSweepsItWhereARivalHoldsTheContainer(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	everywhere := servingEverything(t, store)
	specs, err := store.CreateFieldInGroup(t.Context(), everywhere.ID, sectionOn(t, "specs"))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	color := declaredInside(t, store, specs, "color", content.FieldKindText)
	rivalOnTruck(t, store, "specs")
	plantTyped(t, pool, author, "truck", "one", `{"specs": {"color": "red"}}`)

	if _, err := store.MoveField(t.Context(), color.ID, everywhere.ID, 0); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := valuesSlugged(t, pool, "one"); held != `{"specs": {}}` {
		t.Errorf("stored values = %s, want the color swept from the specs the moving group serves", held)
	}
}

func TestMovingAFieldItsGroupServesIntoAContainerSweepsItWhereARivalHoldsTheKey(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	everywhere := servingEverything(t, store)
	specs, err := store.CreateFieldInGroup(t.Context(), everywhere.ID, sectionOn(t, "specs"))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(specs) error = %v, want nil", err)
	}
	title, err := store.CreateFieldInGroup(
		t.Context(), everywhere.ID, fieldOn(t, "", "title", content.FieldKindText, ""))
	if err != nil {
		t.Fatalf("CreateFieldInGroup(title) error = %v, want nil", err)
	}
	rivalOnTruck(t, store, "title")
	plantTyped(t, pool, author, "truck", "one", `{"title": "served words"}`)

	if _, err := store.MoveField(t.Context(), title.ID, everywhere.ID, specs.ID); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	if held := valuesSlugged(t, pool, "one"); held != `{}` {
		t.Errorf("stored values = %s, want the title swept, the rival section never taking it over", held)
	}
}

func TestMovingAFieldOntoAKeyTheContainerHoldsReportsFieldTaken(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	declaredInside(t, store, specs, "title", content.FieldKindText)
	title := declareTypedField(t, store, "car", "title")

	_, err := store.MoveField(t.Context(), title.ID, specs.GroupID, specs.ID)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

func TestMovingAFieldOntoAKeyTheTopHoldsReportsFieldTaken(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	inside := declaredInside(t, store, specs, "title", content.FieldKindText)
	declareTypedField(t, store, "car", "title")

	_, err := store.MoveField(t.Context(), inside.ID, specs.GroupID, 0)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

func TestMovingAFieldOutToAKeyARivalGroupServesReportsFieldTaken(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	inside := declaredInside(t, store, specs, "title", content.FieldKindText)
	extras, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := store.CreateFieldInGroup(
		t.Context(), extras.ID, fieldOn(t, "", "title", content.FieldKindText, "")); err != nil {
		t.Fatalf("CreateFieldInGroup(title) error = %v, want nil", err)
	}

	_, err = store.MoveField(t.Context(), inside.ID, specs.GroupID, 0)

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

func TestMovingAContainerIntoItsOwnTreeIsRefusedByTheStore(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	inner := declaredInside(t, store, specs, "inner", content.FieldKindSection)

	for name, parent := range map[string]int{"itself": specs.ID, "a container inside it": inner.ID} {
		_, err := store.MoveField(t.Context(), specs.ID, specs.GroupID, parent)

		if !errors.Is(err, content.ErrFieldInsideItself) {
			t.Errorf("moving into %s: error = %v, want %v", name, err, content.ErrFieldInsideItself)
		}
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

func TestMovingAFieldReportsAParentTheLandingGroupDoesNotHold(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	title := declareTypedField(t, store, "car", "title")
	extras, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	_, err = store.MoveField(t.Context(), title.ID, extras.ID, specs.ID)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldNotFound)
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

	_, err := timed.MoveField(t.Context(), title.ID, specs.GroupID, specs.ID)

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

			_, err := store.MoveField(t.Context(), title.ID, specs.GroupID, specs.ID)

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

			_, err = store.MoveField(t.Context(), maker.ID, specs.GroupID, specs.ID)

			if err == nil {
				t.Errorf("MoveField() error = nil, want %s reported", name)
			}
		})
	}
}
