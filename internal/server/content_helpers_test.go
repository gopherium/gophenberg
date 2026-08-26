// SPDX-License-Identifier: Apache-2.0

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/gouncer"
	"github.com/gopherium/gouncer/authkit"
	"github.com/gopherium/gouncer/authkit/testkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/server"
)

var _ content.Store = (*fakePostStore)(nil)

// fakePostStore is an in-memory post store double with per-method error injection.
type fakePostStore struct {
	posts        map[uuid.UUID]content.Content
	lastFilter   content.Filter
	createErr    error
	byIDErr      error
	publishedErr error
	listErr      error
	updateErr    error
	trashErr     error
	restoreErr   error
	deleteErr    error
	countsErr    error
	childrenErr  error
	depthErr     error
	targetsErr   error
	relatedErr   error
	byIDErrFor   map[uuid.UUID]error

	revisions         []content.Revision
	revisionsErr      error
	revisionErr       error
	deleteRevisionErr error
	saveAutosaveErr   error
	autosaveErr       error
	deleteAutosaveErr error
	lastSnapshot      *content.Revision
	lastRevisionCap   int
}

// Depth returns how many levels of content nest below the item.
func (s *fakePostStore) Depth(ctx context.Context, id uuid.UUID) (int, error) {
	if s.depthErr != nil {
		return 0, s.depthErr
	}
	below := 0
	for _, stored := range s.posts {
		if stored.ParentID == nil || *stored.ParentID != id {
			continue
		}
		under, err := s.Depth(ctx, stored.ID)
		if err != nil {
			return 0, err
		}
		below = max(below, under+1)
	}
	return below, nil
}

// Children returns how many items nest under the item.
func (s *fakePostStore) Children(_ context.Context, id uuid.UUID) (int, error) {
	if s.childrenErr != nil {
		return 0, s.childrenErr
	}
	held := 0
	for _, stored := range s.posts {
		if stored.ParentID != nil && *stored.ParentID == id {
			held++
		}
	}
	return held, nil
}

// PublishedByPath returns the published item at the address, or [content.ErrNotFound].
func (s *fakePostStore) PublishedByPath(_ context.Context, path string) (content.Content, error) {
	if s.publishedErr != nil {
		return content.Content{}, s.publishedErr
	}
	for _, stored := range s.posts {
		if stored.Path == path && stored.Status == content.StatusPublished {
			return stored, nil
		}
	}
	return content.Content{}, content.ErrNotFound
}

// newFakePostStore returns an empty in-memory post store double.
func newFakePostStore() *fakePostStore {
	return &fakePostStore{posts: map[uuid.UUID]content.Content{}}
}

// add stores p directly.
func (s *fakePostStore) add(p content.Content) content.Content {
	s.posts[p.ID] = p
	return p
}

// ordered returns the stored posts newest first.
func (s *fakePostStore) ordered() []content.Content {
	posts := make([]content.Content, 0, len(s.posts))
	for _, p := range s.posts {
		posts = append(posts, p)
	}
	slices.SortFunc(posts, func(a, b content.Content) int {
		return strings.Compare(b.ID.String(), a.ID.String())
	})
	return posts
}

// Create stores p unless a create error is injected.
func (s *fakePostStore) Create(_ context.Context, p content.Content) (content.Content, error) {
	if s.createErr != nil {
		return content.Content{}, s.createErr
	}
	return s.add(p), nil
}

// ByID returns the stored post, or [content.ErrNotFound].
func (s *fakePostStore) ByID(_ context.Context, id uuid.UUID) (content.Content, error) {
	if held, found := s.byIDErrFor[id]; found {
		return content.Content{}, held
	}
	if s.byIDErr != nil {
		return content.Content{}, s.byIDErr
	}
	p, ok := s.posts[id]
	if !ok {
		return content.Content{}, content.ErrNotFound
	}
	return p, nil
}

