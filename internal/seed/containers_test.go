// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// holdingTypeStore reports the post type carrying whatever fields the seeding declared on it.
type holdingTypeStore struct {
	stubTypeStore
	declared []content.Field
	inside   []content.Field
}

// List returns the post type carrying the fields declared on it so far.
func (s *holdingTypeStore) List(ctx context.Context) ([]content.Type, error) {
	types, err := s.stubTypeStore.List(ctx)
	if err != nil || len(types) == 0 {
		return types, err
	}
	types[0].Fields = s.declared
	return types, nil
}

// ByKey returns the type carrying the key, or reports it missing.
func (s *holdingTypeStore) ByKey(ctx context.Context, key string) (content.Type, error) {
	types, err := s.List(ctx)
	if err != nil {
		return content.Type{}, err
	}
	for _, listed := range types {
		if listed.Key == key {
			return listed, nil
		}
	}
	return content.Type{}, content.ErrTypeNotFound
}

// ListGroups returns one group holding every declared field, with the rows declared inside each.
func (s *holdingTypeStore) ListGroups(context.Context) ([]content.Group, error) {
	fields := make([]content.Field, len(s.declared))
	copy(fields, s.declared)
	for i := range fields {
		for _, row := range s.inside {
			if row.ParentID == fields[i].ID {
				fields[i].Fields = append(fields[i].Fields, row)
			}
		}
	}
	return []content.Group{{ID: 1, Key: "post-fields", Title: "Post fields", Fields: fields}}, nil
}

// CreateField records the declaration and hands it back.
func (s *holdingTypeStore) CreateField(_ context.Context, f content.Field) (content.Field, error) {
	f.ID = len(s.declared) + 1
	s.declared = append(s.declared, f)
	return f, nil
}

// CreateSubField records the declaration inside a container and hands it back.
func (s *holdingTypeStore) CreateSubField(
	_ context.Context, parentID int, f content.Field,
) (content.Field, error) {
	f.ParentID = parentID
	s.inside = append(s.inside, f)
	return f, nil
}

func TestContainersDeclaresTheRepeaterAndItsRowFields(t *testing.T) {
	t.Parallel()

	types := &holdingTypeStore{}

	if err := Containers(t.Context(), content.NewRegistry(types)); err != nil {
		t.Fatalf("Containers() error = %v, want nil", err)
	}

	if len(types.declared) != 1 || types.declared[0].Key != TeamFieldKey {
		t.Fatalf("the store holds %+v, want the repeater declared", types.declared)
	}
	if len(types.inside) != 2 {
		t.Fatalf("the repeater holds %d row fields, want two", len(types.inside))
	}
	for _, held := range types.inside {
		if held.ParentID != types.declared[0].ID {
			t.Errorf("%q stands under %d, want the repeater %d",
				held.Key, held.ParentID, types.declared[0].ID)
		}
	}
}

func TestContainersLeavesARepeaterTheTypeAlreadyCarries(t *testing.T) {
	t.Parallel()

	types := &holdingTypeStore{}
	registry := content.NewRegistry(types)
	if err := Containers(t.Context(), registry); err != nil {
		t.Fatalf("the first seeding: %v, want nil", err)
	}

	if err := Containers(t.Context(), registry); err != nil {
		t.Fatalf("Containers() again error = %v, want nil", err)
	}

	if len(types.declared) != 1 || len(types.inside) != 2 {
		t.Errorf("the store holds %d fields and %d inside, want the second seeding to declare none",
			len(types.declared), len(types.inside))
	}
}

func TestContainersReportsWhatItCannotDeclare(t *testing.T) {
	t.Parallel()

	for name, registry := range map[string]*content.Registry{
		"the type it cannot read": content.NewRegistry(&categoryTypeStore{listErr: errStub}),
		"the field it cannot store": content.NewRegistry(
			&categoryTypeStore{createFieldErr: errStub}),
		"the row field it cannot store": content.NewRegistry(&categoryTypeStore{subFieldErr: errStub}),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Containers(t.Context(), registry)

			if !errors.Is(err, errStub) {
				t.Errorf("Containers() error = %v, want %v", err, errStub)
			}
		})
	}
}

func TestTeamRepeaterCarriesTheSubFieldsARowHolds(t *testing.T) {
	t.Parallel()

	held := TeamField()

	if held.Kind != content.FieldKindRepeater {
		t.Errorf("Kind = %q, want %q", held.Kind, content.FieldKindRepeater)
	}
	if held.TypeKey != content.TypePost {
		t.Errorf("TypeKey = %q, want the post type", held.TypeKey)
	}
	if err := held.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want the seeded repeater to stand", err)
	}
}

func TestTeamRowsDeclareANameAndARole(t *testing.T) {
	t.Parallel()

	rows := TeamRowFields()

	if len(rows) != 2 {
		t.Fatalf("the row declares %d fields, want the name and the role", len(rows))
	}
	for _, held := range rows {
		if held.Kind != content.FieldKindText {
			t.Errorf("%q holds %q, want text", held.Key, held.Kind)
		}
		if _, err := content.NewSubField(held, content.FieldKindRepeater); err != nil {
			t.Errorf("NewSubField(%q) error = %v, want it to stand inside a repeater", held.Key, err)
		}
	}
	if rows[0].Key != "name" || rows[1].Key != "role" {
		t.Errorf("the row declares %q then %q, want name then role", rows[0].Key, rows[1].Key)
	}
}

// AdoptType takes a plugin's type over as the site's own.
func (s *holdingTypeStore) AdoptType(context.Context, string) error {
	return nil
}

// AdoptGroup takes a plugin's group over as the site's own.
func (s *holdingTypeStore) AdoptGroup(context.Context, string) error {
	return nil
}
