// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// CreateField stores a field on its type.
func (s *fakeTypeStore) CreateField(_ context.Context, f content.Field) (content.Field, error) {
	if s.createFieldErr != nil {
		return content.Field{}, s.createFieldErr
	}
	for i, stored := range s.types {
		if stored.Key != f.TypeKey {
			continue
		}
		f.ID = s.nextFieldID()
		s.types[i].Fields = append(s.types[i].Fields, f)
		return f, nil
	}
	return content.Field{}, content.ErrTypeNotFound
}

// nextFieldID hands out a fresh definition identity.
func (s *fakeTypeStore) nextFieldID() int {
	s.fieldIDs++
	return s.fieldIDs
}

// colorField returns a text field ready to declare on the post type.
func colorField(t *testing.T) content.Field {
	t.Helper()
	built, err := content.NewField(content.Field{
		TypeKey: "post", Key: "color", Label: "Color", Kind: content.FieldKindText,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	return built
}

func TestRegistryDeclaresAFieldOnAType(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	created, err := registry.CreateField(t.Context(), colorField(t))

	if err != nil {
		t.Fatalf("CreateField() error = %v, want nil", err)
	}
	if created.Key != "color" {
		t.Errorf("CreateField() key = %q, want %q", created.Key, "color")
	}
	held, err := registry.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 1 || held.Fields[0].Key != "color" {
		t.Errorf("ByKey() fields = %+v, want the declared field listed", held.Fields)
	}
}

func TestRegistryRefusesAFieldOnAnUnknownType(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	f := colorField(t)
	f.TypeKey = "car"

	_, err := registry.CreateField(t.Context(), f)

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestRegistryRefusesATakenFieldKey(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	if _, err := registry.CreateField(t.Context(), colorField(t)); err != nil {
		t.Fatalf("declaring the first field: %v, want nil", err)
	}

	_, err := registry.CreateField(t.Context(), colorField(t))

	if !errors.Is(err, content.ErrFieldTaken) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrFieldTaken)
	}
}

func TestRegistryRefusesARelationToAnUnknownTarget(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	f, err := content.NewField(content.Field{
		TypeKey: "post", Key: "engine", Label: "Engine",
		Kind: content.FieldKindRelation, RelatesTo: "engine-type",
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}

	_, err = registry.CreateField(t.Context(), f)

	if !errors.Is(err, content.ErrTargetUnknown) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrTargetUnknown)
	}
}

func TestRegistryRefusesAMalformedField(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	_, err := registry.CreateField(t.Context(), content.Field{
		TypeKey: "post", Key: "taste", Label: "Taste", Kind: "flavor",
	})

	if !errors.Is(err, content.ErrInvalidFieldKind) {
		t.Fatalf("CreateField() error = %v, want %v", err, content.ErrInvalidFieldKind)
	}
}

func TestRegistryKeepsATypeAFieldTargets(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}
	f, err := content.NewField(content.Field{
		TypeKey: "post", Key: "cars", Label: "Cars",
		Kind: content.FieldKindRelation, RelatesTo: "car", Many: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if _, err := registry.CreateField(t.Context(), f); err != nil {
		t.Fatalf("declaring the relation: %v, want nil", err)
	}

	err = registry.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeTargeted) {
		t.Fatalf("Delete() error = %v, want %v", err, content.ErrTypeTargeted)
	}
}

// groupedStore is a fake exposing groups beside the per type surface.
type groupedStore struct {
	*fakeTypeStore
	groups []content.Group
}

// ListGroups returns the held groups.
func (s *groupedStore) ListGroups(context.Context) ([]content.Group, error) {
	return s.groups, nil
}

func TestRegistryRefusesDeletingATypeARestingGroupStillTargets(t *testing.T) {
	t.Parallel()

	store := &groupedStore{fakeTypeStore: newFakeTypeStore(), groups: []content.Group{{
		ID: 1, Title: "Resting extras", Active: false,
		Fields: []content.Field{{
			Key: "cars", Label: "Cars", Kind: content.FieldKindRelation, RelatesTo: "car", Many: true,
		}},
	}}}
	registry := content.NewRegistry(store)
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}

	err := registry.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeTargeted) {
		t.Fatalf("Delete() error = %v, want the inactive group's relation still guarding", err)
	}
	var refused *content.Error
	if !errors.As(err, &refused) || refused.Held["group"] != "Resting extras" {
		t.Errorf("details = %v, want the guarding group named", err)
	}
}

