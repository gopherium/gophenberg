// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// unreadSource declares a car type, a cars group holding a maker relation pointing at posts, and an empty makers group.
func unreadSource(t *testing.T, registry *content.Registry) (content.Group, content.Group) {
	t.Helper()
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("Create(car) error = %v, want nil", err)
	}
	cars, err := registry.CreateGroup(t.Context(), content.Group{
		Key: "cars", Title: "Cars", Location: namingType("car"), Active: true,
	})
	if err != nil {
		t.Fatalf("CreateGroup(cars) error = %v, want nil", err)
	}
	relationOnCars(t, registry, cars, "maker", content.TypePost)
	return cars, groupNaming(t, registry, "Makers", namingPost())
}

// relationOnCars declares a relation on the cars group pointing at the type.
func relationOnCars(t *testing.T, registry *content.Registry, cars content.Group, key, relatesTo string) {
	t.Helper()
	if _, err := registry.CreateFieldInGroup(t.Context(), cars.ID, content.Field{
		Key: key, Label: key, Kind: content.FieldKindRelation, RelatesTo: relatesTo,
	}); err != nil {
		t.Fatalf("CreateFieldInGroup(%s) error = %v, want nil", key, err)
	}
}

// relationGoes deletes, as another admin would meanwhile, the relation the key names in the group.
func relationGoes(t *testing.T, groupID int, key string) func(*groupingStore) {
	return func(s *groupingStore) {
		if err := s.DeleteFieldInGroup(context.Background(), groupID, key, nil); err != nil {
			t.Fatalf("deleting %s meanwhile: %v, want nil", key, err)
		}
	}
}

// readerArrives declares, as another admin would meanwhile, a backlinks in the group reading the cars path.
func readerArrives(t *testing.T, groupID int, path ...any) func(*groupingStore) {
	return func(s *groupingStore) {
		reader := backlinksField(map[string]any{content.SettingSourceGroup: "cars", content.SettingSourceField: path})
		reader.Key = "arrived"
		if _, err := s.CreateFieldInGroup(context.Background(), groupID, reader, nil); err != nil {
			t.Fatalf("declaring a reader meanwhile: %v, want nil", err)
		}
	}
}

// groupGoes deletes, as another admin would meanwhile, the group carrying the identity.
func groupGoes(t *testing.T, id int) func(*groupingStore) {
	return func(s *groupingStore) {
		if err := s.DeleteGroup(context.Background(), id, nil); err != nil {
			t.Fatalf("deleting group %d meanwhile: %v, want nil", id, err)
		}
	}
}

func TestRegistryReportsAWriteWhoseGroupGoesMeanwhile(t *testing.T) {
	t.Parallel()

	for name, write := range map[string]func(context.Context, *content.Registry, content.Group) error{
		"a declaration": func(ctx context.Context, r *content.Registry, makers content.Group) error {
			return firstErr(r.CreateFieldInGroup(ctx, makers.ID, backlinksField(namingSource())))
		},
		"a group edit": func(ctx context.Context, r *content.Registry, makers content.Group) error {
			makers.Title = "Renamed"
			return firstErr(r.UpdateGroup(ctx, makers))
		},
		"a group delete": func(ctx context.Context, r *content.Registry, makers content.Group) error {
			return r.DeleteGroup(ctx, makers.ID)
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			store := newGroupingStore()
			registry := content.NewRegistry(store)
			_, makers := unreadSource(t, registry)
			store.meanwhile = groupGoes(t, makers.ID)

			err := write(t.Context(), registry, makers)

			if !errors.Is(err, content.ErrGroupNotFound) {
				t.Errorf("error = %v, want %v", err, content.ErrGroupNotFound)
			}
		})
	}
}

func TestRegistryReportsAMoveWhoseFieldGoesMeanwhile(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, _ := unreadSource(t, registry)
	details := sectionIn(t, registry, cars.ID)
	store.meanwhile = relationGoes(t, cars.ID, "maker")

	_, err := registry.MoveField(t.Context(), topFieldIn(t, registry, cars.ID, "maker").ID, cars.ID, details.ID)

	if !errors.Is(err, content.ErrFieldNotFound) {
		t.Errorf("MoveField() error = %v, want %v", err, content.ErrFieldNotFound)
	}
}

func TestRegistryReportsTypesItCannotReadWhileJudgingADelete(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, _ := unreadSource(t, registry)
	store.listErr = errStoreDown

	err := registry.DeleteFieldInGroup(t.Context(), cars.ID, "maker")

	if !errors.Is(err, errStoreDown) {
		t.Errorf("DeleteFieldInGroup() error = %v, want %v", err, errStoreDown)
	}
}

func TestRegistryReportsTypesItCannotReadWhileJudgingAMove(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, _ := unreadSource(t, registry)
	details := sectionIn(t, registry, cars.ID)
	maker := topFieldIn(t, registry, cars.ID, "maker")
	store.listErr = errStoreDown

	_, err := registry.MoveField(t.Context(), maker.ID, cars.ID, details.ID)

	if !errors.Is(err, errStoreDown) {
		t.Errorf("MoveField() error = %v, want %v", err, errStoreDown)
	}
}

