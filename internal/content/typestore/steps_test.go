// SPDX-License-Identifier: Apache-2.0

package typestore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// unreadableLocation is a stored location no decoder reads.
var unreadableLocation = []byte("{not json")

// undecodedText is what the decoder answers the unreadable location with.
var undecodedText = decodeFailure(unreadableLocation)

// decodeFailure returns the text the decoder answers the raw location with, or nothing when it reads.
func decodeFailure(raw []byte) string {
	var rules content.Rules
	if err := json.Unmarshal(raw, &rules); err != nil {
		return err.Error()
	}
	return ""
}

// storeUnder returns a store on the engine whose statements and transactions all reach the fake.
func storeUnder(f *fakeBackend, engine dbkit.Engine) *Store {
	return New(Backend{
		Engine:    engine,
		Pool:      fakeQueries{fake: f},
		InTx:      f.inTx,
		Rules:     tailRules,
		ReadError: readBrokenRule,
	})
}

// libraryWith returns the library shelf as the change leaves it.
func libraryWith(change func(stocked *shelf)) func() shelf {
	return func() shelf {
		stocked := library()
		change(&stocked)
		return stocked
	}
}

// shelvedField returns the field row stamped as every row of the library shelf is.
func shelvedField(row FieldRow) FieldRow {
	row.CreatedAt, row.UpdatedAt = shelvedAt, shelvedAt
	return row
}

// takenInBookDetails returns what the key check answers when the book details group already holds the key.
func takenInBookDetails(key string) error {
	return content.Refuse(content.ErrFieldTaken, "field_taken",
		fmt.Sprintf("%s: %s in Book details", content.ErrFieldTaken, key),
		content.Details{"field": key, "group": "Book details"})
}

// flatNovel returns the book type edited into novels that stop nesting.
func flatNovel() content.Type {
	t := novelUnder("books")
	t.Hierarchical = false
	return t
}

// flatNovelRow returns what writing the book type edited into novels that stop nesting binds.
func flatNovelRow() TypeRow {
	row := novelRow("books")
	row.Hierarchical = false
	return row
}

// epigraph returns a required text field asked for inside a container.
func epigraph() content.Field {
	return content.Field{
		Key: "epigraph", Label: "Epigraph", Kind: content.FieldKindText, Required: true, Origin: "library",
	}
}

// isbn returns a text field asked for at a group's top.
func isbn() content.Field {
	return content.Field{
		Key: "isbn", Label: "ISBN", Kind: content.FieldKindText, CreatedAt: editedAt, UpdatedAt: editedAt,
	}
}

// arm is one branch of a step the golden lists leave out, with the statements and the answer today's step gives.
type arm struct {
	stocked func() shelf
	run     func(ctx context.Context, s *Store) (any, error)
	log     []step
	ends    error
	text    string
	kept    any
}

