// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"math/rand/v2"
	"regexp"
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

// Revisions returns the item's revisions and autosaves newest first, without their content.
func (s *memoryContent) Revisions(_ context.Context, contentID uuid.UUID) ([]content.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	listed := append([]content.Revision{}, s.revisions[contentID]...)
	for held, parked := range s.autosaves {
		if held.contentID == contentID {
			listed = append(listed, parked)
		}
	}
	for i := range listed {
		listed[i].Content = ""
	}
	slices.SortFunc(listed, newestFirst)
	return listed, nil
}

// newestFirst orders two revisions by when they were written and then by identity, the newest first.
func newestFirst(one, other content.Revision) int {
	return cmp.Or(other.CreatedAt.Compare(one.CreatedAt), bytes.Compare(other.ID[:], one.ID[:]))
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

// DeleteRevision removes the item's revision, reporting [content.ErrRevisionNotFound] or [content.ErrTrashed].
func (s *memoryContent) DeleteRevision(_ context.Context, contentID, revisionID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.editable(contentID); err != nil {
		return err
	}
	held := s.revisions[contentID]
	at := slices.IndexFunc(held, func(stored content.Revision) bool { return stored.ID == revisionID })
	if at < 0 {
		return content.ErrRevisionNotFound
	}
	s.revisions[contentID] = slices.Delete(held, at, at+1)
	return nil
}

// editable reports [content.ErrNotFound] or [content.ErrTrashed] when the item may not be written.
func (s *memoryContent) editable(id uuid.UUID) error {
	stored, found := s.items[id]
	if !found {
		return content.ErrNotFound
	}
	return stored.Editable()
}

// SaveAutosave stores the author's autosave of the item under its first row id, unless the item is in the trash.
func (s *memoryContent) SaveAutosave(_ context.Context, autosave content.Revision) (content.Revision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.editable(autosave.ContentID); err != nil {
		return content.Revision{}, err
	}
	key := autosaveKey{autosave.ContentID, autosave.AuthorID}
	autosave.Kind = content.RevisionKindAutosave
	if parked, found := s.autosaves[key]; found {
		autosave.ID = parked.ID
	}
	s.autosaves[key] = autosave
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
	s.drop(id)
	return nil
}

// drop removes the item together with its revisions and autosaves.
func (s *memoryContent) drop(id uuid.UUID) {
	delete(s.items, id)
	delete(s.revisions, id)
	for held := range s.autosaves {
		if held.contentID == id {
			delete(s.autosaves, held)
		}
	}
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
		s.drop(id)
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
	prefix, err := s.fileable(c, content.AddressPrefix(c.Path, c.Slug))
	if err != nil {
		return content.Content{}, err
	}
	if err := s.valuesDeclared(c); err != nil {
		return content.Content{}, err
	}
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		slug := numberedSlug(c.Slug, attempt)
		if s.addressHeld(content.AddressUnder(prefix, slug), c.ID) {
			continue
		}
		return s.created(c, prefix, slug)
	}
	slug := identifiedSlug(c.Slug, c.ID)
	if s.addressHeld(content.AddressUnder(prefix, slug), c.ID) {
		return content.Content{}, content.ErrSlugTaken
	}
	return s.created(c, prefix, slug)
}

// created stores the item under the slug once its author holds an account and every target it names is allowed.
func (s *memoryContent) created(c content.Content, prefix, slug string) (content.Content, error) {
	if !s.accounts.holdsUser(c.AuthorID) {
		return content.Content{}, fmt.Errorf("memory: create content: author %s holds no account", c.AuthorID)
	}
	if err := s.holdTargets(c, nil); err != nil {
		return content.Content{}, err
	}
	stored := c.Place(prefix, slug)
	s.items[stored.ID] = stored
	return stored, nil
}

// nestable returns the reason the item's type no longer takes it under a parent.
func (s *memoryContent) nestable(c content.Content) error {
	if c.ParentID == nil || s.types.nests(c.Type) {
		return nil
	}
	return content.ErrNotHierarchical
}

// fileable returns the address prefix a new item takes, or the reason its parent cannot hold it.
func (s *memoryContent) fileable(c content.Content, prefix string) (string, error) {
	if err := s.nestable(c); err != nil {
		return "", err
	}
	if c.ParentID == nil {
		return prefix, nil
	}
	return s.parentHolds(*c.ParentID, c.ID)
}

// movable returns the address prefix an edit files the stored item under, or the reason it may not move there.
func (s *memoryContent) movable(c, stored content.Content, prefix string) (string, error) {
	if c.ParentID == nil {
		return prefix, nil
	}
	if err := s.nestable(c); err != nil {
		return "", err
	}
	if sameParent(stored.ParentID, c.ParentID) {
		return prefix, nil
	}
	return s.parentHolds(*c.ParentID, c.ID)
}

// parentHolds returns the parent's address, refusing one gone, in the trash, or under the item.
func (s *memoryContent) parentHolds(id, child uuid.UUID) (string, error) {
	parent, found := s.items[id]
	if !found {
		return "", content.ErrParentType
	}
	if parent.Status == content.StatusTrash {
		return "", content.ErrParentTrashed
	}
	if s.chainHolds(id, child) {
		return "", content.ErrCycle
	}
	return parent.Path, nil
}

// chainHolds reports whether the item or any item above it is the child.
func (s *memoryContent) chainHolds(id, child uuid.UUID) bool {
	seen := make(map[uuid.UUID]bool)
	for !seen[id] {
		if id == child {
			return true
		}
		seen[id] = true
		above, found := s.items[id]
		if !found || above.ParentID == nil {
			return false
		}
		id = *above.ParentID
	}
	return false
}

