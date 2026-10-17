// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// shelvedAt is when every row of the library shelf was created and last updated.
var shelvedAt = time.Date(2025, time.March, 2, 9, 0, 0, 0, time.UTC)

// editedAt is the stamp a caller hands an edit.
var editedAt = time.Date(2025, time.April, 6, 14, 30, 0, 0, time.UTC)

// staleAt is a stamp no stored row carries.
var staleAt = time.Date(2025, time.February, 20, 8, 0, 0, 0, time.UTC)

// The locations the library groups are stored under.
const (
	bookLocation   = `[[{"source":"content_type","operator":"==","value":"book"}]]`
	personLocation = `[[{"source":"content_type","operator":"==","value":"person"}]]`
	everyLocation  = `[[{"source":"content_type","operator":"==","value":"*"}]]`
)

// everyTypeRules is the location serving every type.
var everyTypeRules = content.Rules{{{
	Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.AnyContentType,
}}}

// library returns a shelf of three types, four groups and the fields they hold, the page type holding the root.
func library() shelf {
	stocked := shelf{
		types: []TypeRow{
			{
				Key: "page", SingularLabel: "Page", PluralLabel: "Pages", PageKind: "single",
				Hierarchical: true, Revisions: true, IsDefault: true, Active: true, RevisionCap: 100,
			},
			{
				Key: "book", SingularLabel: "Book", PluralLabel: "Books", Description: "Printed works",
				RouteWord: "books", PageKind: "single", Origin: "library",
				Hierarchical: true, Revisions: true, Active: true, RevisionCap: 25,
			},
			{
				Key: "person", SingularLabel: "Person", PluralLabel: "People", RouteWord: "people",
				PageKind: "archive", Active: true,
			},
		},
		groups: []GroupRow{
			{
				ID: 1, Position: 1, Key: "book-details", Title: "Book details",
				Location: []byte(bookLocation), Active: true,
			},
			{
				ID: 2, Position: 2, Key: "person-details", Title: "Person details", Origin: "library",
				Location: []byte(personLocation), Active: true,
			},
			{ID: 3, Position: 3, Key: "shared", Title: "Shared", Location: []byte(everyLocation), Active: true},
			{ID: 4, Position: 4, Key: "archive", Title: "Archive", Location: []byte(bookLocation)},
		},
		fields: []FieldRow{
			{ID: 10, GroupID: 1, Key: "rating", Label: "Rating", Kind: "number"},
			{ID: 11, GroupID: 1, Key: "author", Label: "Author", Kind: "relation", RelatesTo: "person"},
			{ID: 12, GroupID: 1, Key: "address", Label: "Address", Kind: "section"},
			{ID: 13, GroupID: 1, ParentID: 12, Depth: 1, Key: "street", Label: "Street", Kind: "text"},
			{
				ID: 14, GroupID: 1, ParentID: 12, Depth: 1, Key: "agent", Label: "Agent", Kind: "relation",
				RelatesTo: "person",
			},
			{ID: 15, GroupID: 1, Key: "chapters", Label: "Chapters", Kind: "flexible"},
			{ID: 16, GroupID: 1, ParentID: 15, Depth: 1, Key: "chapter", Label: "Chapter", Kind: "layout"},
			{
				ID: 17, GroupID: 1, ParentID: 16, Depth: 2, Key: "cited", Label: "Cited", Kind: "relation",
				RelatesTo: "book", Many: true,
			},
			{ID: 20, GroupID: 2, Key: "rating", Label: "Rating", Kind: "number"},
			{ID: 21, GroupID: 2, Key: "author", Label: "Author", Kind: "relation", RelatesTo: "book"},
			{ID: 30, GroupID: 3, Key: "subtitle", Label: "Subtitle", Kind: "text", Settings: []byte(`{"max":80}`)},
			{ID: 40, GroupID: 4, Key: "summary", Label: "Summary", Kind: "text"},
		},
		nested: map[string]int64{},
	}
	for i := range stocked.types {
		stocked.types[i].CreatedAt, stocked.types[i].UpdatedAt = shelvedAt, shelvedAt
	}
	for i := range stocked.groups {
		stocked.groups[i].CreatedAt, stocked.groups[i].UpdatedAt = shelvedAt, shelvedAt
	}
	for i := range stocked.fields {
		stocked.fields[i].CreatedAt, stocked.fields[i].UpdatedAt = shelvedAt, shelvedAt
	}
	return stocked
}

// rootless returns the library shelf with no type holding the root.
func rootless() shelf {
	stocked := library()
	stocked.types[0].IsDefault = false
	return stocked
}

// accentedRoot returns the library shelf with the root type labelled with accented letters.
func accentedRoot() shelf {
	stocked := library()
	stocked.types[0].PluralLabel = "Résumés"
	return stocked
}

// markedRoot returns the library shelf with the root type's plural label slugging to a marked word.
func markedRoot() shelf {
	stocked := library()
	stocked.types[0].PluralLabel = "Pages 2️⃣"
	return stocked
}

// The type keys the library shelf holds, in stored order, and the sweeps that spare one of them.
var (
	everyTypeKey  = []string{"page", "book", "person"}
	pageAndBook   = []string{"page", "book"}
	pageAndPerson = []string{"page", "person"}
)

