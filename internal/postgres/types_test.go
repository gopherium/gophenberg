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

func TestUpdateCarriesContentToTheNewRouteWord(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v", err)
	}
	about := mustNest(t, store, nil, "About", author)
	team := mustNest(t, store, &about, "Team", author)

	moved := pageType()
	moved.RouteWord, moved.UpdatedAt = "sections", time.Now().UTC()
	if _, err := types.Update(t.Context(), moved); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if got := addressOf(t, store, about.ID); got != "sections/about" {
		t.Errorf("root path = %q, want it under the new route word", got)
	}
	if got := addressOf(t, store, team.ID); got != "sections/about/team" {
		t.Errorf("nested path = %q, want the whole tree carried", got)
	}
}

func TestUpdateCarriesContentDownFromTheRoot(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	built, err := content.New(postType(), nil, "Hello World", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	post, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	moved := postType()
	moved.RouteWord, moved.UpdatedAt = "blog", time.Now().UTC()
	if _, err := types.Update(t.Context(), moved); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if got := addressOf(t, store, post.ID); got != "blog/hello-world" {
		t.Errorf("path = %q, want the root content carried under the new word", got)
	}
}

func TestUpdateLeavesContentAloneWhenTheRouteWordStays(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v", err)
	}
	about := mustNest(t, store, nil, "About", author)

	relabeled := pageType()
	relabeled.PluralLabel, relabeled.UpdatedAt = "Sections", time.Now().UTC()
	if _, err := types.Update(t.Context(), relabeled); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if got := addressOf(t, store, about.ID); got != "pages/about" {
		t.Errorf("path = %q, want it left where it answers", got)
	}
}

func TestUpdateHandsTheRootToAnotherType(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v", err)
	}
	built, err := content.New(postType(), nil, "Hello World", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	post, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	about := mustNest(t, store, nil, "About", author)

	promoted := pageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()
	if _, err := types.Update(t.Context(), promoted); err != nil {
		t.Fatalf("Update() handing over the root error = %v, want nil", err)
	}

	registered, err := types.List(t.Context())
	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	held := map[string]content.Type{}
	for _, listed := range registered {
		held[listed.Key] = listed
	}
	if !held["page"].Default || held["page"].RouteWord != "" {
		t.Errorf("page = %+v, want it holding the root", held["page"])
	}
	if held[content.TypePost].Default || held[content.TypePost].RouteWord != "posts" {
		t.Errorf("post = %+v, want it moved off the root", held[content.TypePost])
	}
	if got := addressOf(t, store, about.ID); got != "about" {
		t.Errorf("page address = %q, want it lifted to the root", got)
	}
	if got := addressOf(t, store, post.ID); got != "posts/hello-world" {
		t.Errorf("post address = %q, want it under its own word", got)
	}
}

func TestUpdateRefusesToHandTheRootToAReservedAddress(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v", err)
	}
	relabelled := postType()
	relabelled.SingularLabel, relabelled.PluralLabel = "Medium", "Media"
	relabelled.UpdatedAt = time.Now().UTC()
	if _, err := types.Update(t.Context(), relabelled); err != nil {
		t.Fatalf("relabelling the default type: %v", err)
	}
	about := mustNest(t, store, nil, "About", author)

	promoted := pageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()

	_, err := types.Update(t.Context(), promoted)

	if !errors.Is(err, content.ErrRouteWordReserved) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrRouteWordReserved)
	}
	if got := addressOf(t, store, about.ID); got != "pages/about" {
		t.Errorf("page address = %q, want the refused hand over to have moved nothing", got)
	}
}

func TestUpdateRefusesToHandTheRootToAnUnusableAddress(t *testing.T) {
	t.Parallel()

	_, _, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), pageType()); err != nil {
		t.Fatalf("registering the page type: %v", err)
	}
	relabelled := postType()
	relabelled.SingularLabel, relabelled.PluralLabel = "3D Model", "3D Models"
	relabelled.UpdatedAt = time.Now().UTC()
	if _, err := types.Update(t.Context(), relabelled); err != nil {
		t.Fatalf("relabelling the default type: %v", err)
	}

	promoted := pageType()
	promoted.Default, promoted.UpdatedAt = true, time.Now().UTC()

	_, err := types.Update(t.Context(), promoted)

	if !errors.Is(err, content.ErrInvalidRouteWord) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrInvalidRouteWord)
	}
}

// closedTypeStore returns a type store whose pool is already closed.
func closedTypeStore(t *testing.T) *postgres.TypeStore {
	t.Helper()
	_, _, pool := newContentStoreWithPool(t)
	store := postgres.NewTypeStore(pool)
	pool.Close()
	return store
}

func TestTypeStoreReportsADatabaseItCannotReach(t *testing.T) {
	t.Parallel()

	store := closedTypeStore(t)
	field := content.Field{TypeKey: content.TypePost, Key: "color", Label: "Colour", Kind: content.FieldKindText}

	for name, run := range map[string]func() error{
		"listing the registry": func() error {
			_, err := store.List(t.Context())
			return err
		},
		"reading one type": func() error {
			_, err := store.ByKey(t.Context(), content.TypePost)
			return err
		},
		"editing a field": func() error {
			_, err := store.UpdateField(t.Context(), field)
			return err
		},
		"removing a field": func() error {
			return store.DeleteField(t.Context(), content.TypePost, "color")
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if err := run(); err == nil {
				t.Errorf("%s: error = nil, want the closed pool reported", name)
			}
		})
	}
}
