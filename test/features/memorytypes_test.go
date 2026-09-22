// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"context"
	"fmt"
	"slices"
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
	if g.Key == "" {
		g.Key = s.mintGroupKey(g.Title)
	}
	for _, held := range s.groups {
		if held.Key == g.Key {
			return content.Group{}, content.ErrGroupKeyTaken
		}
	}
	s.nextGroupID++
	g.ID, g.Active, g.Position = s.nextGroupID, true, len(s.groups)+1
	s.groups = append(s.groups, g)
	return g, nil
}

// mintGroupKey returns a key from the title that no stored group holds.
func (s *memoryTypes) mintGroupKey(title string) string {
	taken := make(map[string]bool, len(s.groups))
	for _, held := range s.groups {
		taken[held.Key] = true
	}
	stem := content.GroupKeyFrom(title)
	key := stem
	for n := 2; taken[key]; n++ {
		key = fmt.Sprintf("%s-%d", stem, n)
	}
	return key
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

// DeleteGroup removes the group and every field it holds, carrying what it stores inside other groups' containers.
func (s *memoryTypes) DeleteGroup(_ context.Context, id int) error {
	served, dropped := s.dropGroup(id)
	if !dropped {
		return content.ErrGroupNotFound
	}
	for key, typeKeys := range served {
		for _, typeKey := range typeKeys {
			s.content.clearField(typeKey, key)
			s.content.clearRelation(typeKey, key)
		}
	}
	return nil
}

// DeleteFieldsOfGroup removes every named field from its group and sweeps its values.
func (s *memoryTypes) DeleteFieldsOfGroup(ctx context.Context, groupID int, keys []string) error {
	for _, key := range keys {
		if err := s.DeleteFieldInGroup(ctx, groupID, key); err != nil {
			return err
		}
	}
	return nil
}

// dropGroup removes the group with its fields, reporting the types it alone served each top level key on.
func (s *memoryTypes) dropGroup(id int) (map[string][]string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		if held.ID != id {
			continue
		}
		served := map[string][]string{}
		for _, f := range held.Fields {
			if s.content != nil {
				served[f.Key] = s.servedAlong(s.typesMatchedBy(held), held.ID, []string{f.Key})
			}
		}
		s.groups = append(s.groups[:i], s.groups[i+1:]...)
		for j, kept := range s.groups {
			s.groups[j].Fields = carriedInto(kept.Fields, kept.ID)
		}
		return served, true
	}
	return nil, false
}

// carriedInto returns the declared fields, and every field inside them, stored under the group.
func carriedInto(declared []content.Field, groupID int) []content.Field {
	carried := make([]content.Field, len(declared))
	for i, held := range declared {
		held.GroupID = groupID
		held.Fields = carriedInto(held.Fields, groupID)
		carried[i] = held
	}
	return carried
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

// CreateSubField declares the field inside the container the parent names.
func (s *memoryTypes) CreateSubField(_ context.Context, parentID int, f content.Field) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fieldIDs++
	f.ID, f.ParentID = s.fieldIDs, parentID
	for i, held := range s.groups {
		f.GroupID = held.ID
		grown, found := grownInside(held.Fields, parentID, f)
		if !found {
			continue
		}
		s.groups[i].Fields = grown
		return f, nil
	}
	return content.Field{}, content.ErrFieldNotFound
}

// DeleteSubField removes the field standing inside a container.
func (s *memoryTypes) DeleteSubField(_ context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		pruned, found := prunedInside(held.Fields, id)
		if !found {
			continue
		}
		s.groups[i].Fields = pruned
		return nil
	}
	return content.ErrFieldNotFound
}

// UpdateSubField carries the edit onto the field standing inside a container.
func (s *memoryTypes) UpdateSubField(
	_ context.Context, id int, f content.Field, expectedUpdatedAt time.Time,
) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		edited, stored, err := editedInside(held.Fields, id, f, expectedUpdatedAt)
		if err != nil {
			return content.Field{}, err
		}
		if stored.ID == 0 {
			continue
		}
		s.groups[i].Fields = edited
		return stored, nil
	}
	return content.Field{}, content.ErrFieldNotFound
}

// editedInside returns the declared fields with the edit carried onto the one the identity names.
func editedInside(
	declared []content.Field, id int, f content.Field, expectedUpdatedAt time.Time,
) ([]content.Field, content.Field, error) {
	edited := make([]content.Field, len(declared))
	copy(edited, declared)
	for i, held := range edited {
		if held.ID == id {
			if !expectedUpdatedAt.Equal(held.UpdatedAt) {
				return declared, content.Field{}, content.ErrConflict
			}
			held.Label, held.Required, held.Settings = f.Label, f.Required, f.Settings
			held.UpdatedAt = f.UpdatedAt
			edited[i] = held
			return edited, held, nil
		}
		inside, stored, err := editedInside(held.Fields, id, f, expectedUpdatedAt)
		if err != nil {
			return declared, content.Field{}, err
		}
		if stored.ID != 0 {
			edited[i].Fields = inside
			return edited, stored, nil
		}
	}
	return declared, content.Field{}, nil
}

