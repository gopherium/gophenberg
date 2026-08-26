// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

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

func TestUpdateFieldReadsPastAGroupThatDoesNotMatch(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")
	storeType(t, store, "book")
	elsewhere, err := store.CreateGroup(t.Context(), content.Group{Title: "Elsewhere", Location: locationOf("book")})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	if _, err := store.CreateFieldInGroup(
		t.Context(), elsewhere.ID, fieldOn(t, "", "subtitle", content.FieldKindText, ""),
	); err != nil {
		t.Fatalf("CreateFieldInGroup() error = %v, want nil", err)
	}
	declareTypedField(t, store, "car", "subtitle")

	updated, err := store.UpdateField(t.Context(), content.Field{
		TypeKey: "car", Key: "subtitle", Label: "Renamed", Required: true,
	})

	if err != nil {
		t.Fatalf("UpdateField() error = %v, want the car's own field found", err)
	}
	if updated.GroupID == elsewhere.ID {
		t.Errorf("GroupID = %d, want the field of the matching group rather than %d", updated.GroupID, elsewhere.ID)
	}
}

func TestUpdateFieldReportsAFieldNoMatchingGroupDeclares(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")

	_, err := store.UpdateField(t.Context(), content.Field{TypeKey: "car", Key: "absent", Label: "Absent"})

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("UpdateField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestUpdateFieldReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	_, err := store.UpdateField(t.Context(), content.Field{TypeKey: "car", Key: "subtitle", Label: "Renamed"})

	if err == nil {
		t.Error("UpdateField() error = nil, want the unreadable groups reported")
	}
}

func TestUpdateFieldReportsALabelItCannotStore(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")
	raiseOn(t, pool, "core.content_fields", "UPDATE")

	_, err := store.UpdateField(t.Context(), content.Field{
		TypeKey: "car", Key: "subtitle", Label: "Renamed",
	})

	if err == nil {
		t.Error("UpdateField() error = nil, want the refused write reported")
	}
}

func TestReorderFieldsLeavesATypeWithNoGroupAlone(t *testing.T) {
	t.Parallel()

	store, _, _ := typedStore(t)
	storeType(t, store, "car")

	if err := store.ReorderFields(t.Context(), "car", nil); err != nil {
		t.Errorf("ReorderFields() error = %v, want a type holding no group left alone", err)
	}
}

func TestReorderFieldsReportsAnOrderItCannotStore(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")
	raiseOn(t, pool, "core.content_fields", "UPDATE")

	if err := store.ReorderFields(t.Context(), "car", []string{"subtitle"}); err == nil {
		t.Error("ReorderFields() error = nil, want the refused write reported")
	}
}

func TestDeleteFieldReportsGroupsItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	sabotage(t, pool, "ALTER TABLE core.field_groups RENAME COLUMN title TO retired")

	if err := store.DeleteField(t.Context(), "car", "subtitle"); err == nil {
		t.Error("DeleteField() error = nil, want the unreadable groups reported")
	}
}

func TestDeleteFieldReportsTypesItCannotRead(t *testing.T) {
	t.Parallel()

	store, _, pool := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")
	sabotage(t, pool, "ALTER TABLE core.content_types RENAME COLUMN key TO retired")

	if err := store.DeleteField(t.Context(), "car", "subtitle"); err == nil {
		t.Error("DeleteField() error = nil, want the unreadable types reported")
	}
}

func TestDeleteFieldReportsRevisionValuesItCannotSweep(t *testing.T) {
	t.Parallel()

	store, author, pool := typedStore(t)
	storeType(t, store, "car")
	declareTypedField(t, store, "car", "subtitle")
	plantTyped(t, pool, author, "car", "one-car", `{"subtitle": "car words"}`)
	raiseOn(t, pool, "core.content_revisions", "UPDATE")

	if err := store.DeleteField(t.Context(), "car", "subtitle"); err == nil {
		t.Error("DeleteField() error = nil, want the refused revision sweep reported")
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