// typeArms holds the branches of the type steps the golden lists leave out or answer without a value.
var typeArms = map[string]arm{
	"ByKey on a type no row holds answers type not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.ByKey(ctx, "film"))
		},
		log:  []step{sent("GetContentType", keyed{"film"})},
		ends: content.ErrTypeNotFound,
	},
	"Update on a type no row holds answers type not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, content.Type{
				Key: "film", SingularLabel: "Film", PluralLabel: "Films", RouteWord: "films",
				PageKind: content.PageKindSingle, Active: true, UpdatedAt: editedAt,
			}))
		},
		log:  []step{begun, sent("LockContentType", keyed{"film"}), rolledBack},
		ends: content.ErrTypeNotFound,
	},
	"Update keeping its route word answers the type as stored": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, novelUnder("books")))
		},
		log: []step{
			begun, sent("LockContentType", keyed{"book"}), sent("UpdateContentType", novelRow("books")), committed,
		},
		kept: content.Type{
			Key: "book", SingularLabel: "Novel", PluralLabel: "Novels", Description: "Long fiction",
			RouteWord: "books", Hierarchical: true, RevisionCap: 5, PageKind: content.PageKindArchive,
			Active: true, Origin: "library", CreatedAt: shelvedAt, UpdatedAt: editedAt,
		},
	},
	"Update cannot replace a stored route word with page": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, novelUnder("page")))
		},
		log:  []step{begun, sent("LockContentType", keyed{"book"}), rolledBack},
		ends: content.ErrRouteWordReserved,
	},
	"Update stopping nesting while items nest answers how many and rolls back": {
		stocked: libraryWith(func(stocked *shelf) { stocked.nested["book"] = 3 }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, flatNovel()))
		},
		log: []step{
			begun, sent("LockContentType", keyed{"book"}), sent("CountNestedContent", keyed{"book"}), rolledBack,
		},
		ends: content.NestingInUse("book", 3),
	},
	"Update stopping nesting while a single item nests answers it and rolls back": {
		stocked: libraryWith(func(stocked *shelf) { stocked.nested["book"] = 1 }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, flatNovel()))
		},
		log: []step{
			begun, sent("LockContentType", keyed{"book"}), sent("CountNestedContent", keyed{"book"}), rolledBack,
		},
		ends: content.NestingInUse("book", 1),
	},
	"Update stopping nesting with no item nesting answers the type as stored": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, flatNovel()))
		},
		log: []step{
			begun, sent("LockContentType", keyed{"book"}), sent("CountNestedContent", keyed{"book"}),
			sent("UpdateContentType", flatNovelRow()), committed,
		},
		kept: content.Type{
			Key: "book", SingularLabel: "Novel", PluralLabel: "Novels", Description: "Long fiction",
			RouteWord: "books", RevisionCap: 5, PageKind: content.PageKindArchive,
			Active: true, Origin: "library", CreatedAt: shelvedAt, UpdatedAt: editedAt,
		},
	},
	"Update taking the root no type holds answers the type at the root": {
		stocked: rootless,
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, rootBook(true)))
		},
		log: []step{
			begun, sent("LockContentType", keyed{"book"}), lockedRoot,
			sent("UpdateContentType", rootBookRow(true)), deferredAddresses,
			sent("RetypeContentPaths", retyped{Key: "book", Was: "books", UpdatedAt: editedAt}),
			committed,
		},
		kept: content.Type{
			Key: "book", SingularLabel: "Book", PluralLabel: "Books", Description: "Printed works",
			Hierarchical: true, Revisions: true, RevisionCap: 25, PageKind: content.PageKindSingle,
			Default: true, Active: true, Origin: "library", CreatedAt: shelvedAt, UpdatedAt: editedAt,
		},
	},
	"Update taking the root from a type whose plural label words a reserved route answers it reserved": {
		stocked: libraryWith(func(stocked *shelf) { stocked.types[0].PluralLabel = "Media" }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, rootBook(true)))
		},
		log:  []step{begun, sent("LockContentType", keyed{"book"}), lockedRoot, rolledBack},
		ends: content.DerivedRouteWordReserved(content.Type{Key: "page", PluralLabel: "Media", RouteWord: "media"}),
	},
	"Update taking the root cannot move the previous root under page": {
		stocked: libraryWith(func(stocked *shelf) { stocked.types[0].PluralLabel = "Page" }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, rootBook(true)))
		},
		log:  []step{begun, sent("LockContentType", keyed{"book"}), lockedRoot, rolledBack},
		ends: content.DerivedRouteWordReserved(content.Type{Key: "page", PluralLabel: "Page", RouteWord: "page"}),
	},
	"Update taking the root from a type with no plural label wraps the label failure": {
		stocked: libraryWith(func(stocked *shelf) { stocked.types[0].PluralLabel = "" }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, rootBook(true)))
		},
		log:  []step{begun, sent("LockContentType", keyed{"book"}), lockedRoot, rolledBack},
		ends: content.ErrInvalidLabel,
		text: "postgres: update content type: content: invalid type label",
	},
	"Delete on a type no row holds answers type not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.Delete(ctx, "film")
		},
		log:  []step{sent("DeleteContentType", keyed{"film"})},
		ends: content.ErrTypeNotFound,
	},
	"Nested answers how many items of the type nest": {
		stocked: libraryWith(func(stocked *shelf) { stocked.nested["book"] = 3 }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Nested(ctx, "book"))
		},
		log:  []step{sent("CountNestedContent", keyed{"book"})},
		kept: 3,
	},
	"AdoptType on a type no row holds answers type not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.AdoptType(ctx, "film")
		},
		log:  []step{sent("AdoptContentType", stamping{Key: "film", At: mintedStamp})},
		ends: content.ErrTypeNotFound,
	},
	"AdoptGroup on a group no row holds answers group not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.AdoptGroup(ctx, "reading-notes")
		},
		log: []step{
			begun, sent("AdoptFieldGroup", stamping{Key: "reading-notes", At: mintedStamp}), rolledBack,
		},
		ends: content.ErrGroupNotFound,
	},
}

// unreadableGroup returns the library shelf with the person details location stored unreadable.
func unreadableGroup() shelf {
	stocked := library()
	stocked.groups[1].Location = unreadableLocation
	return stocked
}