// The entries a call logs with nothing bound.
var (
	begun             = step{name: beginEvent}
	committed         = step{name: commitEvent}
	rolledBack        = step{name: rollbackEvent}
	lockedGroups      = step{name: "LockFieldGroups"}
	readGroups        = step{name: "ListFieldGroups"}
	readFields        = step{name: "ListContentFields"}
	readTypeKeys      = step{name: "TypeKeys"}
	readGroupKeys     = step{name: "FieldGroupKeys"}
	readTypes         = step{name: "ListContentTypes"}
	lockedRoot        = step{name: "LockDefaultContentType"}
	deferredAddresses = step{name: "DeferAddressCheck"}
)

// checkReads lists the reads the caller's check is handed, in the order a step sends them.
var checkReads = []step{readGroups, readFields, readTypeKeys}

// checkUnderLock lists the entries a check on the library shelf logs once the field groups are locked.
var checkUnderLock = append(slices.Clone(checkReads), step{name: checkEvent, args: checked{
	Paths: []string{
		"book-details", "book-details/rating", "book-details/author", "book-details/address",
		"book-details/address/street", "book-details/address/agent", "book-details/chapters",
		"book-details/chapters/chapter", "book-details/chapters/chapter/cited",
		"person-details", "person-details/rating", "person-details/author",
		"shared", "shared/subtitle", "archive", "archive/summary",
	},
	Types: everyTypeKey,
}})

// storeSentinels lists every sentinel a type store method answers with today.
var storeSentinels = slices.Concat(everySentinel, []error{
	content.ErrConflict, content.ErrGroupKeyTaken, content.ErrTypeTaken, content.ErrRouteWordTaken,
	content.ErrSlugTaken, content.ErrTypeInUse, content.ErrTypeTargeted, content.ErrTargetUnknown,
})

// sent returns the entry of a statement binding the columns.
func sent(name string, args any) step {
	return step{name: name, args: args}
}

// fieldSwept returns the entries removing the group's top field under the key and clearing its values from the types.
func fieldSwept(groupID int, key string, types []string) []step {
	return []step{
		sent("DeleteContentField", fieldColumns{GroupID: groupID, Key: key}),
		sent("ClearContentFieldValues", clearing{Key: key, Types: types}),
		sent("ClearRevisionFieldValues", clearing{Key: key, Types: types}),
	}
}

// relationsDropped returns the entry removing the index rows of the relation fields from the types.
func relationsDropped(types []string, fields ...int) step {
	return sent("DeleteRelationsOfFields", relating{Types: types, Fields: fields})
}

// pathStripped returns the entries stripping the path from the items and the revisions of the types.
func pathStripped(path, types []string) []step {
	strip := Strip{Key: path[0], Path: path, Types: types}
	return []step{sent("StripContentFieldPath", strip), sent("StripRevisionFieldPath", strip)}
}

// layoutStripped returns the entries stripping the layout rows at the path from the items and the revisions.
func layoutStripped(path, types []string) []step {
	strip := Strip{Key: path[0], Path: path, Types: types}
	return []step{sent("StripContentLayout", strip), sent("StripRevisionLayout", strip)}
}

// novelUnder returns the book type edited into novels answering under the route word.
func novelUnder(routeWord string) content.Type {
	return content.Type{
		Key: "book", SingularLabel: "Novel", PluralLabel: "Novels", Description: "Long fiction",
		RouteWord: routeWord, Hierarchical: true, RevisionCap: 5, PageKind: content.PageKindArchive,
		Active: true, Origin: "library", UpdatedAt: editedAt,
	}
}

// novelRow returns what writing the book type edited into novels under the route word binds.
func novelRow(routeWord string) TypeRow {
	return TypeRow{
		Key: "book", SingularLabel: "Novel", PluralLabel: "Novels", Description: "Long fiction",
		RouteWord: routeWord, PageKind: "archive", Hierarchical: true, Active: true, RevisionCap: 5,
		UpdatedAt: editedAt,
	}
}

// rootBook returns the book type asking for the root, nesting or not.
func rootBook(hierarchical bool) content.Type {
	return content.Type{
		Key: "book", SingularLabel: "Book", PluralLabel: "Books", Description: "Printed works",
		RouteWord: "books", Hierarchical: hierarchical, Revisions: true, RevisionCap: 25,
		PageKind: content.PageKindSingle, Default: true, Active: true, UpdatedAt: editedAt,
	}
}

// rootBookRow returns what writing the book type at the root binds, nesting or not.
func rootBookRow(hierarchical bool) TypeRow {
	return TypeRow{
		Key: "book", SingularLabel: "Book", PluralLabel: "Books", Description: "Printed works",
		PageKind: "single", Hierarchical: hierarchical, Revisions: true, IsDefault: true, Active: true,
		RevisionCap: 25, UpdatedAt: editedAt,
	}
}

// demotedPage is what writing the page type under a word of its own binds when it hands the root over.
var demotedPage = TypeRow{
	Key: "page", SingularLabel: "Page", PluralLabel: "Pages", RouteWord: "pages", PageKind: "single",
	Hierarchical: true, Revisions: true, Active: true, RevisionCap: 100, UpdatedAt: editedAt,
}

