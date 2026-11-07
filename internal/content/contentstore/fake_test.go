// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// errInjected is the failure the fake answers the entry it is told to fail with.
var errInjected = errors.New("the database stopped answering")

// noFailure tells the fake to answer every entry.
const noFailure = -1

// The entries the fake logs around the statements.
const (
	beginEvent      = "begin"
	commitEvent     = "commit"
	rollbackEvent   = "rollback"
	savepointEvent  = "savepoint"
	releaseEvent    = "release"
	rollbackToEvent = "rollback to savepoint"
)

// suffixShape stands in the log for a trash suffix of the shape the restore statement strips.
const suffixShape = "-trashed-########"

// trashSuffixPattern is the suffix the restore statement strips from a trashed slug and address.
var trashSuffixPattern = regexp.MustCompile(`-trashed-[a-z0-9]{8}$`)

// brokeRule is a statement failure carrying the code and the name of the stored rule it broke.
type brokeRule struct{ code, name string }

// Error returns the code and the name of the broken rule.
func (b brokeRule) Error() string { return b.code + " on " + b.name }

// struckIn is a statement failure carrying a code and the context the engine struck it in.
type struckIn struct{ code, context string }

// Error returns the code and the context of the failure.
func (s struckIn) Error() string { return s.code + " " + s.context }

// readBrokenRule returns the code, the name and the context a test failure carries, and whether err is one.
func readBrokenRule(err error) (typestore.Failure, bool) {
	var broke brokeRule
	if errors.As(err, &broke) {
		return typestore.Failure{Code: broke.code, Name: broke.name}, true
	}
	var struck struckIn
	if errors.As(err, &struck) {
		return typestore.Failure{Code: struck.code, Context: struck.context}, true
	}
	return typestore.Failure{}, false
}

// The failures a statement answers when it breaks the address rule, a relation key or deadlocks on the address check.
var (
	addressTaken        = brokeRule{"unique", "content path"}
	targetGone          = brokeRule{"reference", "relation target"}
	addressDeadlocked   = struckIn{"deadlock", "while checking the content path index"}
	elsewhereDeadlocked = struckIn{"deadlock", "while locking the parent row"}
)

// contentRules lends a rule for the address, one for a relation key and one for a deadlock on the address check.
var contentRules = []typestore.Rule{
	{Name: "content path", Broken: typestore.ContentPathUnique},
	{Code: "reference", Name: "relation target", Broken: typestore.RelationTargetExists},
	{Code: "deadlock", Context: "content path", Broken: typestore.AddressCheckDeadlocked},
}

// step is one entry of the fake's log, a statement with the columns it binds or an event around the statements.
type step struct {
	name string
	args any
}

// String returns the entry as a failure message prints it.
func (s step) String() string {
	if s.args == nil {
		return s.name
	}
	return fmt.Sprintf("%s %+v", s.name, s.args)
}

// identified holds the one identity a statement binds.
type identified struct{ ID uuid.UUID }

// keyed holds the one key a statement binds.
type keyed struct{ Key string }

// pathed holds the one address a statement binds.
type pathed struct{ Path string }

// identities holds the identities a statement reads.
type identities struct{ IDs []uuid.UUID }

// editing holds what an item edit binds: the columns and the stamp the stored row must still carry.
type editing struct {
	Row      ContentRow
	Expected time.Time
}

// stamped holds the item a statement writes and the stamp it writes.
type stamped struct {
	ID uuid.UUID
	At time.Time
}

// trashing holds what marking an item trashed binds, its random suffix standing as its shape.
type trashing struct {
	ID     uuid.UUID
	Suffix string
	At     time.Time
}

// sweeping holds what emptying a type's trash binds, the author named or not.
type sweeping struct {
	Type   string
	Author *uuid.UUID
}

// parenting holds the parent a lock holds and the child filed under it.
type parenting struct{ ID, Child uuid.UUID }

// moving holds what carrying the descendants to a new address binds.
type moving struct {
	ID   uuid.UUID
	Path string
	At   time.Time
}

// paired holds the item and the other identity a revision or autosave statement binds.
type paired struct{ ContentID, ID uuid.UUID }

// pruning holds the item whose revisions a prune trims and how many it keeps.
type pruning struct {
	ContentID uuid.UUID
	Keep      int
}

// clearing holds the item and the relation field whose index rows a write clears.
type clearing struct {
	From    uuid.UUID
	FieldID int
}

