// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// memoryContent holds one scenario's content in memory.
type memoryContent struct {
	content.Store
	mu        sync.Mutex
	items     map[uuid.UUID]content.Content
	revisions map[uuid.UUID][]content.Revision
	autosaves map[autosaveKey]content.Revision
}

// autosaveKey names one author's parked buffer over one item.
type autosaveKey struct {
	contentID uuid.UUID
	authorID  uuid.UUID
}

// newMemoryContent returns an empty in-memory content store.
func newMemoryContent() *memoryContent {
	return &memoryContent{
		items:     make(map[uuid.UUID]content.Content),
		revisions: make(map[uuid.UUID][]content.Revision),
		autosaves: make(map[autosaveKey]content.Revision),
	}
}

// clearField sweeps the field's values from the type's items and their snapshots.
func (s *memoryContent) clearField(typeKey, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, stored := range s.items {
		if stored.Type != typeKey {
			continue
		}
		delete(stored.Fields, key)
		s.items[id] = stored
		for i := range s.revisions[id] {
			delete(s.revisions[id][i].Fields, key)
		}
		for held, parked := range s.autosaves {
			if held.contentID == id {
				delete(parked.Fields, key)
			}
		}
	}
}

// Revisions returns the item's revisions newest first, without their content.
func (s *memoryContent) Revisions(_ context.Context, contentID uuid.UUID) ([]content.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	held := s.revisions[contentID]
	listed := make([]content.Revision, len(held))
	for i, stored := range held {
		listed[len(held)-1-i] = stored
		listed[len(held)-1-i].Content = ""
	}
	return listed, nil
}

// RevisionByID returns the item's revision, or [content.ErrRevisionNotFound].
func (s *memoryContent) RevisionByID(
	_ context.Context, contentID, revisionID uuid.UUID,
) (content.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stored := range s.revisions[contentID] {
		if stored.ID == revisionID {
			return stored, nil
		}
	}
	return content.Revision{}, content.ErrRevisionNotFound
}

// SaveAutosave stores the author's autosave of the item, replacing any earlier one.
func (s *memoryContent) SaveAutosave(_ context.Context, autosave content.Revision) (content.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, found := s.items[autosave.ContentID]; !found {
		return content.Revision{}, content.ErrNotFound
	}
	s.autosaves[autosaveKey{autosave.ContentID, autosave.AuthorID}] = autosave
	return autosave, nil
}

// Autosave returns the author's autosave of the item, or [content.ErrRevisionNotFound].
func (s *memoryContent) Autosave(
	_ context.Context, contentID, authorID uuid.UUID,
) (content.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	parked, found := s.autosaves[autosaveKey{contentID, authorID}]
	if !found {
		return content.Revision{}, content.ErrRevisionNotFound
	}
	return parked, nil
}

// DeleteAutosave removes the author's autosave of the item.
func (s *memoryContent) DeleteAutosave(_ context.Context, contentID, authorID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.autosaves, autosaveKey{contentID, authorID})
	return nil
}

// Create stores a new content item, suffixing its slug until its address is free.
func (s *memoryContent) Create(_ context.Context, c content.Content) (content.Content, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prefix := content.AddressPrefix(c.Path, c.Slug)
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		slug := numberedSlug(c.Slug, attempt)
		if s.addressHeld(content.AddressUnder(prefix, slug), c.ID) {
			continue
		}
		stored := c.Place(prefix, slug)
		s.items[stored.ID] = stored
		return stored, nil
	}
	return content.Content{}, content.ErrSlugTaken
}

// slugAttempts bounds the suffixes tried when an address is taken.
const slugAttempts = 20

// numberedSlug returns slug for the first attempt, and slug with the attempt number after that.
func numberedSlug(slug string, attempt int) string {
	if attempt == 1 {
		return slug
	}
	return slug + "-" + strconv.Itoa(attempt)
}

// addressHeld reports whether another item already answers at the address.
func (s *memoryContent) addressHeld(path string, except uuid.UUID) bool {
	for _, stored := range s.items {
		if stored.Path == path && stored.ID != except {
			return true
		}
	}
	return false
}

// carryDescendants moves everything nested under the item to follow its address.
func (s *memoryContent) carryDescendants(moved content.Content, was string) {
	if was == moved.Path {
		return
	}
	for id, stored := range s.items {
		if stored.ID == moved.ID || !strings.HasPrefix(stored.Path, was+"/") {
			continue
		}
		stored.Path = moved.Path + strings.TrimPrefix(stored.Path, was)
		s.items[id] = stored
	}
}