// groupArms holds the branches of the group steps the golden lists leave out or answer without a value.
var groupArms = map[string]arm{
	"ListGroups on a location that does not decode answers the decode failure": {
		stocked: unreadableGroup,
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.ListGroups(ctx))
		},
		log:  []step{readGroups, readFields},
		text: "postgres: reading a group location: " + undecodedText,
	},
	"UpdateGroup on a location that does not decode rolls back and wraps the decode failure": {
		stocked: unreadableGroup,
		run: func(ctx context.Context, s *Store) (any, error) {
			g := content.Group{ID: 1, Title: "Book facts", Location: bookOnly, Active: true}
			return valued(s.UpdateGroup(ctx, g, nil, nil))
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		text: "postgres: update field group: postgres: reading a group location: " + undecodedText,
	},
	"CreateGroup minting a key from a title no group holds": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.CreateGroup(ctx, content.Group{Title: "Reading notes", Location: bookOnly}))
		},
		log: []step{
			begun, lockedGroups, readGroupKeys,
			sent("CreateFieldGroup", groupColumns{
				Key: "reading-notes", Title: "Reading notes", Location: bookLocation,
				CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
			}),
			committed,
		},
		kept: content.Group{
			ID: 5, Key: "reading-notes", Title: "Reading notes", Location: bookOnly, Position: 5, Active: true,
			CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
		},
	},
	"CreateGroup minting a key past two its title takes": {
		stocked: libraryWith(func(stocked *shelf) {
			stocked.groups = append(stocked.groups, GroupRow{
				ID: 5, Position: 5, Key: "book-details-2", Title: "Book details", Location: []byte(bookLocation),
				CreatedAt: shelvedAt, UpdatedAt: shelvedAt,
			})
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.CreateGroup(ctx, content.Group{Title: "Book details", Location: everyTypeRules}))
		},
		log: []step{
			begun, lockedGroups, readGroupKeys,
			sent("CreateFieldGroup", groupColumns{
				Key: "book-details-3", Title: "Book details", Location: everyLocation,
				CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
			}),
			committed,
		},
		kept: content.Group{
			ID: 6, Key: "book-details-3", Title: "Book details", Location: everyTypeRules, Position: 6, Active: true,
			CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
		},
	},
	"UpdateGroup on a group no row holds answers group not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			g := content.Group{ID: 9, Title: "Gone", Location: bookOnly, Active: true}
			return valued(s.UpdateGroup(ctx, g, nil, nil))
		},
		log: []step{
			begun, lockedGroups, readGroups, readFields,
			sent("UpdateFieldGroup", groupColumns{
				ID: 9, Title: "Gone", Location: bookLocation, Active: true, UpdatedAt: mintedStamp,
			}),
			rolledBack,
		},
		ends: content.ErrGroupNotFound,
	},
	"UpdateGroup pointing a group at a type a rival serves one of its keys on answers field taken": {
		run: func(ctx context.Context, s *Store) (any, error) {
			g := content.Group{ID: 2, Title: "Person details", Location: bookOnly, Active: true}
			return valued(s.UpdateGroup(ctx, g, nil, nil))
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, readTypeKeys, rolledBack},
		ends: takenInBookDetails("rating"),
	},
	"UpdateGroup answers the group as stored once it is written": {
		run: func(ctx context.Context, s *Store) (any, error) {
			g := content.Group{ID: 3, Title: "Shared fields", Location: everyTypeRules}
			return valued(s.UpdateGroup(ctx, g, nil, nil))
		},
		log: []step{
			begun, lockedGroups, readGroups, readFields, readTypeKeys,
			sent("UpdateFieldGroup", groupColumns{
				ID: 3, Title: "Shared fields", Location: everyLocation, UpdatedAt: mintedStamp,
			}),
			committed,
		},
		kept: content.Group{
			ID: 3, Key: "shared", Title: "Shared fields", Location: everyTypeRules, Position: 3,
			CreatedAt: shelvedAt, UpdatedAt: mintedStamp,
		},
	},
	"UpdateGroup pointing anew a field whose stamp moved rolls back and wraps the conflict": {
		run: func(ctx context.Context, s *Store) (any, error) {
			g := content.Group{ID: 1, Title: "Book details", Location: bookOnly, Active: true}
			repointed := []content.Field{{Key: "rating", Label: "Score", UpdatedAt: staleAt}}
			return valued(s.UpdateGroup(ctx, g, repointed, nil))
		},
		log: []step{
			begun, lockedGroups, readGroups, readFields, readTypeKeys,
			sent("UpdateFieldGroup", groupColumns{
				ID: 1, Title: "Book details", Location: bookLocation, Active: true, UpdatedAt: mintedStamp,
			}),
			sent("UpdateContentField", fieldColumns{
				GroupID: 1, Key: "rating", Label: "Score", Settings: "{}", UpdatedAt: mintedStamp, Expected: staleAt,
			}),
			rolledBack,
		},
		ends: content.ErrConflict,
		text: "postgres: update field group: content: conflicting update",
	},
	"DeleteGroup on a group no row holds answers group not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteGroup(ctx, 9, nil)
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrGroupNotFound,
	},
	"DeleteFieldsOfGroup on a group no row holds answers group not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteFieldsOfGroup(ctx, 9, []string{"rating"}, nil)
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrGroupNotFound,
	},
	"DeleteFieldsOfGroup naming a key the group does not declare answers field not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteFieldsOfGroup(ctx, 1, []string{"rating", "isbn"}, nil)
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrFieldNotFound,
	},
	"DeleteFieldInGroup on a key the group does not declare answers field not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteFieldInGroup(ctx, 1, "isbn", nil)
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrFieldNotFound,
	},
	"DeleteFieldInGroup sweeping every type when only an inactive group also holds the key": {
		stocked: libraryWith(func(stocked *shelf) {
			stocked.fields = append(stocked.fields, shelvedField(FieldRow{
				ID: 41, GroupID: 4, Key: "subtitle", Label: "Subtitle", Kind: "text",
			}))
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteFieldInGroup(ctx, 3, "subtitle", nil)
		},
		log: slices.Concat(
			[]step{begun, lockedGroups, readGroups, readFields, readTypeKeys},
			fieldSwept(3, "subtitle", everyTypeKey),
			[]step{committed},
		),
	},
	"DeleteFieldInGroup sparing the type an active group also serves the key on": {
		stocked: libraryWith(func(stocked *shelf) {
			stocked.groups[3].Active = true
			stocked.fields = append(stocked.fields, shelvedField(FieldRow{
				ID: 41, GroupID: 4, Key: "subtitle", Label: "Subtitle", Kind: "text",
			}))
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteFieldInGroup(ctx, 3, "subtitle", nil)
		},
		log: slices.Concat(
			[]step{begun, lockedGroups, readGroups, readFields, readTypeKeys},
			fieldSwept(3, "subtitle", pageAndPerson),
			[]step{committed},
		),
	},
}

// fieldArms holds the branches of the field steps the golden lists leave out or answer without a value.
var fieldArms = map[string]arm{
	"UpdateFieldInGroup in a group no row holds answers group not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			f := content.Field{Key: "rating", Label: "Score", UpdatedAt: editedAt}
			return valued(s.UpdateFieldInGroup(ctx, 9, f, shelvedAt, nil))
		},
		log: []step{
			begun, lockedGroups,
			sent("UpdateContentField", fieldColumns{
				GroupID: 9, Key: "rating", Label: "Score", Settings: "{}", UpdatedAt: editedAt, Expected: shelvedAt,
			}),
			readGroups, readFields, rolledBack,
		},
		ends: content.ErrGroupNotFound,
	},
	"UpdateFieldInGroup on a key the group does not declare answers field not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			f := content.Field{Key: "isbn", Label: "ISBN", UpdatedAt: editedAt}
			return valued(s.UpdateFieldInGroup(ctx, 1, f, shelvedAt, nil))
		},
		log: []step{
			begun, lockedGroups,
			sent("UpdateContentField", fieldColumns{
				GroupID: 1, Key: "isbn", Label: "ISBN", Settings: "{}", UpdatedAt: editedAt, Expected: shelvedAt,
			}),
			readGroups, readFields, rolledBack,
		},
		ends: content.ErrFieldNotFound,
	},
	"UpdateSubField on an identity no field carries answers field not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			f := content.Field{Label: "Street line", UpdatedAt: editedAt}
			return valued(s.UpdateSubField(ctx, 99, f, shelvedAt))
		},
		log: []step{
			sent("UpdateSubContentField", fieldColumns{
				ID: 99, Label: "Street line", Settings: "{}", UpdatedAt: editedAt, Expected: shelvedAt,
			}),
			readGroups, readFields,
		},
		ends: content.ErrFieldNotFound,
	},
	"UpdateSubField answers the sub field carrying the edit": {
		run: func(ctx context.Context, s *Store) (any, error) {
			f := content.Field{Label: "Street line", Required: true, UpdatedAt: editedAt}
			return valued(s.UpdateSubField(ctx, 13, f, shelvedAt))
		},
		log: []step{sent("UpdateSubContentField", fieldColumns{
			ID: 13, Label: "Street line", Settings: "{}", Required: true, UpdatedAt: editedAt, Expected: shelvedAt,
		})},
		kept: content.Field{
			ID: 13, GroupID: 1, ParentID: 12, Key: "street", Label: "Street line", Kind: content.FieldKindText,
			Required: true, CreatedAt: shelvedAt, UpdatedAt: editedAt,
		},
	},
	"ReorderSubFields under an identity no field carries answers field not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.ReorderSubFields(ctx, 99, []string{"line"})
		},
		log: []step{
			sent("ReorderSubContentFields", ordering{ParentID: 99, Keys: []string{"line"}}), readGroups, readFields,
		},
		ends: content.ErrFieldNotFound,
	},
	"ReorderFieldsInGroup in a group no row holds answers group not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.ReorderFieldsInGroup(ctx, 9, []string{"rating"})
		},
		log: []step{
			sent("ReorderContentFields", ordering{GroupID: 9, Keys: []string{"rating"}}), readGroups, readFields,
		},
		ends: content.ErrGroupNotFound,
	},
	"DeleteSubField on an identity no field carries answers field not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteSubField(ctx, 99, nil)
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrFieldNotFound,
	},
	"DeleteSubField sparing the type another group serves the whole path on": {
		stocked: libraryWith(func(stocked *shelf) {
			stocked.fields = append(stocked.fields,
				shelvedField(FieldRow{ID: 22, GroupID: 2, Key: "address", Label: "Address", Kind: "section"}),
				shelvedField(FieldRow{
					ID: 23, GroupID: 2, ParentID: 22, Depth: 1, Key: "street", Label: "Street", Kind: "text",
				}),
			)
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteSubField(ctx, 13, nil)
		},
		log: slices.Concat(
			[]step{begun, lockedGroups, readGroups, readFields, readTypeKeys, sent("DeleteFieldByID", identified{13})},
			pathStripped([]string{"address", "street"}, pageAndBook),
			[]step{committed},
		),
	},
	"DeleteSubField sweeping the type another group serves the top key on without the path": {
		stocked: libraryWith(func(stocked *shelf) {
			stocked.fields = append(stocked.fields,
				shelvedField(FieldRow{ID: 22, GroupID: 2, Key: "address", Label: "Address", Kind: "section"}),
				shelvedField(FieldRow{
					ID: 23, GroupID: 2, ParentID: 22, Depth: 1, Key: "city", Label: "City", Kind: "text",
				}),
			)
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteSubField(ctx, 13, nil)
		},
		log: slices.Concat(
			[]step{begun, lockedGroups, readGroups, readFields, readTypeKeys, sent("DeleteFieldByID", identified{13})},
			pathStripped([]string{"address", "street"}, everyTypeKey),
			[]step{committed},
		),
	},
	"DeleteSubField sweeping the type an inactive group seeded ahead of its own declares the path on": {
		stocked: libraryWith(func(stocked *shelf) {
			stocked.groups = slices.Insert(stocked.groups, 0, GroupRow{
				ID: 5, Key: "retired-details", Title: "Retired details", Location: []byte(bookLocation),
				CreatedAt: shelvedAt, UpdatedAt: shelvedAt,
			})
			stocked.fields = append(stocked.fields,
				shelvedField(FieldRow{ID: 50, GroupID: 5, Key: "address", Label: "Address", Kind: "section"}),
				shelvedField(FieldRow{
					ID: 51, GroupID: 5, ParentID: 50, Depth: 1, Key: "street", Label: "Street", Kind: "text",
				}),
			)
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteSubField(ctx, 13, nil)
		},
		log: slices.Concat(
			[]step{begun, lockedGroups, readGroups, readFields, readTypeKeys, sent("DeleteFieldByID", identified{13})},
			pathStripped([]string{"address", "street"}, everyTypeKey),
			[]step{committed},
		),
	},
}

// moveArms holds the branches of the field move the golden lists leave out or answer without a value.
var moveArms = map[string]arm{
	"MoveField on an identity no field carries answers field not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 99, 1, 0, 3, nil))
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrFieldNotFound,
	},
	"MoveField to a group no row holds answers group not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 10, 9, 0, 3, nil))
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrGroupNotFound,
	},
	"MoveField into a container the landing group does not hold answers field not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 10, 2, 12, 3, nil))
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrFieldNotFound,
	},
	"MoveField into a field of its own tree answers the field standing inside itself": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 12, 1, 13, 3, nil))
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.MovesInsideItself("address"),
	},
	"MoveField landing past the depth limit answers too deep": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 15, 1, 12, 2, nil))
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrFieldTooDeep,
	},
	"MoveField where the field already stands answers it as it stands": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 13, 1, 12, 3, nil))
		},
		log: []step{begun, lockedGroups, readGroups, readFields, committed},
		kept: content.Field{
			ID: 13, GroupID: 1, ParentID: 12, Key: "street", Label: "Street", Kind: content.FieldKindText,
			CreatedAt: shelvedAt, UpdatedAt: shelvedAt,
		},
	},
	"MoveField into a container answers the field standing in its new place": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 10, 1, 12, 3, nil))
		},
		log: slices.Concat(
			[]step{
				begun, lockedGroups, readGroups, readFields,
				sent("ReparentContentField", reparenting{ID: 10, ToGroup: 1, ToParent: 12, UpdatedAt: mintedStamp}),
				sent("RecountContentFieldDepth", recounting{ID: 10, ToGroup: 1, Depth: 1}),
				readTypeKeys,
			},
			pathStripped([]string{"rating"}, pageAndBook),
			[]step{committed},
		),
		kept: content.Field{
			ID: 10, GroupID: 1, ParentID: 12, Key: "rating", Label: "Rating", Kind: content.FieldKindNumber,
			CreatedAt: shelvedAt, UpdatedAt: mintedStamp,
		},
	},
	"MoveField to a group's top where a rival holds the key answers field taken": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 20, 3, 0, 3, nil))
		},
		log: []step{
			begun, lockedGroups, readGroups, readFields, readGroups, readFields, readTypeKeys, rolledBack,
		},
		ends: takenInBookDetails("rating"),
	},
	"MoveField out of a container to another group's top where its own group holds the key answers field taken": {
		stocked: libraryWith(func(stocked *shelf) {
			stocked.fields = append(stocked.fields, shelvedField(FieldRow{
				ID: 18, GroupID: 1, ParentID: 12, Depth: 1, Key: "chapters", Label: "Chapters", Kind: "text",
			}))
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 18, 3, 0, 3, nil))
		},
		log: []step{
			begun, lockedGroups, readGroups, readFields, readGroups, readFields, readTypeKeys, rolledBack,
		},
		ends: takenInBookDetails("chapters"),
	},
	"MoveField from a group's top to a group serving every type checks only the rivals and sweeps no type": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.MoveField(ctx, 15, 3, 0, 3, nil))
		},
		log: []step{
			begun, lockedGroups, readGroups, readFields, readGroups, readFields, readTypeKeys,
			sent("ReparentContentField", reparenting{ID: 15, ToGroup: 3, UpdatedAt: mintedStamp}),
			sent("RecountContentFieldDepth", recounting{ID: 15, ToGroup: 3}),
			readTypeKeys, committed,
		},
		kept: content.Field{
			ID: 15, GroupID: 3, Key: "chapters", Label: "Chapters", Kind: content.FieldKindFlexible,
			CreatedAt: shelvedAt, UpdatedAt: mintedStamp,
		},
	},
}