// demotedResumes is what writing the accented root type under a plain word binds when it hands the root over.
var demotedResumes = TypeRow{
	Key: "page", SingularLabel: "Page", PluralLabel: "Résumés", RouteWord: "resumes", PageKind: "single",
	Hierarchical: true, Revisions: true, Active: true, RevisionCap: 100, UpdatedAt: editedAt,
}

// demotedPageByKey is what writing the root type under its key binds when its label has no valid route word.
var demotedPageByKey = TypeRow{
	Key: "page", SingularLabel: "Page", PluralLabel: "Pages 2️⃣", RouteWord: "page", PageKind: "single",
	Hierarchical: true, Revisions: true, Active: true, RevisionCap: 100, UpdatedAt: editedAt,
}

// relabeledRootPage returns the page type keeping the root under a new plural label.
func relabeledRootPage() content.Type {
	return content.Type{
		Key: "page", SingularLabel: "Page", PluralLabel: "Site pages", Hierarchical: true, Revisions: true,
		RevisionCap: 100, PageKind: content.PageKindSingle, Default: true, Active: true, UpdatedAt: editedAt,
	}
}

// relabeledRootPageRow is what writing the page type at the root under a new plural label binds.
var relabeledRootPageRow = TypeRow{
	Key: "page", SingularLabel: "Page", PluralLabel: "Site pages", PageKind: "single",
	Hierarchical: true, Revisions: true, IsDefault: true, Active: true, RevisionCap: 100, UpdatedAt: editedAt,
}

// flatAuthors returns the person type edited into authors that still do not nest.
func flatAuthors() content.Type {
	return content.Type{
		Key: "person", SingularLabel: "Author", PluralLabel: "Authors", RouteWord: "people",
		PageKind: content.PageKindArchive, Active: true, UpdatedAt: editedAt,
	}
}

// flatAuthorsRow is what writing the person type edited into authors that still do not nest binds.
var flatAuthorsRow = TypeRow{
	Key: "person", SingularLabel: "Author", PluralLabel: "Authors", RouteWord: "people", PageKind: "archive",
	Active: true, UpdatedAt: editedAt,
}

// valued returns the answer of a method with its value boxed.
func valued[T any](value T, err error) (any, error) {
	return value, err
}

// sequence is one method call on a shelf with the entries today's steps log for it.
type sequence struct {
	stocked func() shelf
	run     func(ctx context.Context, s *Store, check content.Recheck) (any, error)
	golden  []step
	ends    error
	kept    any
	rawTx   bool
}

// runOn runs the call on a fresh fake failing the entry at the position, returning the fake, the value and the error.
func (sq sequence) runOn(t *testing.T, failAt int) (*fakeBackend, any, error) {
	stocked := library()
	if sq.stocked != nil {
		stocked = sq.stocked()
	}
	f := newFake(t, stocked, failAt)
	value, err := sq.run(t.Context(), f.store(), f.recheck())
	f.to = time.Now()
	return f, value, err
}

