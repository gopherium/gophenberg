// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// newTypeStore returns a type store over a migrated database.
func newTypeStore(t *testing.T) *postgres.TypeStore {
	t.Helper()
	_, _, pool := newContentStoreWithPool(t)
	return postgres.NewTypeStore(pool)
}

// carType returns a car content type ready to store.
func carType(t *testing.T) content.Type {
	t.Helper()
	stored, err := content.NewType("car", "Car", "Cars", "cars")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	return stored
}

func TestTypeStoreListsTheBuiltInType(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)

	types, err := store.List(t.Context())

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if len(types) != 1 {
		t.Fatalf("List() returned %d types, want the built-in one", len(types))
	}
	post := types[0]
	if post.Key != content.TypePost || post.PluralLabel != "Posts" || !post.Default || !post.Active {
		t.Errorf("List()[0] = %+v, want the active default post type", post)
	}
	if post.RouteWord != "" || post.PageKind != content.PageKindSingle || !post.Revisions {
		t.Errorf("List()[0] = %+v, want the rooted single-page type keeping revisions", post)
	}
}

func TestTypeStoreCreatesAndReadsBackAType(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)

	created, err := store.Create(t.Context(), carType(t))

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Key != "car" || created.RouteWord != "cars" || created.SingularLabel != "Car" {
		t.Errorf("Create() = %+v, want the car type", created)
	}
	read, err := store.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	if read.Key != created.Key || read.CreatedAt.Location() != time.UTC {
		t.Errorf("ByKey() = %+v, want the stored car with UTC stamps", read)
	}
}

func TestTypeStoreReportsATakenKey(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)
	if _, err := store.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	_, err := store.Create(t.Context(), carType(t))

	if !errors.Is(err, content.ErrTypeTaken) {
		t.Errorf("Create() error = %v, want %v", err, content.ErrTypeTaken)
	}
}

func TestTypeStoreReportsATakenRouteWord(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)
	if _, err := store.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	other, err := content.NewType("van", "Van", "Vans", "cars")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}

	_, err = store.Create(t.Context(), other)

	if !errors.Is(err, content.ErrRouteWordTaken) {
		t.Errorf("Create() error = %v, want %v", err, content.ErrRouteWordTaken)
	}
}

func TestTypeStoreReportsAMissingType(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)

	_, err := store.ByKey(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ByKey() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestTypeStoreUpdatesTheEditableFields(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)
	created, err := store.Create(t.Context(), carType(t))
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	edited := created
	edited.SingularLabel, edited.PluralLabel = "Vehicle", "Vehicles"
	edited.RouteWord, edited.Hierarchical, edited.Active = "vehicles", true, false
	edited.UpdatedAt = created.UpdatedAt.Add(time.Second)

	updated, err := store.Update(t.Context(), edited)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.PluralLabel != "Vehicles" || updated.RouteWord != "vehicles" {
		t.Errorf("Update() = %+v, want the relabeled type", updated)
	}
	if !updated.Hierarchical || updated.Active {
		t.Errorf("Update() = %+v, want it nesting and deactivated", updated)
	}
	if updated.Key != "car" || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("Update() = %+v, want the key and creation stamp untouched", updated)
	}
}

func TestTypeStoreUpdateReportsAMissingType(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)

	_, err := store.Update(t.Context(), carType(t))

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestTypeStoreDeletesAnEmptyType(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)
	if _, err := store.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	if err := store.Delete(t.Context(), "car"); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if _, err := store.ByKey(t.Context(), "car"); !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ByKey() after Delete error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestTypeStoreUpdateReportsATakenRouteWord(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)
	if _, err := store.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	van, err := content.NewType("van", "Van", "Vans", "vans")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	if _, err := store.Create(t.Context(), van); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	van.RouteWord = "cars"

	_, err = store.Update(t.Context(), van)

	if !errors.Is(err, content.ErrRouteWordTaken) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrRouteWordTaken)
	}
}

func TestTypeStoreWrapsDatabaseFailures(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	store := postgres.NewTypeStore(pool)
	pool.Close()

	_, list := store.List(t.Context())
	_, byKey := store.ByKey(t.Context(), content.TypePost)
	_, created := store.Create(t.Context(), carType(t))
	_, updated := store.Update(t.Context(), carType(t))
	deleted := store.Delete(t.Context(), content.TypePost)

	for name, err := range map[string]error{
		"List": list, "ByKey": byKey, "Create": created, "Update": updated, "Delete": deleted,
	} {
		if err == nil {
			t.Errorf("%s() on a closed pool error = nil, want a failure", name)
		}
	}
}

func TestTypeStoreDeleteReportsAMissingType(t *testing.T) {
	t.Parallel()

	store := newTypeStore(t)

	err := store.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("Delete() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestTypeStoreKeepsATypeHoldingContent(t *testing.T) {
	t.Parallel()

	contentStore, author, pool := newContentStoreWithPool(t)
	store := postgres.NewTypeStore(pool)
	car := carType(t)
	if _, err := store.Create(t.Context(), car); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	item, err := content.New(car, nil, "Ford Focus", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if _, err := contentStore.Create(t.Context(), item); err != nil {
		t.Fatalf("storing the car: %v, want nil", err)
	}

	err = store.Delete(t.Context(), "car")

	if !errors.Is(err, content.ErrTypeInUse) {
		t.Errorf("Delete() error = %v, want %v", err, content.ErrTypeInUse)
	}
}