// createArms holds the branches of the field creates the golden lists leave out or answer without a value.
var createArms = map[string]arm{
	"CreateSubField under an identity no field carries answers field not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.CreateSubField(ctx, 99, epigraph(), 3))
		},
		log:  []step{begun, lockedGroups, sent("FieldByID", identified{99}), rolledBack},
		ends: content.ErrFieldNotFound,
	},
	"CreateSubField under a field holding no sub fields answers the wrong shape": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.CreateSubField(ctx, 13, epigraph(), 3))
		},
		log: []step{begun, lockedGroups, sent("FieldByID", identified{13}), rolledBack},
		ends: content.Refuse(content.ErrFieldShape, "field_parent_holds_none",
			"content: wrong kind of value: text holds no sub fields", content.Details{"kind": "text"}),
	},
	"CreateSubField of a field with no label rolls back and wraps the label failure": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.CreateSubField(ctx, 16, content.Field{Key: "epigraph", Kind: content.FieldKindText}, 3))
		},
		log:  []step{begun, lockedGroups, sent("FieldByID", identified{16}), rolledBack},
		ends: content.ErrInvalidFieldLabel,
		text: "postgres: create content field: content: invalid field label",
	},
	"CreateSubField past the depth limit answers too deep and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.CreateSubField(ctx, 16, epigraph(), 1))
		},
		log:  []step{begun, lockedGroups, sent("FieldByID", identified{16}), rolledBack},
		ends: content.ErrFieldTooDeep,
	},
	"CreateSubField answers the field stored inside its container": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.CreateSubField(ctx, 16, epigraph(), 3))
		},
		log: []step{
			begun, lockedGroups, sent("FieldByID", identified{16}),
			sent("CreateSubContentField", fieldColumns{
				GroupID: 1, ParentID: 16, Depth: 2, Key: "epigraph", Label: "Epigraph", Kind: "text",
				Origin: "library", Settings: "{}", Required: true, CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
			}),
			committed,
		},
		kept: content.Field{
			ID: 41, GroupID: 1, ParentID: 16, Key: "epigraph", Label: "Epigraph", Kind: content.FieldKindText,
			Required: true, Origin: "library", CreatedAt: mintedStamp, UpdatedAt: mintedStamp,
		},
	},
	"CreateFieldInGroup in a group no row holds answers group not found and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.CreateFieldInGroup(ctx, 9, isbn(), nil))
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, rolledBack},
		ends: content.ErrGroupNotFound,
	},
	"CreateFieldInGroup under a key a rival group serves answers field taken and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			f := content.Field{Key: "rating", Label: "Rating", Kind: content.FieldKindNumber}
			return valued(s.CreateFieldInGroup(ctx, 3, f, nil))
		},
		log:  []step{begun, lockedGroups, readGroups, readFields, readTypeKeys, rolledBack},
		ends: takenInBookDetails("rating"),
	},
	"CreateFieldInGroup answers the field stored at the group's top": {
		run: func(ctx context.Context, s *Store) (any, error) {
			f := content.Field{
				Key: "related", Label: "Related", Kind: content.FieldKindRelation, RelatesTo: "book", Many: true,
				CreatedAt: editedAt, UpdatedAt: editedAt,
			}
			return valued(s.CreateFieldInGroup(ctx, 3, f, nil))
		},
		log: []step{
			begun, lockedGroups, readGroups, readFields, readTypeKeys,
			sent("CreateContentField", fieldColumns{
				GroupID: 3, Key: "related", Label: "Related", Kind: "relation", RelatesTo: "book", Settings: "{}",
				Many: true, CreatedAt: editedAt, UpdatedAt: editedAt,
			}),
			committed,
		},
		kept: content.Field{
			ID: 41, GroupID: 3, Key: "related", Label: "Related", Kind: content.FieldKindRelation, RelatesTo: "book",
			Many: true, CreatedAt: editedAt, UpdatedAt: editedAt,
		},
	},
}