// List returns the stored posts matching the filter's status and search.
func (s *fakePostStore) List(_ context.Context, f content.Filter) ([]content.Content, int, error) {
	s.lastFilter = f
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	matched := make([]content.Content, 0, len(s.posts))
	for _, p := range s.ordered() {
		if p.Type != f.Type {
			continue
		}
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

// RelatedTo returns the published items pointing at the target, newest first, and the total.
func (s *fakePostStore) RelatedTo(
	_ context.Context, target uuid.UUID, page, perPage int,
) ([]content.Content, int, error) {
	if s.relatedErr != nil {
		return nil, 0, s.relatedErr
	}
	matched := make([]content.Content, 0, len(s.posts))
	for _, p := range s.ordered() {
		if p.Status != content.StatusPublished || !pointsAt(p.Relations, target) {
			continue
		}
		p.Content = ""
		matched = append(matched, p)
	}
	total := len(matched)
	start := min((page-1)*perPage, total)
	return matched[start:min(start+perPage, total)], total, nil
}

// Update stores the post's fields and any snapshot unless an update error is injected.
func (s *fakePostStore) Update(
	_ context.Context, p content.Content, expectedUpdatedAt time.Time, snapshot *content.Revision, revisionCap int,
) (content.Content, error) {
	if s.updateErr != nil {
		return content.Content{}, s.updateErr
	}
	existing, ok := s.posts[p.ID]
	if !ok {
		return content.Content{}, content.ErrNotFound
	}
	if !existing.UpdatedAt.Equal(expectedUpdatedAt) {
		return content.Content{}, content.ErrConflict
	}
	s.lastSnapshot, s.lastRevisionCap = snapshot, revisionCap
	if snapshot != nil {
		s.revisions = append(s.revisions, *snapshot)
	}
	return s.add(p), nil
}

// Trash marks the stored post trashed.
func (s *fakePostStore) Trash(_ context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	if s.trashErr != nil {
		return content.Content{}, s.trashErr
	}
	p, ok := s.posts[id]
	if !ok {
		return content.Content{}, content.ErrNotFound
	}
	p.Status = content.StatusTrash
	p.Slug += "-trashed-abcd1234"
	p.UpdatedAt = updatedAt
	return s.add(p), nil
}

// Restore returns the stored post to draft.
func (s *fakePostStore) Restore(_ context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	if s.restoreErr != nil {
		return content.Content{}, s.restoreErr
	}
	p, ok := s.posts[id]
	if !ok {
		return content.Content{}, content.ErrNotFound
	}
	p.Status = content.StatusDraft
	p.Slug = strings.TrimSuffix(p.Slug, "-trashed-abcd1234")
	p.UpdatedAt = updatedAt
	return s.add(p), nil
}

// Delete removes the stored content.
func (s *fakePostStore) Delete(_ context.Context, id uuid.UUID) error {
	if s.deleteErr != nil {
		return s.deleteErr
	}
	if _, ok := s.posts[id]; !ok {
		return content.ErrNotFound
	}
	delete(s.posts, id)
	return nil
}

// Revisions returns the post's stored revisions newest first, without content.
func (s *fakePostStore) Revisions(_ context.Context, postID uuid.UUID) ([]content.Revision, error) {
	if s.revisionsErr != nil {
		return nil, s.revisionsErr
	}
	stored := make([]content.Revision, 0, len(s.revisions))
	for _, r := range s.revisions {
		if r.ContentID != postID {
			continue
		}
		r.Content = ""
		stored = append(stored, r)
	}
	slices.SortFunc(stored, func(a, b content.Revision) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return bytes.Compare(b.ID[:], a.ID[:])
	})
	return stored, nil
}

// RevisionByID returns the stored revision, or [content.ErrRevisionNotFound].
func (s *fakePostStore) RevisionByID(_ context.Context, postID, revisionID uuid.UUID) (content.Revision, error) {
	if s.revisionErr != nil {
		return content.Revision{}, s.revisionErr
	}
	for _, r := range s.revisions {
		if r.ID == revisionID && r.ContentID == postID {
			return r, nil
		}
	}
	return content.Revision{}, content.ErrRevisionNotFound
}

// DeleteRevision removes the stored revision, or reports [content.ErrRevisionNotFound].
func (s *fakePostStore) DeleteRevision(_ context.Context, postID, revisionID uuid.UUID) error {
	if s.deleteRevisionErr != nil {
		return s.deleteRevisionErr
	}
	for i, r := range s.revisions {
		if r.ID == revisionID && r.ContentID == postID {
			s.revisions = append(s.revisions[:i], s.revisions[i+1:]...)
			return nil
		}
	}
	return content.ErrRevisionNotFound
}

// SaveAutosave stores the author's autosave, replacing any earlier one.
func (s *fakePostStore) SaveAutosave(_ context.Context, autosave content.Revision) (content.Revision, error) {
	if s.saveAutosaveErr != nil {
		return content.Revision{}, s.saveAutosaveErr
	}
	for i, r := range s.revisions {
		if r.Kind == content.RevisionKindAutosave && r.ContentID == autosave.ContentID && r.AuthorID == autosave.AuthorID {
			autosave.ID = r.ID
			s.revisions[i] = autosave
			return autosave, nil
		}
	}
	s.revisions = append(s.revisions, autosave)
	return autosave, nil
}

// DeleteAutosave removes the author's autosave of the content.
func (s *fakePostStore) DeleteAutosave(_ context.Context, postID, authorID uuid.UUID) error {
	if s.deleteAutosaveErr != nil {
		return s.deleteAutosaveErr
	}
	for i, r := range s.revisions {
		if r.Kind == content.RevisionKindAutosave && r.ContentID == postID && r.AuthorID == authorID {
			s.revisions = append(s.revisions[:i], s.revisions[i+1:]...)
			return nil
		}
	}
	return nil
}

// Autosave returns the author's autosave, or [content.ErrRevisionNotFound].
func (s *fakePostStore) Autosave(_ context.Context, postID, authorID uuid.UUID) (content.Revision, error) {
	if s.autosaveErr != nil {
		return content.Revision{}, s.autosaveErr
	}
	for _, r := range s.revisions {
		if r.Kind == content.RevisionKindAutosave && r.ContentID == postID && r.AuthorID == authorID {
			return r, nil
		}
	}
	return content.Revision{}, content.ErrRevisionNotFound
}

// Counts returns the number of stored posts of the type in each status.
func (s *fakePostStore) Counts(_ context.Context, typeKey string) (map[content.Status]int, error) {
	if s.countsErr != nil {
		return nil, s.countsErr
	}
	counts := map[content.Status]int{}
	for _, p := range s.posts {
		if p.Type == typeKey {
			counts[p.Status]++
		}
	}
	return counts, nil
}

// versionedBody returns a write request body carrying the version and the given fields.
func versionedBody(t *testing.T, version time.Time, fields map[string]any) string {
	t.Helper()
	body := map[string]any{"updated_at": version}
	maps.Copy(body, fields)
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("Marshal(%v) error = %v, want nil", body, err)
	}
	return string(encoded)
}

