// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/content"
)

// testRegistry returns a registry holding the built-in post type.
func testRegistry() *content.Registry {
	return content.NewRegistry(stubTypeStore{})
}

// stubTypeStore answers with the built-in post type alone.
type stubTypeStore struct{}

// List returns the built-in post type.
func (stubTypeStore) List(context.Context) ([]content.Type, error) {
	return []content.Type{{
		Key: content.TypePost, SingularLabel: "Post", PluralLabel: "Posts",
		Revisions: true, RevisionCap: 100, PageKind: content.PageKindSingle,
		Default: true, Active: true,
	}}, nil
}

// ByKey returns the built-in post type, or reports the key missing.
func (s stubTypeStore) ByKey(ctx context.Context, key string) (content.Type, error) {
	types, _ := s.List(ctx)
	if key == content.TypePost {
		return types[0], nil
	}
	return content.Type{}, content.ErrTypeNotFound
}

// Create refuses to register a type in a seeding run.
func (stubTypeStore) Create(context.Context, content.Type) (content.Type, error) {
	return content.Type{}, content.ErrTypeTaken
}

// Update refuses to edit a type in a seeding run.
func (stubTypeStore) Update(context.Context, content.Type) (content.Type, error) {
	return content.Type{}, content.ErrTypeNotFound
}

// Delete refuses to remove a type in a seeding run.
func (stubTypeStore) Delete(context.Context, string) error { return content.ErrTypeNotFound }

func TestPostsReportsStoreFailures(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		store content.Store
		users gouncer.Store
	}{
		"admin lookup": {store: stubPostStore{}, users: stubUserStore{err: errStub}},
		"post lookup":  {store: stubPostStore{byIDErr: errStub}, users: stubUserStore{}},
		"build":        {store: stubPostStore{byIDErr: content.ErrNotFound}, users: stubUserStore{id: uuid.Nil}},
		"create": {
			store: stubPostStore{byIDErr: content.ErrNotFound, createErr: errStub},
			users: stubUserStore{id: uuid.New()},
		},
		"trash": {
			store: stubPostStore{byIDErr: content.ErrNotFound, trashErr: errStub},
			users: stubUserStore{id: uuid.New()},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := Posts(t.Context(), test.store, testRegistry(), test.users); err == nil {
				t.Error("Posts() error = nil, want a failure")
			}
		})
	}
}

func TestPostsStoresEveryScriptedPost(t *testing.T) {
	t.Parallel()

	store := &countingPostStore{}

	if err := Posts(t.Context(), store, testRegistry(), stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Posts() error = %v, want nil", err)
	}

	if store.created != len(demoPosts()) {
		t.Errorf("created %d posts, want %d", store.created, len(demoPosts()))
	}
	if store.trashed != 1 {
		t.Errorf("trashed %d posts, want 1", store.trashed)
	}
}

func TestPostsLeavesPostsItAlreadyStored(t *testing.T) {
	t.Parallel()

	store := &countingPostStore{found: true}

	if err := Posts(t.Context(), store, testRegistry(), stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Posts() error = %v, want nil", err)
	}

	if store.created != 0 {
		t.Errorf("created %d posts, want none over a seeded database", store.created)
	}
}

func TestStoreDemoPostRejectsAnUnknownStatus(t *testing.T) {
	t.Parallel()

	scripted := demoPost{title: "Unknown", status: content.Status("nonsense")}

	postType, err := testRegistry().ByKey(t.Context(), content.TypePost)
	if err != nil {
		t.Fatalf("ByKey() error = %v, want nil", err)
	}

	err = storeDemoPost(t.Context(), stubPostStore{}, postType, scripted, uuid.New(), uuid.New())

	if err == nil {
		t.Error("storeDemoPost() with an unknown status error = nil, want a failure")
	}
}

// errStub is the failure reported by the seeding stubs.
var errStub = errors.New("stub failure")

// countingPostStore is a post store counting what the seeding stored.
type countingPostStore struct {
	content.Store
	found   bool
	created int
	trashed int
}

// ByID reports whether the post was already stored.
func (s *countingPostStore) ByID(_ context.Context, _ uuid.UUID) (content.Content, error) {
	if s.found {
		return content.Content{}, nil
	}
	return content.Content{}, content.ErrNotFound
}

// Create counts the post as stored.
func (s *countingPostStore) Create(_ context.Context, p content.Content) (content.Content, error) {
	s.created++
	return p, nil
}

// Trash counts the post as trashed.
func (s *countingPostStore) Trash(_ context.Context, _ uuid.UUID, _ time.Time) (content.Content, error) {
	s.trashed++
	return content.Content{}, nil
}

// stubUserStore is a user store returning a scripted admin.
type stubUserStore struct {
	gouncer.Store
	id  uuid.UUID
	err error
}

// UserByEmail returns the scripted admin.
func (s stubUserStore) UserByEmail(_ context.Context, _ string) (gouncer.User, error) {
	return gouncer.User{ID: s.id}, s.err
}

// stubPostStore is a post store reporting scripted failures.
type stubPostStore struct {
	content.Store
	byIDErr   error
	createErr error
	trashErr  error
}

// ByID reports the scripted lookup failure.
func (s stubPostStore) ByID(_ context.Context, _ uuid.UUID) (content.Content, error) {
	return content.Content{}, s.byIDErr
}

// Create reports the scripted storage failure.
func (s stubPostStore) Create(_ context.Context, p content.Content) (content.Content, error) {
	return p, s.createErr
}

// Trash reports the scripted trashing failure.
func (s stubPostStore) Trash(_ context.Context, _ uuid.UUID, _ time.Time) (content.Content, error) {
	return content.Content{}, s.trashErr
}