// paging holds what a page of related items binds.
type paging struct {
	Target                 uuid.UUID
	FieldID, Limit, Offset int
}

// shelf holds what the fake serves: the stored items, revisions and definitions, and the canned list answers.
type shelf struct {
	items     []ContentRow
	revisions []RevisionRow
	groups    []typestore.GroupRow
	fields    []typestore.FieldRow
	nesting   map[string]bool
	taken     map[string]bool
	gone      map[uuid.UUID]bool
	listed    []ListedRow
	related   []ContentRow
	pointers  []content.Pointer
	targets   []content.Target
	byAuthor  []AuthorCount
	depth     int64
	commits   error
}

// fakeBackend serves a shelf through the statements and the transactions a store sends, logging each entry.
type fakeBackend struct {
	shelf    shelf
	failAt   int
	breaks   map[string]error
	breaksAt map[string]map[int]error
	calls    map[string]int
	log      []step
	open     bool
	saving   bool
	swept    int64
	complain func(format string, args ...any)
}

// newFake returns a fake serving the shelf that fails the entry at the position, reporting misuse on the test.
func newFake(t *testing.T, stocked shelf, failAt int) *fakeBackend {
	return &fakeBackend{
		shelf: stocked, failAt: failAt, breaks: map[string]error{}, breaksAt: map[string]map[int]error{},
		calls: map[string]int{}, complain: t.Errorf,
	}
}

// store returns a store on the engine whose statements and transactions all reach the fake.
func (f *fakeBackend) store(engine dbkit.Engine) *Store {
	return New(Backend{
		Engine:    engine,
		Pool:      fakeQueries{fake: f},
		InTx:      f.inTx,
		Rules:     contentRules,
		ReadError: readBrokenRule,
	})
}

// enter logs the entry, answering with the injected failure where the fake fails, or with what this call breaks.
func (f *fakeBackend) enter(name string, args any) error {
	f.log = append(f.log, step{name: name, args: args})
	f.calls[name]++
	if len(f.log)-1 == f.failAt {
		return errInjected
	}
	if err, held := f.breaksAt[name][f.calls[name]]; held {
		return err
	}
	return f.breaks[name]
}

// inTx runs write on statements bound to one transaction, logging the begin and then the commit or the rollback.
func (f *fakeBackend) inTx(_ context.Context, write func(Tx) error) error {
	if f.open {
		f.complain("a transaction began inside another")
	}
	if err := f.enter(beginEvent, nil); err != nil {
		return err
	}
	f.open = true
	defer f.close()
	if err := write(fakeTx{fakeQueries{fake: f, bound: true}}); err != nil {
		f.log = append(f.log, step{name: rollbackEvent})
		return err
	}
	if err := f.enter(commitEvent, nil); err != nil {
		return err
	}
	return f.shelf.commits
}

// close ends the open transaction.
func (f *fakeBackend) close() {
	f.open = false
}

// fakeTx sends the statements of one write to the fake and opens savepoints on it.
type fakeTx struct {
	fakeQueries
}

// Savepoint runs attempt on a savepoint, releasing it when attempt passes and rolling back to it when it fails.
func (t fakeTx) Savepoint(_ context.Context, attempt func(Queries) error) error {
	f := t.fake
	if f.saving {
		f.complain("a savepoint opened inside another")
	}
	if err := t.send(savepointEvent, nil); err != nil {
		return err
	}
	f.saving = true
	err := attempt(fakeQueries{fake: f, bound: true, inside: true})
	f.saving = false
	if err != nil {
		_ = f.enter(rollbackToEvent, nil)
		return err
	}
	return t.send(releaseEvent, nil)
}

// fakeQueries sends each statement to the fake, on the pool, on the open transaction or on its savepoint.
type fakeQueries struct {
	fake   *fakeBackend
	bound  bool
	inside bool
}

// send logs the statement, flagging one sent on the pool inside a transaction or on a transaction or savepoint over.
func (q fakeQueries) send(name string, args any) error {
	if !q.bound && q.fake.open {
		q.fake.complain("%s went to the pool while a transaction was open", name)
	}
	if q.bound && !q.fake.open {
		q.fake.complain("%s went to a transaction already over", name)
	}
	if q.inside && !q.fake.saving {
		q.fake.complain("%s went to a savepoint already over", name)
	}
	return q.fake.enter(name, args)
}

