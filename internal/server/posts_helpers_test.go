// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"context"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/authkit"
	"github.com/gopherium/gouncer/authkit/testkit"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/server"
)

var _ post.Store = (*fakePostStore)(nil)

// fakePostStore is an in-memory post store double with per-method error injection.
type fakePostStore struct {
	posts      map[uuid.UUID]post.Post
	lastFilter post.Filter
	createErr  error
	byIDErr    error
	listErr    error
	updateErr  error
	trashErr   error
	restoreErr error
	deleteErr  error
	countsErr  error

	revisions         []post.Revision
	revisionsErr      error
	revisionErr       error
	deleteRevisionErr error
	saveAutosaveErr   error
	autosaveErr       error
	deleteAutosaveErr error
	lastSnapshot      *post.Revision
	lastRevisionCap   int
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
	s.lastFilter = f
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

// Update stores the post's fields and any snapshot unless an update error is injected.
func (s *fakePostStore) Update(
	_ context.Context, p post.Post, expectedUpdatedAt time.Time, snapshot *post.Revision, revisionCap int,
) (post.Post, error) {
	if s.updateErr != nil {
		return post.Post{}, s.updateErr
	}
	existing, ok := s.posts[p.ID]
	if !ok {
		return post.Post{}, post.ErrNotFound
	}
	if !existing.UpdatedAt.Equal(expectedUpdatedAt) {
		return post.Post{}, post.ErrConflict
	}
	s.lastSnapshot, s.lastRevisionCap = snapshot, revisionCap
	if snapshot != nil {
		s.revisions = append(s.revisions, *snapshot)
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

// Revisions returns the post's stored revisions newest first, without content.
func (s *fakePostStore) Revisions(_ context.Context, postID uuid.UUID) ([]post.Revision, error) {
	if s.revisionsErr != nil {
		return nil, s.revisionsErr
	}
	stored := make([]post.Revision, 0, len(s.revisions))
	for _, r := range s.revisions {
		if r.PostID != postID {
			continue
		}
		r.Content = ""
		stored = append(stored, r)
	}
	slices.SortFunc(stored, func(a, b post.Revision) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return bytes.Compare(b.ID[:], a.ID[:])
	})
	return stored, nil
}

// RevisionByID returns the stored revision, or [post.ErrRevisionNotFound].
func (s *fakePostStore) RevisionByID(_ context.Context, postID, revisionID uuid.UUID) (post.Revision, error) {
	if s.revisionErr != nil {
		return post.Revision{}, s.revisionErr
	}
	for _, r := range s.revisions {
		if r.ID == revisionID && r.PostID == postID {
			return r, nil
		}
	}
	return post.Revision{}, post.ErrRevisionNotFound
}

// DeleteRevision removes the stored revision, or reports [post.ErrRevisionNotFound].
func (s *fakePostStore) DeleteRevision(_ context.Context, postID, revisionID uuid.UUID) error {
	if s.deleteRevisionErr != nil {
		return s.deleteRevisionErr
	}
	for i, r := range s.revisions {
		if r.ID == revisionID && r.PostID == postID {
			s.revisions = append(s.revisions[:i], s.revisions[i+1:]...)
			return nil
		}
	}
	return post.ErrRevisionNotFound
}

// SaveAutosave stores the author's autosave, replacing any earlier one.
func (s *fakePostStore) SaveAutosave(_ context.Context, autosave post.Revision) (post.Revision, error) {
	if s.saveAutosaveErr != nil {
		return post.Revision{}, s.saveAutosaveErr
	}
	for i, r := range s.revisions {
		if r.Kind == post.RevisionKindAutosave && r.PostID == autosave.PostID && r.AuthorID == autosave.AuthorID {
			autosave.ID = r.ID
			s.revisions[i] = autosave
			return autosave, nil
		}
	}
	s.revisions = append(s.revisions, autosave)
	return autosave, nil
}

// DeleteAutosave removes the author's autosave of the post.
func (s *fakePostStore) DeleteAutosave(_ context.Context, postID, authorID uuid.UUID) error {
	if s.deleteAutosaveErr != nil {
		return s.deleteAutosaveErr
	}
	for i, r := range s.revisions {
		if r.Kind == post.RevisionKindAutosave && r.PostID == postID && r.AuthorID == authorID {
			s.revisions = append(s.revisions[:i], s.revisions[i+1:]...)
			return nil
		}
	}
	return nil
}

// Autosave returns the author's autosave, or [post.ErrRevisionNotFound].
func (s *fakePostStore) Autosave(_ context.Context, postID, authorID uuid.UUID) (post.Revision, error) {
	if s.autosaveErr != nil {
		return post.Revision{}, s.autosaveErr
	}
	for _, r := range s.revisions {
		if r.Kind == post.RevisionKindAutosave && r.PostID == postID && r.AuthorID == authorID {
			return r, nil
		}
	}
	return post.Revision{}, post.ErrRevisionNotFound
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
	handler := server.NewServer(serverConfig(users, posts))
	cookie := loginCookie(t, handler)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
	return authed, posts, ada
}

// serverConfig returns a server config over the given stores.
func serverConfig(users authkit.AdminStore, posts post.Store) server.Config {
	return server.Config{Users: users, Posts: posts}
}

// serverWithStores returns an unauthenticated handler over the given stores.
func serverWithStores(users authkit.AdminStore, posts post.Store) http.Handler {
	return server.NewServer(serverConfig(users, posts))
}

// failingUserStore is a user store whose listing always fails.
type failingUserStore struct {
	*testkit.Store
}

// ListUsers reports a failure.
func (failingUserStore) ListUsers(_ context.Context) ([]gouncer.User, error) {
	return nil, context.DeadlineExceeded
}
