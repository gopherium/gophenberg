// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// errInjected is the failure the fake answers the entry it is told to fail with.
var errInjected = errors.New("the database stopped answering")

// mintedStamp stands in a log or a value for an instant a step read from the clock during the call.
var mintedStamp = time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)

// noFailure tells the fake to answer every entry.
const noFailure = -1

// The entries the fake logs around the statements.
const (
	beginEvent    = "begin"
	commitEvent   = "commit"
	rollbackEvent = "rollback"
	checkEvent    = "check"
)

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

// keyed holds the one key a statement binds.
type keyed struct{ Key string }

// identified holds the one identity a statement binds.
type identified struct{ ID int }

// stamping holds the key an adoption binds and the stamp it writes.
type stamping struct {
	Key string
	At  time.Time
}

// retyped holds what moving the addresses of a type binds.
type retyped struct {
	Key, RouteWord, Was string
	UpdatedAt           time.Time
}

// outsideOf holds what counting the addresses of a type that do not start with a route word binds.
type outsideOf struct{ Key, Was string }

// groupColumns holds the columns a group write binds, its location as text, and the stamp an edit expects.
type groupColumns struct {
	ID                             int
	Key, Title, Location, Origin   string
	Active                         bool
	CreatedAt, UpdatedAt, Expected time.Time
}

// fieldColumns holds the columns a field statement binds, its settings as text.
type fieldColumns struct {
	ID, GroupID, ParentID, Depth                  int
	Key, Label, Kind, RelatesTo, Origin, Settings string
	Many, Required                                bool
	CreatedAt, UpdatedAt, Expected                time.Time
}

// ordering holds what a reorder binds: the group or the container and the order asked.
type ordering struct {
	GroupID, ParentID int
	IDs               []int
	Keys              []string
}

// reparenting holds what standing a field in another place binds.
type reparenting struct {
	ID, ToGroup, ToParent int
	UpdatedAt             time.Time
}

// recounting holds what recounting the depths below a moved field binds.
type recounting struct{ ID, ToGroup, Depth int }

// clearing holds the key a value sweep removes and the types it sweeps.
type clearing struct {
	Key   string
	Types []string
}

// relating holds the types and the relation fields an index sweep removes the rows of.
type relating struct {
	Types  []string
	Fields []int
}

// checked holds what the caller's check was handed: the path of every group and field, and the type keys.
type checked struct{ Paths, Types []string }

// shelf holds what the fake serves: the types, groups and fields, nesting and outside counts and retyped addresses.
type shelf struct {
	types   []TypeRow
	groups  []GroupRow
	fields  []FieldRow
	nested  map[string]int64
	carried map[string][]string
	outside map[string]OutsideRow
}

// fakeBackend serves a shelf through the statements and the transactions a store sends, logging each entry.
type fakeBackend struct {
	shelf    shelf
	failAt   int
	from, to time.Time
	log      []step
	minted   []time.Time
	checks   int
	open     bool
	complain func(format string, args ...any)
}

// newFake returns a fake serving the shelf that fails the entry at the position, reporting misuse on the test.
func newFake(t *testing.T, stocked shelf, failAt int) *fakeBackend {
	return &fakeBackend{shelf: stocked, failAt: failAt, from: time.Now(), complain: t.Errorf}
}

// store returns a store on the SQLite engine whose statements and transactions all reach the fake.
func (f *fakeBackend) store() *Store {
	return New(Backend{
		Engine:    dbkit.SQLite,
		Pool:      fakeQueries{fake: f},
		InTx:      f.inTx,
		Rules:     tailRules,
		ReadError: readBrokenRule,
	})
}

// enter logs the entry, answering with the injected failure when the entry stands where the fake fails.
func (f *fakeBackend) enter(name string, args any) error {
	f.log = append(f.log, step{name: name, args: args})
	if len(f.log)-1 == f.failAt {
		return errInjected
	}
	return nil
}

// inTx runs write on statements bound to one transaction, logging the begin and then the commit or the rollback.
func (f *fakeBackend) inTx(_ context.Context, write func(Queries) error) error {
	if f.open {
		f.complain("a transaction began inside another")
	}
	if err := f.enter(beginEvent, nil); err != nil {
		return err
	}
	f.open = true
	defer f.close()
	if err := write(fakeQueries{fake: f, bound: true}); err != nil {
		f.log = append(f.log, step{name: rollbackEvent})
		return err
	}
	return f.enter(commitEvent, nil)
}