// carryType moves every address of the type from the route word it answered under.
func (s *memoryContent) carryType(key, was, now string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, stored := range s.items {
		if stored.Type != key {
			continue
		}
		stored.Path = content.AddressUnder(now, strings.TrimPrefix(strings.TrimPrefix(stored.Path, was), "/"))
		s.items[id] = stored
	}
}

// ByID returns the item carrying the id, or [content.ErrNotFound].
func (s *memoryContent) ByID(_ context.Context, id uuid.UUID) (content.Content, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, found := s.items[id]
	if !found {
		return content.Content{}, content.ErrNotFound
	}
	return stored, nil
}

// PublishedByPath returns the published item answering at the address, or [content.ErrNotFound].
func (s *memoryContent) PublishedByPath(_ context.Context, path string) (content.Content, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stored := range s.items {
		if stored.Path == path && stored.Status == content.StatusPublished {
			return stored, nil
		}
	}
	return content.Content{}, content.ErrNotFound
}

// Depth returns how many levels of content nest below the item.
func (s *memoryContent) Depth(ctx context.Context, id uuid.UUID) (int, error) {
	s.mu.Lock()
	held := make([]content.Content, 0, len(s.items))
	for _, stored := range s.items {
		if stored.ParentID != nil && *stored.ParentID == id {
			held = append(held, stored)
		}
	}
	s.mu.Unlock()
	below := 0
	for _, child := range held {
		under, err := s.Depth(ctx, child.ID)
		if err != nil {
			return 0, err
		}
		below = max(below, under+1)
	}
	return below, nil
}

// Children returns how many items nest directly under the item.
func (s *memoryContent) Children(_ context.Context, id uuid.UUID) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	held := 0
	for _, stored := range s.items {
		if stored.ParentID != nil && *stored.ParentID == id {
			held++
		}
	}
	return held, nil
}

// List returns the items the filter matches, newest first, with their total.
func (s *memoryContent) List(_ context.Context, f content.Filter) ([]content.Content, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := make([]content.Content, 0, len(s.items))
	for _, stored := range s.items {
		if stored.Type == f.Type && (f.Status == "" || stored.Status == f.Status) {
			matched = append(matched, stored)
		}
	}
	slices.SortFunc(matched, func(a, b content.Content) int { return b.CreatedAt.Compare(a.CreatedAt) })
	return paged(matched, f), len(matched), nil
}

// paged returns the page of items the filter asks for.
func paged(matched []content.Content, f content.Filter) []content.Content {
	if f.PerPage <= 0 {
		return matched
	}
	start := min((max(f.Page, 1)-1)*f.PerPage, len(matched))
	return matched[start:min(start+f.PerPage, len(matched))]
}

// Update stores the item's editable fields, or reports it missing or stale.
func (s *memoryContent) Update(
	_ context.Context, c content.Content, expectedUpdatedAt time.Time, snapshot *content.Revision, _ int,
) (content.Content, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, found := s.items[c.ID]
	if !found {
		return content.Content{}, content.ErrNotFound
	}
	if !stored.UpdatedAt.Equal(expectedUpdatedAt) {
		return content.Content{}, content.ErrConflict
	}
	if snapshot != nil {
		s.revisions[c.ID] = append(s.revisions[c.ID], *snapshot)
	}
	prefix := content.AddressPrefix(c.Path, c.Slug)
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		slug := numberedSlug(c.Slug, attempt)
		if s.addressHeld(content.AddressUnder(prefix, slug), c.ID) {
			continue
		}
		settled := c.Place(prefix, slug)
		s.items[settled.ID] = settled
		s.carryDescendants(settled, stored.Path)
		return settled, nil
	}
	return content.Content{}, content.ErrSlugTaken
}

// Trash marks the item trashed and frees its address, or refuses while it holds children.
func (s *memoryContent) Trash(ctx context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	held, err := s.Children(ctx, id)
	if err != nil {
		return content.Content{}, err
	}
	if held > 0 {
		return content.Content{}, content.ErrHoldsChildren
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, found := s.items[id]
	if !found {
		return content.Content{}, content.ErrNotFound
	}
	if stored.Status == content.StatusTrash {
		return content.Content{}, content.ErrInvalidTransition
	}
	stored.Status, stored.UpdatedAt = content.StatusTrash, updatedAt
	stored = stored.Place(content.AddressPrefix(stored.Path, stored.Slug), stored.Slug+"-trashed")
	s.items[id] = stored
	return stored, nil
}

// Counts returns how many items of the type hold each status.
func (s *memoryContent) Counts(_ context.Context, contentType string) (map[content.Status]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	counts := make(map[content.Status]int)
	for _, stored := range s.items {
		if stored.Type == contentType {
			counts[stored.Status]++
		}
	}
	return counts, nil
}
