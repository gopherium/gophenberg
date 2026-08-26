// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
)

// memoryTypes holds one scenario's content type registry in memory.
type memoryTypes struct {
	mu       sync.Mutex
	types    []content.Type
	fieldIDs int
	content  *memoryContent
}

// newMemoryTypes returns a registry holding the built-in post type the migration registers.
func newMemoryTypes(items *memoryContent) *memoryTypes {
	now := time.Now().UTC()
	return &memoryTypes{content: items, types: []content.Type{{
		Key:           content.TypePost,
		SingularLabel: "Post",
		PluralLabel:   "Posts",
		Revisions:     true,
		RevisionCap:   100,
		PageKind:      content.PageKindSingle,
		Default:       true,
		Active:        true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}}}
}

// ReorderFields stores the given declaration order on the type.
func (s *memoryTypes) ReorderFields(_ context.Context, typeKey string, keys []string) error {
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

// List returns every stored type in registration order.
func (s *memoryTypes) List(context.Context) ([]content.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored := make([]content.Type, len(s.types))
	copy(stored, s.types)
	return stored, nil
}

// CreateGroup hands the group back unstored.
func (s *memoryTypes) CreateGroup(_ context.Context, g content.Group) (content.Group, error) {
	return g, nil
}

// UpdateGroup hands the group back unstored.
func (s *memoryTypes) UpdateGroup(_ context.Context, g content.Group) (content.Group, error) {
	return g, nil
}

// DeleteGroup removes no group.
func (s *memoryTypes) DeleteGroup(context.Context, int) error { return nil }

// ReorderGroups stores no order.
func (s *memoryTypes) ReorderGroups(context.Context, []int) error { return nil }

// CreateFieldInGroup hands the field back undeclared.
func (s *memoryTypes) CreateFieldInGroup(_ context.Context, _ int, f content.Field) (content.Field, error) {
	return f, nil
}

// MoveField carries no field.
func (s *memoryTypes) MoveField(context.Context, int, string, int) (content.Field, error) {
	return content.Field{}, content.ErrFieldNotFound
}

// ListGroups returns one group per stored type holding the fields it declares.
func (s *memoryTypes) ListGroups(context.Context) ([]content.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	groups := make([]content.Group, 0, len(s.types))
	for i, t := range s.types {
		groups = append(groups, content.Group{
			ID: i + 1, Title: t.SingularLabel + " fields", Active: true, Fields: t.Fields,
		})
	}
	return groups, nil
}

// ByKey returns the stored type carrying the key, or [content.ErrTypeNotFound].
func (s *memoryTypes) ByKey(_ context.Context, key string) (content.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stored := range s.types {
		if stored.Key == key {
			return stored, nil
		}
	}
	return content.Type{}, content.ErrTypeNotFound
}

// Create stores a new type, or reports the key taken.
func (s *memoryTypes) Create(_ context.Context, t content.Type) (content.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stored := range s.types {
		if stored.Key == t.Key {
			return content.Type{}, content.ErrTypeTaken
		}
	}
	s.types = append(s.types, t)
	return t, nil
}

// Update stores the edited type and carries its content to the route word, or
// reports it missing.
func (s *memoryTypes) Update(_ context.Context, t content.Type) (content.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key != t.Key {
			continue
		}
		if t.Default && !stored.Default {
			s.handRootOver()
			t.RouteWord = ""
		}
		s.types[i] = t
		if stored.RouteWord != t.RouteWord && s.content != nil {
			s.content.carryType(t.Key, stored.RouteWord, t.RouteWord)
		}
		return t, nil
	}
	return content.Type{}, content.ErrTypeNotFound
}

// handRootOver moves the type holding the root under a word of its own.
func (s *memoryTypes) handRootOver() {
	for i, stored := range s.types {
		if !stored.Default {
			continue
		}
		was := stored.RouteWord
		stored.RouteWord, stored.Default = content.Slugify(stored.PluralLabel), false
		s.types[i] = stored
		if s.content != nil {
			s.content.carryType(stored.Key, was, stored.RouteWord)
		}
		return
	}
}

// CreateField stores a field on its type, mirroring the schema's identity column.
func (s *memoryTypes) CreateField(_ context.Context, f content.Field) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key != f.TypeKey {
			continue
		}
		s.fieldIDs++
		f.ID = s.fieldIDs
		s.types[i].Fields = append(s.types[i].Fields, f)
		return f, nil
	}
	return content.Field{}, content.ErrTypeNotFound
}

// UpdateField stores the edited field on its type, or reports it missing.
func (s *memoryTypes) UpdateField(_ context.Context, f content.Field) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key != f.TypeKey {
			continue
		}
		for j, held := range stored.Fields {
			if held.Key == f.Key {
				f.ID = held.ID
				s.types[i].Fields[j] = f
				return f, nil
			}
		}
	}
	return content.Field{}, content.ErrFieldNotFound
}

// DeleteField removes the field from its type, or reports it missing.
func (s *memoryTypes) DeleteField(_ context.Context, typeKey, key string) error {
	if !s.dropField(typeKey, key) {
		return content.ErrFieldNotFound
	}
	if s.content != nil {
		s.content.clearField(typeKey, key)
		s.content.clearRelation(typeKey, key)
	}
	return nil
}

// dropField removes the declaration, reporting whether the type held one.
func (s *memoryTypes) dropField(typeKey, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key != typeKey {
			continue
		}
		for j, held := range stored.Fields {
			if held.Key == key {
				s.types[i].Fields = append(stored.Fields[:j], stored.Fields[j+1:]...)
				return true
			}
		}
	}
	return false
}

// targeted reports whether the field of the type may point at an item of the stored type.
func (s *memoryTypes) targeted(typeKey, key, stored string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, held := range s.types {
		if held.Key != typeKey {
			continue
		}
		for _, f := range held.Fields {
			if f.Key == key {
				if f.RelatesTo != stored {
					return fmt.Errorf("%w: %s holds %s", content.ErrTargetType, key, stored)
				}
				return nil
			}
		}
	}
	return nil
}

// Delete removes the type, or reports it missing or still holding content.
func (s *memoryTypes) Delete(ctx context.Context, key string) error {
	if s.holdsContent(ctx, key) {
		return content.ErrTypeInUse
	}
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

// holdsContent reports whether the scenario's content store holds items of the type.
func (s *memoryTypes) holdsContent(ctx context.Context, key string) bool {
	if s.content == nil {
		return false
	}
	items, _, err := s.content.List(ctx, content.Filter{Type: key})
	return err == nil && len(items) > 0
}

// serving reports whether the type is active enough to appear on a term page.
func (s *memoryTypes) serving(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stored := range s.types {
		if stored.Key == key {
			return stored.Active
		}
	}
	return false
}
