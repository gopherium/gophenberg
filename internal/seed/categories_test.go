// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// categoryTypeStore answers with the post type, and the category type once it is registered.
type categoryTypeStore struct {
	stubTypeStore
	createErr      error
	createFieldErr error
	listErr        error
	postless       bool
	forgetful      bool
	registered     bool
	fielded        bool
}

// List returns the post type, carrying the categories field once it is declared.
func (s *categoryTypeStore) List(ctx context.Context) ([]content.Type, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	types, err := s.stubTypeStore.List(ctx)
	if err != nil {
		return nil, err
	}
	if s.postless {
		types = nil
	}
	if s.fielded && len(types) > 0 {
		types[0].Fields = []content.Field{CategoriesField()}
	}
	if s.registered {
		types = append(types, CategoryType())
	}
	return types, nil
}

// ByKey returns the type carrying the key, or reports it missing.
func (s *categoryTypeStore) ByKey(ctx context.Context, key string) (content.Type, error) {
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
func (s *categoryTypeStore) Create(_ context.Context, t content.Type) (content.Type, error) {
	if s.createErr != nil {
		return content.Type{}, s.createErr
	}
	s.registered = !s.forgetful
	return t, nil
}

// CreateField records the declaration, or reports the scripted failure.
func (s *categoryTypeStore) CreateField(_ context.Context, f content.Field) (content.Field, error) {
	if s.createFieldErr != nil {
		return content.Field{}, s.createFieldErr
	}
	s.fielded = true
	return f, nil
}

// filingStore is a content store recording what the category seeding stored and filed.
type filingStore struct {
	content.Store
	held      map[uuid.UUID]content.Content
	created   []content.Content
	filed     []content.Content
	byIDErr   error
	createErr error
	updateErr error
}

// newFilingStore returns a store holding the post the demo files under a category.
func newFilingStore() *filingStore {
	post := content.Content{
		ID:       uuid.MustParse(demoCategories()[0].filed),
		Type:     content.TypePost,
		Title:    "A seeded post",
		AuthorID: uuid.New(),
	}
	return &filingStore{held: map[uuid.UUID]content.Content{post.ID: post}}
}

// ByID returns what the store holds, or the scripted failure.
func (s *filingStore) ByID(_ context.Context, id uuid.UUID) (content.Content, error) {
	if s.byIDErr != nil {
		return content.Content{}, s.byIDErr
	}
	held, found := s.held[id]
	if !found {
		return content.Content{}, content.ErrNotFound
	}
	return held, nil
}

// Create records what the seeding stored, or reports the scripted failure.
func (s *filingStore) Create(_ context.Context, c content.Content) (content.Content, error) {
	if s.createErr != nil {
		return content.Content{}, s.createErr
	}
	s.created = append(s.created, c)
	s.held[c.ID] = c
	return c, nil
}

// Update records what the seeding filed, or reports the scripted failure.
func (s *filingStore) Update(
	_ context.Context, c content.Content, _ time.Time, _ *content.Revision, _ int,
) (content.Content, error) {
	if s.updateErr != nil {
		return content.Content{}, s.updateErr
	}
	s.filed = append(s.filed, c)
	s.held[c.ID] = c
	return c, nil
}

func TestTheCategoryTypeServesTermPages(t *testing.T) {
	t.Parallel()

	held := CategoryType()

	if held.PageKind != content.PageKindArchive {
		t.Errorf("the category type serves %q pages, want %q", held.PageKind, content.PageKindArchive)
	}
	if held.RouteWord != "categories" || !held.Hierarchical || !held.Active {
		t.Errorf("the category type = %+v, want it nesting under categories and serving", held)
	}
	if err := held.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want the seeded type storable", err)
	}
}

func TestTheCategoriesFieldPointsPostsAtCategories(t *testing.T) {
	t.Parallel()

	held := CategoriesField()

	if held.TypeKey != content.TypePost || held.RelatesTo != CategoryTypeKey {
		t.Errorf("the field = %+v, want posts pointing at categories", held)
	}
	if held.Kind != content.FieldKindRelation || !held.Many {
		t.Errorf("the field = %+v, want a relation holding many", held)
	}
	if err := held.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want the seeded field storable", err)
	}
}

func TestTheCategoriesFieldCarriesInstructions(t *testing.T) {
	t.Parallel()

	held := CategoriesField()

	if written, ok := held.Settings["instructions"].(string); !ok || written == "" {
		t.Errorf("settings = %v, want the demo to show what instructions look like", held.Settings)
	}
}

func TestTheSeededCategoryCarriesAFixedIdentity(t *testing.T) {
	t.Parallel()

	held := demoCategories()

	if len(held) != 1 {
		t.Fatalf("the seed scripts %d categories, want the one the demo files under", len(held))
	}
	if held[0].title != "News" {
		t.Errorf("the seeded category is %q, want News", held[0].title)
	}
	if held[0].id == "" || held[0].filed == "" {
		t.Errorf("the seeded category = %+v, want a fixed identity and a post to file", held[0])
	}
}

// categoryRegistry returns a registry over a store that has not registered the category type yet.
func categoryRegistry() (*content.Registry, *categoryTypeStore) {
	types := &categoryTypeStore{}
	return content.NewRegistry(types), types
}

