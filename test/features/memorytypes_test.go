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
	mu          sync.Mutex
	types       []content.Type
	groups      []content.Group
	nextGroupID int
	fieldIDs    int
	content     *memoryContent
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
	for i, held := range stored {
		stored[i].Fields = s.flattened(held.Key, held.Fields)
	}
	return stored, nil
}

// memoryParams holds the rule sources the memory store evaluates locations with.
var memoryParams = content.DefaultParamRegistry(nil)

// flattened returns the type's own fields beside those its matching groups place on it.
func (s *memoryTypes) flattened(typeKey string, own []content.Field) []content.Field {
	fields := append([]content.Field(nil), own...)
	served := make(map[string]bool, len(fields))
	for _, f := range fields {
		served[f.Key] = true
	}
	screen := content.Screen{content.ScreenContentType: typeKey}
	for _, g := range s.groups {
		if !g.Active || !g.Location.Match(screen, memoryParams) {
			continue
		}
		for _, f := range g.Fields {
			if served[f.Key] {
				continue
			}
			served[f.Key] = true
			f.TypeKey = typeKey
			fields = append(fields, f)
		}
	}
	return fields
}

// CreateGroup stores a new field group at the end of the order.
func (s *memoryTypes) CreateGroup(_ context.Context, g content.Group) (content.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextGroupID++
	g.ID, g.Active, g.Position = s.nextGroupID, true, len(s.groups)+1
	s.groups = append(s.groups, g)
	return g, nil
}

// UpdateGroup stores the group's title, location and resting flag.
func (s *memoryTypes) UpdateGroup(_ context.Context, g content.Group) (content.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		if held.ID != g.ID {
			continue
		}
		held.Title, held.Location, held.Active = g.Title, g.Location, g.Active
		s.groups[i] = held
		return held, nil
	}
	return content.Group{}, content.ErrGroupNotFound
}

// DeleteGroup removes the group and every field it holds.
func (s *memoryTypes) DeleteGroup(_ context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		if held.ID == id {
			s.groups = append(s.groups[:i], s.groups[i+1:]...)
			return nil
		}
	}
	return content.ErrGroupNotFound
}

// ReorderGroups stores the given order on the groups.
func (s *memoryTypes) ReorderGroups(_ context.Context, ids []int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ordered := make([]content.Group, 0, len(ids))
	for _, id := range ids {
		for _, held := range s.groups {
			if held.ID == id {
				ordered = append(ordered, held)
			}
		}
	}
	s.groups = ordered
	return nil
}

// CreateFieldInGroup declares the field inside the group.
func (s *memoryTypes) CreateFieldInGroup(_ context.Context, groupID int, f content.Field) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		for _, stored := range held.Fields {
			if stored.Key == f.Key {
				return content.Field{}, content.ErrFieldTaken
			}
		}
		s.fieldIDs++
		f.ID, f.GroupID = s.fieldIDs, groupID
		s.groups[i].Fields = append(held.Fields, f)
		return f, nil
	}
	return content.Field{}, content.ErrGroupNotFound
}

// UpdateFieldInGroup stores the field's label and required flag inside its group.
func (s *memoryTypes) UpdateFieldInGroup(_ context.Context, groupID int, f content.Field) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		for j, stored := range held.Fields {
			if stored.Key == f.Key {
				s.groups[i].Fields[j] = f
				return f, nil
			}
		}
	}
	return content.Field{}, content.ErrFieldNotFound
}

// DeleteFieldInGroup removes the field from its group.
func (s *memoryTypes) DeleteFieldInGroup(_ context.Context, groupID int, key string) error {
	reached, dropped := s.dropFieldInGroup(groupID, key)
	if !dropped {
		return content.ErrFieldNotFound
	}
	if s.content != nil {
		for _, typeKey := range reached {
			s.content.clearField(typeKey, key)
			s.content.clearRelation(typeKey, key)
		}
	}
	return nil
}

// dropFieldInGroup removes the declaration, reporting the types the group reached.
func (s *memoryTypes) dropFieldInGroup(groupID int, key string) ([]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		for j, stored := range held.Fields {
			if stored.Key != key {
				continue
			}
			s.groups[i].Fields = append(held.Fields[:j], held.Fields[j+1:]...)
			return s.typesMatchedBy(held), true
		}
	}
	return nil, false
}

// typesMatchedBy returns the keys of the types the group's location reaches.
func (s *memoryTypes) typesMatchedBy(g content.Group) []string {
	matched := make([]string, 0, len(s.types))
	for _, t := range s.types {
		if g.Location.Match(content.Screen{content.ScreenContentType: t.Key}, memoryParams) {
			matched = append(matched, t.Key)
		}
	}
	return matched
}

// ReorderFieldsInGroup stores the given order on the group's fields.
func (s *memoryTypes) ReorderFieldsInGroup(_ context.Context, groupID int, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		ordered := make([]content.Field, 0, len(keys))
		for _, key := range keys {
			for _, stored := range held.Fields {
				if stored.Key == key {
					ordered = append(ordered, stored)
				}
			}
		}
		s.groups[i].Fields = ordered
		return nil
	}
	return content.ErrGroupNotFound
}

// MoveField carries the field into another group.
func (s *memoryTypes) MoveField(_ context.Context, groupID int, key string, toGroup int) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	landing := -1
	for i, held := range s.groups {
		if held.ID == toGroup {
			landing = i
		}
	}
	if landing < 0 {
		return content.Field{}, content.ErrGroupNotFound
	}
	var carried content.Field
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		for at, f := range held.Fields {
			if f.Key == key {
				carried = f
				s.groups[i].Fields = append(held.Fields[:at], held.Fields[at+1:]...)
				break
			}
		}
	}
	if carried.Key == "" {
		return content.Field{}, content.ErrFieldNotFound
	}
	carried.GroupID = toGroup
	s.groups[landing].Fields = append(s.groups[landing].Fields, carried)
	return carried, nil
}

// ListGroups returns the stored groups beside one per type already holding fields.
func (s *memoryTypes) ListGroups(context.Context) ([]content.Group, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	groups := make([]content.Group, 0, len(s.types)+len(s.groups))
	for i, t := range s.types {
		if len(t.Fields) == 0 {
			continue
		}
		groups = append(groups, content.Group{
			ID: -(i + 1), Title: t.SingularLabel + " fields", Active: true, Fields: t.Fields,
		})
	}
	return append(groups, s.groups...), nil
}

// ByKey returns the stored type carrying the key, or [content.ErrTypeNotFound].
func (s *memoryTypes) ByKey(_ context.Context, key string) (content.Type, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, stored := range s.types {
		if stored.Key == key {
			stored.Fields = s.flattened(key, stored.Fields)
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
		for _, f := range s.flattened(typeKey, held.Fields) {
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
