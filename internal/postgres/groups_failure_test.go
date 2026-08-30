// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// lockTimedStore returns a store whose connections give up quickly on a held lock.
func lockTimedStore(t *testing.T, pool *pgxpool.Pool) *postgres.TypeStore {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(pool.Config().ConnString())
	if err != nil {
		t.Fatalf("parsing the pool address: %v", err)
	}
	cfg.ConnConfig.RuntimeParams["lock_timeout"] = "200ms"
	timed, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatalf("opening the timed pool: %v", err)
	}
	t.Cleanup(timed.Close)
	return postgres.NewTypeStore(timed)
}

// holdFieldGroupsLock holds the field groups lock until the test ends.
func holdFieldGroupsLock(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	conn, err := pool.Acquire(t.Context())
	if err != nil {
		t.Fatalf("acquiring a connection: %v", err)
	}
	if _, err := conn.Exec(t.Context(),
		"SELECT pg_advisory_lock(hashtext('core.field_groups'))"); err != nil {
		conn.Release()
		t.Fatalf("holding the lock: %v", err)
	}
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(),
			"SELECT pg_advisory_unlock(hashtext('core.field_groups'))"); err != nil {
			t.Errorf("releasing the lock: %v", err)
		}
		conn.Release()
	})
}

func TestListGroupsReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	if _, err := store.ListGroups(t.Context()); err == nil {
		t.Error("ListGroups() error = nil, want the unreadable groups reported")
	}
}

func TestListGroupsReportsFieldsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	sabotage(t, pool, "ALTER TABLE core.content_fields RENAME COLUMN label TO retired")

	if _, err := store.ListGroups(t.Context()); err == nil {
		t.Error("ListGroups() error = nil, want the unreadable fields reported")
	}
}

func TestListGroupsReportsALocationItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	sabotage(t, pool,
		"INSERT INTO core.field_groups (title, location, created_at, updated_at) "+
			"VALUES ('Corrupt', '[1, 2]', now(), now())")

	_, err := store.ListGroups(t.Context())

	if err == nil {
		t.Fatal("ListGroups() error = nil, want the unreadable location reported")
	}
}

func TestCreateGroupReportsAGroupItCannotStore(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	raiseOn(t, pool, "core.field_groups", "INSERT")

	_, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})

	if err == nil {
		t.Error("CreateGroup() error = nil, want the refused write reported")
	}
}

func TestUpdateGroupReportsAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)

	_, err := store.UpdateGroup(t.Context(), content.Group{ID: 4242, Title: "Vanished"})

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("UpdateGroup() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}

func TestUpdateGroupReportsAGroupItCannotStore(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	stored, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	raiseOn(t, pool, "core.field_groups", "UPDATE")

	stored.Title = "Renamed"
	if _, err := store.UpdateGroup(t.Context(), stored); err == nil {
		t.Error("UpdateGroup() error = nil, want the refused write reported")
	}
}

func TestDeleteGroupReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	if err := store.DeleteGroup(t.Context(), 1); err == nil {
		t.Error("DeleteGroup() error = nil, want the unreadable groups reported")
	}
}

func TestDeleteGroupReportsTypesItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	stored, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN key TO retired")

	if err := store.DeleteGroup(t.Context(), stored.ID); err == nil {
		t.Error("DeleteGroup() error = nil, want the unreadable types reported")
	}
}

func TestDeleteGroupReportsAGroupItCannotRemove(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	stored, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	raiseOn(t, pool, "core.field_groups", "DELETE")

	if err := store.DeleteGroup(t.Context(), stored.ID); err == nil {
		t.Error("DeleteGroup() error = nil, want the refused removal reported")
	}
}

func TestDeleteGroupReportsFieldValuesItCannotSweep(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")
	groups, err := store.ListGroups(t.Context())
	if err != nil || len(groups) != 1 {
		t.Fatalf("ListGroups() = %v, %v, want the one raised group", groups, err)
	}
	raiseOn(t, pool, "core.content_fields", "DELETE")

	if err := store.DeleteGroup(t.Context(), groups[0].ID); err == nil {
		t.Error("DeleteGroup() error = nil, want the refused field removal reported")
	}
}

func TestReorderGroupsReportsAnOrderItCannotStore(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	raiseOn(t, pool, "core.field_groups", "UPDATE")

	if err := store.ReorderGroups(t.Context(), []int{1, 2}); err == nil {
		t.Error("ReorderGroups() error = nil, want the refused write reported")
	}
}