// ReorderSubFields stands the fields inside the container in the order the keys name.
func (s *memoryTypes) ReorderSubFields(_ context.Context, parentID int, keys []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		stood, found := stoodInside(held.Fields, parentID, keys)
		if !found {
			continue
		}
		s.groups[i].Fields = stood
		return nil
	}
	return content.ErrFieldNotFound
}

// stoodInside returns the declared fields with the container's own fields in the order the keys name.
func stoodInside(declared []content.Field, parentID int, keys []string) ([]content.Field, bool) {
	stood := make([]content.Field, len(declared))
	copy(stood, declared)
	for i, held := range stood {
		if held.ID == parentID {
			stood[i].Fields = orderedInside(held.Fields, keys)
			return stood, true
		}
		if inside, found := stoodInside(held.Fields, parentID, keys); found {
			stood[i].Fields = inside
			return stood, true
		}
	}
	return declared, false
}

// orderedInside returns the fields standing in the order the keys name.
func orderedInside(declared []content.Field, keys []string) []content.Field {
	stood := make([]content.Field, 0, len(declared))
	for _, key := range keys {
		for _, held := range declared {
			if held.Key == key {
				stood = append(stood, held)
			}
		}
	}
	return stood
}

// storedUnder returns the field and every field inside it stored under the group.
func storedUnder(f content.Field, groupID int) content.Field {
	f.GroupID = groupID
	inside := make([]content.Field, len(f.Fields))
	for i, held := range f.Fields {
		inside[i] = storedUnder(held, groupID)
	}
	f.Fields = inside
	return f
}

// storeFieldUnder stores the sub field inside the named parent under the group, reporting whether one stands there.
func (s *memoryTypes) storeFieldUnder(key, parent string, groupID int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		for j, container := range held.Fields {
			if container.Key != parent {
				continue
			}
			for k, inside := range container.Fields {
				if inside.Key == key {
					s.groups[i].Fields[j].Fields[k].GroupID = groupID
					return true
				}
			}
		}
	}
	return false
}

// grownInside returns the declared fields with the new one placed inside the parent it names.
func grownInside(declared []content.Field, parentID int, f content.Field) ([]content.Field, bool) {
	grown := make([]content.Field, len(declared))
	copy(grown, declared)
	for i, held := range grown {
		if held.ID == parentID {
			grown[i].Fields = append(append([]content.Field{}, held.Fields...), f)
			return grown, true
		}
		if inside, found := grownInside(held.Fields, parentID, f); found {
			grown[i].Fields = inside
			return grown, true
		}
	}
	return declared, false
}

// prunedInside returns the declared fields without the one the identity names.
func prunedInside(declared []content.Field, id int) ([]content.Field, bool) {
	for i, held := range declared {
		if held.ID == id {
			return append(append([]content.Field{}, declared[:i]...), declared[i+1:]...), true
		}
		if inside, found := prunedInside(held.Fields, id); found {
			pruned := make([]content.Field, len(declared))
			copy(pruned, declared)
			pruned[i].Fields = inside
			return pruned, true
		}
	}
	return declared, false
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

// UpdateFieldInGroup stores the field's label, required flag and settings when the expectation still holds.
func (s *memoryTypes) UpdateFieldInGroup(
	_ context.Context, groupID int, f content.Field, expectedUpdatedAt time.Time,
) (content.Field, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, held := range s.groups {
		if held.ID != groupID {
			continue
		}
		for j, stored := range held.Fields {
			if stored.Key == f.Key {
				if !expectedUpdatedAt.Equal(stored.UpdatedAt) {
					return content.Field{}, content.ErrConflict
				}
				s.groups[i].Fields[j] = f
				return f, nil
			}
		}
	}
	return content.Field{}, content.ErrFieldNotFound
}

// DeleteFieldInGroup removes the field from its group and its values from the types the group served it on.
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

// dropFieldInGroup removes the declaration, reporting the types the group served the key on.
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
			served := s.servedAlong(s.typesMatchedBy(held), held.ID, []string{key})
			s.groups[i].Fields = append(held.Fields[:j], held.Fields[j+1:]...)
			return served, true
		}
	}
	return nil, false
}

// servedAlong returns the type keys on which the group serves the path, or no other active group serves it whole.
func (s *memoryTypes) servedAlong(typeKeys []string, groupID int, path []string) []string {
	held := make([]string, 0, len(typeKeys))
	for _, typeKey := range typeKeys {
		by, found := s.servingGroup(typeKey, path[0])
		if !found || by.ID == groupID || !declaresAlong(by.Fields, path) {
			held = append(held, typeKey)
		}
	}
	return held
}

// servingGroup returns the active group serving the top key on the type, if any does.
func (s *memoryTypes) servingGroup(typeKey, key string) (content.Group, bool) {
	screen := content.Screen{content.ScreenContentType: typeKey}
	for _, g := range s.groups {
		if !g.Active || !g.Location.Match(screen, memoryParams) {
			continue
		}
		for _, f := range g.Fields {
			if f.Key == key {
				return g, true
			}
		}
	}
	return content.Group{}, false
}