// everyArm lists every set of branches the golden lists leave out.
var everyArm = []map[string]arm{typeArms, groupArms, fieldArms, moveArms, createArms}

// runArm runs the arm on a fresh fake under the engine, returning the fake, the value and the error.
func runArm(t *testing.T, engine dbkit.Engine, a arm) (*fakeBackend, any, error) {
	stocked := library()
	if a.stocked != nil {
		stocked = a.stocked()
	}
	f := newFake(t, stocked, noFailure)
	value, err := a.run(t.Context(), storeUnder(f, engine))
	f.to = time.Now()
	return f, value, err
}

// checkEnds fails the test unless the error is today's: the very error, or its text as the engine prints it.
func checkEnds(t *testing.T, engine dbkit.Engine, a arm, err error) {
	t.Helper()
	if a.text == "" {
		if !reflect.DeepEqual(err, a.ends) {
			t.Errorf("error = %#v, want %#v", err, a.ends)
		}
		return
	}
	if want := underEngine(engine, a.text); err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
	if a.ends != nil && !errors.Is(err, a.ends) {
		t.Errorf("errors.Is(%v, %v) = false, want it carried", err, a.ends)
	}
}

// settledValue returns the value with each stamp read during the call standing as the minted stand-in.
func settledValue(f *fakeBackend, value any) any {
	field, isField := value.(content.Field)
	if !isField {
		return f.unminted(value)
	}
	field.CreatedAt, field.UpdatedAt = f.settled(field.CreatedAt), f.settled(field.UpdatedAt)
	return field
}

