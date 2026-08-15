// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// CreateField stores a field on its type.
func (s *fakeTypeStore) CreateField(_ context.Context, f content.Field) (content.Field, error) {
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

// UpdateField stores the edited field on its type.
func (s *fakeTypeStore) UpdateField(_ context.Context, f content.Field) (content.Field, error) {
	for i, stored := range s.types {
		if stored.Key != f.TypeKey {
			continue
		}
		for j, held := range stored.Fields {
			if held.Key == f.Key {
				f.ID = held.ID
				s.types[i].Fields[j] = f
				return f, nil
			}
		}
	}
	return content.Field{}, content.ErrFieldNotFound
}

// DeleteField removes the field from its type.
func (s *fakeTypeStore) DeleteField(_ context.Context, typeKey, key string) error {
	for i, stored := range s.types {
		if stored.Key != typeKey {
			continue
		}
		for j, held := range stored.Fields {
			if held.Key == key {
				s.types[i].Fields = append(stored.Fields[:j], stored.Fields[j+1:]...)
				return nil
			}
		}
	}
	return content.ErrFieldNotFound
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

func TestRegistryRelabelsAFieldKeepingItsShape(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	if _, err := registry.CreateField(t.Context(), colorField(t)); err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}
	edit := colorField(t)
	edit.Label, edit.Kind, edit.Required = "Paint", content.FieldKindNumber, true

	updated, err := registry.UpdateField(t.Context(), edit)

	if err != nil {
		t.Fatalf("UpdateField() error = %v, want nil", err)
	}
	if updated.Label != "Paint" || !updated.Required {
		t.Errorf("UpdateField() = %+v, want the label and the flag carried", updated)
	}
	if updated.Kind != content.FieldKindText {
		t.Errorf("UpdateField() kind = %q, want the stored kind kept", updated.Kind)
	}
}

func TestRegistryRefusesRelabelingAnUnknownField(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	_, err := registry.UpdateField(t.Context(), colorField(t))

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Fatalf("UpdateField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestRegistryDeletesAField(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	if _, err := registry.CreateField(t.Context(), colorField(t)); err != nil {
		t.Fatalf("declaring the field: %v, want nil", err)
	}

	if err := registry.DeleteField(t.Context(), "post", "color"); err != nil {
		t.Fatalf("DeleteField() error = %v, want nil", err)
	}

	held, err := registry.ByKey(t.Context(), "post")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if len(held.Fields) != 0 {
		t.Errorf("ByKey() fields = %+v, want the field gone", held.Fields)
	}
}

func TestRegistryRefusesDeletingAnUnknownField(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	err := registry.DeleteField(t.Context(), "post", "color")

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Fatalf("DeleteField() error = %v, want %v", err, content.ErrFieldNotFound)
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

func TestRegistryLetsASelfTargetingTypeGo(t *testing.T) {
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

	if err := registry.Delete(t.Context(), "car"); err != nil {
		t.Fatalf("Delete() error = %v, want the self targeting type released", err)
	}
}
