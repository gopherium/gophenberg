// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/authkit/testkit"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/server"
)

var _ post.Store = (*fakePostStore)(nil)

// fakePostStore is an in-memory post store double with per-method error injection.
type fakePostStore struct {
	posts      map[uuid.UUID]post.Post
	createErr  error
	byIDErr    error
	listErr    error
	updateErr  error
	trashErr   error
	restoreErr error
	deleteErr  error
	countsErr  error
}

// newFakePostStore returns an empty in-memory post store double.
func newFakePostStore() *fakePostStore {
	return &fakePostStore{posts: map[uuid.UUID]post.Post{}}
}

// add stores p directly.
func (s *fakePostStore) add(p post.Post) post.Post {
	s.posts[p.ID] = p
	return p
}

// ordered returns the stored posts newest first.
func (s *fakePostStore) ordered() []post.Post {
	posts := make([]post.Post, 0, len(s.posts))
	for _, p := range s.posts {
		posts = append(posts, p)
	}
	slices.SortFunc(posts, func(a, b post.Post) int {
		return strings.Compare(b.ID.String(), a.ID.String())
	})
	return posts
}

// Create stores p unless a create error is injected.
func (s *fakePostStore) Create(_ context.Context, p post.Post) (post.Post, error) {
	if s.createErr != nil {
		return post.Post{}, s.createErr
	}
	return s.add(p), nil
}

// ByID returns the stored post, or [post.ErrNotFound].
func (s *fakePostStore) ByID(_ context.Context, id uuid.UUID) (post.Post, error) {
	if s.byIDErr != nil {
		return post.Post{}, s.byIDErr
	}
	p, ok := s.posts[id]
	if !ok {
		return post.Post{}, post.ErrNotFound
	}
	return p, nil
}

// List returns the stored posts matching the filter's status and search.
func (s *fakePostStore) List(_ context.Context, f post.Filter) ([]post.Post, int, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	matched := make([]post.Post, 0, len(s.posts))
	for _, p := range s.ordered() {
		if f.Status != "" && p.Status != f.Status {
			continue
		}
		if f.Search != "" && !strings.Contains(strings.ToLower(p.Title), strings.ToLower(f.Search)) {
			continue
		}
		p.Content = ""
		matched = append(matched, p)
	}
	total := len(matched)
	start := min((f.Page-1)*f.PerPage, total)
	return matched[start:min(start+f.PerPage, total)], total, nil
}

// Update stores the post's fields unless an update error is injected.
func (s *fakePostStore) Update(_ context.Context, p post.Post, _ *post.Revision, _ int) (post.Post, error) {
	if s.updateErr != nil {
		return post.Post{}, s.updateErr
	}
	if _, ok := s.posts[p.ID]; !ok {
		return post.Post{}, post.ErrNotFound
	}
	return s.add(p), nil
}

// Trash marks the stored post trashed.
func (s *fakePostStore) Trash(_ context.Context, id uuid.UUID, updatedAt time.Time) (post.Post, error) {
	if s.trashErr != nil {
		return post.Post{}, s.trashErr
	}
	p, ok := s.posts[id]
	if !ok {
		return post.Post{}, post.ErrNotFound
	}
	p.Status = post.StatusTrash
	p.Slug += "-trashed-abcd1234"
	p.UpdatedAt = updatedAt
	return s.add(p), nil
}

// Restore returns the stored post to draft.
func (s *fakePostStore) Restore(_ context.Context, id uuid.UUID, updatedAt time.Time) (post.Post, error) {
	if s.restoreErr != nil {
		return post.Post{}, s.restoreErr
	}
	p, ok := s.posts[id]
	if !ok {
		return post.Post{}, post.ErrNotFound
	}
	p.Status = post.StatusDraft
	p.Slug = strings.TrimSuffix(p.Slug, "-trashed-abcd1234")
	p.UpdatedAt = updatedAt
	return s.add(p), nil
}

// Delete removes the stored post.
func (s *fakePostStore) Delete(_ context.Context, id uuid.UUID) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.posts[id]; !ok {
		return post.ErrNotFound
	}
	delete(s.posts, id)
	return nil
}

// Counts returns the number of stored posts in each status.
func (s *fakePostStore) Counts(_ context.Context, _ string) (map[post.Status]int, error) {
	if s.countsErr != nil {
		return nil, s.countsErr
	}
	counts := map[post.Status]int{}
	for _, p := range s.posts {
		counts[p.Status]++
	}
	return counts, nil
}

// newPost returns a draft post authored by author.
func newPost(t *testing.T, title string, author uuid.UUID) post.Post {
	t.Helper()
	p, err := post.New(post.TypePost, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	return p
}

// authedPostServer returns a handler that carries a session, the post store behind it, and the signed-in user.
func authedPostServer(t *testing.T) (http.Handler, *fakePostStore, gouncer.User) {
	t.Helper()
	users := newFakeUserStore()
	ada := addAda(t, users)
	posts := newFakePostStore()
	handler := server.NewServer(server.Config{Users: users, Posts: posts})
	cookie := loginCookie(t, handler)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
	return authed, posts, ada
}

// failingUserStore is a user store whose listing always fails.
type failingUserStore struct {
	*testkit.Store
}

// ListUsers reports a failure.
func (failingUserStore) ListUsers(_ context.Context) ([]gouncer.User, error) {
	return nil, context.DeadlineExceeded
}
