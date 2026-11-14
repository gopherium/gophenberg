// SPDX-License-Identifier: Apache-2.0

package contentchanges_test

import (
	"context"
	"slices"
	"sync"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// types answers every type active but the inactive one, or its failure from the read numbered failFrom on.
type types struct {
	mu       sync.Mutex
	inactive string
	err      error
	failFrom int
	reads    int
}

// Active answers the type, ErrTypeInactive for the inactive one, or the failure once its reads reach failFrom.
func (t *types) Active(_ context.Context, key string) (content.Type, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.reads++
	if t.err != nil && t.reads >= t.failFrom {
		return content.Type{}, t.err
	}
	if key == t.inactive {
		return content.Type{}, content.ErrTypeInactive
	}
	return content.Type{Key: key}, nil
}

// items holds stored items by id, listing the published ones of a type, and each item as a later write left it.
type items struct {
	mu      sync.Mutex
	held    []content.Content
	listErr error
	byIDErr error
	// since is each item as a write after the listing left it, nil for one deleted since.
	since map[uuid.UUID]*content.Content
}

// rewrite keeps written as the item a later write left, or the item id deleted when written is nil.
func (s *items) rewrite(id uuid.UUID, written *content.Content) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.since == nil {
		s.since = map[uuid.UUID]*content.Content{}
	}
	s.since[id] = written
}

// Children counts the items whose parent is id.
func (s *items) Children(_ context.Context, id uuid.UUID) (int, error) {
	count := 0
	for _, held := range s.held {
		if held.ParentID != nil && *held.ParentID == id {
			count++
		}
	}
	return count, nil
}

// List returns one page of the held items of the filter's type in the filter's statuses.
func (s *items) List(_ context.Context, f content.Filter) ([]content.ListedItem, int, error) {
	if s.listErr != nil {
		return nil, 0, s.listErr
	}
	var matched []content.ListedItem
	for _, held := range s.held {
		if held.Type == f.Type && slices.Contains(f.Statuses, held.Status) {
			matched = append(matched, content.ListedItem{Content: held})
		}
	}
	start := min((f.Page-1)*f.PerPage, len(matched))
	return matched[start:min(start+f.PerPage, len(matched))], len(matched), nil
}

// ByID returns the item with id as the last write left it.
func (s *items) ByID(_ context.Context, id uuid.UUID) (content.Content, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.byIDErr != nil {
		return content.Content{}, s.byIDErr
	}
	if written, ok := s.since[id]; ok {
		return rewritten(written)
	}
	return s.find(id)
}

// rewritten returns the item a later write left, or ErrNotFound for one it deleted.
func rewritten(written *content.Content) (content.Content, error) {
	if written == nil {
		return content.Content{}, content.ErrNotFound
	}
	return *written, nil
}

// find returns the held item with id, or ErrNotFound.
func (s *items) find(id uuid.UUID) (content.Content, error) {
	for _, held := range s.held {
		if held.ID == id {
			return held, nil
		}
	}
	return content.Content{}, content.ErrNotFound
}