// newPost returns a draft post authored by author.
func newPost(t *testing.T, title string, author uuid.UUID) content.Content {
	t.Helper()
	p, err := content.New(postType(), nil, title, author)
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

// typedPostServer returns a signed in handler, the post store, the type store behind it, and the signed-in user.
func typedPostServer(t *testing.T) (http.Handler, *fakePostStore, *fakeTypeStore, gouncer.User) {
	t.Helper()
	users := newFakeUserStore()
	ada := addAda(t, users)
	posts := newFakePostStore()
	types := newFakeTypeStore()
	handler := server.NewServer(server.Config{Users: users, Content: posts, Types: types})
	cookie := loginCookie(t, handler)
	authed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.AddCookie(cookie)
		handler.ServeHTTP(w, r)
	})
	return authed, posts, types, ada
}

// serverConfig returns a server config over the given stores.
func serverConfig(users authkit.AdminStore, posts content.Store) server.Config {
	return server.Config{Users: users, Content: posts, Types: newFakeTypeStore()}
}

// postType returns the built-in post type as the registry holds it.
func postType() content.Type {
	return content.Type{
		Key: content.TypePost, SingularLabel: "Post", PluralLabel: "Posts",
		Revisions: true, RevisionCap: 100, PageKind: content.PageKindSingle,
		Default: true, Active: true,
	}
}

