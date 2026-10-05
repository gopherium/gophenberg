// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
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
	types     *memoryTypes
	accounts  *memoryStore
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

// identitiesIn returns every identity a stored value names, descending into containers.
func identitiesIn(value any) []uuid.UUID {
	switch held := value.(type) {
	case []any:
		var found []uuid.UUID
		for _, member := range held {
			found = append(found, identitiesIn(member)...)
		}
		return found
	case map[string]any:
		var found []uuid.UUID
		for _, member := range held {
			found = append(found, identitiesIn(member)...)
		}
		return found
	case string:
		if id, err := uuid.Parse(held); err == nil {
			return []uuid.UUID{id}
		}
	}
	return nil
}

// holdTargets reports whether every target exists and is the type its field points at.
func (s *memoryContent) holdTargets(c content.Content, before content.Values) error {
	if s.types == nil {
		return nil
	}
	declared, err := s.types.ByKey(context.Background(), c.Type)
	if err != nil {
		return nil
	}
	pointing, err := content.HeldTargets(declared.Fields, c.Fields)
	if err != nil {
		return err
	}
	kept := content.HeldIdentities(declared.Fields, before)
	for _, ft := range pointing {
		if err := s.targetsAllowed(ft, kept[ft.Field.ID]); err != nil {
			return err
		}
	}
	return nil
}

// targetsAllowed reports whether every target the field names may be stored.
func (s *memoryContent) targetsAllowed(ft content.FieldTargets, kept map[uuid.UUID]bool) error {
	for _, target := range ft.Targets {
		held, found := s.items[target]
		if !found {
			if kept[target] {
				continue
			}
			return fmt.Errorf("%w: %s", content.ErrTargetNotFound, target)
		}
		if held.Type != ft.Field.RelatesTo {
			return fmt.Errorf("%w: %s holds %s", content.ErrTargetType, ft.Field.Key, held.Type)
		}
	}
	return nil
}

// clearRelation drops the field's targets from every item of the type.
func (s *memoryContent) clearRelation(typeKey, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, stored := range s.items {
		if stored.Type != typeKey {
			continue
		}
		delete(stored.Fields, key)
		s.items[id] = stored
	}
}

// sweepPath takes what stands at the path out of every item of the types, their revisions and their autosaves.
func (s *memoryContent) sweepPath(typeKeys []string, path []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id, stored := range s.items {
		if !slices.Contains(typeKeys, stored.Type) {
			continue
		}
		strippedAt(stored.Fields, path)
		for i := range s.revisions[id] {
			strippedAt(s.revisions[id][i].Fields, path)
		}
		for held, parked := range s.autosaves {
			if held.contentID == id {
				strippedAt(parked.Fields, path)
			}
		}
	}
}