// declaresAlong reports whether a field stands at the path among the fields.
func declaresAlong(fields []content.Field, path []string) bool {
	if len(path) == 0 {
		return true
	}
	for _, f := range fields {
		if f.Key == path[0] {
			return declaresAlong(f.Fields, path[1:])
		}
	}
	return false
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

// MoveField carries the field to the top of the group, or inside the container the parent names, sweeping its old path.
func (s *memoryTypes) MoveField(_ context.Context, id, toGroup, toParent int) (content.Field, error) {
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
	if toParent != 0 && !holdsIdentity(s.groups[landing].Fields, toParent) {
		return content.Field{}, content.ErrFieldNotFound
	}
	source, carried, path, found := s.placedIn(id)
	if !found {
		return content.Field{}, content.ErrFieldNotFound
	}
	swept := s.sweptByMove(source, s.groups[landing], carried.ParentID != toParent, path)
	s.takenOut(id)
	moved := storedUnder(carried, toGroup)
	moved.ParentID = toParent
	if toParent == 0 {
		s.groups[landing].Fields = append(s.groups[landing].Fields, moved)
	} else {
		s.groups[landing].Fields, _ = grownInside(s.groups[landing].Fields, toParent, moved)
	}
	if s.content != nil {
		s.content.sweepPath(swept, path)
	}
	return moved, nil
}

// sweptByMove returns the types a moved field's old values leave, only those the landing misses between two tops.
func (s *memoryTypes) sweptByMove(source, landing content.Group, containerChanged bool, path []string) []string {
	matched := s.typesMatchedBy(source)
	if !containerChanged {
		matched = slices.DeleteFunc(matched, func(key string) bool {
			return landing.Location.Match(content.Screen{content.ScreenContentType: key}, memoryParams)
		})
	}
	return s.servedAlong(matched, source.ID, path)
}

// placedIn returns the group holding the field carrying the identity, the field and its path.
func (s *memoryTypes) placedIn(id int) (content.Group, content.Field, []string, bool) {
	for _, held := range s.groups {
		if carried, path, found := placedInside(held.Fields, id); found {
			return held, carried, path, true
		}
	}
	return content.Group{}, content.Field{}, nil, false
}

// takenOut removes the field carrying the identity from its group.
func (s *memoryTypes) takenOut(id int) {
	for i, held := range s.groups {
		s.groups[i].Fields, _ = prunedInside(held.Fields, id)
	}
}

// placedInside returns the field carrying the identity and the keys reaching it, however deep it stands.
func placedInside(declared []content.Field, id int) (content.Field, []string, bool) {
	for _, held := range declared {
		if held.ID == id {
			return held, []string{held.Key}, true
		}
		if inside, path, found := placedInside(held.Fields, id); found {
			return inside, append([]string{held.Key}, path...), true
		}
	}
	return content.Field{}, nil, false
}

// holdsIdentity reports whether a field carrying the identity stands among the fields, however deep.
func holdsIdentity(declared []content.Field, id int) bool {
	_, _, found := placedInside(declared, id)
	return found
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
// reports it missing or still nesting.
func (s *memoryTypes) Update(ctx context.Context, t content.Type) (content.Type, error) {
	nested, err := s.Nested(ctx, t.Key)
	if err != nil {
		return content.Type{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, stored := range s.types {
		if stored.Key != t.Key {
			continue
		}
		if stored.Hierarchical && !t.Hierarchical && nested > 0 {
			return content.Type{}, content.NestingInUse(t.Key, nested)
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

// Nested returns how many items of the type the scenario's content store holds under a parent.
func (s *memoryTypes) Nested(ctx context.Context, key string) (int, error) {
	if s.content == nil {
		return 0, nil
	}
	items, _, err := s.content.List(ctx, content.Filter{Type: key})
	if err != nil {
		return 0, err
	}
	held := 0
	for _, stored := range items {
		if stored.ParentID != nil {
			held++
		}
	}
	return held, nil
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

// AdoptType takes the plugin's type over as the site's own.
func (s *memoryTypes) AdoptType(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.types {
		if s.types[i].Key == key {
			s.types[i].Origin = ""
			return nil
		}
	}
	return content.ErrTypeNotFound
}

// AdoptGroup takes the plugin's group and its fields over as the site's own.
func (s *memoryTypes) AdoptGroup(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.groups {
		if s.groups[i].Key != key {
			continue
		}
		s.groups[i].Origin = ""
		adoptFields(s.groups[i].Fields)
		return nil
	}
	return content.ErrGroupNotFound
}

// adoptFields clears the plugin origin from the fields and from every field standing inside them.
func adoptFields(fields []content.Field) {
	for i := range fields {
		fields[i].Origin = ""
		adoptFields(fields[i].Fields)
	}
}