// valuesDeclared refuses a value whose field no group declares at all.
func (s *memoryContent) valuesDeclared(c content.Content) error {
	declared := s.types.declaredKeys()
	for key := range c.Fields {
		if !declared[key] {
			return fmt.Errorf("%w: %s", content.ErrUnknownField, key)
		}
	}
	return nil
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

// identifiedSlug returns slug carrying the id of the content item holding it.
func identifiedSlug(slug string, id uuid.UUID) string {
	return slug + "-" + strings.ReplaceAll(id.String(), "-", "")
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
			matched = append(matched, content.ListedItem{
				Content: stored, AuthorName: s.accounts.nameOf(stored.AuthorID), ParentTitle: parentTitle(s.items, stored),
			})
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

// parentTitle returns the title of the item's parent, empty for an item with no parent.
func parentTitle(held map[uuid.UUID]content.Content, item content.Content) string {
	if item.ParentID == nil {
		return ""
	}
	return held[*item.ParentID].Title
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

// Update stores the item's editable fields and any snapshot, or reports it missing or stale.
func (s *memoryContent) Update(
	_ context.Context, c content.Content, expectedUpdatedAt time.Time, snapshot *content.Revision, revisionCap int,
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
	prefix, err := s.movable(c, stored, content.AddressPrefix(c.Path, c.Slug))
	if err != nil {
		return content.Content{}, err
	}
	if err := s.valuesDeclared(c); err != nil {
		return content.Content{}, err
	}
	if err := s.holdTargets(c, stored.Fields); err != nil {
		return content.Content{}, err
	}
	slug, err := s.freeSiblingSlug(c)
	if err != nil {
		return content.Content{}, err
	}
	settled := c.Place(prefix, slug)
	if s.addressHeld(settled.Path, c.ID) {
		return content.Content{}, content.ErrSlugTaken
	}
	if err := s.snapshotRevision(c.ID, snapshot, revisionCap); err != nil {
		return content.Content{}, err
	}
	s.items[settled.ID] = settled
	s.carryDescendants(settled, stored.Path)
	return settled, nil
}

// snapshotRevision stores any snapshot of the item and prunes its revisions past the cap, refusing a reused id.
func (s *memoryContent) snapshotRevision(id uuid.UUID, snapshot *content.Revision, revisionCap int) error {
	if snapshot == nil {
		return nil
	}
	if slices.ContainsFunc(s.revisions[id], func(stored content.Revision) bool { return stored.ID == snapshot.ID }) {
		return fmt.Errorf("memory: create revision: revision %s is already stored", snapshot.ID)
	}
	s.revisions[id] = pruned(append(s.revisions[id], *snapshot), revisionCap)
	return nil
}

// pruned returns the revisions with only the newest kept up to the cap, every one kept when the cap is zero.
func pruned(held []content.Revision, revisionCap int) []content.Revision {
	if revisionCap <= 0 || len(held) <= revisionCap {
		return held
	}
	slices.SortFunc(held, newestFirst)
	return held[:revisionCap]
}

// freeSiblingSlug returns the slug the item may carry beside its siblings, or [content.ErrSlugTaken].
func (s *memoryContent) freeSiblingSlug(c content.Content) (string, error) {
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		slug := numberedSlug(c.Slug, attempt)
		if !s.siblingSlugTaken(c, slug) {
			return slug, nil
		}
	}
	return "", content.ErrSlugTaken
}

// siblingSlugTaken reports whether another item of the type under the same parent carries the slug.
func (s *memoryContent) siblingSlugTaken(c content.Content, slug string) bool {
	for _, stored := range s.items {
		if stored.Type == c.Type && stored.Slug == slug && stored.ID != c.ID && sameParent(stored.ParentID, c.ParentID) {
			return true
		}
	}
	return false
}

// sameParent reports whether two parent identities name the same parent, two absent ones included.
func sameParent(one, other *uuid.UUID) bool {
	if one == nil || other == nil {
		return one == other
	}
	return *one == *other
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
	stored = stored.Place(content.AddressPrefix(stored.Path, stored.Slug), stored.Slug+trashSuffix())
	s.items[id] = stored
	for held := range s.autosaves {
		if held.contentID == id {
			delete(s.autosaves, held)
		}
	}
	return stored, nil
}

// trashSuffixAlphabet holds the characters a trashed slug suffix draws from.
const trashSuffixAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// trashSuffixLength is the number of random characters in a trashed slug suffix.
const trashSuffixLength = 8

// trashSuffix returns the marker appended to a trashed content item's slug.
func trashSuffix() string {
	var b strings.Builder
	b.WriteString("-trashed-")
	for range trashSuffixLength {
		b.WriteByte(trashSuffixAlphabet[rand.IntN(len(trashSuffixAlphabet))])
	}
	return b.String()
}

// trashMarker matches the suffix a trashed slug and address carry.
var trashMarker = regexp.MustCompile(`-trashed-[a-z0-9]{8}$`)

// Restore returns a trashed item to draft under its original slug when free, and refuses one out of the trash.
func (s *memoryContent) Restore(_ context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	stored, found := s.items[id]
	if !found {
		return content.Content{}, content.ErrNotFound
	}
	if err := stored.Restore(); err != nil {
		return content.Content{}, err
	}
	stored.UpdatedAt = updatedAt
	restored := stored
	restored.Slug = trashMarker.ReplaceAllString(stored.Slug, "")
	restored.Path = trashMarker.ReplaceAllString(stored.Path, "")
	if s.addressHeld(restored.Path, id) {
		restored = stored
	}
	s.items[id] = restored
	return restored, nil
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
