// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// errStoreDown stands for a registry the database cannot answer for.
var errStoreDown = errors.New("the registry is unreachable")

// fakeTypeStore holds content types in memory and counts what it was asked to read.
type fakeTypeStore struct {
	types     []content.Type
	fieldIDs  int
	listCalls int
	listErr   error
	createErr error
	updateErr error
	deleteErr error

	createFieldErr  error
	updateFieldErr  error
	deleteFieldErr  error
	reorderFieldErr error
}

// newFakeTypeStore returns a store holding the built-in post type.
func newFakeTypeStore() *fakeTypeStore {
	return &fakeTypeStore{types: []content.Type{postType()}}
}

// List returns every stored type in registration order.
func (s *fakeTypeStore) List(context.Context) ([]content.Type, error) {
	s.listCalls++
	if s.listErr != nil {
		return nil, s.listErr
	}
	stored := make([]content.Type, len(s.types))
	copy(stored, s.types)
	return stored, nil
}

// ByKey returns the stored type carrying the key.
func (s *fakeTypeStore) ByKey(_ context.Context, key string) (content.Type, error) {
	for _, t := range s.types {
		if t.Key == key {
			return t, nil
		}
	}
	return content.Type{}, content.ErrTypeNotFound
}

// Create stores a new type.
func (s *fakeTypeStore) Create(_ context.Context, t content.Type) (content.Type, error) {
	if s.createErr != nil {
		return content.Type{}, s.createErr
	}
	s.types = append(s.types, t)
	return t, nil
}

// Update stores the edited type.
func (s *fakeTypeStore) Update(_ context.Context, t content.Type) (content.Type, error) {
	if s.updateErr != nil {
		return content.Type{}, s.updateErr
	}
	for i, stored := range s.types {
		if stored.Key == t.Key {
			s.types[i] = t
			return t, nil
		}
	}
	return content.Type{}, content.ErrTypeNotFound
}

// Delete removes the type carrying the key.
func (s *fakeTypeStore) Delete(_ context.Context, key string) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	for i, stored := range s.types {
		if stored.Key == key {
			s.types = append(s.types[:i], s.types[i+1:]...)
			return nil
		}
	}
	return content.ErrTypeNotFound
}

// carType returns a car content type ready to register.
func carType(t *testing.T) content.Type {
	t.Helper()
	built, err := content.NewType("car", "Car", "Cars", "cars")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	return built
}

func TestRegistryReadsThroughOnceAndCaches(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	registry := content.NewRegistry(store)

	for range 3 {
		if _, err := registry.ByKey(t.Context(), content.TypePost); err != nil {
			t.Fatalf("ByKey() error = %v, want nil", err)
		}
	}

	if store.listCalls != 1 {
		t.Errorf("the store was read %d times, want the registry cached after the first", store.listCalls)
	}
}

func TestRegistryRereadsAfterAWrite(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	registry := content.NewRegistry(store)
	if _, err := registry.All(t.Context()); err != nil {
		t.Fatalf("All() error = %v, want nil", err)
	}

	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	found, err := registry.ByKey(t.Context(), "car")
	if err != nil {
		t.Fatalf("ByKey() after Create error = %v, want nil", err)
	}
	if found.Key != "car" || store.listCalls != 2 {
		t.Errorf("ByKey() = %+v after %d reads, want the write to have refreshed the cache",
			found, store.listCalls)
	}
}

func TestRegistryReportsAMissingType(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	_, err := registry.ByKey(t.Context(), "never-registered")

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("ByKey() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestRegistryAnswersTheDefaultType(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	found, err := registry.Default(t.Context())

	if err != nil {
		t.Fatalf("Default() error = %v, want nil", err)
	}
	if found.Key != content.TypePost || found.RouteWord != "" {
		t.Errorf("Default() = %+v, want the post type at the root", found)
	}
}

func TestRegistryHidesAnInactiveType(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	registry := content.NewRegistry(store)
	car := carType(t)
	car.Active = false
	store.types = append(store.types, car)

	_, active := registry.Active(t.Context(), "car")
	found, err := registry.ByKey(t.Context(), "car")

	if !errors.Is(active, content.ErrTypeInactive) {
		t.Errorf("Active() error = %v, want %v", active, content.ErrTypeInactive)
	}
	if err != nil || found.Key != "car" {
		t.Errorf("ByKey() = %+v, %v, want the inactive type still readable", found, err)
	}
}

func TestRegistryRefusesATakenKeyOrRouteWord(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	sameWord, err := content.NewType("van", "Van", "Vans", "cars")
	if err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}

	_, takenKey := registry.Create(t.Context(), carType(t))
	_, takenWord := registry.Create(t.Context(), sameWord)

	if !errors.Is(takenKey, content.ErrTypeTaken) {
		t.Errorf("Create() error = %v, want %v", takenKey, content.ErrTypeTaken)
	}
	if !errors.Is(takenWord, content.ErrRouteWordTaken) {
		t.Errorf("Create() error = %v, want %v", takenWord, content.ErrRouteWordTaken)
	}
}