func TestRegistryReportsGroupsItCannotReadBeforeDeletingFields(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, _ := unreadSource(t, registry)
	store.groupsErr = errStoreDown

	err := registry.DeleteFieldsOfGroupSettled(t.Context(), cars.ID, []string{"maker"}, nil)

	if !errors.Is(err, errStoreDown) {
		t.Errorf("DeleteFieldsOfGroupSettled() error = %v, want %v", err, errStoreDown)
	}
}

func TestRegistryRefusesALinkedFromWhoseRelationGoesMeanwhile(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, makers := unreadSource(t, registry)
	store.meanwhile = relationGoes(t, cars.ID, "maker")

	_, err := registry.CreateFieldInGroup(t.Context(), makers.ID, backlinksField(namingSource()))

	if codeOf(err) != "backlinks_source_unknown" {
		t.Errorf("CreateFieldInGroup() error = %v, want backlinks_source_unknown", err)
	}
}

func TestRegistryRefusesAnEditedLinkedFromWhoseRelationGoesMeanwhile(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, makers := readableSource(t, registry)
	relationOnCars(t, registry, cars, "rival", content.TypePost)
	held := backlinksIn(t, registry, makers.ID)
	held.Settings = map[string]any{content.SettingSourceGroup: "cars", content.SettingSourceField: []any{"rival"}}
	store.meanwhile = relationGoes(t, cars.ID, "rival")

	_, err := registry.UpdateFieldInGroup(t.Context(), makers.ID, held, held.UpdatedAt)

	if codeOf(err) != "backlinks_source_unknown" {
		t.Errorf("UpdateFieldInGroup() error = %v, want backlinks_source_unknown", err)
	}
}

func TestRegistryRefusesAGroupMoveWhoseNewSourceGoesMeanwhile(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, makers := readableSource(t, registry)
	twinOnCars(t, registry, cars)
	makers.Location = namingType("car")
	asked := readingOnCars(t, registry, makers.ID, "linked-from", "twin")
	store.meanwhile = relationGoes(t, cars.ID, "twin")

	_, err := registry.UpdateGroupRepointed(t.Context(), makers, asked)

	if codeOf(err) != "backlinks_source_unknown" {
		t.Errorf("UpdateGroupRepointed() error = %v, want backlinks_source_unknown", err)
	}
}

func TestRegistryRefusesAMoveWhoseReaderArrivesMeanwhile(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, makers := unreadSource(t, registry)
	details := sectionIn(t, registry, cars.ID)
	store.meanwhile = readerArrives(t, makers.ID, "maker")

	_, err := registry.MoveField(t.Context(), topFieldIn(t, registry, cars.ID, "maker").ID, cars.ID, details.ID)

	if codeOf(err) != "field_referenced" {
		t.Errorf("MoveField() error = %v, want field_referenced", err)
	}
}

func TestRegistryRefusesADeleteWhoseReaderArrivesMeanwhile(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, makers := unreadSource(t, registry)
	store.meanwhile = readerArrives(t, makers.ID, "maker")

	err := registry.DeleteFieldInGroup(t.Context(), cars.ID, "maker")

	if codeOf(err) != "field_referenced" {
		t.Errorf("DeleteFieldInGroup() error = %v, want field_referenced", err)
	}
}

func TestRegistryRefusesAFieldsDeleteWhoseReaderArrivesMeanwhile(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, makers := unreadSource(t, registry)
	store.meanwhile = readerArrives(t, makers.ID, "maker")

	err := registry.DeleteFieldsOfGroupSettled(t.Context(), cars.ID, []string{"maker"}, nil)

	if codeOf(err) != "field_referenced" {
		t.Errorf("DeleteFieldsOfGroupSettled() error = %v, want field_referenced", err)
	}
}

func TestRegistryRefusesASubFieldDeleteWhoseReaderArrivesMeanwhile(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, makers := unreadSource(t, registry)
	details := sectionIn(t, registry, cars.ID)
	engine, err := registry.CreateSubField(t.Context(), details.ID, content.Field{
		Key: "engine", Label: "Engine", Kind: content.FieldKindRelation, RelatesTo: content.TypePost,
	})
	if err != nil {
		t.Fatalf("CreateSubField(engine) error = %v, want nil", err)
	}
	store.meanwhile = readerArrives(t, makers.ID, "details", "engine")

	err = registry.DeleteSubField(t.Context(), engine.ID)

	if codeOf(err) != "field_referenced" {
		t.Errorf("DeleteSubField() error = %v, want field_referenced", err)
	}
}

func TestRegistryRefusesAGroupDeleteWhoseReaderArrivesMeanwhile(t *testing.T) {
	t.Parallel()

	store := newGroupingStore()
	registry := content.NewRegistry(store)
	cars, makers := unreadSource(t, registry)
	store.meanwhile = readerArrives(t, makers.ID, "maker")

	err := registry.DeleteGroup(t.Context(), cars.ID)

	if codeOf(err) != "field_referenced" {
		t.Errorf("DeleteGroup() error = %v, want field_referenced", err)
	}
}