// typeSequences holds the calls of the type methods with the entries today's steps log for each.
var typeSequences = map[string]sequence{
	"List": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.List(ctx))
		},
		golden: []step{readTypes, readGroups, readFields},
	},
	"ByKey on a stored type": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.ByKey(ctx, "person"))
		},
		golden: []step{sent("GetContentType", keyed{"person"}), readGroups, readFields},
	},
	"ByKey on a type no row holds": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.ByKey(ctx, "film"))
		},
		golden: []step{sent("GetContentType", keyed{"film"})},
		ends:   content.ErrTypeNotFound,
	},
	"Create": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Create(ctx, content.Type{
				Key: "film", SingularLabel: "Film", PluralLabel: "Films", Description: "Moving pictures",
				RouteWord: "films", Revisions: true, RevisionCap: 10, PageKind: content.PageKindArchive,
				Active: true, Origin: "library", CreatedAt: editedAt, UpdatedAt: editedAt,
			}))
		},
		golden: []step{sent("CreateContentType", TypeRow{
			Key: "film", SingularLabel: "Film", PluralLabel: "Films", Description: "Moving pictures",
			RouteWord: "films", PageKind: "archive", Origin: "library", Revisions: true, Active: true,
			RevisionCap: 10, CreatedAt: editedAt, UpdatedAt: editedAt,
		})},
	},
	"Update keeping its route word": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Update(ctx, novelUnder("books")))
		},
		golden: []step{
			begun, sent("LockContentType", keyed{"book"}), sent("UpdateContentType", novelRow("books")), committed,
		},
	},
	"Update moving its route word": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Update(ctx, novelUnder("novels")))
		},
		golden: []step{
			begun, sent("LockContentType", keyed{"book"}), sent("UpdateContentType", novelRow("novels")),
			deferredAddresses,
			sent("RetypeContentPaths", retyped{Key: "book", RouteWord: "novels", Was: "books", UpdatedAt: editedAt}),
			committed,
		},
	},
	"Update stopping nesting and taking the root another type holds": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Update(ctx, rootBook(false)))
		},
		golden: []step{
			begun, sent("LockContentType", keyed{"book"}), sent("CountNestedContent", keyed{"book"}), lockedRoot,
			sent("UpdateContentType", demotedPage), deferredAddresses,
			sent("RetypeContentPaths", retyped{Key: "page", RouteWord: "pages", UpdatedAt: editedAt}),
			sent("UpdateContentType", rootBookRow(false)), deferredAddresses,
			sent("RetypeContentPaths", retyped{Key: "book", Was: "books", UpdatedAt: editedAt}),
			committed,
		},
	},
	"Update taking the root a type with an accented label holds": {
		stocked: accentedRoot,
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Update(ctx, rootBook(true)))
		},
		golden: []step{
			begun, sent("LockContentType", keyed{"book"}), lockedRoot,
			sent("UpdateContentType", demotedResumes), deferredAddresses,
			sent("RetypeContentPaths", retyped{Key: "page", RouteWord: "resumes", UpdatedAt: editedAt}),
			sent("UpdateContentType", rootBookRow(true)), deferredAddresses,
			sent("RetypeContentPaths", retyped{Key: "book", Was: "books", UpdatedAt: editedAt}),
			committed,
		},
	},
	"Update taking the root a type with a marked label falls back to its key": {
		stocked: markedRoot,
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Update(ctx, rootBook(true)))
		},
		golden: []step{
			begun, sent("LockContentType", keyed{"book"}), lockedRoot,
			sent("UpdateContentType", demotedPageByKey), deferredAddresses,
			sent("RetypeContentPaths", retyped{Key: "page", RouteWord: "page", UpdatedAt: editedAt}),
			sent("UpdateContentType", rootBookRow(true)), deferredAddresses,
			sent("RetypeContentPaths", retyped{Key: "book", Was: "books", UpdatedAt: editedAt}),
			committed,
		},
	},
	"Update taking the root no type holds": {
		stocked: rootless,
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Update(ctx, rootBook(true)))
		},
		golden: []step{
			begun, sent("LockContentType", keyed{"book"}), lockedRoot,
			sent("UpdateContentType", rootBookRow(true)), deferredAddresses,
			sent("RetypeContentPaths", retyped{Key: "book", Was: "books", UpdatedAt: editedAt}),
			committed,
		},
	},
	"Update keeping the root on the type holding it": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Update(ctx, relabeledRootPage()))
		},
		golden: []step{
			begun, sent("LockContentType", keyed{"page"}), sent("UpdateContentType", relabeledRootPageRow), committed,
		},
	},
	"Update on a flat type staying flat": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Update(ctx, flatAuthors()))
		},
		golden: []step{
			begun, sent("LockContentType", keyed{"person"}), sent("UpdateContentType", flatAuthorsRow), committed,
		},
	},
	"Delete": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.Delete(ctx, "person")
		},
		golden: []step{sent("DeleteContentType", keyed{"person"})},
	},
	"Nested": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.Nested(ctx, "book"))
		},
		golden: []step{sent("CountNestedContent", keyed{"book"})},
	},
	"AdoptType": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.AdoptType(ctx, "book")
		},
		golden: []step{sent("AdoptContentType", stamping{Key: "book", At: mintedStamp})},
	},
	"AdoptGroup": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.AdoptGroup(ctx, "person-details")
		},
		golden: []step{
			begun,
			sent("AdoptFieldGroup", stamping{Key: "person-details", At: mintedStamp}),
			sent("AdoptFieldsInGroup", stamping{Key: "person-details", At: mintedStamp}),
			committed,
		},
		rawTx: true,
	},
}

