// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
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

func TestDeletingATopFieldLeavesASubFieldSharingItsKey(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareTypedField(t, store, "car", "specs")
	title := declareTypedField(t, store, "car", "title")
	if _, err := plantSubField(t, pool, specs.GroupID, &specs.ID, "title"); err != nil {
		t.Fatalf("planting title under specs: %v, want nil", err)
	}

	if err := store.DeleteFieldInGroup(t.Context(), title.GroupID, "title"); err != nil {
		t.Fatalf("DeleteFieldInGroup() error = %v, want nil", err)
	}

	if held := fieldRowsHolding(t, pool, specs.GroupID, "title"); held != 1 {
		t.Errorf("rows holding title = %d, want the sub field left standing", held)
	}
}

func TestMovingATopFieldLeavesASubFieldSharingItsKey(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareTypedField(t, store, "car", "specs")
	title := declareTypedField(t, store, "car", "title")
	sub, planted := plantSubField(t, pool, specs.GroupID, &specs.ID, "title")
	if planted != nil {
		t.Fatalf("planting title under specs: %v, want nil", planted)
	}
	landing, err := store.CreateGroup(
		t.Context(), content.Group{Title: "Elsewhere", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	if _, err := store.MoveField(t.Context(), title.GroupID, "title", landing.ID); err != nil {
		t.Fatalf("MoveField() error = %v, want nil", err)
	}

	var parked int
	if err := pool.QueryRow(t.Context(),
		`SELECT group_id FROM core.content_fields WHERE id = $1`, sub).Scan(&parked); err != nil {
		t.Fatalf("reading the sub field: %v, want nil", err)
	}
	if parked != specs.GroupID {
		t.Errorf("the sub field sits in group %d, want it left in %d", parked, specs.GroupID)
	}
}

func TestCreateSubFieldStoresTheKeyTheDomainSettledOn(t *testing.T) {
	t.Parallel()
	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")

	created, err := store.CreateSubField(t.Context(), specs.ID, content.Field{
		Key: "  title  ", Label: "  Title  ", Kind: content.FieldKindText,
	})

	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	if created.Key != "title" || created.Label != "Title" {
		t.Errorf("stored key %q label %q, want them trimmed to title and Title", created.Key, created.Label)
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

func TestCreatingASubFieldStoresItUnderItsParent(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")

	held, err := store.CreateSubField(
		t.Context(), specs.ID, fieldOn(t, "", "title", content.FieldKindText, ""))

	if err != nil {
		t.Fatalf("CreateSubField() error = %v, want nil", err)
	}
	if held.ParentID != specs.ID {
		t.Errorf("ParentID = %d, want %d", held.ParentID, specs.ID)
	}
	if held.GroupID != specs.GroupID {
		t.Errorf("GroupID = %d, want the parent's group %d", held.GroupID, specs.GroupID)
	}
}

func TestCreatingASubFieldOrdersItAfterItsSiblings(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	specs := declareSection(t, store, "specs")
	other := declareSection(t, store, "extras")
	if _, err := store.CreateSubField(
		t.Context(), other.ID, fieldOn(t, "", "away", content.FieldKindText, "")); err != nil {
		t.Fatalf("declaring a sibling elsewhere: %v, want nil", err)
	}

	for _, key := range []string{"title", "colour"} {
		if _, err := store.CreateSubField(
			t.Context(), specs.ID, fieldOn(t, "", key, content.FieldKindText, "")); err != nil {
			t.Fatalf("CreateSubField(%s) error = %v, want nil", key, err)
		}
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	held, found := groupNumbered(groups, specs.GroupID)
	if !found {
		t.Fatalf("the group %d is not served", specs.GroupID)
	}
	inside := subFieldsOf(held, "specs")
	if len(inside) != 2 {
		t.Fatalf("specs holds %d sub fields, want the two declared inside it alone", len(inside))
	}
	if inside[0].Key != "title" || inside[1].Key != "colour" {
		t.Errorf("specs holds %q then %q, want them in the order they were declared",
			inside[0].Key, inside[1].Key)
	}
}

// subFieldsOf returns the sub fields the group's named field holds.
func subFieldsOf(held content.Group, key string) []content.Field {
	for _, f := range held.Fields {
		if f.Key == key {
			return f.Fields
		}
	}
	return nil
}

func TestCreatingASubFieldReportsAParentThatIsGone(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")

	_, err := store.CreateSubField(
		t.Context(), 424242, fieldOn(t, "", "title", content.FieldKindText, ""))

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("CreateSubField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestCreatingASubFieldRefusesAParentHoldingNone(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	plain := declareTypedField(t, store, "car", "subtitle")

	_, err := store.CreateSubField(
		t.Context(), plain.ID, fieldOn(t, "", "title", content.FieldKindText, ""))

	if !errors.Is(err, content.ErrFieldShape) {
		t.Errorf("CreateSubField() error = %v, want %v", err, content.ErrFieldShape)
	}
}

func TestCreatingASubFieldRefusesAParentChainTooDeep(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	at := declareSection(t, store, "specs")
	for depth := 2; depth <= content.MaxFieldDepth; depth++ {
		grown, err := store.CreateSubField(
			t.Context(), at.ID, sectionOn(t, fmt.Sprintf("held%d", depth)))
		if err != nil {
			t.Fatalf("nesting a container to depth %d: %v, want nil", depth, err)
		}
		at = grown
	}

	if _, err := store.CreateSubField(
		t.Context(), at.ID, fieldOn(t, "", "title", content.FieldKindText, "")); err != nil {
		t.Fatalf("a field inside the last container: %v, want the depth taken", err)
	}
	further, err := store.CreateSubField(t.Context(), at.ID, sectionOn(t, "further"))
	if err != nil {
		t.Fatalf("a container at the last depth: %v, want it taken", err)
	}

	_, err = store.CreateSubField(
		t.Context(), further.ID, fieldOn(t, "", "title", content.FieldKindText, ""))

	if !errors.Is(err, content.ErrFieldTooDeep) {
		t.Errorf("CreateSubField() one container too deep error = %v, want %v", err, content.ErrFieldTooDeep)
	}
}

// sectionOn returns a section field ready to nest under a parent.
func sectionOn(t *testing.T, key string) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{Key: key, Label: key, Kind: content.FieldKindSection})
	if err != nil {
		t.Fatalf("NewField(section %s) error = %v, want nil", key, err)
	}
	return built
}

// declareSection stores a section at the top of the car type and returns it.
func declareSection(t *testing.T, store *postgres.TypeStore, key string) content.Field {
	t.Helper()
	held := sectionOn(t, key)
	held.TypeKey = "car"
	stored, err := store.CreateField(t.Context(), held)
	if err != nil {
		t.Fatalf("CreateField(section %s) error = %v, want nil", key, err)
	}
	return stored
}

func TestAGroupServesItsSubFieldsInsideTheirParent(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	specs := declareTypedField(t, store, "car", "specs")
	if _, err := plantSubField(t, pool, specs.GroupID, &specs.ID, "title"); err != nil {
		t.Fatalf("planting title under specs: %v, want nil", err)
	}

	groups, err := store.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}

	held, found := groupNumbered(groups, specs.GroupID)
	if !found {
		t.Fatalf("the group %d is not served", specs.GroupID)
	}
	if len(held.Fields) != 1 {
		t.Fatalf("the group serves %d fields, want the sub field held inside its parent", len(held.Fields))
	}
	if len(held.Fields[0].Fields) != 1 || held.Fields[0].Fields[0].Key != "title" {
		t.Errorf("specs holds %+v, want the title sub field", held.Fields[0].Fields)
	}
}

// groupNumbered returns the served group carrying the identity.
func groupNumbered(groups []content.Group, id int) (content.Group, bool) {
	for _, held := range groups {
		if held.ID == id {
			return held, true
		}
	}
	return content.Group{}, false
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
		t.Context(), top.ID, fieldOn(t, "", "title", content.FieldKindText, ""))

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
		t.Context(), top.ID, fieldOn(t, "", "title", content.FieldKindText, ""))

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
		t.Context(), top.ID, fieldOn(t, "", "title", content.FieldKindText, ""))

	if err == nil {
		t.Error("CreateSubField() error = nil, want the refused write reported")
	}
}