func TestRegistryDeletesATypeNoRelationTargets(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}

	if err := registry.Delete(t.Context(), "car"); err != nil {
		t.Fatalf("Delete() error = %v, want the untargeted type released", err)
	}

	if _, err := registry.ByKey(t.Context(), "car"); !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ByKey() error = %v, want the type gone from the registry", err)
	}
}

func TestRegistryHoldsATypeItsOwnGroupStillTargets(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}
	f, err := content.NewField(content.Field{
		TypeKey: "car", Key: "sibling", Label: "Sibling",
		Kind: content.FieldKindRelation, RelatesTo: "car",
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if _, err := registry.CreateField(t.Context(), f); err != nil {
		t.Fatalf("declaring the relation: %v, want nil", err)
	}

	err = registry.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeTargeted) {
		t.Fatalf("Delete() error = %v, want the surviving relation holding the type", err)
	}
}

// threeFieldCar returns a registry holding the car type with three text fields declared in order.
func threeFieldCar(t *testing.T) *content.Registry {
	t.Helper()
	registry := content.NewRegistry(newFakeTypeStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("registering the car type: %v, want nil", err)
	}
	for _, key := range []string{"color", "engine", "doors"} {
		f, err := content.NewField(content.Field{
			TypeKey: "car", Key: key, Label: key, Kind: content.FieldKindText,
		})
		if err != nil {
			t.Fatalf("NewField(%q) error = %v, want nil", key, err)
		}
		if _, err := registry.CreateField(t.Context(), f); err != nil {
			t.Fatalf("declaring %q: %v, want nil", key, err)
		}
	}
	return registry
}

// fieldKeysOf returns the field keys of a type in declared order.
func fieldKeysOf(t *testing.T, registry *content.Registry, typeKey string) []string {
	t.Helper()
	held, err := registry.ByKey(t.Context(), typeKey)
	if err != nil {
		t.Fatalf("ByKey(%q) error = %v, want nil", typeKey, err)
	}
	keys := make([]string, len(held.Fields))
	for i, f := range held.Fields {
		keys[i] = f.Key
	}
	return keys
}

// carGroup returns the id of the group serving the car's fields.
func carGroup(t *testing.T, registry *content.Registry) int {
	t.Helper()
	groups, err := registry.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups() error = %v, want nil", err)
	}
	for _, g := range groups {
		for _, f := range g.Fields {
			if f.Key == "color" {
				return g.ID
			}
		}
	}
	t.Fatal("no group serves the car's fields")
	return 0
}

func TestRegistryRefusesAReorderNamingAStranger(t *testing.T) {
	t.Parallel()

	registry := threeFieldCar(t)

	_, err := registry.ReorderFieldsInGroup(t.Context(), carGroup(t, registry), []string{"color", "engine", "finish"})

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want %v", err, content.ErrFieldNotFound)
	}
	if got := fieldKeysOf(t, registry, "car"); !slices.Equal(got, []string{"color", "engine", "doors"}) {
		t.Errorf("fields after the refusal = %v, want the declared order kept", got)
	}
}

func TestRegistryRefusesAReorderLeavingAFieldOut(t *testing.T) {
	t.Parallel()

	registry := threeFieldCar(t)

	_, err := registry.ReorderFieldsInGroup(t.Context(), carGroup(t, registry), []string{"doors", "color"})

	if !errors.Is(err, content.ErrFieldOrder) {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want %v", err, content.ErrFieldOrder)
	}
	if code, _ := content.CodeOf(err); code != "field_order_incomplete" {
		t.Errorf("code = %q, want field_order_incomplete", code)
	}
}

func TestRegistryRefusesAReorderRepeatingAField(t *testing.T) {
	t.Parallel()

	registry := threeFieldCar(t)

	_, err := registry.ReorderFieldsInGroup(t.Context(), carGroup(t, registry), []string{"doors", "color", "color"})

	if !errors.Is(err, content.ErrFieldOrder) {
		t.Fatalf("ReorderFieldsInGroup() error = %v, want %v", err, content.ErrFieldOrder)
	}
}