// answer sends the statement and returns what the shelf answers, or the zero value beside a failure.
func answer[T any](q fakeQueries, name string, args any, value T, missing error) (T, error) {
	if err := q.send(name, args); err != nil {
		var zero T
		return zero, err
	}
	return value, missing
}

// firstRow returns the first row matching, or no row.
func firstRow[T any](rows []T, match func(T) bool) (T, error) {
	for _, row := range rows {
		if match(row) {
			return row, nil
		}
	}
	var none T
	return none, sql.ErrNoRows
}

// rowsMatching returns how many rows match.
func rowsMatching[T any](rows []T, match func(T) bool) int64 {
	var held int64
	for _, row := range rows {
		if match(row) {
			held++
		}
	}
	return held
}

// itemOf returns the stored item carrying the identity, or no row.
func (s shelf) itemOf(id uuid.UUID) (ContentRow, error) {
	return firstRow(s.items, func(row ContentRow) bool { return row.ID == id })
}

// holds reports whether the child stands at or above the item on its chain of parents.
func (s shelf) holds(id, child uuid.UUID) bool {
	for at, err := s.itemOf(id); err == nil; at, err = s.itemOf(*at.ParentID) {
		if at.ID == child {
			return true
		}
		if at.ParentID == nil {
			return false
		}
	}
	return false
}

// trashOf returns how many trashed items of the type the shelf holds, only the author's when one is named.
func (s shelf) trashOf(typeKey string, author *uuid.UUID) int64 {
	return rowsMatching(s.items, func(row ContentRow) bool {
		return row.Type == typeKey && row.Status == "trash" && (author == nil || row.AuthorID == *author)
	})
}

// restored returns the item back as a draft under the address its trash suffix hid, or no row.
func (s shelf) restored(id uuid.UUID, at time.Time) (ContentRow, error) {
	row, err := s.itemOf(id)
	if err != nil {
		return row, err
	}
	row.Status, row.UpdatedAt = "draft", at
	row.Slug = trashSuffixPattern.ReplaceAllString(row.Slug, "")
	row.Path = trashSuffixPattern.ReplaceAllString(row.Path, "")
	if s.taken[row.Path] {
		return ContentRow{}, addressTaken
	}
	return row, nil
}

// descendantPaths returns the addresses the item's descendants move to under the path, those under page alone.
func (s shelf) descendantPaths(id uuid.UUID, path string) []string {
	stored, err := s.itemOf(id)
	if err != nil {
		return nil
	}
	var moved []string
	for _, row := range s.items {
		after, below := strings.CutPrefix(row.Path, stored.Path+"/")
		if below && strings.HasPrefix(path+"/"+after, content.PageWord+"/") {
			moved = append(moved, path+"/"+after)
		}
	}
	return moved
}

// declaredKeys returns the key of every stored field in key order.
func (s shelf) declaredKeys() []string {
	keys := make([]string, len(s.fields))
	for i, f := range s.fields {
		keys[i] = f.Key
	}
	slices.Sort(keys)
	return keys
}

// shapedSuffix returns a suffix of the shape the restore statement strips as its stand-in, and any other as it is.
func shapedSuffix(suffix string) string {
	if trashSuffixPattern.FindString(suffix) == suffix {
		return suffixShape
	}
	return suffix
}

// CreateContent answers with the row as given, or a broken address rule when the shelf holds the address.
func (q fakeQueries) CreateContent(_ context.Context, c ContentRow) (ContentRow, error) {
	var taken error
	if q.fake.shelf.taken[c.Path] {
		taken = addressTaken
	}
	return answer(q, "CreateContent", c, c, taken)
}

// GetContent answers with the stored item carrying the identity, or no row.
func (q fakeQueries) GetContent(_ context.Context, id uuid.UUID) (ContentRow, error) {
	row, err := q.fake.shelf.itemOf(id)
	return answer(q, "GetContent", identified{id}, row, err)
}

// GetPublishedContentByPath answers with the published item at the address, or no row.
func (q fakeQueries) GetPublishedContentByPath(_ context.Context, path string) (ContentRow, error) {
	row, err := firstRow(q.fake.shelf.items, func(row ContentRow) bool {
		return row.Path == path && row.Status == "published"
	})
	return answer(q, "GetPublishedContentByPath", pathed{path}, row, err)
}