func TestRegistryRefusesAMisshapenRouteWordOrPageKind(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	shouted := carType(t)
	shouted.RouteWord = "Cars"
	unknownKind := carType(t)
	unknownKind.PageKind = content.PageKind("gallery")

	_, word := registry.Create(t.Context(), shouted)
	_, kind := registry.Create(t.Context(), unknownKind)

	if !errors.Is(word, content.ErrInvalidRouteWord) {
		t.Errorf("Create() error = %v, want %v", word, content.ErrInvalidRouteWord)
	}
	if !errors.Is(kind, content.ErrInvalidPageKind) {
		t.Errorf("Create() error = %v, want %v", kind, content.ErrInvalidPageKind)
	}
}

func TestRegistryReportsAnEmptyRegistryHasNoDefault(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	store.types = nil
	registry := content.NewRegistry(store)

	_, err := registry.Default(t.Context())

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("Default() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestRegistryActiveSurfacesAMissingType(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	_, err := registry.Active(t.Context(), "never-registered")

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("Active() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestRegistryDeleteReportsAMissingType(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	err := registry.Delete(t.Context(), "never-registered")

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("Delete() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestRegistryUpdateReportsAMissingType(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	_, err := registry.Update(t.Context(), carType(t))

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestRegistryRefusesAReservedRouteWord(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	for _, reserved := range content.ReservedRouteWords {
		_, err := content.NewType("car", "Car", "Cars", reserved)

		if !errors.Is(err, content.ErrRouteWordReserved) {
			t.Errorf("NewType(%q) error = %v, want %v", reserved, err, content.ErrRouteWordReserved)
		}
	}
	if _, err := registry.All(t.Context()); err != nil {
		t.Fatalf("All() error = %v, want nil", err)
	}
}

func TestRegistryKeepsTheRootForOneType(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	registry := content.NewRegistry(store)
	car := carType(t)
	if _, err := registry.Create(t.Context(), car); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	claimed := car
	claimed.RouteWord = ""

	_, err := registry.Update(t.Context(), claimed)

	if !errors.Is(err, content.ErrRootTaken) {
		t.Errorf("Update() error = %v, want %v", err, content.ErrRootTaken)
	}
}

func TestRegistryKeepsTheDefaultTypeServing(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	post, err := registry.ByKey(t.Context(), content.TypePost)
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	deactivated := post
	deactivated.Active = false
	demoted := post
	demoted.Default, demoted.RouteWord = false, "blog"

	_, off := registry.Update(t.Context(), deactivated)
	_, down := registry.Update(t.Context(), demoted)
	removal := registry.Delete(t.Context(), content.TypePost)

	for name, err := range map[string]error{"deactivate": off, "demote": down, "delete": removal} {
		if !errors.Is(err, content.ErrDefaultRequired) {
			t.Errorf("%s the default type error = %v, want %v", name, err, content.ErrDefaultRequired)
		}
	}
}

func TestRegistryLetsTheDefaultLeaveTheRootWhileStayingDefault(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	post, err := registry.ByKey(t.Context(), content.TypePost)
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}
	moved := post
	moved.RouteWord = "blog"

	updated, err := registry.Update(t.Context(), moved)

	if err != nil {
		t.Fatalf("Update() error = %v, want the first act of a transfer allowed", err)
	}
	if !updated.Default || updated.RouteWord != "blog" {
		t.Errorf("Update() = %+v, want the default type answering under blog", updated)
	}
}

func TestRegistryRegistersAnArchivePageKind(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	car := carType(t)
	car.PageKind = content.PageKindArchive

	created, err := registry.Create(t.Context(), car)

	if err != nil {
		t.Fatalf("Create() error = %v, want the archive page kind registered", err)
	}
	if created.PageKind != content.PageKindArchive {
		t.Errorf("Create() page kind = %q, want %q", created.PageKind, content.PageKindArchive)
	}
}

func TestRegistryRefusesAnUnstorableType(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())
	nameless := carType(t)
	nameless.Key = "Not A Key"
	unlabeled := carType(t)
	unlabeled.PluralLabel = ""
	uncapped := carType(t)
	uncapped.RevisionCap = -1

	_, key := registry.Create(t.Context(), nameless)
	_, label := registry.Create(t.Context(), unlabeled)
	_, cap := registry.Update(t.Context(), uncapped)

	for want, err := range map[error]error{
		content.ErrInvalidKey:         key,
		content.ErrInvalidLabel:       label,
		content.ErrInvalidRevisionCap: cap,
	} {
		if !errors.Is(err, want) {
			t.Errorf("registry error = %v, want %v", err, want)
		}
	}
}

func TestRegistrySurfacesStoreFailures(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	store.listErr = errStoreDown
	registry := content.NewRegistry(store)

	_, all := registry.All(t.Context())
	_, byKey := registry.ByKey(t.Context(), content.TypePost)
	_, byDefault := registry.Default(t.Context())
	_, created := registry.Create(t.Context(), carType(t))
	_, updated := registry.Update(t.Context(), carType(t))
	deleted := registry.Delete(t.Context(), content.TypePost)

	for name, err := range map[string]error{
		"All": all, "ByKey": byKey, "Default": byDefault,
		"Create": created, "Update": updated, "Delete": deleted,
	} {
		if !errors.Is(err, errStoreDown) {
			t.Errorf("%s() error = %v, want %v", name, err, errStoreDown)
		}
	}
}

func TestRegistrySurfacesWriteFailures(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	store.createErr, store.updateErr, store.deleteErr = errStoreDown, errStoreDown, errStoreDown
	registry := content.NewRegistry(store)
	car := carType(t)
	if _, err := content.NewType("van", "Van", "Vans", "vans"); err != nil {
		t.Fatalf("NewType() error = %v, want nil", err)
	}
	store.types = append(store.types, car)

	_, created := registry.Create(t.Context(), content.Type{
		Key: "van", SingularLabel: "Van", PluralLabel: "Vans", RouteWord: "vans",
		PageKind: content.PageKindSingle, Active: true, CreatedAt: time.Now().UTC(),
	})
	_, updated := registry.Update(t.Context(), car)
	deleted := registry.Delete(t.Context(), "car")

	for name, err := range map[string]error{"Create": created, "Update": updated, "Delete": deleted} {
		if !errors.Is(err, errStoreDown) {
			t.Errorf("%s() error = %v, want %v", name, err, errStoreDown)
		}
	}
}

func TestRegistryAnswersTheTypeUnderARouteWord(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	store.types = append(store.types, pageType())
	registry := content.NewRegistry(store)

	page, err := registry.ByRouteWord(t.Context(), "pages")
	if err != nil {
		t.Fatalf("ByRouteWord(%q) error = %v, want nil", "pages", err)
	}
	root, err := registry.ByRouteWord(t.Context(), "")
	if err != nil {
		t.Fatalf("ByRouteWord(%q) error = %v, want the default type", "", err)
	}

	if page.Key != "page" {
		t.Errorf("key = %q, want the type answering under the word", page.Key)
	}
	if root.Key != content.TypePost || !root.Default {
		t.Errorf("root key = %q default = %v, want the default type at the root", root.Key, root.Default)
	}
}

func TestRegistryAnswersNoTypeForAnUnusedWord(t *testing.T) {
	t.Parallel()

	registry := content.NewRegistry(newFakeTypeStore())

	_, err := registry.ByRouteWord(t.Context(), "nowhere")

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Fatalf("ByRouteWord error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

// blockingTypeStore holds one read inside List until the test releases it.
type blockingTypeStore struct {
	*fakeTypeStore
	held    chan struct{}
	blocked chan struct{}
	once    bool
}

// List reports the stored types as they were, holding the first read until the test releases it.
func (s *blockingTypeStore) List(ctx context.Context) ([]content.Type, error) {
	types, err := s.fakeTypeStore.List(ctx)
	if !s.once {
		s.once = true
		close(s.blocked)
		<-s.held
	}
	return types, err
}

func TestRegistryKeepsAWriteThatLandsDuringARead(t *testing.T) {
	t.Parallel()

	store := &blockingTypeStore{
		fakeTypeStore: newFakeTypeStore(),
		held:          make(chan struct{}),
		blocked:       make(chan struct{}),
	}
	registry := content.NewRegistry(store)
	reads := make(chan []content.Type, 1)
	go func() {
		types, err := registry.All(t.Context())
		if err != nil {
			t.Errorf("All() error = %v, want nil", err)
		}
		reads <- types
	}()
	<-store.blocked

	if _, err := registry.Create(t.Context(), carType(t)); err != nil {
		t.Fatalf("Create() during a read error = %v, want nil", err)
	}
	close(store.held)
	during := <-reads

	if len(during) == 0 {
		t.Fatal("All() during a write answered with an empty registry, want the types it holds")
	}
	if _, err := registry.ByKey(t.Context(), "car"); err != nil {
		t.Errorf("ByKey() after a write that landed during a read error = %v, want the stored type", err)
	}
}

func TestRegistryHidesAnInactiveTypeFromItsRouteWord(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	closed := pageType()
	closed.Active = false
	store.types = append(store.types, closed)
	registry := content.NewRegistry(store)

	_, err := registry.ByRouteWord(t.Context(), "pages")

	if !errors.Is(err, content.ErrTypeNotFound) {
		t.Fatalf("ByRouteWord on an inactive type error = %v, want %v", err, content.ErrTypeNotFound)
	}
}

func TestRegistryHandsTheRootToAnotherType(t *testing.T) {
	t.Parallel()

	store := newFakeTypeStore()
	store.types = append(store.types, pageType())
	registry := content.NewRegistry(store)

	promoted := pageType()
	promoted.Default = true

	if _, err := registry.Update(t.Context(), promoted); err != nil {
		t.Fatalf("Update() handing over the root error = %v, want nil", err)
	}
}