// groupSequences holds the calls of the group methods with the entries today's steps log for each.
var groupSequences = map[string]sequence{
	"ListGroups": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.ListGroups(ctx))
		},
		golden: []step{readGroups, readFields},
	},
	"CreateGroup under the key it asks for": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.CreateGroup(ctx, content.Group{
				Key: "reading-notes", Title: "Reading notes", Location: content.Rules{bookOnly[0], {}},
				Origin: "library",
			}))
		},
		golden: []step{
			begun, lockedGroups,
			sent("CreateFieldGroup", groupColumns{
				Key: "reading-notes", Title: "Reading notes", Location: bookLocation, Origin: "library",
				CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
			}),
			committed,
		},
		kept: content.Group{
			ID: 5, Key: "reading-notes", Title: "Reading notes", Location: bookOnly, Position: 5, Active: true,
			Origin: "library", CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
		},
		rawTx: true,
	},
	"CreateGroup minting a key past the one its title takes": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.CreateGroup(ctx, content.Group{Title: "Book details", Location: everyTypeRules}))
		},
		golden: []step{
			begun, lockedGroups, readGroupKeys,
			sent("CreateFieldGroup", groupColumns{
				Key: "book-details-2", Title: "Book details", Location: everyLocation,
				CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
			}),
			committed,
		},
		kept: content.Group{
			ID: 5, Key: "book-details-2", Title: "Book details", Location: everyTypeRules, Position: 5, Active: true,
			CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
		},
		rawTx: true,
	},
	"UpdateGroup with a check pointing two fields anew": {
		run: func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
			repointed := []content.Field{
				{
					Key: "rating", Label: "Score", Required: true, Settings: map[string]any{"max": 5},
					UpdatedAt: shelvedAt,
				},
				{Key: "author", Label: "Writer", UpdatedAt: shelvedAt},
			}
			g := content.Group{ID: 1, Title: "Book facts", Location: bookOnly, Active: true}
			return valued(s.UpdateGroup(ctx, g, repointed, check))
		},
		golden: slices.Concat([]step{begun, lockedGroups}, checkUnderLock, checkReads, []step{
			sent("UpdateFieldGroup", groupColumns{
				ID: 1, Title: "Book facts", Location: bookLocation, Active: true, UpdatedAt: mintedStamp,
			}),
			sent("UpdateContentField", fieldColumns{
				GroupID: 1, Key: "rating", Label: "Score", Settings: `{"max":5}`, Required: true,
				UpdatedAt: mintedStamp, Expected: shelvedAt,
			}),
			sent("UpdateContentField", fieldColumns{
				GroupID: 1, Key: "author", Label: "Writer", Settings: "{}", UpdatedAt: mintedStamp, Expected: shelvedAt,
			}),
			committed,
		}),
	},
	"UpdateGroup turning a group off with no field pointed anew": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			g := content.Group{ID: 3, Title: "Shared fields", Location: everyTypeRules}
			return valued(s.UpdateGroup(ctx, g, nil, nil))
		},
		golden: []step{
			begun, lockedGroups, readGroups, readFields, readTypeKeys,
			sent("UpdateFieldGroup", groupColumns{
				ID: 3, Title: "Shared fields", Location: everyLocation, UpdatedAt: mintedStamp,
			}),
			committed,
		},
	},
	"UpdateGroup on a group no row holds": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			g := content.Group{ID: 9, Title: "Gone", Location: bookOnly, Active: true}
			return valued(s.UpdateGroup(ctx, g, nil, nil))
		},
		golden: []step{
			begun, lockedGroups, readGroups, readFields,
			sent("UpdateFieldGroup", groupColumns{
				ID: 9, Title: "Gone", Location: bookLocation, Active: true, UpdatedAt: mintedStamp,
			}),
			rolledBack,
		},
		ends: content.ErrGroupNotFound,
	},
	"DeleteGroup with a check and one plain field": {
		run: func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
			return nil, s.DeleteGroup(ctx, 4, check)
		},
		golden: slices.Concat(
			[]step{begun, lockedGroups}, checkUnderLock,
			[]step{readGroups, readFields, sent("CarryStrayFieldsOfGroup", identified{4}), readTypeKeys},
			fieldSwept(4, "summary", everyTypeKey),
			[]step{sent("DeleteFieldGroup", identified{4}), committed},
		),
	},
	"DeleteGroup sweeping relations and sparing the types another group serves": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.DeleteGroup(ctx, 1, nil)
		},
		golden: slices.Concat(
			[]step{
				begun, lockedGroups, readGroups, readFields,
				sent("CarryStrayFieldsOfGroup", identified{1}), readTypeKeys,
			},
			fieldSwept(1, "rating", pageAndBook),
			fieldSwept(1, "author", pageAndBook), []step{relationsDropped(pageAndBook, 11, 21)},
			fieldSwept(1, "address", everyTypeKey), []step{relationsDropped(everyTypeKey, 14)},
			fieldSwept(1, "chapters", everyTypeKey), []step{relationsDropped(everyTypeKey, 17)},
			[]step{sent("DeleteFieldGroup", identified{1}), committed},
		),
	},
	"DeleteFieldsOfGroup with a check in the order the group holds them": {
		run: func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
			return nil, s.DeleteFieldsOfGroup(ctx, 1, []string{"address", "rating"}, check)
		},
		golden: slices.Concat(
			[]step{begun, lockedGroups}, checkUnderLock, checkReads,
			fieldSwept(1, "rating", pageAndBook),
			fieldSwept(1, "address", everyTypeKey), []step{relationsDropped(everyTypeKey, 14)},
			[]step{committed},
		),
	},
	"DeleteFieldInGroup sparing the type another group serves": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.DeleteFieldInGroup(ctx, 2, "author", nil)
		},
		golden: slices.Concat(
			[]step{begun, lockedGroups, readGroups, readFields, readTypeKeys},
			fieldSwept(2, "author", pageAndPerson),
			[]step{relationsDropped(pageAndPerson, 11, 21), committed},
		),
	},
	"ReorderGroups": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.ReorderGroups(ctx, []int{3, 1, 2, 4})
		},
		golden: []step{sent("ReorderFieldGroups", ordering{IDs: []int{3, 1, 2, 4}})},
	},
}