// UpdateContent answers with the stored item carrying the edit while its stamp holds, or no row.
func (q fakeQueries) UpdateContent(_ context.Context, c ContentRow, expected time.Time) (ContentRow, error) {
	bound := editing{Row: c, Expected: expected}
	row, err := firstRow(q.fake.shelf.items, func(row ContentRow) bool {
		return row.ID == c.ID && row.UpdatedAt.Equal(expected)
	})
	if err == nil {
		c.Type, c.AuthorID, c.CreatedAt = row.Type, row.AuthorID, row.CreatedAt
		row = c
	}
	return answer(q, "UpdateContent", bound, row, err)
}

// SiblingSlugTaken answers whether another item of the type under the same parent carries the slug.
func (q fakeQueries) SiblingSlugTaken(_ context.Context, s SiblingSlug) (bool, error) {
	taken := rowsMatching(q.fake.shelf.items, func(row ContentRow) bool {
		sameParent := (row.ParentID == nil) == (s.ParentID == nil) &&
			(row.ParentID == nil || *row.ParentID == *s.ParentID)
		return row.Type == s.Type && sameParent && row.Slug == s.Slug && row.ID != s.ID
	})
	return answer(q, "SiblingSlugTaken", s, taken > 0, nil)
}

// CountChildren answers with how many stored items file under the item.
func (q fakeQueries) CountChildren(_ context.Context, id uuid.UUID) (int64, error) {
	held := rowsMatching(q.fake.shelf.items, func(row ContentRow) bool {
		return row.ParentID != nil && *row.ParentID == id
	})
	return answer(q, "CountChildren", identified{id}, held, nil)
}

// ContentDepth answers with the depth the shelf holds.
func (q fakeQueries) ContentDepth(_ context.Context, id uuid.UUID) (int64, error) {
	return answer(q, "ContentDepth", identified{id}, q.fake.shelf.depth, nil)
}

// ValuesOfContent answers with the values the stored item holds, or no row.
func (q fakeQueries) ValuesOfContent(_ context.Context, id uuid.UUID) (content.Values, error) {
	row, err := q.fake.shelf.itemOf(id)
	return answer(q, "ValuesOfContent", identified{id}, row.Fields, err)
}

// TrashContent answers with the stored item marked trashed under the suffix, or no row.
func (q fakeQueries) TrashContent(_ context.Context, id uuid.UUID, suffix string, at time.Time) (ContentRow, error) {
	row, err := q.fake.shelf.itemOf(id)
	if err == nil {
		row.Status, row.Slug, row.Path, row.UpdatedAt = "trash", row.Slug+suffix, row.Path+suffix, at
	}
	return answer(q, "TrashContent", trashing{ID: id, Suffix: shapedSuffix(suffix), At: at}, row, err)
}

// RestoreContent answers with the item back as a draft under its address, a broken address rule, or no row.
func (q fakeQueries) RestoreContent(_ context.Context, id uuid.UUID, at time.Time) (ContentRow, error) {
	row, err := q.fake.shelf.restored(id, at)
	return answer(q, "RestoreContent", stamped{ID: id, At: at}, row, err)
}

// RestoreContentKeepingSlug answers with the item back as a draft under its trashed address, or no row.
func (q fakeQueries) RestoreContentKeepingSlug(_ context.Context, id uuid.UUID, at time.Time) (ContentRow, error) {
	row, err := q.fake.shelf.itemOf(id)
	if err == nil {
		row.Status, row.UpdatedAt = "draft", at
	}
	return answer(q, "RestoreContentKeepingSlug", stamped{ID: id, At: at}, row, err)
}

// DeleteContent answers with how many stored items carry the identity.
func (q fakeQueries) DeleteContent(_ context.Context, id uuid.UUID) (int64, error) {
	gone := rowsMatching(q.fake.shelf.items, func(row ContentRow) bool { return row.ID == id })
	return answer(q, "DeleteContent", identified{id}, gone, nil)
}

// DeleteTrash answers with how many trashed items of the type it removes, keeping the count for the next read.
func (q fakeQueries) DeleteTrash(_ context.Context, typeKey string, author *uuid.UUID) (int64, error) {
	swept := q.fake.shelf.trashOf(typeKey, author)
	deleted, err := answer(q, "DeleteTrash", sweeping{Type: typeKey, Author: author}, swept, nil)
	q.fake.swept = deleted
	return deleted, err
}

// CountTrash answers with how many trashed items of the type the last sweep left.
func (q fakeQueries) CountTrash(_ context.Context, typeKey string) (int64, error) {
	return answer(q, "CountTrash", keyed{typeKey}, q.fake.shelf.trashOf(typeKey, nil)-q.fake.swept, nil)
}

