// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gopherium/gophenberg/internal/media"
)

// memoryMedia holds one scenario's media library in memory.
type memoryMedia struct {
	mu    sync.Mutex
	next  int64
	items map[int64]media.Media
}

// newMemoryMedia returns an empty in-memory media store.
func newMemoryMedia() *memoryMedia {
	return &memoryMedia{items: make(map[int64]media.Media)}
}

// Create stores a new media item and returns it with its assigned identifier.
func (s *memoryMedia) Create(_ context.Context, m media.Media) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	m.ID = s.next
	s.items[m.ID] = m
	return m, nil
}

// ByID returns the media item with the given id, or [media.ErrNotFound].
func (s *memoryMedia) ByID(_ context.Context, id int64) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, found := s.items[id]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	return m, nil
}

// List returns the media items matching the filter, newest first, and the
// total number matching it.
func (s *memoryMedia) List(_ context.Context, f media.Filter) ([]media.Media, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := make([]media.Media, 0, len(s.items))
	for _, m := range s.items {
		if matches(m, f) {
			matched = append(matched, m)
		}
	}
	sort.Slice(matched, func(i, j int) bool { return matched[i].ID > matched[j].ID })
	return page(matched, f), len(matched), nil
}

// matches reports whether the item satisfies the filter.
func matches(m media.Media, f media.Filter) bool {
	if f.Type != "" && m.Type != f.Type {
		return false
	}
	if !matchesMime(m, f.Mimes) {
		return false
	}
	if f.Search == "" {
		return true
	}
	search := strings.ToLower(f.Search)
	return strings.Contains(strings.ToLower(m.Title), search) ||
		strings.Contains(strings.ToLower(m.File), search)
}

// matchesMime reports whether the item's content type starts with any prefix.
func matchesMime(m media.Media, prefixes []string) bool {
	if len(prefixes) == 0 {
		return true
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(m.MimeType, prefix) {
			return true
		}
	}
	return false
}

// page returns the slice of items the filter's page holds.
func page(items []media.Media, f media.Filter) []media.Media {
	start := (f.Page - 1) * f.PerPage
	if start >= len(items) {
		return nil
	}
	end := min(start+f.PerPage, len(items))
	return items[start:end]
}

// Update stores the media item's descriptions, or reports a missing item or a stale edit.
func (s *memoryMedia) Update(_ context.Context, m media.Media, expectedUpdatedAt time.Time) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, found := s.items[m.ID]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	if !stored.UpdatedAt.Equal(expectedUpdatedAt) {
		return media.Media{}, media.ErrConflict
	}
	stored.Title = m.Title
	stored.AltText = m.AltText
	stored.Caption = m.Caption
	stored.Description = m.Description
	stored.UpdatedAt = m.UpdatedAt
	s.items[m.ID] = stored
	return stored, nil
}

// Delete removes the media item and returns it, or reports [media.ErrNotFound].
func (s *memoryMedia) Delete(_ context.Context, id int64) (media.Media, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, found := s.items[id]
	if !found {
		return media.Media{}, media.ErrNotFound
	}
	delete(s.items, id)
	return m, nil
}
