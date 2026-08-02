// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gouncer"

	"github.com/gopherium/gophenberg/internal/post"
)

func TestPostsReportsStoreFailures(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		store post.Store
		users gouncer.Store
	}{
		"admin lookup": {store: stubPostStore{}, users: stubUserStore{err: errStub}},
		"post lookup":  {store: stubPostStore{byIDErr: errStub}, users: stubUserStore{}},
		"build":        {store: stubPostStore{byIDErr: post.ErrNotFound}, users: stubUserStore{id: uuid.Nil}},
		"create": {
			store: stubPostStore{byIDErr: post.ErrNotFound, createErr: errStub},
			users: stubUserStore{id: uuid.New()},
		},
		"trash": {
			store: stubPostStore{byIDErr: post.ErrNotFound, trashErr: errStub},
			users: stubUserStore{id: uuid.New()},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if err := Posts(t.Context(), test.store, test.users); err == nil {
				t.Error("Posts() error = nil, want a failure")
			}
		})
	}
}

func TestPostsStoresEveryScriptedPost(t *testing.T) {
	t.Parallel()

	store := &countingPostStore{}

	if err := Posts(t.Context(), store, stubUserStore{id: uuid.New()}); err != nil {
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

	if err := Posts(t.Context(), store, stubUserStore{id: uuid.New()}); err != nil {
		t.Fatalf("Posts() error = %v, want nil", err)
	}

	if store.created != 0 {
		t.Errorf("created %d posts, want none over a seeded database", store.created)
	}
}

func TestStoreDemoPostRejectsAnUnknownStatus(t *testing.T) {
	t.Parallel()

	scripted := demoPost{title: "Unknown", status: post.Status("nonsense")}

	err := storeDemoPost(t.Context(), stubPostStore{}, scripted, uuid.New(), uuid.New())

	if err == nil {
		t.Error("storeDemoPost() with an unknown status error = nil, want a failure")
	}
}

// errStub is the failure reported by the seeding stubs.
var errStub = errors.New("stub failure")

// countingPostStore is a post store counting what the seeding stored.
type countingPostStore struct {
	post.Store
	found   bool
	created int
	trashed int
}

// ByID reports whether the post was already stored.
func (s *countingPostStore) ByID(_ context.Context, _ uuid.UUID) (post.Post, error) {
	if s.found {
		return post.Post{}, nil
	}
	return post.Post{}, post.ErrNotFound
}

// Create counts the post as stored.
func (s *countingPostStore) Create(_ context.Context, p post.Post) (post.Post, error) {
	s.created++
	return p, nil
}

// Trash counts the post as trashed.
func (s *countingPostStore) Trash(_ context.Context, _ uuid.UUID, _ time.Time) (post.Post, error) {
	s.trashed++
	return post.Post{}, nil
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
	post.Store
	byIDErr   error
	createErr error
	trashErr  error
}

// ByID reports the scripted lookup failure.
func (s stubPostStore) ByID(_ context.Context, _ uuid.UUID) (post.Post, error) {
	return post.Post{}, s.byIDErr
}

// Create reports the scripted storage failure.
func (s stubPostStore) Create(_ context.Context, p post.Post) (post.Post, error) {
	return p, s.createErr
}

// Trash reports the scripted trashing failure.
func (s stubPostStore) Trash(_ context.Context, _ uuid.UUID, _ time.Time) (post.Post, error) {
	return post.Post{}, s.trashErr
}