// CountContentByAuthor answers with the counts the shelf holds.
func (q fakeQueries) CountContentByAuthor(_ context.Context, typeKey string) ([]AuthorCount, error) {
	return answer(q, "CountContentByAuthor", keyed{typeKey}, q.fake.shelf.byAuthor, nil)
}

// CountContent answers with how many listed rows the shelf holds.
func (q fakeQueries) CountContent(_ context.Context, n Narrowing) (int64, error) {
	return answer(q, "CountContent", n, int64(len(q.fake.shelf.listed)), nil)
}

// ListContent answers with the listed rows the shelf holds.
func (q fakeQueries) ListContent(_ context.Context, p Page) ([]ListedRow, error) {
	return answer(q, "ListContent", p, q.fake.shelf.listed, nil)
}

// ListNestedContent answers with the listed rows the shelf holds.
func (q fakeQueries) ListNestedContent(_ context.Context, p Page) ([]ListedRow, error) {
	return answer(q, "ListNestedContent", p, q.fake.shelf.listed, nil)
}

// LockContent answers with the stored item carrying the identity, or no row.
func (q fakeQueries) LockContent(_ context.Context, id uuid.UUID) (ContentRow, error) {
	row, err := q.fake.shelf.itemOf(id)
	return answer(q, "LockContent", identified{id}, row, err)
}

// LockTypeNesting answers whether the type nests, or no row for a type the shelf does not hold.
func (q fakeQueries) LockTypeNesting(_ context.Context, typeKey string) (bool, error) {
	nests, held := q.fake.shelf.nesting[typeKey]
	return answer(q, "LockTypeNesting", keyed{typeKey}, nests, missingUnless(held))
}

// LockTypeForMove answers whether the type nests, or no row for a type the shelf does not hold.
func (q fakeQueries) LockTypeForMove(_ context.Context, typeKey string) (bool, error) {
	nests, held := q.fake.shelf.nesting[typeKey]
	return answer(q, "LockTypeForMove", keyed{typeKey}, nests, missingUnless(held))
}

// missingUnless returns no row unless the row is held.
func missingUnless(held bool) error {
	if held {
		return nil
	}
	return sql.ErrNoRows
}

// LockAncestors answers with the items above the item held.
func (q fakeQueries) LockAncestors(_ context.Context, id uuid.UUID) error {
	return q.send("LockAncestors", identified{id})
}

// LockParent answers with the parent's state and address and whether the child stands above it, or no row.
func (q fakeQueries) LockParent(_ context.Context, id, child uuid.UUID) (ParentRow, error) {
	row, err := q.fake.shelf.itemOf(id)
	parent := ParentRow{Status: row.Status, Path: row.Path, HoldsChild: q.fake.shelf.holds(id, child)}
	return answer(q, "LockParent", parenting{ID: id, Child: child}, parent, err)
}

// LockDescendants answers with the items below the item held.
func (q fakeQueries) LockDescendants(_ context.Context, id uuid.UUID) error {
	return q.send("LockDescendants", identified{id})
}

// MoveDescendants answers with the addresses under page the descendants move to.
func (q fakeQueries) MoveDescendants(_ context.Context, id uuid.UUID, path string, at time.Time) ([]string, error) {
	moved := q.fake.shelf.descendantPaths(id, path)
	return answer(q, "MoveDescendants", moving{ID: id, Path: path, At: at}, moved, nil)
}

// LockDeclaredFieldKeys answers with the key of every stored field in key order.
func (q fakeQueries) LockDeclaredFieldKeys(context.Context) ([]string, error) {
	return answer(q, "LockDeclaredFieldKeys", nil, q.fake.shelf.declaredKeys(), nil)
}

// DeferAddressCheck answers with the address check deferred.
func (q fakeQueries) DeferAddressCheck(context.Context) error {
	return q.send("DeferAddressCheck", nil)
}

// CreateRevision answers with the revision stored.
func (q fakeQueries) CreateRevision(_ context.Context, r RevisionRow) error {
	return q.send("CreateRevision", r)
}

// ListRevisions answers with the item's stored revisions, without their content or values.
func (q fakeQueries) ListRevisions(_ context.Context, contentID uuid.UUID) ([]RevisionRow, error) {
	var listed []RevisionRow
	for _, row := range q.fake.shelf.revisions {
		if row.ContentID == contentID && row.Kind == "revision" {
			row.Content, row.Fields = "", nil
			listed = append(listed, row)
		}
	}
	return answer(q, "ListRevisions", identified{contentID}, listed, nil)
}