// checkArmValue fails the test unless the value is the one kept, or the zero value when the arm keeps none.
func checkArmValue(t *testing.T, f *fakeBackend, kept, value any) {
	t.Helper()
	if kept == nil {
		if value != nil && !reflect.ValueOf(value).IsZero() {
			t.Errorf("value = %+v, want the zero value", value)
		}
		return
	}
	if got := settledValue(f, value); !reflect.DeepEqual(got, kept) {
		t.Errorf("value = %+v, want %+v", got, kept)
	}
}

func TestEachBranchTheGoldenListsLeaveOutAnswersAsTodayWithTodaysStatements(t *testing.T) {
	t.Parallel()

	for _, arms := range everyArm {
		for name, a := range arms {
			for _, engine := range everyEngine {
				t.Run(name+" under "+engine.String(), func(t *testing.T) {
					t.Parallel()

					f, value, err := runArm(t, engine, a)

					checkEnds(t, engine, a, err)
					checkLog(t, f.log, a.log)
					checkArmValue(t, f, a.kept, value)
					checkSettled(t, f)
				})
			}
		}
	}
}

// refusingCheck returns a caller's check that logs what it was handed and raises errCheckRaised.
func refusingCheck(f *fakeBackend) content.Recheck {
	logged := f.recheck()
	return func(groups []content.Group, types []content.Type) error {
		if err := logged(groups, types); err != nil {
			return err
		}
		return errCheckRaised
	}
}