// close ends the open transaction.
func (f *fakeBackend) close() {
	f.open = false
}

// recheck returns a caller's check that logs what it was handed and passes.
func (f *fakeBackend) recheck() content.Recheck {
	return func(groups []content.Group, types []content.Type) error {
		f.checks++
		f.log = append(f.log, step{name: checkEvent, args: checked{Paths: outline(groups), Types: typeKeysOf(types)}})
		return nil
	}
}

// settled returns the minted stand-in for an instant read during the call, and any other time as it is.
func (f *fakeBackend) settled(at time.Time) time.Time {
	if at.Before(f.from) {
		return at
	}
	return mintedStamp
}

// stamp returns the time as the log holds it, keeping each instant read during the call.
func (f *fakeBackend) stamp(at time.Time) time.Time {
	if !at.Before(f.from) {
		f.minted = append(f.minted, at)
	}
	return f.settled(at)
}

// unminted returns a group with each stamp read during the call standing as the minted stand-in, any other value as is.
func (f *fakeBackend) unminted(value any) any {
	group, isGroup := value.(content.Group)
	if !isGroup {
		return value
	}
	group.CreatedAt, group.UpdatedAt = f.settled(group.CreatedAt), f.settled(group.UpdatedAt)
	return group
}

// created returns the columns a field create binds.
func (f *fakeBackend) created(row FieldRow) fieldColumns {
	return fieldColumns{
		GroupID: row.GroupID, Key: row.Key, Label: row.Label, Kind: row.Kind, RelatesTo: row.RelatesTo,
		Origin: row.Origin, Settings: string(row.Settings), Many: row.Many, Required: row.Required,
		CreatedAt: f.stamp(row.CreatedAt), UpdatedAt: f.stamp(row.UpdatedAt),
	}
}

// edit returns the columns a field edit binds beside the field it names.
func (f *fakeBackend) edit(row FieldRow, expected time.Time) fieldColumns {
	return fieldColumns{
		Label: row.Label, Required: row.Required, Settings: string(row.Settings),
		UpdatedAt: f.stamp(row.UpdatedAt), Expected: f.stamp(expected),
	}
}

// outline returns the key of each group followed by the path of every field it holds, however deep.
func outline(groups []content.Group) []string {
	var paths []string
	for _, g := range groups {
		paths = pathsOf(append(paths, g.Key), g.Key, g.Fields)
	}
	return paths
}

// pathsOf appends the path of each field under the prefix, every field before the fields it holds.
func pathsOf(paths []string, prefix string, fields []content.Field) []string {
	for _, f := range fields {
		path := prefix + "/" + f.Key
		paths = pathsOf(append(paths, path), path, f.Fields)
	}
	return paths
}