func TestCreateFieldInGroupReportsAGroupThatIsGone(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)

	_, err := store.CreateFieldInGroup(
		t.Context(), 4242, fieldOn(t, "", "orphan", content.FieldKindText, ""),
	)

	if !errors.Is(err, content.ErrGroupNotFound) {
		t.Errorf("CreateFieldInGroup() error = %v, want %v", err, content.ErrGroupNotFound)
	}
}

func TestCreateFieldReportsATypeThatIsNotRegistered(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)

	_, err := store.CreateField(t.Context(), fieldOn(t, "vanished", "subtitle", content.FieldKindText, ""))

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("CreateField() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestCreateFieldJoinsAGroupMatchingWithoutNamingTheTypeLiterally(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	shared, err := store.CreateGroup(t.Context(), content.Group{
		Title:    "Everywhere",
		Location: content.Rules{{{Source: "content_type", Operator: content.OperatorIs, Value: content.AnyContentType}}},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	resting, err := store.CreateGroup(t.Context(), content.Group{Title: "Resting", Location: locationOf("book")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}

	declared := declareTypedField(t, store, "car", "subtitle")

	if declared.GroupID != shared.ID {
		t.Errorf("GroupID = %d, want the matching group %d rather than %d", declared.GroupID, shared.ID, resting.ID)
	}
}

func TestCreateFieldReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	_, err := store.CreateField(t.Context(), fieldOn(t, "car", "subtitle", content.FieldKindText, ""))

	if err == nil {
		t.Error("CreateField() error = nil, want the unreadable groups reported")
	}
}

func TestCreateFieldReportsFieldsItCannotReadWhileSeekingAGroup(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	sabotage(t, pool, "ALTER TABLE core.content_fields RENAME COLUMN label TO retired")

	_, err := store.CreateField(t.Context(), fieldOn(t, "car", "subtitle", content.FieldKindText, ""))

	if err == nil {
		t.Error("CreateField() error = nil, want the unreadable fields reported")
	}
}

func TestCreateFieldReportsATypeItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN singular_label TO retired")

	_, err := store.CreateField(t.Context(), fieldOn(t, "car", "subtitle", content.FieldKindText, ""))

	if err == nil {
		t.Error("CreateField() error = nil, want the unreadable type reported")
	}
}

func TestCreateFieldReportsAGroupItCannotRaise(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	raiseOn(t, pool, "core.field_groups", "INSERT")

	_, err := store.CreateField(t.Context(), fieldOn(t, "car", "subtitle", content.FieldKindText, ""))

	if err == nil {
		t.Error("CreateField() error = nil, want the refused group raise reported")
	}
}

func TestContentWriteReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Hello world", author)
	created.UpdatedAt = time.Now().UTC()
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	_, err := store.Update(t.Context(), created, created.CreatedAt, nil, 0)

	if err == nil {
		t.Error("Update() error = nil, want the unreadable groups reported")
	}
}

func TestContentWriteReportsALocationItCannotRead(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Hello world", author)
	created.UpdatedAt = time.Now().UTC()
	sabotage(t, pool,
		"INSERT INTO core.field_groups (title, location, created_at, updated_at) "+
			"VALUES ('Corrupt', '[1, 2]', now(), now())")

	_, err := store.Update(t.Context(), created, created.CreatedAt, nil, 0)

	if err == nil {
		t.Error("Update() error = nil, want the unreadable location reported")
	}
}

func TestUpdateFieldInGroupReportsALabelItCannotStore(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declared := declareTypedField(t, store, "car", "subtitle")
	raiseOn(t, pool, "core.content_fields", "UPDATE")

	_, err := store.UpdateFieldInGroup(t.Context(), declared.GroupID, content.Field{
		Key: "subtitle", Label: "Renamed",
	}, declared.UpdatedAt)

	if err == nil {
		t.Error("UpdateFieldInGroup() error = nil, want the refused write reported")
	}
}

func TestUpdateFieldInGroupReportsAGroupListingItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declared := declareTypedField(t, store, "car", "subtitle")
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	_, err := store.UpdateFieldInGroup(t.Context(), declared.GroupID, content.Field{
		Key: "subtitle", Label: "Renamed", UpdatedAt: declared.UpdatedAt,
	}, declared.UpdatedAt.Add(-time.Hour))

	if err == nil {
		t.Error("UpdateFieldInGroup() error = nil, want the unreadable listing reported")
	}
}

func TestDeleteFieldInGroupReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	if err := store.DeleteFieldInGroup(t.Context(), 1, "subtitle"); err == nil {
		t.Error("DeleteFieldInGroup() error = nil, want the unreadable groups reported")
	}
}

func TestDeleteFieldInGroupReportsTypesItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declared := declareTypedField(t, store, "car", "subtitle")
	sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN key TO retired")

	if err := store.DeleteFieldInGroup(t.Context(), declared.GroupID, "subtitle"); err == nil {
		t.Error("DeleteFieldInGroup() error = nil, want the unreadable types reported")
	}
}

func TestDeleteFieldInGroupReportsValuesItCannotSweep(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	declared := declareTypedField(t, store, "car", "subtitle")
	plantTyped(t, pool, author, "car", "one-car", `{"subtitle": "car words"}`)
	raiseOn(t, pool, "core.content_fields", "DELETE")

	if err := store.DeleteFieldInGroup(t.Context(), declared.GroupID, "subtitle"); err == nil {
		t.Error("DeleteFieldInGroup() error = nil, want the refused removal reported")
	}
}

func TestReorderFieldsInGroupReportsAnOrderItCannotStore(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declared := declareTypedField(t, store, "car", "subtitle")
	raiseOn(t, pool, "core.content_fields", "UPDATE")

	if err := store.ReorderFieldsInGroup(t.Context(), declared.GroupID, []string{"subtitle"}); err == nil {
		t.Error("ReorderFieldsInGroup() error = nil, want the refused write reported")
	}
}

func TestByKeyReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	if _, err := store.ByKey(t.Context(), "car"); err == nil {
		t.Error("ByKey() error = nil, want the unreadable groups reported")
	}
}

func TestListReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	if _, err := store.List(t.Context()); err == nil {
		t.Error("List() error = nil, want the unreadable groups reported")
	}
}

func TestUpdateGroupReportsGroupsItCannotList(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	stored, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	stored.Title = "Renamed"
	if _, err := store.UpdateGroup(t.Context(), stored); err == nil {
		t.Error("UpdateGroup() error = nil, want the unreadable groups reported")
	}
}

func TestUpdateGroupReportsTypesItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	stored, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN key TO retired")

	stored.Title = "Renamed"
	if _, err := store.UpdateGroup(t.Context(), stored); err == nil {
		t.Error("UpdateGroup() error = nil, want the unreadable types reported")
	}
}

func TestCreateFieldInGroupReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	stored, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	_, err = store.CreateFieldInGroup(t.Context(), stored.ID, fieldOn(t, "", "subtitle", content.FieldKindText, ""))

	if err == nil {
		t.Error("CreateFieldInGroup() error = nil, want the unreadable groups reported")
	}
}

func TestCreateFieldInGroupReportsTypesItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	stored, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN key TO retired")

	_, err = store.CreateFieldInGroup(t.Context(), stored.ID, fieldOn(t, "", "subtitle", content.FieldKindText, ""))

	if err == nil {
		t.Error("CreateFieldInGroup() error = nil, want the unreadable types reported")
	}
}

func TestDeleteFieldInGroupReportsContentItCannotSweep(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declared := declareTypedField(t, store, "car", "subtitle")
	raiseOn(t, pool, "core.content", "UPDATE")

	if err := store.DeleteFieldInGroup(t.Context(), declared.GroupID, "subtitle"); err == nil {
		t.Error("DeleteFieldInGroup() error = nil, want the refused sweep reported")
	}
}

func TestUpdateGroupReportsALockItCannotTake(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	stored, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	holdFieldGroupsLock(t, pool)
	timed := lockTimedStore(t, pool)

	stored.Title = "Renamed"
	if _, err := timed.UpdateGroup(t.Context(), stored); err == nil {
		t.Error("UpdateGroup() error = nil, want the held lock reported")
	}
}

func TestCreateFieldInGroupReportsALockItCannotTake(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	stored, err := store.CreateGroup(t.Context(), content.Group{Title: "Extras", Location: locationOf("car")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	holdFieldGroupsLock(t, pool)
	timed := lockTimedStore(t, pool)

	_, err = timed.CreateFieldInGroup(t.Context(), stored.ID, fieldOn(t, "", "subtitle", content.FieldKindText, ""))

	if err == nil {
		t.Error("CreateFieldInGroup() error = nil, want the held lock reported")
	}
}

func TestMoveFieldReportsAFieldItCannotCarry(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	declared := declareTypedField(t, store, "car", "subtitle")
	resting, err := store.CreateGroup(t.Context(), content.Group{Title: "Resting", Location: locationOf("book")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	raiseOn(t, pool, "core.content_fields", "UPDATE")

	_, err = store.MoveField(t.Context(), declared.GroupID, "subtitle", resting.ID)

	if err == nil {
		t.Error("MoveField() error = nil, want the refused carry reported")
	}
}