// strippedAt removes what stands at the path inside the values, walking into every row on the way.
func strippedAt(values map[string]any, path []string) {
	if len(path) == 1 {
		delete(values, path[0])
		return
	}
	switch inside := values[path[0]].(type) {
	case map[string]any:
		strippedAt(inside, path[1:])
	case []any:
		for _, row := range inside {
			if held, ok := row.(map[string]any); ok {
				strippedAt(held, path[1:])
			}
		}
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

// Delete removes the item outright, leaving what pointed at it to drop the identity on read.
func (s *memoryContent) Delete(_ context.Context, id uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, found := s.items[id]; !found {
		return content.ErrNotFound
	}
	delete(s.items, id)
	return nil
}

// EmptyTrash removes the trashed items of the type, only the author's when one is named, counting what it left.
func (s *memoryContent) EmptyTrash(_ context.Context, contentType string, author *uuid.UUID) (int, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted, kept := 0, 0
	for id, stored := range s.items {
		if stored.Type != contentType || stored.Status != content.StatusTrash {
			continue
		}
		if author != nil && stored.AuthorID != *author {
			kept++
			continue
		}
		delete(s.items, id)
		deleted++
	}
	return deleted, kept, nil
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

// List returns the items the filter matches, sorted, nested and paged as it asks, with their total.
func (s *memoryContent) List(_ context.Context, f content.Filter) ([]content.ListedItem, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := make([]content.ListedItem, 0, len(s.items))
	for _, stored := range s.items {
		if stored.Type == f.Type && keeps(f, stored) && narrowed(stored.Fields, f.Fields) {
			matched = append(matched, content.ListedItem{Content: stored, AuthorName: s.accounts.nameOf(stored.AuthorID)})
		}
	}
	slices.SortFunc(matched, func(a, b content.ListedItem) int {
		return cmp.Or(listOrder(f, s.items, a, b), bytes.Compare(b.ID[:], a.ID[:]))
	})
	if f.Hierarchy {
		matched = nestedUnderParents(matched)
	}
	return paged(matched, f), len(matched), nil
}

// keeps reports whether the item stands within the statuses, authors, dates and words the filter narrows to.
func keeps(f content.Filter, item content.Content) bool {
	at := listDate(item)
	switch {
	case len(f.Statuses) > 0 && !slices.Contains(f.Statuses, item.Status):
		return false
	case len(f.Authors) > 0 && !slices.Contains(f.Authors, item.AuthorID):
		return false
	case slices.Contains(f.ExcludeAuthors, item.AuthorID):
		return false
	case f.Before != nil && !at.Before(*f.Before), f.After != nil && !at.After(*f.After):
		return false
	}
	search := strings.ToLower(f.Search)
	return strings.Contains(strings.ToLower(item.Title), search) || strings.Contains(strings.ToLower(item.Content), search)
}

// listDate returns the date a listing shows, sorts and narrows an item by.
func listDate(item content.Content) time.Time {
	if item.PublishedAt != nil {
		return *item.PublishedAt
	}
	return item.UpdatedAt
}

// listOrder compares two listed items by the column and in the direction the filter names.
func listOrder(f content.Filter, held map[uuid.UUID]content.Content, a, b content.ListedItem) int {
	var by int
	switch f.OrderBy {
	case content.OrderByTitle:
		by = collated(a.Title, b.Title)
	case content.OrderByAuthor:
		by = collated(a.AuthorName, b.AuthorName)
	case content.OrderBySlug:
		by = collated(a.Slug, b.Slug)
	case content.OrderByParent:
		by = parentWritten(held, a.Content).Compare(parentWritten(held, b.Content))
	default:
		by = listDate(a.Content).Compare(listDate(b.Content))
	}
	if f.Order != content.OrderAsc {
		return -by
	}
	return by
}

// collated compares two words the way the database collation does, case first set aside.
func collated(a, b string) int {
	return cmp.Or(strings.Compare(strings.ToLower(a), strings.ToLower(b)), strings.Compare(a, b))
}

// parentWritten returns when the item's parent was written, the zero time standing for no parent.
func parentWritten(held map[uuid.UUID]content.Content, item content.Content) time.Time {
	if item.ParentID == nil {
		return time.Time{}
	}
	return held[*item.ParentID].CreatedAt
}

// nestedUnderParents returns the sorted items each under its parent, the children of a parent left out last.
func nestedUnderParents(sorted []content.ListedItem) []content.ListedItem {
	present := make(map[uuid.UUID]bool, len(sorted))
	for _, item := range sorted {
		present[item.ID] = true
	}
	children := make(map[uuid.UUID][]content.ListedItem)
	var tops []content.ListedItem
	var strays []uuid.UUID
	for _, item := range sorted {
		if item.ParentID == nil {
			tops = append(tops, item)
			continue
		}
		if !present[*item.ParentID] && len(children[*item.ParentID]) == 0 {
			strays = append(strays, *item.ParentID)
		}
		children[*item.ParentID] = append(children[*item.ParentID], item)
	}
	ordered := make([]content.ListedItem, 0, len(sorted))
	var walk func([]content.ListedItem)
	walk = func(items []content.ListedItem) {
		for _, item := range items {
			ordered = append(ordered, item)
			walk(children[item.ID])
		}
	}
	walk(tops)
	for _, parent := range strays {
		walk(children[parent])
	}
	return ordered
}

// narrowed reports whether the stored values hold every term the filter names.
func narrowed(values content.Values, terms map[string]any) bool {
	for key, term := range terms {
		if !holdsTerm(values[key], term) {
			return false
		}
	}
	return true
}

// holdsTerm reports whether a stored value carries the term, a list carrying it as a member.
func holdsTerm(held, term any) bool {
	wanted, listed := term.([]any)
	if !listed {
		return held == term
	}
	members, ok := held.([]any)
	if !ok {
		return false
	}
	for _, want := range wanted {
		if !among(members, want) {
			return false
		}
	}
	return true
}

// among reports whether the members carry the wanted value.
func among(members []any, wanted any) bool {
	for _, member := range members {
		if member == wanted {
			return true
		}
	}
	return false
}

// paged returns the page of items the filter asks for.
func paged[T any](matched []T, f content.Filter) []T {
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
	if err := s.holdTargets(c, stored.Fields); err != nil {
		return content.Content{}, err
	}
	prefix := content.AddressPrefix(c.Path, c.Slug)
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		slug := numberedSlug(c.Slug, attempt)
		if s.addressHeld(content.AddressUnder(prefix, slug), c.ID) {
			continue
		}
		if snapshot != nil {
			s.revisions[c.ID] = append(s.revisions[c.ID], *snapshot)
		}
		settled := c.Place(prefix, slug)
		s.items[settled.ID] = settled
		s.carryDescendants(settled, stored.Path)
		return settled, nil
	}
	return content.Content{}, content.ErrSlugTaken
}

// Trash marks the item trashed, frees its address and drops its parked words, or refuses while it holds children.
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
		return content.Content{}, content.Refuse(content.ErrInvalidTransition, "content_already_trashed",
			"content: the item is already in the trash", nil)
	}
	stored.Status, stored.UpdatedAt = content.StatusTrash, updatedAt
	stored = stored.Place(content.AddressPrefix(stored.Path, stored.Slug), stored.Slug+"-trashed")
	s.items[id] = stored
	for held := range s.autosaves {
		if held.contentID == id {
			delete(s.autosaves, held)
		}
	}
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

// RelatedTo returns the published items of active types pointing at the target, newest first.
func (s *memoryContent) RelatedTo(
	_ context.Context, target uuid.UUID, page, perPage int,
) ([]content.Content, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := make([]content.Content, 0, len(s.items))
	for _, stored := range s.items {
		if stored.Status != content.StatusPublished || !pointsAt(stored.Fields, target) {
			continue
		}
		if s.types != nil && !s.types.serving(stored.Type) {
			continue
		}
		matched = append(matched, stored)
	}
	slices.SortFunc(matched, func(a, b content.Content) int {
		if held := sortedAt(b).Compare(sortedAt(a)); held != 0 {
			return held
		}
		return strings.Compare(a.ID.String(), b.ID.String())
	})
	return paged(matched, content.Filter{Page: page, PerPage: perPage}), len(matched), nil
}

// pointsAt reports whether any value the item holds names the target.
func pointsAt(held content.Values, target uuid.UUID) bool {
	for _, value := range held {
		if slices.Contains(identitiesIn(value), target) {
			return true
		}
	}
	return false
}

// sortedAt returns the stamp a term page orders an item by.
func sortedAt(c content.Content) time.Time {
	if c.PublishedAt != nil {
		return *c.PublishedAt
	}
	return c.CreatedAt
}

// PointingAt returns the published items pointing at the target through the field, and how many there are.
func (s *memoryContent) PointingAt(
	ctx context.Context, target uuid.UUID, field, page, perPage int,
) ([]content.Pointer, int, error) {
	key, named := s.fieldKeyed(ctx, field)
	if !named {
		return nil, 0, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	held := make([]content.Pointer, 0, len(s.items))
	for _, item := range s.items {
		if item.Status != content.StatusPublished || !slices.Contains(identitiesIn(item.Fields[key]), target) {
			continue
		}
		held = append(held, content.Pointer{
			ID: item.ID, Type: item.Type, Title: item.Title, Path: item.Path,
		})
	}
	slices.SortFunc(held, func(one, other content.Pointer) int {
		return strings.Compare(other.ID.String(), one.ID.String())
	})
	return pagedPointers(held, page, perPage), len(held), nil
}

// fieldKeyed returns the key the field identity names, or reports that no declared field carries it.
func (s *memoryContent) fieldKeyed(ctx context.Context, field int) (string, bool) {
	if s.types == nil {
		return "", false
	}
	groups, err := s.types.ListGroups(ctx)
	if err != nil {
		return "", false
	}
	for _, g := range groups {
		for _, f := range g.Fields {
			if f.ID == field {
				return f.Key, true
			}
		}
	}
	return "", false
}

// pagedPointers returns the page of pointers the numbers ask for.
func pagedPointers(held []content.Pointer, page, perPage int) []content.Pointer {
	from := (page - 1) * perPage
	if from >= len(held) {
		return nil
	}
	return held[from:min(from+perPage, len(held))]
}

// TargetsByIDs returns the published items of active types the identities name.
func (s *memoryContent) TargetsByIDs(_ context.Context, ids []uuid.UUID) ([]content.Target, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	held := make([]content.Target, 0, len(ids))
	for _, id := range ids {
		pointed, stored := s.items[id]
		if !stored || pointed.Status != content.StatusPublished {
			continue
		}
		if s.types != nil && !s.types.serving(pointed.Type) {
			continue
		}
		held = append(held, content.Target{ID: pointed.ID, Title: pointed.Title, Path: pointed.Path})
	}
	return held, nil
}