// checkedCalls holds a call of every method taking the caller's check, handing it the check.
var checkedCalls = map[string]func(ctx context.Context, s *Store, check content.Recheck) (any, error){
	"UpdateGroup": func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
		g := content.Group{ID: 1, Title: "Book facts", Location: bookOnly, Active: true}
		return valued(s.UpdateGroup(ctx, g, nil, check))
	},
	"DeleteGroup": func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
		return nil, s.DeleteGroup(ctx, 4, check)
	},
	"DeleteFieldsOfGroup": func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
		return nil, s.DeleteFieldsOfGroup(ctx, 1, []string{"rating"}, check)
	},
	"DeleteFieldInGroup": func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
		return nil, s.DeleteFieldInGroup(ctx, 1, "rating", check)
	},
	"DeleteSubField": func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
		return nil, s.DeleteSubField(ctx, 13, check)
	},
	"UpdateFieldInGroup": func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
		f := content.Field{Key: "rating", Label: "Score", UpdatedAt: editedAt}
		return valued(s.UpdateFieldInGroup(ctx, 1, f, shelvedAt, check))
	},
	"MoveField": func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
		return valued(s.MoveField(ctx, 13, 1, 0, 3, check))
	},
	"CreateFieldInGroup": func(ctx context.Context, s *Store, check content.Recheck) (any, error) {
		return valued(s.CreateFieldInGroup(ctx, 1, isbn(), check))
	},
}

func TestEachMethodTakingACheckAnswersWhatTheCheckRaisesAsItWroteItAndRollsBack(t *testing.T) {
	t.Parallel()

	for name, call := range checkedCalls {
		for _, engine := range everyEngine {
			t.Run(name+" under "+engine.String(), func(t *testing.T) {
				t.Parallel()

				f := newFake(t, library(), noFailure)
				value, err := call(t.Context(), storeUnder(f, engine), refusingCheck(f))
				f.to = time.Now()

				if err == nil || err.Error() != errCheckRaised.Error() || !errors.Is(err, errCheckRaised) {
					t.Errorf("error = %v, want %q carried as the check wrote it", err, errCheckRaised)
				}
				checkLog(t, f.log, slices.Concat([]step{begun, lockedGroups}, checkUnderLock, []step{rolledBack}))
				checkArmValue(t, f, nil, value)
				checkSettled(t, f)
			})
		}
	}
}