// typeKeysOf returns the key of each type.
func typeKeysOf(types []content.Type) []string {
	keys := make([]string, len(types))
	for i, t := range types {
		keys[i] = t.Key
	}
	return keys
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

// nextNumber returns one past the highest number the rows carry.
func nextNumber[T any](rows []T, number func(T) int) int {
	highest := 0
	for _, row := range rows {
		highest = max(highest, number(row))
	}
	return highest + 1
}

// copied returns the values in a slice of their own, nothing for none.
func copied[T any](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	return slices.Clone(values)
}

// byID returns a match for the field carrying the identity.
func byID(id int) func(FieldRow) bool {
	return func(row FieldRow) bool { return row.ID == id }
}

// typeUnder returns the stored type under the key, or no row.
func (s shelf) typeUnder(key string) (TypeRow, error) {
	return firstRow(s.types, func(t TypeRow) bool { return t.Key == key })
}

// typesUnder returns how many stored types hold the key.
func (s shelf) typesUnder(key string) int64 {
	return rowsMatching(s.types, func(t TypeRow) bool { return t.Key == key })
}

// typeKeys returns the key of every stored type in stored order.
func (s shelf) typeKeys() []string {
	keys := make([]string, len(s.types))
	for i, t := range s.types {
		keys[i] = t.Key
	}
	return keys
}

// groupKeys returns the key of every stored group in key order.
func (s shelf) groupKeys() []string {
	keys := make([]string, len(s.groups))
	for i, g := range s.groups {
		keys[i] = g.Key
	}
	slices.Sort(keys)
	return keys
}

// edited returns the first stored field matching, carrying the label, required flag, settings and stamp of the edit.
func (s shelf) edited(edit FieldRow, match func(FieldRow) bool) (FieldRow, error) {
	row, err := firstRow(s.fields, match)
	if err != nil {
		return row, err
	}
	row.Label, row.Required, row.Settings, row.UpdatedAt = edit.Label, edit.Required, edit.Settings, edit.UpdatedAt
	return row, nil
}

// nextFieldID returns the identity the next stored field takes.
func (s shelf) nextFieldID() int {
	return nextNumber(s.fields, func(row FieldRow) int { return row.ID })
}

// fakeQueries sends each statement to the fake, on the pool or on the open transaction.
type fakeQueries struct {
	fake  *fakeBackend
	bound bool
}

// send logs the statement, flagging one sent on the pool inside a transaction or on a transaction already over.
func (q fakeQueries) send(name string, args any) error {
	if !q.bound && q.fake.open {
		q.fake.complain("%s went to the pool while a transaction was open", name)
	}
	if q.bound && !q.fake.open {
		q.fake.complain("%s went to a transaction already over", name)
	}
	return q.fake.enter(name, args)
}

// answer sends the statement and returns what the shelf answers, or the zero value beside the injected failure.
func answer[T any](q fakeQueries, name string, args any, value T, missing error) (T, error) {
	if err := q.send(name, args); err != nil {
		var zero T
		return zero, err
	}
	return value, missing
}

// ListContentTypes answers with every stored type.
func (q fakeQueries) ListContentTypes(context.Context) ([]TypeRow, error) {
	return answer(q, "ListContentTypes", nil, copied(q.fake.shelf.types), nil)
}

// GetContentType answers with the stored type under the key, or no row.
func (q fakeQueries) GetContentType(_ context.Context, key string) (TypeRow, error) {
	row, err := q.fake.shelf.typeUnder(key)
	return answer(q, "GetContentType", keyed{key}, row, err)
}

// CreateContentType answers with the type as given, logging every column it binds.
func (q fakeQueries) CreateContentType(_ context.Context, t TypeRow) (TypeRow, error) {
	bound := t
	bound.CreatedAt, bound.UpdatedAt = q.fake.stamp(t.CreatedAt), q.fake.stamp(t.UpdatedAt)
	return answer(q, "CreateContentType", bound, t, nil)
}

// LockContentType answers with the stored type under the key, or no row.
func (q fakeQueries) LockContentType(_ context.Context, key string) (TypeRow, error) {
	row, err := q.fake.shelf.typeUnder(key)
	return answer(q, "LockContentType", keyed{key}, row, err)
}

// LockDefaultContentType answers with the stored type holding the root, or no row.
func (q fakeQueries) LockDefaultContentType(context.Context) (TypeRow, error) {
	row, err := firstRow(q.fake.shelf.types, func(t TypeRow) bool { return t.IsDefault })
	return answer(q, "LockDefaultContentType", nil, row, err)
}

// UpdateContentType answers with the stored type under the key carrying the edit, or no row.
func (q fakeQueries) UpdateContentType(_ context.Context, t TypeRow) (TypeRow, error) {
	bound := t
	bound.Origin, bound.CreatedAt, bound.UpdatedAt = "", time.Time{}, q.fake.stamp(t.UpdatedAt)
	row, err := q.fake.shelf.typeUnder(t.Key)
	if err == nil {
		t.Origin, t.CreatedAt = row.Origin, row.CreatedAt
		row = t
	}
	return answer(q, "UpdateContentType", bound, row, err)
}

// DeferAddressCheck answers with the address check deferred.
func (q fakeQueries) DeferAddressCheck(context.Context) error {
	return q.send("DeferAddressCheck", nil)
}

// RetypeContentPaths answers with the addresses the shelf says a retype of the type moves.
func (q fakeQueries) RetypeContentPaths(_ context.Context, t TypeRow, was string) ([]string, error) {
	bound := retyped{Key: t.Key, RouteWord: t.RouteWord, Was: was, UpdatedAt: q.fake.stamp(t.UpdatedAt)}
	return answer(q, "RetypeContentPaths", bound, copied(q.fake.shelf.carried[t.Key]), nil)
}

// CountContentOutsideRouteWord answers with how many addresses of the type the shelf says sit outside the word.
func (q fakeQueries) CountContentOutsideRouteWord(_ context.Context, key, was string) (OutsideRow, error) {
	return answer(q, "CountContentOutsideRouteWord", outsideOf{Key: key, Was: was}, q.fake.shelf.outside[key], nil)
}

// CountNestedContent answers with how many items of the type the shelf says nest.
func (q fakeQueries) CountNestedContent(_ context.Context, key string) (int64, error) {
	return answer(q, "CountNestedContent", keyed{key}, q.fake.shelf.nested[key], nil)
}

// DeleteContentType answers with how many stored types hold the key.
func (q fakeQueries) DeleteContentType(_ context.Context, key string) (int64, error) {
	return answer(q, "DeleteContentType", keyed{key}, q.fake.shelf.typesUnder(key), nil)
}

// AdoptContentType answers with how many stored types hold the key.
func (q fakeQueries) AdoptContentType(_ context.Context, key string, at time.Time) (int64, error) {
	bound := stamping{Key: key, At: q.fake.stamp(at)}
	return answer(q, "AdoptContentType", bound, q.fake.shelf.typesUnder(key), nil)
}

// TypeKeys answers with the key of every stored type in stored order.
func (q fakeQueries) TypeKeys(context.Context) ([]string, error) {
	return answer(q, "TypeKeys", nil, q.fake.shelf.typeKeys(), nil)
}

// LockFieldGroups answers with the lock taken.
func (q fakeQueries) LockFieldGroups(context.Context) error {
	return q.send("LockFieldGroups", nil)
}

// ListFieldGroups answers with every stored group.
func (q fakeQueries) ListFieldGroups(context.Context) ([]GroupRow, error) {
	return answer(q, "ListFieldGroups", nil, copied(q.fake.shelf.groups), nil)
}

// FieldGroupKeys answers with the key of every stored group in key order.
func (q fakeQueries) FieldGroupKeys(context.Context) ([]string, error) {
	return answer(q, "FieldGroupKeys", nil, q.fake.shelf.groupKeys(), nil)
}

// CreateFieldGroup answers with the group stored active and last in the order under a new identity.
func (q fakeQueries) CreateFieldGroup(_ context.Context, g GroupRow) (GroupRow, error) {
	bound := groupColumns{
		Key: g.Key, Title: g.Title, Location: string(g.Location), Origin: g.Origin,
		CreatedAt: q.fake.stamp(g.CreatedAt), UpdatedAt: q.fake.stamp(g.UpdatedAt),
	}
	g.ID = nextNumber(q.fake.shelf.groups, func(row GroupRow) int { return row.ID })
	g.Position = nextNumber(q.fake.shelf.groups, func(row GroupRow) int { return row.Position })
	g.Active = true
	return answer(q, "CreateFieldGroup", bound, g, nil)
}

// UpdateFieldGroup answers with the stored group under the identity carrying the edit while its stamp holds, or no row.
func (q fakeQueries) UpdateFieldGroup(_ context.Context, g GroupRow, expected time.Time) (GroupRow, error) {
	bound := groupColumns{
		ID: g.ID, Title: g.Title, Location: string(g.Location), Active: g.Active,
		UpdatedAt: q.fake.stamp(g.UpdatedAt), Expected: q.fake.stamp(expected),
	}
	row, err := firstRow(q.fake.shelf.groups, func(held GroupRow) bool {
		return held.ID == g.ID && held.UpdatedAt.Equal(expected)
	})
	if err == nil {
		row.Title, row.Location, row.Active, row.UpdatedAt = g.Title, g.Location, g.Active, g.UpdatedAt
	}
	return answer(q, "UpdateFieldGroup", bound, row, err)
}

// ReorderFieldGroups answers with the order stored.
func (q fakeQueries) ReorderFieldGroups(_ context.Context, ids []int) error {
	return q.send("ReorderFieldGroups", ordering{IDs: copied(ids)})
}

// CarryStrayFieldsOfGroup answers with the strays carried.
func (q fakeQueries) CarryStrayFieldsOfGroup(_ context.Context, groupID int) error {
	return q.send("CarryStrayFieldsOfGroup", identified{groupID})
}

// DeleteFieldGroup answers with how many stored groups carry the identity.
func (q fakeQueries) DeleteFieldGroup(_ context.Context, id int) (int64, error) {
	gone := rowsMatching(q.fake.shelf.groups, func(g GroupRow) bool { return g.ID == id })
	return answer(q, "DeleteFieldGroup", identified{id}, gone, nil)
}

// AdoptFieldGroup answers with how many stored groups hold the key.
func (q fakeQueries) AdoptFieldGroup(_ context.Context, key string, at time.Time) (int64, error) {
	adopted := rowsMatching(q.fake.shelf.groups, func(g GroupRow) bool { return g.Key == key })
	return answer(q, "AdoptFieldGroup", stamping{Key: key, At: q.fake.stamp(at)}, adopted, nil)
}

// AdoptFieldsInGroup answers with the fields of the group adopted.
func (q fakeQueries) AdoptFieldsInGroup(_ context.Context, key string, at time.Time) error {
	return q.send("AdoptFieldsInGroup", stamping{Key: key, At: q.fake.stamp(at)})
}

// ListContentFields answers with every stored field.
func (q fakeQueries) ListContentFields(context.Context) ([]FieldRow, error) {
	return answer(q, "ListContentFields", nil, copied(q.fake.shelf.fields), nil)
}

// FieldByID answers with the stored field carrying the identity, or no row.
func (q fakeQueries) FieldByID(_ context.Context, id int) (FieldRow, error) {
	row, err := firstRow(q.fake.shelf.fields, byID(id))
	return answer(q, "FieldByID", identified{id}, row, err)
}

// CreateContentField answers with the field stored at its group's top under a new identity.
func (q fakeQueries) CreateContentField(_ context.Context, f FieldRow) (FieldRow, error) {
	bound := q.fake.created(f)
	f.ID, f.ParentID, f.Depth = q.fake.shelf.nextFieldID(), 0, 0
	return answer(q, "CreateContentField", bound, f, nil)
}

// CreateSubContentField answers with the field stored inside its container under a new identity.
func (q fakeQueries) CreateSubContentField(_ context.Context, f FieldRow) (FieldRow, error) {
	bound := q.fake.created(f)
	bound.ParentID, bound.Depth = f.ParentID, f.Depth
	f.ID = q.fake.shelf.nextFieldID()
	return answer(q, "CreateSubContentField", bound, f, nil)
}

// UpdateContentField answers with the group's top field under the key carrying the edit while its stamp holds.
func (q fakeQueries) UpdateContentField(_ context.Context, f FieldRow, expected time.Time) (FieldRow, error) {
	bound := q.fake.edit(f, expected)
	bound.GroupID, bound.Key = f.GroupID, f.Key
	row, err := q.fake.shelf.edited(f, func(held FieldRow) bool {
		return held.ParentID == 0 && held.GroupID == f.GroupID && held.Key == f.Key && held.UpdatedAt.Equal(expected)
	})
	return answer(q, "UpdateContentField", bound, row, err)
}

// UpdateSubContentField answers with the sub field carrying the identity and the edit while its stamp holds.
func (q fakeQueries) UpdateSubContentField(_ context.Context, f FieldRow, expected time.Time) (FieldRow, error) {
	bound := q.fake.edit(f, expected)
	bound.ID = f.ID
	row, err := q.fake.shelf.edited(f, func(held FieldRow) bool {
		return held.ID == f.ID && held.ParentID != 0 && held.UpdatedAt.Equal(expected)
	})
	return answer(q, "UpdateSubContentField", bound, row, err)
}

// ReorderContentFields answers with how many top fields of the group carry a key of the order.
func (q fakeQueries) ReorderContentFields(_ context.Context, groupID int, keys []string) (int64, error) {
	moved := rowsMatching(q.fake.shelf.fields, func(f FieldRow) bool {
		return f.ParentID == 0 && f.GroupID == groupID && slices.Contains(keys, f.Key)
	})
	return answer(q, "ReorderContentFields", ordering{GroupID: groupID, Keys: copied(keys)}, moved, nil)
}

// ReorderSubContentFields answers with how many fields of the container carry a key of the order.
func (q fakeQueries) ReorderSubContentFields(_ context.Context, parentID int, keys []string) (int64, error) {
	moved := rowsMatching(q.fake.shelf.fields, func(f FieldRow) bool {
		return f.ParentID == parentID && slices.Contains(keys, f.Key)
	})
	return answer(q, "ReorderSubContentFields", ordering{ParentID: parentID, Keys: copied(keys)}, moved, nil)
}

// ReparentContentField answers with the stored field carrying the identity standing in its new place, or no row.
func (q fakeQueries) ReparentContentField(
	_ context.Context, id, toGroup, toParent int, at time.Time,
) (FieldRow, error) {
	bound := reparenting{ID: id, ToGroup: toGroup, ToParent: toParent, UpdatedAt: q.fake.stamp(at)}
	row, err := firstRow(q.fake.shelf.fields, byID(id))
	if err == nil {
		row.GroupID, row.ParentID, row.UpdatedAt = toGroup, toParent, at
	}
	return answer(q, "ReparentContentField", bound, row, err)
}

// RecountContentFieldDepth answers with the depths below the field recounted.
func (q fakeQueries) RecountContentFieldDepth(_ context.Context, id, toGroup, depth int) error {
	return q.send("RecountContentFieldDepth", recounting{ID: id, ToGroup: toGroup, Depth: depth})
}

// DeleteContentField answers with how many top fields of the group carry the key.
func (q fakeQueries) DeleteContentField(_ context.Context, groupID int, key string) (int64, error) {
	gone := rowsMatching(q.fake.shelf.fields, func(f FieldRow) bool {
		return f.ParentID == 0 && f.GroupID == groupID && f.Key == key
	})
	return answer(q, "DeleteContentField", fieldColumns{GroupID: groupID, Key: key}, gone, nil)
}

// DeleteFieldByID answers with how many stored fields carry the identity.
func (q fakeQueries) DeleteFieldByID(_ context.Context, id int) (int64, error) {
	return answer(q, "DeleteFieldByID", identified{id}, rowsMatching(q.fake.shelf.fields, byID(id)), nil)
}

// ClearContentFieldValues answers with the values under the key cleared from the items of the types.
func (q fakeQueries) ClearContentFieldValues(_ context.Context, key string, types []string) error {
	return q.send("ClearContentFieldValues", clearing{Key: key, Types: copied(types)})
}

// ClearRevisionFieldValues answers with the values under the key cleared from the revisions of the types.
func (q fakeQueries) ClearRevisionFieldValues(_ context.Context, key string, types []string) error {
	return q.send("ClearRevisionFieldValues", clearing{Key: key, Types: copied(types)})
}

// StripContentFieldPath answers with the path stripped from the items of the types.
func (q fakeQueries) StripContentFieldPath(_ context.Context, s Strip) error {
	return q.send("StripContentFieldPath", Strip{Key: s.Key, Path: copied(s.Path), Types: copied(s.Types)})
}

// StripRevisionFieldPath answers with the path stripped from the revisions of the types.
func (q fakeQueries) StripRevisionFieldPath(_ context.Context, s Strip) error {
	return q.send("StripRevisionFieldPath", Strip{Key: s.Key, Path: copied(s.Path), Types: copied(s.Types)})
}

// StripContentLayout answers with the layout rows stripped from the items of the types.
func (q fakeQueries) StripContentLayout(_ context.Context, s Strip) error {
	return q.send("StripContentLayout", Strip{Key: s.Key, Path: copied(s.Path), Types: copied(s.Types)})
}

// StripRevisionLayout answers with the layout rows stripped from the revisions of the types.
func (q fakeQueries) StripRevisionLayout(_ context.Context, s Strip) error {
	return q.send("StripRevisionLayout", Strip{Key: s.Key, Path: copied(s.Path), Types: copied(s.Types)})
}

// DeleteRelationsOfFields answers with the index rows of the fields removed from the types.
func (q fakeQueries) DeleteRelationsOfFields(_ context.Context, types []string, fields []int) error {
	return q.send("DeleteRelationsOfFields", relating{Types: copied(types), Fields: copied(fields)})
}

func TestTheFakeFlagsAStatementSentOutsideTheTransactionItBelongsTo(t *testing.T) {
	t.Parallel()

	for name, misuse := range map[string]func(ctx context.Context, f *fakeBackend){
		"a pool statement inside a transaction": func(ctx context.Context, f *fakeBackend) {
			_ = f.inTx(ctx, func(Queries) error { return fakeQueries{fake: f}.LockFieldGroups(ctx) })
		},
		"a statement on a transaction already over": func(ctx context.Context, f *fakeBackend) {
			var leaked Queries
			_ = f.inTx(ctx, func(bound Queries) error {
				leaked = bound
				return nil
			})
			_ = leaked.LockFieldGroups(ctx)
		},
		"a transaction inside another": func(ctx context.Context, f *fakeBackend) {
			_ = f.inTx(ctx, func(Queries) error {
				return f.inTx(ctx, func(Queries) error { return nil })
			})
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFake(t, library(), noFailure)
			var flagged []string
			f.complain = func(format string, args ...any) { flagged = append(flagged, fmt.Sprintf(format, args...)) }

			misuse(t.Context(), f)

			if len(flagged) != 1 {
				t.Errorf("flagged = %q, want the misuse flagged once", flagged)
			}
		})
	}
}
