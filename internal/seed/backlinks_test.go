// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// filingTypeStore reports the post type pointing at the category type, and what the seeding declared.
type filingTypeStore struct {
	stubTypeStore
	declared  []content.Field
	listErr   error
	groupsErr error
	createErr error
}

// List returns the post and category types, the category carrying what the seeding declared on it.
func (s *filingTypeStore) List(context.Context) ([]content.Type, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return []content.Type{{
		Key: content.TypePost, SingularLabel: "Post", PluralLabel: "Posts",
		Revisions: true, RevisionCap: 100, PageKind: content.PageKindSingle,
		Default: true, Active: true, Fields: []content.Field{CategoriesField()},
	}, {
		Key: CategoryTypeKey, SingularLabel: "Category", PluralLabel: "Categories",
		Revisions: true, RevisionCap: 100, PageKind: content.PageKindArchive,
		Active: true, Fields: s.declared,
	}}, nil
}

// ByKey returns the type carrying the key, or reports it missing.
func (s *filingTypeStore) ByKey(ctx context.Context, key string) (content.Type, error) {
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

// ListGroups returns the post type's group holding the relation, and the category's holding what was declared.
func (s *filingTypeStore) ListGroups(context.Context) ([]content.Group, error) {
	if s.groupsErr != nil {
		return nil, s.groupsErr
	}
	return []content.Group{{
		ID: 1, Key: "post-fields", Title: "Post fields", Active: true,
		Location: content.Rules{{{
			Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.TypePost,
		}}},
		Fields: []content.Field{CategoriesField()},
	}, {
		ID: 2, Key: "category-fields", Title: "Category fields", Active: true,
		Location: content.Rules{{{
			Source: content.ScreenContentType, Operator: content.OperatorIs, Value: CategoryTypeKey,
		}}},
		Fields: s.declared,
	}}, nil
}

// CreateField records the declaration and hands it back.
func (s *filingTypeStore) CreateField(_ context.Context, f content.Field) (content.Field, error) {
	if s.createErr != nil {
		return content.Field{}, s.createErr
	}
	s.declared = append(s.declared, f)
	return f, nil
}

func TestBacklinksDeclaresTheFieldListingWhatIsFiledUnderACategory(t *testing.T) {
	t.Parallel()

	types := &filingTypeStore{}

	if err := Backlinks(t.Context(), content.NewRegistry(types)); err != nil {
		t.Fatalf("Backlinks() error = %v, want nil", err)
	}

	if len(types.declared) != 1 {
		t.Fatalf("the store holds %+v, want the one field listing what points here", types.declared)
	}
	held := types.declared[0]
	if held.Key != PostsFiledFieldKey || held.Kind != content.FieldKindBacklinks {
		t.Errorf("the store holds %+v, want a backlinks field under %s", held, PostsFiledFieldKey)
	}
	if held.TypeKey != CategoryTypeKey {
		t.Errorf("TypeKey = %q, want the field standing on %s", held.TypeKey, CategoryTypeKey)
	}
	if got := content.SourceGroupOf(held); got != "post-fields" {
		t.Errorf("SourceGroupOf() = %q, want the group holding the relation", got)
	}
	if got := content.SourceFieldOf(held); len(got) != 1 || got[0] != CategoriesFieldKey {
		t.Errorf("SourceFieldOf() = %v, want the path reaching %s", got, CategoriesFieldKey)
	}
}

func TestTheSeededBacklinksFieldReadsARelationTheRegistryResolves(t *testing.T) {
	t.Parallel()

	types := &filingTypeStore{}
	registry := content.NewRegistry(types)
	if err := Backlinks(t.Context(), registry); err != nil {
		t.Fatalf("Backlinks() error = %v, want nil", err)
	}
	groups, err := registry.Groups(t.Context())
	if err != nil {
		t.Fatalf("Groups() error = %v, want nil", err)
	}
	listed, err := registry.All(t.Context())
	if err != nil {
		t.Fatalf("All() error = %v, want nil", err)
	}

	source, err := content.BacklinksSource(
		groups, listed, groups[1], types.declared[0], registry.Params(t.Context()),
	)

	if err != nil {
		t.Fatalf("BacklinksSource() error = %v, want the relation it reads", err)
	}
	if source.Key != CategoriesFieldKey {
		t.Errorf("BacklinksSource() = %q, want %q", source.Key, CategoriesFieldKey)
	}
}

func TestBacklinksLeavesAFieldTheCategoryAlreadyCarries(t *testing.T) {
	t.Parallel()

	types := &filingTypeStore{}
	registry := content.NewRegistry(types)
	if err := Backlinks(t.Context(), registry); err != nil {
		t.Fatalf("the first seeding: %v, want nil", err)
	}

	if err := Backlinks(t.Context(), registry); err != nil {
		t.Fatalf("the second seeding: %v, want nil", err)
	}

	if len(types.declared) != 1 {
		t.Errorf("the store holds %+v, want the second seeding declaring nothing", types.declared)
	}
}

func TestBacklinksReportsWhatItCannotDeclare(t *testing.T) {
	t.Parallel()

	for name, types := range map[string]*filingTypeStore{
		"the type it cannot read":   {listErr: errStub},
		"the groups it cannot read": {groupsErr: errStub},
		"the field it cannot store": {createErr: errStub},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := Backlinks(t.Context(), content.NewRegistry(types))

			if !errors.Is(err, errStub) {
				t.Errorf("Backlinks() error = %v, want %v", err, errStub)
			}
		})
	}
}

func TestBacklinksReportsAnAbsentRelationRatherThanNamingNone(t *testing.T) {
	t.Parallel()

	types := &unfiledTypeStore{}

	err := Backlinks(t.Context(), content.NewRegistry(types))

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("Backlinks() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

// unfiledTypeStore reports the two types with no group holding the relation posts file categories through.
type unfiledTypeStore struct {
	filingTypeStore
}

// ListGroups returns the category's group alone, holding no relation for a backlinks field to read.
func (s *unfiledTypeStore) ListGroups(context.Context) ([]content.Group, error) {
	return []content.Group{{
		ID: 2, Key: "category-fields", Title: "Category fields", Active: true,
		Fields: s.declared,
	}}, nil
}