// GetRevision answers with the item's stored revision carrying the identity, or no row.
func (q fakeQueries) GetRevision(_ context.Context, contentID, id uuid.UUID) (RevisionRow, error) {
	row, err := firstRow(q.fake.shelf.revisions, func(row RevisionRow) bool {
		return row.ContentID == contentID && row.ID == id && row.Kind == "revision"
	})
	return answer(q, "GetRevision", paired{ContentID: contentID, ID: id}, row, err)
}

// DeleteRevision answers with how many of the item's stored revisions carry the identity.
func (q fakeQueries) DeleteRevision(_ context.Context, contentID, id uuid.UUID) (int64, error) {
	gone := rowsMatching(q.fake.shelf.revisions, func(row RevisionRow) bool {
		return row.ContentID == contentID && row.ID == id && row.Kind == "revision"
	})
	return answer(q, "DeleteRevision", paired{ContentID: contentID, ID: id}, gone, nil)
}

// PruneRevisions answers with the item's revisions past the cap removed.
func (q fakeQueries) PruneRevisions(_ context.Context, contentID uuid.UUID, keep int) error {
	return q.send("PruneRevisions", pruning{ContentID: contentID, Keep: keep})
}

// UpsertAutosave answers with the autosave as given, marked as an autosave.
func (q fakeQueries) UpsertAutosave(_ context.Context, r RevisionRow) (RevisionRow, error) {
	stored := r
	stored.Kind = "autosave"
	return answer(q, "UpsertAutosave", r, stored, nil)
}

// GetAutosave answers with the author's stored autosave of the item, or no row.
func (q fakeQueries) GetAutosave(_ context.Context, contentID, authorID uuid.UUID) (RevisionRow, error) {
	row, err := firstRow(q.fake.shelf.revisions, func(row RevisionRow) bool {
		return row.ContentID == contentID && row.AuthorID == authorID && row.Kind == "autosave"
	})
	return answer(q, "GetAutosave", paired{ContentID: contentID, ID: authorID}, row, err)
}

// DeleteAutosave answers with the author's autosave of the item removed.
func (q fakeQueries) DeleteAutosave(_ context.Context, contentID, authorID uuid.UUID) error {
	return q.send("DeleteAutosave", paired{ContentID: contentID, ID: authorID})
}

// DeleteAutosavesOfContent answers with every autosave of the item removed.
func (q fakeQueries) DeleteAutosavesOfContent(_ context.Context, contentID uuid.UUID) error {
	return q.send("DeleteAutosavesOfContent", identified{contentID})
}

// TypesOfContent answers with the type of each stored item the identities name.
func (q fakeQueries) TypesOfContent(_ context.Context, ids []uuid.UUID) ([]TypedItem, error) {
	var typed []TypedItem
	for _, row := range q.fake.shelf.items {
		if slices.Contains(ids, row.ID) {
			typed = append(typed, TypedItem{ID: row.ID, Type: row.Type})
		}
	}
	return answer(q, "TypesOfContent", identities{slices.Clone(ids)}, typed, nil)
}

// ClearRelationsOfField answers with the index rows of the field cleared.
func (q fakeQueries) ClearRelationsOfField(_ context.Context, from uuid.UUID, fieldID int) error {
	return q.send("ClearRelationsOfField", clearing{From: from, FieldID: fieldID})
}

// AddRelation answers with the index row stored, or a broken relation key when its target is gone.
func (q fakeQueries) AddRelation(_ context.Context, r RelationRow) error {
	if err := q.send("AddRelation", r); err != nil {
		return err
	}
	if q.fake.shelf.gone[r.ToID] {
		return targetGone
	}
	return nil
}

// RefreshRelationVisibility answers with what a term page reads refreshed.
func (q fakeQueries) RefreshRelationVisibility(_ context.Context, id uuid.UUID) error {
	return q.send("RefreshRelationVisibility", identified{id})
}

// CountRelatedContent answers with how many related items the shelf holds.
func (q fakeQueries) CountRelatedContent(_ context.Context, target uuid.UUID) (int64, error) {
	return answer(q, "CountRelatedContent", identified{target}, int64(len(q.fake.shelf.related)), nil)
}

