// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"errors"
	"testing"
)

// errTypeStoreDown stands for a type store the database cannot answer for.
var errTypeStoreDown = errors.New("the type store cannot be read")

// unreadableTypeStore answers every call with the failure the database reports.
type unreadableTypeStore struct{}

// List reports the failure rather than the stored types.
func (unreadableTypeStore) List(context.Context) ([]Type, error) {
	return nil, errTypeStoreDown
}

// ByKey reports the failure rather than the stored type.
func (unreadableTypeStore) ByKey(context.Context, string) (Type, error) {
	return Type{}, errTypeStoreDown
}

// Create reports the failure rather than storing the type.
func (unreadableTypeStore) Create(context.Context, Type) (Type, error) {
	return Type{}, errTypeStoreDown
}

// Update reports the failure rather than storing the edited type.
func (unreadableTypeStore) Update(context.Context, Type) (Type, error) {
	return Type{}, errTypeStoreDown
}

// Delete reports the failure rather than removing the type.
func (unreadableTypeStore) Delete(context.Context, string) error {
	return errTypeStoreDown
}

// CreateField reports the failure rather than declaring the field.
func (unreadableTypeStore) CreateField(context.Context, Field) (Field, error) {
	return Field{}, errTypeStoreDown
}

// UpdateField reports the failure rather than storing the edited field.
func (unreadableTypeStore) UpdateField(context.Context, Field) (Field, error) {
	return Field{}, errTypeStoreDown
}

// DeleteField reports the failure rather than removing the field.
func (unreadableTypeStore) DeleteField(context.Context, string, string) error {
	return errTypeStoreDown
}

// ReorderFields reports the failure rather than storing the order.
func (unreadableTypeStore) ReorderFields(context.Context, string, []string) error {
	return errTypeStoreDown
}

// ListGroups reports the failure rather than listing the groups.
func (unreadableTypeStore) ListGroups(context.Context) ([]Group, error) {
	return nil, errTypeStoreDown
}

// CreateGroup reports the failure rather than storing the group.
func (unreadableTypeStore) CreateGroup(context.Context, Group) (Group, error) {
	return Group{}, errTypeStoreDown
}

// UpdateGroup reports the failure rather than storing the group.
func (unreadableTypeStore) UpdateGroup(context.Context, Group) (Group, error) {
	return Group{}, errTypeStoreDown
}

// DeleteGroup reports the failure rather than removing the group.
func (unreadableTypeStore) DeleteGroup(context.Context, int) error {
	return errTypeStoreDown
}

// ReorderGroups reports the failure rather than storing the order.
func (unreadableTypeStore) ReorderGroups(context.Context, []int) error {
	return errTypeStoreDown
}

// CreateFieldInGroup reports the failure rather than declaring the field.
func (unreadableTypeStore) CreateFieldInGroup(context.Context, int, Field) (Field, error) {
	return Field{}, errTypeStoreDown
}

// MoveField reports the failure rather than carrying the field.
func (unreadableTypeStore) MoveField(context.Context, int, string, int) (Field, error) {
	return Field{}, errTypeStoreDown
}

// UpdateFieldInGroup reports the failure rather than storing the field.
func (unreadableTypeStore) UpdateFieldInGroup(context.Context, int, Field) (Field, error) {
	return Field{}, errTypeStoreDown
}

// DeleteFieldInGroup reports the failure rather than removing the field.
func (unreadableTypeStore) DeleteFieldInGroup(context.Context, int, string) error {
	return errTypeStoreDown
}

// ReorderFieldsInGroup reports the failure rather than storing the order.
func (unreadableTypeStore) ReorderFieldsInGroup(context.Context, int, []string) error {
	return errTypeStoreDown
}

func TestUntargetedReportsARegistryItCannotRead(t *testing.T) {
	t.Parallel()

	registry := NewRegistry(unreadableTypeStore{})

	err := registry.untargeted(t.Context(), TypePost)

	if err == nil {
		t.Fatal("untargeted() = nil, want the registry it cannot read reported")
	}
	if errors.Is(err, ErrTypeTargeted) {
		t.Fatalf("untargeted() = %v, want the read failure rather than a field still targeting the type", err)
	}
	if !errors.Is(err, errTypeStoreDown) {
		t.Errorf("untargeted() error = %v, want %v", err, errTypeStoreDown)
	}
}
