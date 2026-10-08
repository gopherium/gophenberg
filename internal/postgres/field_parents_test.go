// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// keyCollision reports whether the error is the unique key refused.
func keyCollision(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// plantSubField stores one field row under the given parent and returns its identity.
func plantSubField(t *testing.T, pool *pgxpool.Pool, groupID int, parent *int, key string) (int, error) {
	t.Helper()
	var id int
	err := pool.QueryRow(t.Context(),
		`INSERT INTO core.content_fields (group_id, parent_field_id, key, label, kind, created_at, updated_at)
		VALUES ($1, $2, $3, $3, 'text', now(), now())
		RETURNING id`,
		groupID, parent, key).Scan(&id)
	return id, err
}

// fieldRowsHolding counts the field rows carrying the key inside the group.
func fieldRowsHolding(t *testing.T, pool *pgxpool.Pool, groupID int, key string) int {
	t.Helper()
	var held int
	if err := pool.QueryRow(t.Context(),
		`SELECT count(*) FROM core.content_fields WHERE group_id = $1 AND key = $2`,
		groupID, key).Scan(&held); err != nil {
		t.Fatalf("counting field rows: %v, want nil", err)
	}
	return held
}

func TestFieldKeysRepeatAcrossParentsAndCollideInsideOne(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareTypedField(t, store, "car", "specs")
	extras := declareTypedField(t, store, "car", "extras")

	if _, err := plantSubField(t, pool, specs.GroupID, &specs.ID, "title"); err != nil {
		t.Fatalf("planting title under specs: %v, want nil", err)
	}
	if _, err := plantSubField(t, pool, extras.GroupID, &extras.ID, "title"); err != nil {
		t.Errorf("planting title under extras: %v, want both parents to hold one", err)
	}
	if _, err := plantSubField(t, pool, specs.GroupID, &specs.ID, "title"); !keyCollision(err) {
		t.Errorf("planting title twice under specs: error = %v, want the key collision refused", err)
	}
}

func TestFieldKeysStillCollideAtTheTopOfAGroup(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareTypedField(t, store, "car", "specs")

	if _, err := plantSubField(t, pool, specs.GroupID, nil, "specs"); !keyCollision(err) {
		t.Errorf("planting specs twice at the top: error = %v, want the key collision refused", err)
	}
}

// declaredInside, sectionOn, declareSection, sectionChainOnCar and groupNumbered are shared fixtures these tests use.
var (
	declaredInside    = contenttest.DeclaredInside
	sectionOn         = contenttest.SectionOn
	declareSection    = contenttest.DeclareSection
	sectionChainOnCar = contenttest.SectionChainOnCar
	groupNumbered     = contenttest.GroupOf
)

// groupOfField returns the group the field row sits in.
func groupOfField(t *testing.T, pool *pgxpool.Pool, id int) int {
	t.Helper()
	var held int
	if err := pool.QueryRow(t.Context(),
		`SELECT group_id FROM core.content_fields WHERE id = $1`, id).Scan(&held); err != nil {
		t.Fatalf("reading the group of field %d: %v, want nil", id, err)
	}
	return held
}

func TestGroupsReadASubFieldsGroupFromItsContainer(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	source, landing, section := sectionAcrossGroups(t, store)
	name := declaredInside(t, store, section, "name", content.FieldKindText)
	if _, err := pool.Exec(t.Context(),
		`UPDATE core.content_fields SET group_id = $1 WHERE id = $2`, source.ID, name.ID); err != nil {
		t.Fatalf("leaving the sub field behind, as a move before the repair did: %v, want nil", err)
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}

	for _, g := range groups {
		for _, f := range g.Fields {
			if f.ID != section.ID {
				continue
			}
			if len(f.Fields) != 1 || f.Fields[0].GroupID != landing.ID {
				t.Fatalf("the section holds %+v, want name read in the section's group %d", f.Fields, landing.ID)
			}
			return
		}
	}
	t.Fatal("no group lists the section")
}

func TestMovesStoreEachSubFieldRowInTheGroupOfItsContainer(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	title := declareTypedField(t, store, "car", "title")
	sub := declaredInside(t, store, specs, "title", content.FieldKindText)
	source := groupOn(t, store, "Details", "car")
	section, err := store.CreateFieldInGroup(
		t.Context(), source.ID, fieldOn(t, "", "author", content.FieldKindSection, ""), nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(author) error = %v, want nil", err)
	}
	name := declaredInside(t, store, section, "name", content.FieldKindText)
	rows := declaredInside(t, store, section, "rows", content.FieldKindRepeater)
	deep := declaredInside(t, store, rows, "title", content.FieldKindText)
	landing := groupOn(t, store, "Elsewhere", "car")

	if _, err := store.MoveField(t.Context(), title.ID, landing.ID, 0, deepEnough, nil); err != nil {
		t.Fatalf("MoveField(title) error = %v, want nil", err)
	}
	if _, err := store.MoveField(t.Context(), section.ID, landing.ID, 0, deepEnough, nil); err != nil {
		t.Fatalf("MoveField(author) error = %v, want nil", err)
	}

	if parked := groupOfField(t, pool, sub.ID); parked != specs.GroupID {
		t.Errorf("the sub field sits in group %d, want it left in %d", parked, specs.GroupID)
	}
	if carried := groupOfField(t, pool, name.ID); carried != landing.ID {
		t.Errorf("the sub field sits in group %d, want it carried into %d with its section",
			carried, landing.ID)
	}
	if held := groupOfField(t, pool, deep.ID); held != landing.ID {
		t.Errorf("the field two levels down sits in group %d, want it carried into %d", held, landing.ID)
	}
}

// positionOf returns the stored position of the field the identity names.
func positionOf(t *testing.T, pool *pgxpool.Pool, id int) int {
	t.Helper()
	var held int
	if err := pool.QueryRow(t.Context(),
		`SELECT position FROM core.content_fields WHERE id = $1`, id).Scan(&held); err != nil {
		t.Fatalf("reading the position: %v, want nil", err)
	}
	return held
}

func TestCreatingASubFieldPastTheDefaultStoresItsDepth(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	registry := content.NewRegistry(store).WithFieldDepth(content.DefaultFieldDepth + 1)
	deepest := sectionChainOnCar(t, store, registry, content.DefaultFieldDepth)

	title, err := registry.CreateSubField(t.Context(), deepest.ID, fieldOn(t, "", "title", content.FieldKindText, ""))

	if err != nil {
		t.Fatalf("CreateSubField() within a raised limit error = %v, want nil", err)
	}
	var depth int
	if err := pool.QueryRow(t.Context(),
		`SELECT depth FROM core.content_fields WHERE id = $1`, title.ID).Scan(&depth); err != nil {
		t.Fatalf("reading the stored depth: %v, want nil", err)
	}
	if depth != content.DefaultFieldDepth+1 {
		t.Errorf("stored depth = %d, want %d", depth, content.DefaultFieldDepth+1)
	}
}

func TestDeletingAParentFieldRowTakesEveryDescendant(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareTypedField(t, store, "car", "specs")

	rows, err := plantSubField(t, pool, specs.GroupID, &specs.ID, "rows")
	if err != nil {
		t.Fatalf("planting rows under specs: %v, want nil", err)
	}
	if _, err := plantSubField(t, pool, specs.GroupID, &rows, "title"); err != nil {
		t.Fatalf("planting title under rows: %v, want nil", err)
	}

	if _, err := pool.Exec(t.Context(),
		`DELETE FROM core.content_fields WHERE id = $1`, specs.ID); err != nil {
		t.Fatalf("deleting the parent row: %v, want nil", err)
	}

	if held := fieldRowsHolding(t, pool, specs.GroupID, "title"); held != 0 {
		t.Errorf("rows holding title = %d, want the grandchild gone with its line", held)
	}
}

func TestCreatingASubFieldReportsALockItCannotTake(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	top := declareSection(t, store, "specs")
	holdFieldGroupsLock(t, pool)
	timed := lockTimedStore(t, pool)

	_, err := timed.CreateSubField(
		t.Context(), top.ID, fieldOn(t, "", "title", content.FieldKindText, ""), deepEnough)

	if err == nil {
		t.Error("CreateSubField() error = nil, want the lock it could not take reported")
	}
}

func TestCreatingASubFieldReportsAParentItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	top := declareSection(t, store, "specs")
	sabotage(t, pool, "ALTER TABLE core.content_fields RENAME COLUMN label TO retired")

	_, err := store.CreateSubField(
		t.Context(), top.ID, fieldOn(t, "", "title", content.FieldKindText, ""), deepEnough)

	if err == nil {
		t.Error("CreateSubField() error = nil, want the unreadable parent reported")
	}
}

func TestCreatingASubFieldReportsAWriteItCannotMake(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	top := declareSection(t, store, "specs")
	raiseOn(t, pool, "core.content_fields", "INSERT")

	_, err := store.CreateSubField(
		t.Context(), top.ID, fieldOn(t, "", "title", content.FieldKindText, ""), deepEnough)

	if err == nil {
		t.Error("CreateSubField() error = nil, want the refused write reported")
	}
}