// servedFields returns each field the types carry as the type it serves, its key and the group holding it.
func servedFields(types []content.Type) []string {
	var served []string
	for _, t := range types {
		for _, f := range t.Fields {
			served = append(served, fmt.Sprintf("%s/%s@%d", f.TypeKey, f.Key, f.GroupID))
		}
	}
	return served
}

// readType returns a read of the one type under the key.
func readType(key string) func(ctx context.Context, s *Store) ([]content.Type, error) {
	return func(ctx context.Context, s *Store) ([]content.Type, error) {
		t, err := s.ByKey(ctx, key)
		return []content.Type{t}, err
	}
}

// readEveryType returns every stored type.
func readEveryType(ctx context.Context, s *Store) ([]content.Type, error) {
	return s.List(ctx)
}

// sharedRating returns the library shelf with the group serving every type also declaring a rating.
func sharedRating() shelf {
	stocked := library()
	stocked.fields = append(stocked.fields, shelvedField(FieldRow{
		ID: 31, GroupID: 3, Key: "rating", Label: "Rating", Kind: "number",
	}))
	return stocked
}

func TestTheTypeReadsServeEachKeyOnceFromTheFirstActiveGroupMatchingTheType(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		stocked func() shelf
		read    func(ctx context.Context, s *Store) ([]content.Type, error)
		want    []string
	}{
		"ByKey leaving out the fields of an inactive group matching the type": {
			library, readType("book"),
			[]string{"book/rating@1", "book/author@1", "book/address@1", "book/chapters@1", "book/subtitle@3"},
		},
		"ByKey leaving out the fields of a group matching another type": {
			library, readType("person"), []string{"person/rating@2", "person/author@2", "person/subtitle@3"},
		},
		"ByKey taking a key two groups declare from the first of them": {
			sharedRating, readType("book"),
			[]string{"book/rating@1", "book/author@1", "book/address@1", "book/chapters@1", "book/subtitle@3"},
		},
		"ByKey taking a key from the one group that declares it on the type": {
			sharedRating, readType("page"), []string{"page/subtitle@3", "page/rating@3"},
		},
		"List serving each type the fields of its own groups": {
			library, readEveryType,
			[]string{
				"page/subtitle@3",
				"book/rating@1", "book/author@1", "book/address@1", "book/chapters@1", "book/subtitle@3",
				"person/rating@2", "person/author@2", "person/subtitle@3",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			f := newFake(t, test.stocked(), noFailure)

			types, err := test.read(t.Context(), f.store())
			if err != nil {
				t.Fatalf("read error = %v, want the types read", err)
			}

			if got := servedFields(types); !reflect.DeepEqual(got, test.want) {
				t.Errorf("served fields = %q, want %q", got, test.want)
			}
		})
	}
}

// placedFields returns each group's key and identity, then the path of each field it holds with the group it reads in.
func placedFields(groups []content.Group) []string {
	var placed []string
	for _, g := range groups {
		placed = placedUnder(append(placed, fmt.Sprintf("%s@%d", g.Key, g.ID)), g.Key, g.Fields)
	}
	return placed
}

// placedUnder appends the path of each field under the prefix with the group it reads in, before the fields it holds.
func placedUnder(placed []string, prefix string, fields []content.Field) []string {
	for _, f := range fields {
		path := prefix + "/" + f.Key
		placed = placedUnder(append(placed, fmt.Sprintf("%s@%d", path, f.GroupID)), path, f.Fields)
	}
	return placed
}

func TestListGroupsHangsEachSubFieldUnderItsContainerInTheContainersGroup(t *testing.T) {
	t.Parallel()

	stray := libraryWith(func(stocked *shelf) {
		for i := range stocked.fields {
			if stocked.fields[i].ID == 14 {
				stocked.fields[i].GroupID = 2
			}
		}
	})
	f := newFake(t, stray(), noFailure)

	groups, err := f.store().ListGroups(t.Context())
	if err != nil {
		t.Fatalf("ListGroups() error = %v, want the groups read", err)
	}

	want := []string{
		"book-details@1", "book-details/rating@1", "book-details/author@1", "book-details/address@1",
		"book-details/address/street@1", "book-details/address/agent@1", "book-details/chapters@1",
		"book-details/chapters/chapter@1", "book-details/chapters/chapter/cited@1",
		"person-details@2", "person-details/rating@2", "person-details/author@2",
		"shared@3", "shared/subtitle@3", "archive@4", "archive/summary@4",
	}
	if got := placedFields(groups); !reflect.DeepEqual(got, want) {
		t.Errorf("placed fields = %q, want %q", got, want)
	}
	checkLog(t, f.log, []step{readGroups, readFields})
}