// fieldSequences holds the calls of the field methods with the entries today's steps log for each.
var fieldSequences = map[string]sequence{
	"UpdateFieldInGroup with a check": {
		run: func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
			f := content.Field{
				Key: "rating", Label: "Score", Required: true, Settings: map[string]any{"max": 5}, UpdatedAt: editedAt,
			}
			return valued(s.UpdateFieldInGroup(ctx, 1, f, shelvedAt, check))
		},
		golden: slices.Concat([]step{begun, lockedGroups}, checkUnderLock, []step{
			sent("UpdateContentField", fieldColumns{
				GroupID: 1, Key: "rating", Label: "Score", Settings: `{"max":5}`, Required: true,
				UpdatedAt: editedAt, Expected: shelvedAt,
			}),
			committed,
		}),
		kept: content.Field{
			ID: 10, GroupID: 1, Key: "rating", Label: "Score", Kind: content.FieldKindNumber, Required: true,
			Settings: map[string]any{"max": float64(5)}, CreatedAt: shelvedAt, UpdatedAt: editedAt,
		},
		rawTx: true,
	},
	"UpdateFieldInGroup on a stamp the field no longer carries": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			f := content.Field{Key: "rating", Label: "Score", UpdatedAt: editedAt}
			return valued(s.UpdateFieldInGroup(ctx, 1, f, staleAt, nil))
		},
		golden: []step{
			begun, lockedGroups,
			sent("UpdateContentField", fieldColumns{
				GroupID: 1, Key: "rating", Label: "Score", Settings: "{}", UpdatedAt: editedAt, Expected: staleAt,
			}),
			readGroups, readFields, rolledBack,
		},
		ends:  content.ErrConflict,
		rawTx: true,
	},
	"UpdateSubField": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			f := content.Field{Label: "Street line", Required: true, UpdatedAt: editedAt}
			return valued(s.UpdateSubField(ctx, 13, f, shelvedAt))
		},
		golden: []step{sent("UpdateSubContentField", fieldColumns{
			ID: 13, Label: "Street line", Settings: "{}", Required: true, UpdatedAt: editedAt, Expected: shelvedAt,
		})},
	},
	"UpdateSubField on a stamp the field no longer carries": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			f := content.Field{Label: "Street line", UpdatedAt: editedAt}
			return valued(s.UpdateSubField(ctx, 13, f, staleAt))
		},
		golden: []step{
			sent("UpdateSubContentField", fieldColumns{
				ID: 13, Label: "Street line", Settings: "{}", UpdatedAt: editedAt, Expected: staleAt,
			}),
			readGroups, readFields,
		},
		ends: content.ErrConflict,
	},
	"ReorderSubFields moving fields": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.ReorderSubFields(ctx, 12, []string{"agent", "street"})
		},
		golden: []step{sent("ReorderSubContentFields", ordering{ParentID: 12, Keys: []string{"agent", "street"}})},
	},
	"ReorderSubFields moving none under a stored field": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.ReorderSubFields(ctx, 13, []string{"line"})
		},
		golden: []step{
			sent("ReorderSubContentFields", ordering{ParentID: 13, Keys: []string{"line"}}), readGroups, readFields,
		},
	},
	"ReorderFieldsInGroup moving fields": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.ReorderFieldsInGroup(ctx, 1, []string{"chapters", "address", "author", "rating"})
		},
		golden: []step{sent("ReorderContentFields", ordering{
			GroupID: 1, Keys: []string{"chapters", "address", "author", "rating"},
		})},
	},
	"ReorderFieldsInGroup moving none in a stored group": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.ReorderFieldsInGroup(ctx, 4, []string{"notes"})
		},
		golden: []step{
			sent("ReorderContentFields", ordering{GroupID: 4, Keys: []string{"notes"}}), readGroups, readFields,
		},
	},
	"DeleteSubField with a check on a plain field": {
		run: func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
			return nil, s.DeleteSubField(ctx, 13, check)
		},
		golden: slices.Concat(
			[]step{begun, lockedGroups}, checkUnderLock,
			[]step{readGroups, readFields, readTypeKeys, sent("DeleteFieldByID", identified{13})},
			pathStripped([]string{"address", "street"}, everyTypeKey),
			[]step{committed},
		),
	},
	"DeleteSubField on a layout holding a relation": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return nil, s.DeleteSubField(ctx, 16, nil)
		},
		golden: slices.Concat(
			[]step{
				begun, lockedGroups, readGroups, readFields, readTypeKeys, sent("DeleteFieldByID", identified{16}),
			},
			layoutStripped([]string{"chapters", "chapter"}, everyTypeKey),
			[]step{relationsDropped(everyTypeKey, 17), committed},
		),
	},
	"CreateSubField": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			f := content.Field{
				Key: "epigraph", Label: "Epigraph", Kind: content.FieldKindText, Required: true, Origin: "library",
			}
			return valued(s.CreateSubField(ctx, 16, f, 3))
		},
		golden: []step{
			begun, lockedGroups, sent("FieldByID", identified{16}),
			sent("CreateSubContentField", fieldColumns{
				GroupID: 1, ParentID: 16, Depth: 2, Key: "epigraph", Label: "Epigraph", Kind: "text",
				Origin: "library", Settings: "{}", Required: true, CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
			}),
			committed,
		},
	},
	"CreateFieldInGroup with a check": {
		run: func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
			f := content.Field{
				Key: "isbn", Label: "ISBN", Kind: content.FieldKindText, Required: true, Origin: "library",
				Settings: map[string]any{"max": 13}, CreatedAt: editedAt, UpdatedAt: editedAt,
			}
			return valued(s.CreateFieldInGroup(ctx, 1, f, check))
		},
		golden: slices.Concat([]step{begun, lockedGroups}, checkUnderLock, checkReads, []step{
			sent("CreateContentField", fieldColumns{
				GroupID: 1, Key: "isbn", Label: "ISBN", Kind: "text", Origin: "library", Settings: `{"max":13}`,
				Required: true, CreatedAt: editedAt, UpdatedAt: editedAt,
			}),
			committed,
		}),
	},
	"CreateFieldInGroup of a relation without a check": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			f := content.Field{
				Key: "related", Label: "Related", Kind: content.FieldKindRelation, RelatesTo: "book", Many: true,
				CreatedAt: editedAt, UpdatedAt: editedAt,
			}
			return valued(s.CreateFieldInGroup(ctx, 3, f, nil))
		},
		golden: []step{
			begun, lockedGroups, readGroups, readFields, readTypeKeys,
			sent("CreateContentField", fieldColumns{
				GroupID: 3, Key: "related", Label: "Related", Kind: "relation", RelatesTo: "book", Settings: "{}",
				Many: true, CreatedAt: editedAt, UpdatedAt: editedAt,
			}),
			committed,
		},
	},
}

