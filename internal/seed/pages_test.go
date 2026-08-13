// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// nestingTypeStore answers with the built-in post type and the demo page type.
type nestingTypeStore struct {
	stubTypeStore
	createErr error
	created   int
}

// List returns the post type and the page type the seeding registers.
func (s *nestingTypeStore) List(ctx context.Context) ([]content.Type, error) {
	types, err := s.stubTypeStore.List(ctx)
	if err != nil {
		return nil, err
	}
	if s.created == 0 {
		return types, nil
	}
	return append(types, PageType()), nil
}

// ByKey returns the type carrying the key, or reports it missing.
func (s *nestingTypeStore) ByKey(ctx context.Context, key string) (content.Type, error) {
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

// Create records the registration, or reports the scripted failure.
func (s *nestingTypeStore) Create(_ context.Context, t content.Type) (content.Type, error) {
	if s.createErr != nil {
		return content.Type{}, s.createErr
	}
	s.created++
	return t, nil
}

// nestingStore is a content store recording what the page seeding stored.
type nestingStore struct {
	content.Store
	held    map[uuid.UUID]content.Content
	created []content.Content
	byIDErr error
}

// newNestingStore returns a store holding nothing.
func newNestingStore() *nestingStore {
	return &nestingStore{held: make(map[uuid.UUID]content.Content)}
}

// ByID returns what the store holds, or the scripted failure.
func (s *nestingStore) ByID(_ context.Context, id uuid.UUID) (content.Content, error) {
	if s.byIDErr != nil {
		return content.Content{}, s.byIDErr
	}
	stored, found := s.held[id]
	if !found {
		return content.Content{}, content.ErrNotFound
	}
	return stored, nil
}

// Create records the stored page and hands it back.
func (s *nestingStore) Create(_ context.Context, c content.Content) (content.Content, error) {
	s.held[c.ID] = c
	s.created = append(s.created, c)
	return c, nil
}

func TestTypesRegistersThePageType(t *testing.T) {
	t.Parallel()

	store := &nestingTypeStore{}
	registry := content.NewRegistry(store)

	if err := Types(t.Context(), registry); err != nil {
		t.Fatalf("Types() error = %v, want nil", err)
	}

	if store.created != 1 {
		t.Fatalf("registered %d types, want the page type alone", store.created)
	}
	registered, err := registry.ByKey(t.Context(), PageTypeKey)
	if err != nil {
		t.Fatalf("ByKey() error = %v, want the registered page type", err)
	}
	if registered.RouteWord != "pages" || !registered.Hierarchical || registered.Default {
		t.Errorf("page type = %+v, want it nesting under pages without the root", registered)
	}
}

func TestTypesLeavesATypeItAlreadyRegistered(t *testing.T) {
	t.Parallel()

	store := &nestingTypeStore{created: 1}

	if err := Types(t.Context(), content.NewRegistry(store)); err != nil {
		t.Fatalf("Types() error = %v, want nil", err)
	}

	if store.created != 1 {
		t.Errorf("registered %d types, want the seeding to leave the stored one alone", store.created)
	}
}

func TestTypesReportsRegistryFailures(t *testing.T) {
	t.Parallel()

	tests := map[string]content.TypeStore{
		"lookup":   stubTypeStore{},
		"register": &nestingTypeStore{createErr: errStub},
	}

	for testName, store := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := Types(t.Context(), content.NewRegistry(store)); err == nil {
				t.Error("Types() error = nil, want a failure")
			}
		})
	}
}

