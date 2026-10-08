// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// CarType returns a car content type ready to store.
func CarType(t *testing.T) content.Type {
	t.Helper()
	stored, err := content.NewType("car", "Car", "Cars", "cars")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	return stored
}

// StoreType stores a content type built from the key.
func StoreType(t *testing.T, types content.TypeStore, key string) {
	t.Helper()
	built, err := content.NewType(key, "One "+key, "Many "+key, key+"s")
	if err != nil {
		t.Fatalf("NewType(%s) error = %v, want nil", key, err)
	}
	if _, err := types.Create(t.Context(), built); err != nil {
		t.Fatalf("Create(%s) error = %v, want nil", key, err)
	}
}

// LocationOf returns a one rule location naming the type.
func LocationOf(typeKey string) content.Rules {
	return content.Rules{{{Source: content.ScreenContentType, Operator: content.OperatorIs, Value: typeKey}}}
}

// FieldOn returns a field definition ready to declare on the type.
func FieldOn(t *testing.T, typeKey, key string, kind content.FieldKind, relatesTo string) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: typeKey, Key: key, Label: "A Field", Kind: kind, RelatesTo: relatesTo,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	return built
}

// FieldsGroupOf returns the group keyed after the type, stored at the type's own location when no group holds the key.
func FieldsGroupOf(t *testing.T, types content.TypeStore, typeKey string) content.Group {
	t.Helper()
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	key := typeKey + "-fields"
	for _, g := range groups {
		if g.Key == key {
			return g
		}
	}
	named, err := types.ByKey(t.Context(), typeKey)
	if err != nil {
		t.Fatalf("ByKey(%s) error = %v, want nil", typeKey, err)
	}
	created, err := types.CreateGroup(t.Context(), content.Group{
		Key: key, Title: named.SingularLabel + " fields", Location: LocationOf(typeKey),
	})
	if err != nil {
		t.Fatalf("CreateGroup(%s) error = %v, want nil", key, err)
	}
	return created
}

// DeclareTypedField stores a text field on the type inside the group keyed after the type.
func DeclareTypedField(t *testing.T, types content.TypeStore, typeKey, key string) content.Field {
	t.Helper()
	group := FieldsGroupOf(t, types, typeKey)
	stored, err := types.CreateFieldInGroup(
		t.Context(), group.ID, FieldOn(t, typeKey, key, content.FieldKindText, ""), nil,
	)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(%s on %s) error = %v, want nil", key, typeKey, err)
	}
	return stored
}

// GroupHolding returns the id of the stored group declaring the key.
func GroupHolding(t *testing.T, types content.TypeStore, key string) int {
	t.Helper()
	groups, err := types.ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want nil", err)
	}
	for _, g := range groups {
		for _, f := range g.Fields {
			if f.Key == key {
				return g.ID
			}
		}
	}
	t.Fatalf("no stored group declares %q", key)
	return 0
}

// StoreItem stores an item of the type under the parent and returns it.
func StoreItem(
	t *testing.T, store content.Store, kind content.Type, parent *content.Content, title string, author uuid.UUID,
) content.Content {
	t.Helper()
	built, err := content.New(kind, parent, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	stored, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return stored
}