// moveSequences holds the calls of the field move with the entries today's steps log for each.
var moveSequences = map[string]sequence{
	"MoveField with a check out of a container to its group's top": {
		run: func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
			return valued(s.MoveField(ctx, 13, 1, 0, 3, check))
		},
		golden: slices.Concat(
			[]step{begun, lockedGroups}, checkUnderLock,
			[]step{
				readGroups, readFields, readGroups, readFields, readTypeKeys,
				sent("ReparentContentField", reparenting{ID: 13, ToGroup: 1, UpdatedAt: mintedStamp}),
				sent("RecountContentFieldDepth", recounting{ID: 13, ToGroup: 1}),
				readTypeKeys,
			},
			pathStripped([]string{"address", "street"}, everyTypeKey),
			[]step{committed},
		),
	},
	"MoveField to another group's top sweeping the types it leaves": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.MoveField(ctx, 15, 2, 0, 3, nil))
		},
		golden: slices.Concat(
			[]step{
				begun, lockedGroups, readGroups, readFields, readGroups, readFields, readTypeKeys,
				sent("ReparentContentField", reparenting{ID: 15, ToGroup: 2, UpdatedAt: mintedStamp}),
				sent("RecountContentFieldDepth", recounting{ID: 15, ToGroup: 2}),
				readTypeKeys,
			},
			pathStripped([]string{"chapters"}, pageAndBook),
			[]step{relationsDropped(pageAndBook, 17), relationsDropped(pageAndBook, 17), committed},
		),
	},
	"MoveField of a relation sparing the path a rival serves and dropping its index rows on every type it leaves": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.MoveField(ctx, 11, 4, 0, 3, nil))
		},
		golden: slices.Concat(
			[]step{
				begun, lockedGroups, readGroups, readFields, readGroups, readFields, readTypeKeys,
				sent("ReparentContentField", reparenting{ID: 11, ToGroup: 4, UpdatedAt: mintedStamp}),
				sent("RecountContentFieldDepth", recounting{ID: 11, ToGroup: 4}),
				readTypeKeys,
			},
			pathStripped([]string{"author"}, []string{"page"}),
			[]step{relationsDropped([]string{"page"}, 11, 21), relationsDropped(pageAndPerson, 11), committed},
		),
	},
	"MoveField into a container of its group": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.MoveField(ctx, 10, 1, 12, 3, nil))
		},
		golden: slices.Concat(
			[]step{
				begun, lockedGroups, readGroups, readFields,
				sent("ReparentContentField", reparenting{ID: 10, ToGroup: 1, ToParent: 12, UpdatedAt: mintedStamp}),
				sent("RecountContentFieldDepth", recounting{ID: 10, ToGroup: 1, Depth: 1}),
				readTypeKeys,
			},
			pathStripped([]string{"rating"}, pageAndBook),
			[]step{committed},
		),
	},
	"MoveField where the field already stands": {
		run: func(ctx context.Context, s *Store, _ content.Recheck) (any, error) {
			return valued(s.MoveField(ctx, 13, 1, 12, 3, nil))
		},
		golden: []step{begun, lockedGroups, readGroups, readFields, committed},
	},
}

// sequences holds every call with the entries today's steps log for it.
var sequences = joined(typeSequences, groupSequences, fieldSequences, moveSequences)

// joined returns every sequence of the sets under its name.
func joined(sets ...map[string]sequence) map[string]sequence {
	all := map[string]sequence{}
	for _, set := range sets {
		for name, sq := range set {
			all[name] = sq
		}
	}
	return all
}

// failable reports whether the fake can fail the entry: a statement, the begin or the commit.
func failable(entry step) bool {
	return entry.name != checkEvent && entry.name != rollbackEvent
}

// stoppedAt returns the entries a call logs when the one at the position fails, a write rolling back on a statement.
func stoppedAt(golden []step, at int) []step {
	stopped := slices.Clone(golden[:at+1])
	failed := golden[at].name
	if golden[0].name == beginEvent && failed != beginEvent && failed != commitEvent {
		stopped = append(stopped, rolledBack)
	}
	return stopped
}

