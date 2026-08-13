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
	mu    sync.Mutex
	items map[uuid.UUID]content.Content
}

// newMemoryContent returns an empty in-memory content store.
func newMemoryContent() *memoryContent {
	return &memoryContent{items: make(map[uuid.UUID]content.Content)}
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

// PublishedBySlug reports that the scenario publishes nothing publicly.
func (s *memoryContent) PublishedBySlug(context.Context, string, string) (content.Content, error) {
	return content.Content{}, content.ErrNotFound
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
	_ context.Context, c content.Content, expectedUpdatedAt time.Time, _ *content.Revision, _ int,
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