func TestCategoriesRegistersTheTypeItsFieldAndTheDemoCategory(t *testing.T) {
	t.Parallel()

	store := newFilingStore()
	registry, types := categoryRegistry()

	if err := Categories(t.Context(), store, registry, stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Categories() error = %v, want nil", err)
	}

	if !types.registered {
		t.Error("the category type was never registered")
	}
	if !types.fielded {
		t.Error("the categories field was never declared")
	}
	if len(store.created) != 1 {
		t.Fatalf("stored %d categories, want the one the demo scripts", len(store.created))
	}
	if store.created[0].Status != content.StatusPublished {
		t.Errorf("the seeded category is %q, want it published", store.created[0].Status)
	}
}

func TestCategoriesFilesTheSeededPostUnderTheCategory(t *testing.T) {
	t.Parallel()

	store := newFilingStore()
	registry, _ := categoryRegistry()

	if err := Categories(t.Context(), store, registry, stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Categories() error = %v, want nil", err)
	}

	if len(store.filed) != 1 {
		t.Fatalf("filed %d posts, want the one the demo files", len(store.filed))
	}
	held := store.filed[0].Relations[CategoriesFieldKey]
	if len(held) != 1 || held[0] != uuid.MustParse(demoCategories()[0].id) {
		t.Errorf("the post points at %v, want the seeded category", held)
	}
}

func TestCategoriesIsSafeToRunTwice(t *testing.T) {
	t.Parallel()

	store := newFilingStore()
	registry, _ := categoryRegistry()
	if err := Categories(t.Context(), store, registry, stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("the first Categories() error = %v, want nil", err)
	}

	if err := Categories(t.Context(), store, registry, stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("the second Categories() error = %v, want nil", err)
	}

	if len(store.created) != 1 {
		t.Errorf("stored %d categories over two runs, want the one the demo scripts", len(store.created))
	}
}

func TestCategoriesReportsFailures(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		store func() *filingStore
		types func() *content.Registry
		users stubUserStore
	}{
		"registering the type": {
			store: newFilingStore,
			types: func() *content.Registry { return content.NewRegistry(&categoryTypeStore{createErr: errStub}) },
			users: stubUserStore{id: uuid.New()},
		},
		"declaring the field": {
			store: newFilingStore,
			types: func() *content.Registry {
				return content.NewRegistry(&categoryTypeStore{createFieldErr: errStub})
			},
			users: stubUserStore{id: uuid.New()},
		},
		"reading the registry at all": {
			store: newFilingStore,
			types: func() *content.Registry { return content.NewRegistry(&categoryTypeStore{listErr: errStub}) },
			users: stubUserStore{id: uuid.New()},
		},
		"finding the post type": {
			store: newFilingStore,
			types: func() *content.Registry { return content.NewRegistry(&categoryTypeStore{postless: true}) },
			users: stubUserStore{id: uuid.New()},
		},
		"finding the type it just registered": {
			store: newFilingStore,
			types: func() *content.Registry {
				return content.NewRegistry(&categoryTypeStore{forgetful: true, fielded: true})
			},
			users: stubUserStore{id: uuid.New()},
		},
		"looking the admin up": {
			store: newFilingStore,
			types: func() *content.Registry { return content.NewRegistry(&categoryTypeStore{}) },
			users: stubUserStore{err: errStub},
		},
		"looking the category up": {
			store: func() *filingStore {
				held := newFilingStore()
				held.byIDErr = errStub
				return held
			},
			types: func() *content.Registry { return content.NewRegistry(&categoryTypeStore{}) },
			users: stubUserStore{id: uuid.New()},
		},
		"building the category": {
			store: newFilingStore,
			types: func() *content.Registry { return content.NewRegistry(&categoryTypeStore{}) },
			users: stubUserStore{id: uuid.Nil},
		},
		"storing the category": {
			store: func() *filingStore {
				held := newFilingStore()
				held.createErr = errStub
				return held
			},
			types: func() *content.Registry { return content.NewRegistry(&categoryTypeStore{}) },
			users: stubUserStore{id: uuid.New()},
		},
		"filing the post": {
			store: func() *filingStore {
				held := newFilingStore()
				held.updateErr = errStub
				return held
			},
			types: func() *content.Registry { return content.NewRegistry(&categoryTypeStore{}) },
			users: stubUserStore{id: uuid.New()},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := Categories(t.Context(), test.store(), test.types(), test.users); err == nil {
				t.Error("Categories() error = nil, want a failure")
			}
		})
	}
}

func TestCategoriesReportsAPostItCannotFile(t *testing.T) {
	t.Parallel()

	store := &filingStore{held: map[uuid.UUID]content.Content{}}
	registry, _ := categoryRegistry()

	if err := Categories(t.Context(), store, registry, stubUserStore{id: uuid.New()}); err == nil {
		t.Error("Categories() error = nil, want the missing post reported")
	}
}

func TestTheSeededCategoryFilesAPublishedPost(t *testing.T) {
	t.Parallel()

	published := make(map[string]bool, len(demoPosts()))
	for _, scripted := range publishedDemoPosts() {
		published[scripted.id] = true
	}

	for _, scripted := range demoCategories() {
		if !published[scripted.filed] {
			t.Errorf("the category %q files %q, want a post the demo publishes", scripted.title, scripted.filed)
		}
	}
}