// listed returns the entries one to a line.
func listed(entries []step) string {
	text := make([]string, len(entries))
	for i, entry := range entries {
		text[i] = fmt.Sprintf("\t%d %s", i, entry)
	}
	return strings.Join(text, "\n")
}

// checkLog fails the test unless the fake logged exactly the entries wanted, in order.
func checkLog(t *testing.T, got, want []step) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("log =\n%s\nwant\n%s", listed(got), listed(want))
	}
}

// checkSettled fails the test unless the call left no transaction open and read at most one UTC instant in it.
func checkSettled(t *testing.T, f *fakeBackend) {
	t.Helper()
	if f.open {
		t.Error("a transaction was left open")
	}
	for _, at := range f.minted {
		if !at.Equal(f.minted[0]) || at.After(f.to) || at.Location() != time.UTC {
			t.Errorf("minted stamps = %v, want one UTC instant read during the call", f.minted)
			return
		}
	}
}

// checkKept fails the test unless the value is the one kept, each stamp read during the call as the minted stand-in.
func checkKept(t *testing.T, f *fakeBackend, kept, value any) {
	t.Helper()
	if got := f.unminted(value); !reflect.DeepEqual(got, kept) {
		t.Errorf("value = %+v, want %+v", got, kept)
	}
}

// checkReadsBeforeTheCheck fails the test unless the three reads the check is handed come right before it.
func checkReadsBeforeTheCheck(t *testing.T, golden []step) {
	t.Helper()
	position := slices.IndexFunc(golden, func(entry step) bool { return entry.name == checkEvent })
	if position < 0 {
		return
	}
	if before := golden[max(position-len(checkReads), 0):position]; !reflect.DeepEqual(before, checkReads) {
		t.Errorf("entries before the check = %v, want %v", before, checkReads)
	}
}

// checkInjected fails the test unless the error carries the injected failure, unwrapped exactly where today's is.
func checkInjected(t *testing.T, sq sequence, failed step, err error) {
	t.Helper()
	if !errors.Is(err, errInjected) {
		t.Fatalf("error = %v, want the injected failure carried", err)
	}
	if sq.rawTx && (failed.name == beginEvent || failed.name == commitEvent) {
		if err != errInjected {
			t.Errorf("error = %q, want the injected failure unwrapped", err)
		}
		return
	}
	if !strings.HasPrefix(err.Error(), dbkit.SQLite.String()+": ") || strings.Contains(err.Error(), "postgres") {
		t.Errorf("error = %q, want it wrapped under the engine's name alone", err)
	}
}

// checkNoSentinel fails the test when the error reads as any sentinel a store answers with.
func checkNoSentinel(t *testing.T, err error) {
	t.Helper()
	for _, sentinel := range storeSentinels {
		if errors.Is(err, sentinel) {
			t.Errorf("errors.Is(%v, %v) = true, want no sentinel", err, sentinel)
		}
	}
}

// checkRanPastItsReads fails the test unless the check ran once exactly when every entry before it passed.
func checkRanPastItsReads(t *testing.T, golden []step, at, checks int) {
	t.Helper()
	position := slices.IndexFunc(golden, func(entry step) bool { return entry.name == checkEvent })
	ran := 0
	if position >= 0 && position < at {
		ran = 1
	}
	if checks != ran {
		t.Errorf("check ran %d times, want %d", checks, ran)
	}
}

// checkValueBeside fails the test unless the method answered today's value beside the failure.
func checkValueBeside(t *testing.T, f *fakeBackend, sq sequence, failed step, value any) {
	t.Helper()
	if failed.name == commitEvent && sq.kept != nil {
		checkKept(t, f, sq.kept, value)
		return
	}
	if value != nil && !reflect.ValueOf(value).IsZero() {
		t.Errorf("value = %+v, want the zero value", value)
	}
}

func TestEachMethodSendsTodaysStatementsInOrderWithTheColumnsTheyBind(t *testing.T) {
	t.Parallel()

	for name, sq := range sequences {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f, value, err := sq.runOn(t, noFailure)

			if !errors.Is(err, sq.ends) {
				t.Fatalf("error = %v, want %v", err, sq.ends)
			}
			checkLog(t, f.log, sq.golden)
			checkReadsBeforeTheCheck(t, sq.golden)
			checkSettled(t, f)
			if sq.kept != nil {
				checkKept(t, f, sq.kept, value)
			}
		})
	}
}

func TestEachMethodStopsWhereAStatementTheBeginOrTheCommitFailsAndRollsBackItsWrite(t *testing.T) {
	t.Parallel()

	for name, sq := range sequences {
		for at, entry := range sq.golden {
			if !failable(entry) {
				continue
			}
			t.Run(fmt.Sprintf("%s when entry %d %s fails", name, at, entry.name), func(t *testing.T) {
				t.Parallel()

				f, value, err := sq.runOn(t, at)

				checkInjected(t, sq, entry, err)
				checkNoSentinel(t, err)
				checkLog(t, f.log, stoppedAt(sq.golden, at))
				checkRanPastItsReads(t, sq.golden, at, f.checks)
				checkValueBeside(t, f, sq, entry, value)
				checkSettled(t, f)
			})
		}
	}
}