func TestPagesStoresEveryScriptedPageUnderItsParent(t *testing.T) {
	t.Parallel()

	store := newNestingStore()

	err := Pages(t.Context(), store, content.NewRegistry(&nestingTypeStore{created: 1}), stubUserStore{id: uuid.New()})

	if err != nil {
		t.Fatalf("Pages() error = %v, want nil", err)
	}
	if len(store.created) != len(demoPages()) {
		t.Fatalf("stored %d pages, want %d", len(store.created), len(demoPages()))
	}
	if store.created[0].Path != "pages/about" {
		t.Errorf("the first page answers at %q, want %q", store.created[0].Path, "pages/about")
	}
	if store.created[1].Path != "pages/about/team" {
		t.Errorf("the nested page answers at %q, want %q", store.created[1].Path, "pages/about/team")
	}
	if store.created[1].ParentID == nil || *store.created[1].ParentID != store.created[0].ID {
		t.Errorf("the nested page hangs from %v, want %v", store.created[1].ParentID, store.created[0].ID)
	}
	for _, stored := range store.created {
		if stored.Status != content.StatusPublished {
			t.Errorf("%q is %q, want it published", stored.Title, stored.Status)
		}
	}
}

func TestPagesLeavesPagesItAlreadyStored(t *testing.T) {
	t.Parallel()

	store := newNestingStore()
	for _, scripted := range demoPages() {
		id := uuid.MustParse(scripted.id)
		store.held[id] = content.Content{ID: id}
	}

	err := Pages(t.Context(), store, content.NewRegistry(&nestingTypeStore{created: 1}), stubUserStore{id: uuid.New()})

	if err != nil {
		t.Fatalf("Pages() error = %v, want nil", err)
	}
	if len(store.created) != 0 {
		t.Errorf("stored %d pages, want none over a seeded database", len(store.created))
	}
}

func TestPagesReportsFailures(t *testing.T) {
	t.Parallel()

	registered := content.NewRegistry(&nestingTypeStore{created: 1})
	tests := map[string]struct {
		store content.Store
		types *content.Registry
		users stubUserStore
	}{
		"admin lookup": {store: newNestingStore(), types: registered, users: stubUserStore{err: errStub}},
		"type lookup":  {store: newNestingStore(), types: testRegistry(), users: stubUserStore{id: uuid.New()}},
		"page lookup": {
			store: &nestingStore{held: map[uuid.UUID]content.Content{}, byIDErr: errStub},
			types: registered,
			users: stubUserStore{id: uuid.New()},
		},
		"build": {store: newNestingStore(), types: registered, users: stubUserStore{id: uuid.Nil}},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := Pages(t.Context(), test.store, test.types, test.users); err == nil {
				t.Error("Pages() error = nil, want a failure")
			}
		})
	}
}

func TestStoreDemoPageReportsAMissingParent(t *testing.T) {
	t.Parallel()

	scripted := demoPage{
		id:       "019fb000-0000-7000-8000-0000000000ee",
		parentID: "019fb000-0000-7000-8000-0000000000ff",
		title:    "Orphan",
	}

	err := storeDemoPage(t.Context(), newNestingStore(), PageType(), scripted, uuid.New())

	if err == nil {
		t.Error("storeDemoPage() under a missing parent error = nil, want a failure")
	}
}

func TestTypesReportsARegistryItCouldNotRead(t *testing.T) {
	t.Parallel()

	err := Types(t.Context(), content.NewRegistry(failingTypeStore{}))

	if err == nil {
		t.Error("Types() over an unreadable registry error = nil, want a failure")
	}
}

func TestStoreDemoPageReportsAStorageFailure(t *testing.T) {
	t.Parallel()

	scripted := demoPage{id: "019fb000-0000-7000-8000-0000000000ee", title: "Refused"}

	err := storeDemoPage(t.Context(), stubPostStore{byIDErr: content.ErrNotFound, createErr: errStub},
		PageType(), scripted, uuid.New())

	if err == nil {
		t.Error("storeDemoPage() over a refusing store error = nil, want a failure")
	}
}

// failingTypeStore reports that the registry cannot be read at all.
type failingTypeStore struct{ content.TypeStore }

// List reports the registry unreadable.
func (failingTypeStore) List(context.Context) ([]content.Type, error) { return nil, errStub }