// ListRelatedContent answers with the related items the shelf holds.
func (q fakeQueries) ListRelatedContent(_ context.Context, target uuid.UUID, limit, offset int) ([]ContentRow, error) {
	args := paging{Target: target, Limit: limit, Offset: offset}
	return answer(q, "ListRelatedContent", args, q.fake.shelf.related, nil)
}

// CountPointingAt answers with how many pointing items the shelf holds.
func (q fakeQueries) CountPointingAt(_ context.Context, target uuid.UUID, fieldID int) (int64, error) {
	args := paging{Target: target, FieldID: fieldID}
	return answer(q, "CountPointingAt", args, int64(len(q.fake.shelf.pointers)), nil)
}

// PointingAt answers with the pointing items the shelf holds.
func (q fakeQueries) PointingAt(
	_ context.Context, target uuid.UUID, fieldID, limit, offset int,
) ([]content.Pointer, error) {
	args := paging{Target: target, FieldID: fieldID, Limit: limit, Offset: offset}
	return answer(q, "PointingAt", args, q.fake.shelf.pointers, nil)
}

// SummariesOfTargets answers with the target summaries the shelf holds.
func (q fakeQueries) SummariesOfTargets(_ context.Context, ids []uuid.UUID) ([]content.Target, error) {
	return answer(q, "SummariesOfTargets", identities{slices.Clone(ids)}, q.fake.shelf.targets, nil)
}

// ListFieldGroups answers with every stored group.
func (q fakeQueries) ListFieldGroups(context.Context) ([]typestore.GroupRow, error) {
	return answer(q, "ListFieldGroups", nil, q.fake.shelf.groups, nil)
}

// ListContentFields answers with every stored field.
func (q fakeQueries) ListContentFields(context.Context) ([]typestore.FieldRow, error) {
	return answer(q, "ListContentFields", nil, q.fake.shelf.fields, nil)
}

func TestTheFakeFlagsAStatementSentOutsideTheTransactionItBelongsTo(t *testing.T) {
	t.Parallel()

	for name, misuse := range map[string]func(ctx context.Context, f *fakeBackend){
		"a pool statement inside a transaction": func(ctx context.Context, f *fakeBackend) {
			_ = f.inTx(ctx, func(Tx) error { return fakeQueries{fake: f}.LockAncestors(ctx, aboutID) })
		},
		"a statement on a transaction already over": func(ctx context.Context, f *fakeBackend) {
			var leaked Tx
			_ = f.inTx(ctx, func(bound Tx) error {
				leaked = bound
				return nil
			})
			_ = leaked.LockAncestors(ctx, aboutID)
		},
		"a transaction inside another": func(ctx context.Context, f *fakeBackend) {
			_ = f.inTx(ctx, func(Tx) error {
				return f.inTx(ctx, func(Tx) error { return nil })
			})
		},
		"a statement on a savepoint already over": func(ctx context.Context, f *fakeBackend) {
			_ = f.inTx(ctx, func(bound Tx) error {
				var leaked Queries
				_ = bound.Savepoint(ctx, func(attempt Queries) error {
					leaked = attempt
					return nil
				})
				return leaked.LockAncestors(ctx, aboutID)
			})
		},
		"a savepoint inside another": func(ctx context.Context, f *fakeBackend) {
			_ = f.inTx(ctx, func(bound Tx) error {
				return bound.Savepoint(ctx, func(Queries) error {
					return bound.Savepoint(ctx, func(Queries) error { return nil })
				})
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFake(t, site(), noFailure)
			var flagged []string
			f.complain = func(format string, args ...any) { flagged = append(flagged, fmt.Sprintf(format, args...)) }

			misuse(t.Context(), f)

			if len(flagged) != 1 {
				t.Errorf("flagged = %q, want the misuse flagged once", flagged)
			}
		})
	}
}

func TestTheFakeSavepointIgnoresAFailedRollbackToItAndAnswersTheAttemptsError(t *testing.T) {
	t.Parallel()

	f := newFake(t, site(), 2)
	errAttempt := errors.New("the attempt failed")

	err := f.inTx(t.Context(), func(bound Tx) error {
		return bound.Savepoint(t.Context(), func(Queries) error { return errAttempt })
	})

	if !errors.Is(err, errAttempt) {
		t.Errorf("error = %v, want the attempt's error", err)
	}
	checkLog(t, f.log, []step{begun, savepointOpened, rolledBackToSavepoint, rolledBack})
}