// errRegistryDown stands for a registry the database cannot answer for.
var errRegistryDown = errors.New("the registry is unreachable")

// fakeTypeStore holds the content type registry in memory.
type fakeTypeStore struct {
	mu      sync.Mutex
	types   []content.Type
	listErr error
}

// newFakeTypeStore returns a registry holding the built-in post type.
func newFakeTypeStore() *fakeTypeStore {
	return &fakeTypeStore{types: []content.Type{postType()}}
}

// register stores a type directly.
func (s *fakeTypeStore) register(t content.Type) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.types = append(s.types, t)
}

// List returns every stored type in registration order.
func (s *fakeTypeStore) List(context.Context) ([]content.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listErr != nil {
		return nil, s.listErr
	}
	stored := make([]content.Type, len(s.types))
	copy(stored, s.types)
	return stored, nil
}

// ListGroups returns one group per stored type holding the fields it declares.
func (s *fakeTypeStore) ListGroups(context.Context) ([]content.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listErr != nil {
		return nil, s.listErr
	}
	groups := make([]content.Group, 0, len(s.types))
	for i, t := range s.types {
		groups = append(groups, content.Group{
			ID: i + 1, Title: t.SingularLabel + " fields", Active: true, Fields: t.Fields,
		})
	}
	return groups, nil
}

// ByKey returns the stored type carrying the key.
func (s *fakeTypeStore) ByKey(_ context.Context, key string) (content.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.types {
		if t.Key == key {
			return t, nil
		}
	}
	return content.Type{}, content.ErrTypeNotFound
}

// Create stores a new type.
func (s *fakeTypeStore) Create(_ context.Context, t content.Type) (content.Type, error) {
	s.register(t)
	return t, nil
}

// Update stores the edited type.
func (s *fakeTypeStore) Update(_ context.Context, t content.Type) (content.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key == key {
			s.types = append(s.types[:i], s.types[i+1:]...)
			return nil
		}
	}
	return content.ErrTypeNotFound
}

// ReorderFields stores the given declaration order on the type.
func (s *fakeTypeStore) ReorderFields(_ context.Context, typeKey string, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key != typeKey {
			continue
		}
		reordered := make([]content.Field, 0, len(stored.Fields))
		for _, key := range keys {
			for _, held := range stored.Fields {
				if held.Key == key {
					reordered = append(reordered, held)
				}
			}
		}
		s.types[i].Fields = reordered
		return nil
	}
	return content.ErrTypeNotFound
}

// serverWithStores returns an unauthenticated handler over the given stores.
func serverWithStores(users authkit.AdminStore, posts content.Store) http.Handler {
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

// pointsAt reports whether any of the item's fields names the target.
func pointsAt(held content.Relations, target uuid.UUID) bool {
	for _, targets := range held {
		for _, listed := range targets {
			if listed == target {
				return true
			}
		}
	}
	return false
}

// TargetsOf returns the published targets of active types the item points at.
func (s *fakePostStore) TargetsOf(_ context.Context, from uuid.UUID) (content.Targets, error) {
	if s.targetsErr != nil {
		return nil, s.targetsErr
	}
	held, found := s.posts[from]
	if !found {
		return nil, nil
	}
	targets := make(content.Targets)
	for key, listed := range held.Relations {
		for _, id := range listed {
			pointed, stored := s.posts[id]
			if !stored || pointed.Status != content.StatusPublished {
				continue
			}
			targets[key] = append(targets[key], content.Target{
				ID: pointed.ID, Title: pointed.Title, Path: pointed.Path,
			})
		}
	}
	return targets, nil
}
