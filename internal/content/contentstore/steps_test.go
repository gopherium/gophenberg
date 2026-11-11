// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
)

// unknownID is an identity no row of the site shelf carries.
var unknownID = uuid.MustParse("0196a000-0000-7000-8000-000000000999")

// childID is the identity of a page filed under the about page under a page number.
var childID = uuid.MustParse("0196a000-0000-7000-8000-000000000106")

// unreadableLocation is a stored location no decoder reads.
var unreadableLocation = []byte("{not json")

// arm is one branch of a step the golden lists leave out, with the statements and the answer today's step gives.
type arm struct {
	stocked func() shelf
	prepare func(f *fakeBackend)
	run     func(ctx context.Context, s *Store) (any, error)
	log     []step
	ends    error
	code    string
	text    string
	kept    any
}

// rootPage returns a new page the site does not hold yet at the address under the root, carrying the values.
func rootPage(slug string, fields content.Values) content.Content {
	return content.Content{
		ID: newID, Type: "page", Status: content.StatusDraft, Slug: slug, Title: "New page", Path: "page/" + slug,
		AuthorID: writerID, Fields: fields, CreatedAt: editedAt, UpdatedAt: editedAt,
	}
}

// filedUnder returns a new page the site does not hold yet, filed under the parent, carrying the identity given.
func filedUnder(id, parent uuid.UUID, fields content.Values) content.Content {
	return content.Content{
		ID: id, Type: "page", ParentID: idOf(parent), Status: content.StatusDraft, Slug: "history", Title: "History",
		Path: "history", AuthorID: writerID, Fields: fields, CreatedAt: editedAt, UpdatedAt: editedAt,
	}
}

// leading returns the history page leading to the target, and what storing it binds.
func leading(target uuid.UUID) (content.Content, ContentRow) {
	c := history()
	c.Fields = content.Values{"lead": []any{target.String()}}
	row := historyRow
	row.Fields = c.Fields
	return c, row
}

// heldUnderAbout returns the entries a create under the about page logs up to the values it checks.
func heldUnderAbout(child uuid.UUID) []step {
	return []step{
		begun, sent("LockTypeNesting", keyed{"page"}), on("LockAncestors", aboutID),
		sent("LockParent", parenting{ID: aboutID, Child: child}),
	}
}

// leadingLog returns the entries the history page leading to the target logs up to its target check.
func leadingLog(target uuid.UUID) []step {
	_, row := leading(target)
	return slices.Concat(heldUnderAbout(newID), []step{
		readDeclaredKeys, readGroups, sent("CreateContent", row), readFields,
		sent("TypesOfContent", identities{[]uuid.UUID{target}}),
	})
}

// numberedAddressesTaken returns the site shelf with every numbered address of the new post taken.
func numberedAddressesTaken() shelf {
	stocked := site()
	stocked.taken = map[string]bool{}
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		stocked.taken["posts/"+numberedSlug("launch", attempt)] = true
	}
	return stocked
}

// identifiedLaunch is the slug the new post takes once every numbered one is taken.
var identifiedLaunch = identifiedSlug("launch", newID)

// numberedTries returns the entries a bare create logs trying every numbered address of the new post.
func numberedTries() []step {
	var tries []step
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		tries = append(tries, sent("CreateContent", launchRow(numberedSlug("launch", attempt), content.Values{})))
	}
	return tries
}

// historyRetried returns the log of a create under the about page whose first address fails and second is stored.
func historyRetried() []step {
	return slices.Concat(
		heldUnderAbout(newID), []step{readDeclaredKeys, readGroups, sent("CreateContent", historyRow), rolledBack},
		heldUnderAbout(newID), []step{
			readDeclaredKeys, readGroups, sent("CreateContent", func() ContentRow {
				row := historyRow
				row.Slug, row.Path = "history-2", "about/history-2"
				return row
			}()),
			readFields, sent("TypesOfContent", identities{[]uuid.UUID{adaID}}), fieldsCleared(newID, 21),
			sent("AddRelation", RelationRow{FromID: newID, ToID: adaID, FieldID: 21, Position: 1}),
			on("RefreshRelationVisibility", newID), committed,
		},
	)
}

// createArms holds the branches of the create steps the golden lists leave out.
var createArms = map[string]arm{
	"Create at a page of the root listing is refused before any statement": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, rootPage("2", nil)))
		},
		ends: content.ErrReservedAddress,
		text: content.ReservedAddress("page/2").Error(),
	},
	"Create holding values at a page of the root listing is refused and rolls back": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, rootPage("3", content.Values{"subtitle": "Third"})))
		},
		log:  []step{begun, rolledBack},
		ends: content.ErrReservedAddress,
		text: content.ReservedAddress("page/3").Error(),
	},
	"Create once every numbered address is taken stores the identified one": {
		stocked: numberedAddressesTaken,
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, launch(nil)))
		},
		log:  append(numberedTries(), sent("CreateContent", launchRow(identifiedLaunch, content.Values{}))),
		kept: toContent(launchRow(identifiedLaunch, content.Values{})),
	},
	"Create of a post under a parent is refused since posts do not nest": {
		run: func(ctx context.Context, s *Store) (any, error) {
			c := launch(nil)
			c.ParentID = idOf(helloID)
			return valued(s.Create(ctx, c))
		},
		log:  []step{begun, sent("LockTypeNesting", keyed{"post"}), rolledBack},
		ends: content.ErrNotHierarchical,
	},
	"Create under a trashed parent is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, filedUnder(newID, contactID, nil)))
		},
		log: []step{
			begun, sent("LockTypeNesting", keyed{"page"}), on("LockAncestors", contactID),
			sent("LockParent", parenting{ID: contactID, Child: newID}), rolledBack,
		},
		ends: content.ErrParentTrashed,
	},
	"Create under a parent no row holds answers the parent type": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, filedUnder(newID, unknownID, nil)))
		},
		log: []step{
			begun, sent("LockTypeNesting", keyed{"page"}), on("LockAncestors", unknownID),
			sent("LockParent", parenting{ID: unknownID, Child: newID}), rolledBack,
		},
		ends: content.ErrParentType,
	},
	"Create under an item it already stands above is a cycle": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, filedUnder(aboutID, teamID, nil)))
		},
		log: []step{
			begun, sent("LockTypeNesting", keyed{"page"}), on("LockAncestors", teamID),
			sent("LockParent", parenting{ID: teamID, Child: aboutID}), rolledBack,
		},
		ends: content.ErrCycle,
	},
	"Create holding a value no field declares is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, launch(content.Values{"colour": "red"})))
		},
		log:  []step{begun, readDeclaredKeys, rolledBack},
		ends: content.ErrUnknownField,
		text: fmt.Errorf("%w: %s", content.ErrUnknownField, "colour").Error(),
	},
	"Create pointing at a target no row holds is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			c, _ := leading(unknownID)
			return valued(s.Create(ctx, c))
		},
		log:  append(leadingLog(unknownID), rolledBack),
		ends: content.ErrTargetNotFound,
		text: fmt.Errorf("%w: %s", content.ErrTargetNotFound, unknownID).Error(),
	},
	"Create pointing at a target of another type is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			c, _ := leading(helloID)
			return valued(s.Create(ctx, c))
		},
		log:  append(leadingLog(helloID), rolledBack),
		ends: content.ErrTargetType,
		text: fmt.Errorf("%w: %s holds %s", content.ErrTargetType, "lead", "post").Error(),
	},
	"Create under a parent whose address is taken tries the next numbered one": {
		stocked: siteWith(func(stocked *shelf) { stocked.taken = map[string]bool{"about/history": true} }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, history()))
		},
		log: historyRetried(),
	},
	"Create under a parent whose address check deadlocks tries the next numbered one": {
		prepare: func(f *fakeBackend) { f.breaksAt["CreateContent"] = map[int]error{1: addressDeadlocked} },
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, history()))
		},
		log: historyRetried(),
	},
	"Create whose address check deadlocks tries the next numbered one": {
		prepare: func(f *fakeBackend) { f.breaksAt["CreateContent"] = map[int]error{1: addressDeadlocked} },
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, launch(nil)))
		},
		log: []step{
			sent("CreateContent", launchRow("launch", content.Values{})),
			sent("CreateContent", launchRow("launch-2", content.Values{})),
		},
		kept: toContent(launchRow("launch-2", content.Values{})),
	},
	"Create whose address check deadlocks on every try answers the deadlock": {
		prepare: func(f *fakeBackend) { f.breaks["CreateContent"] = addressDeadlocked },
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, launch(nil)))
		},
		log:  append(numberedTries(), sent("CreateContent", launchRow(identifiedLaunch, content.Values{}))),
		ends: addressDeadlocked,
		text: "sqlite: create content: " + addressDeadlocked.Error(),
	},
	"Create deadlocking anywhere but on the address check answers the deadlock": {
		prepare: func(f *fakeBackend) { f.breaks["CreateContent"] = elsewhereDeadlocked },
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, launch(nil)))
		},
		log:  []step{sent("CreateContent", launchRow("launch", content.Values{}))},
		ends: elsewhereDeadlocked,
		text: "sqlite: create content: " + elsewhereDeadlocked.Error(),
	},
	"Create whose target is removed during the write answers target not found": {
		stocked: siteWith(func(stocked *shelf) { stocked.gone = map[uuid.UUID]bool{adaID: true} }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, history()))
		},
		log: slices.Concat(leadingLog(adaID), []step{
			fieldsCleared(newID, 21),
			sent("AddRelation", RelationRow{FromID: newID, ToID: adaID, FieldID: 21, Position: 1}), rolledBack,
		}),
		ends: content.ErrTargetNotFound,
		text: content.ErrTargetNotFound.Error(),
	},
}

// readArms holds the branches of the reads the golden lists leave out.
var readArms = map[string]arm{
	"ByID on an item no row holds answers not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.ByID(ctx, unknownID))
		},
		log:  []step{on("GetContent", unknownID)},
		ends: content.ErrNotFound,
	},
	"PublishedByPath on an address no published item holds answers not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.PublishedByPath(ctx, "about/team"))
		},
		log:  []step{sent("GetPublishedContentByPath", pathed{"about/team"})},
		ends: content.ErrNotFound,
	},
	"Delete on an item no row holds answers not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.Delete(ctx, unknownID)
		},
		log:  []step{on("DeleteContent", unknownID)},
		ends: content.ErrNotFound,
	},
	"PointingAt with nothing pointing answers an empty list, never nil": {
		stocked: siteWith(func(stocked *shelf) { stocked.pointers = nil }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return withTotal(s.PointingAt(ctx, adaID, 11, 1, 10))
		},
		log: []step{
			sent("CountPointingAt", paging{Target: adaID, FieldID: 11}),
			sent("PointingAt", paging{Target: adaID, FieldID: 11, Limit: 10}),
		},
		kept: counted[[]content.Pointer]{Items: []content.Pointer{}},
	},
	"TargetsByIDs with no target stored answers an empty list, never nil": {
		stocked: siteWith(func(stocked *shelf) { stocked.targets = nil }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.TargetsByIDs(ctx, []uuid.UUID{unknownID}))
		},
		log:  []step{sent("SummariesOfTargets", identities{[]uuid.UUID{unknownID}})},
		kept: []content.Target{},
	},
	"RevisionByID on a revision no row holds answers revision not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.RevisionByID(ctx, helloID, unknownID))
		},
		log:  []step{sent("GetRevision", paired{ContentID: helloID, ID: unknownID})},
		ends: content.ErrRevisionNotFound,
	},
	"Autosave the author never saved answers revision not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Autosave(ctx, helloID, writerID))
		},
		log:  []step{sent("GetAutosave", paired{ContentID: helloID, ID: writerID})},
		ends: content.ErrRevisionNotFound,
	},
}

// helloUpTo returns the entries an edit of the post logs up to its value reads, its slug read first.
func helloUpTo() []step {
	return []step{
		sent("SiblingSlugTaken", SiblingSlug{ID: helloID, Type: "post", Slug: "hello"}),
		begun, deferredAddresses, on("GetContent", helloID), readDeclaredKeys, readGroups,
		on("ValuesOfContent", helloID), readFields,
	}
}

// aboutAsPage is the about page renamed to the word of the root listing.
var aboutAsPage = edited(aboutID, func(c *content.Content) { c.Slug, c.Path = "page", "page" })

// servicesAsRootPage is the services page renamed onto the second page of the root listing.
var servicesAsRootPage = edited(servicesID, func(c *content.Content) { c.Slug, c.Path = "2", "page/2" })

// helloWriterAs returns the post naming the value as its writer.
func helloWriterAs(value any) content.Content {
	return edited(helloID, func(c *content.Content) {
		c.Fields = content.Values{"rating": float64(4), "writer": value}
	})
}

// otherPersonID is the identity of a person stored in place of the one an edit keeps naming.
var otherPersonID = uuid.MustParse("0196a000-0000-7000-8000-000000000302")

// helloByBoth is the post naming as its writers the person it held, gone since, and then a person still stored.
var helloByBoth = edited(helloID, func(c *content.Content) {
	c.Fields = content.Values{"rating": float64(4), "writer": []any{adaID.String(), otherPersonID.String()}}
})

// legacyPageRow is a published page stored at the second page of the root listing before that address was reserved.
func legacyPageRow() ContentRow {
	return storedRow(ContentRow{ID: childID, Type: "page", Status: "published", Slug: "2", Title: "Two", Path: "page/2"})
}

// retitledLegacyPage is the page stored at the second page of the root listing, retitled where it stands.
var retitledLegacyPage = func() content.Content {
	c := toContent(legacyPageRow())
	c.Title, c.UpdatedAt = "Second", editedAt
	return c
}()

// updateArms holds the branches of the edit steps the golden lists leave out.
var updateArms = map[string]arm{
	"Update of an item already at a page of the root listing keeps its address": {
		stocked: siteWith(func(stocked *shelf) { stocked.items = append(stocked.items, legacyPageRow()) }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledLegacyPage, shelvedAt, nil, 0))
		},
		log: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: childID, Type: "page", Slug: "2"}),
			begun, deferredAddresses, on("GetContent", childID), readDeclaredKeys, readGroups,
			on("ValuesOfContent", childID), readFields, sent("UpdateContent", editRow(retitledLegacyPage)),
			fieldsCleared(childID, 21), on("RefreshRelationVisibility", childID), committed,
		},
		kept: toContent(storedRowAt(ContentRow{
			ID: childID, Type: "page", Status: "published", Slug: "2", Title: "Second", Path: "page/2",
		}, editedAt)),
	},
	"Update on a stamp the item no longer carries answers conflict": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledHello, staleAt, nil, 0))
		},
		log: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: helloID, Type: "post", Slug: "hello"}),
			begun, deferredAddresses, on("GetContent", helloID), rolledBack,
		},
		ends: content.ErrConflict,
	},
	"Update on an item no row holds answers not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			c := retitledHello
			c.ID, c.Slug, c.Path = unknownID, "gone", "posts/gone"
			return valued(s.Update(ctx, c, shelvedAt, nil, 0))
		},
		log: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: unknownID, Type: "post", Slug: "gone"}),
			begun, deferredAddresses, on("GetContent", unknownID), rolledBack,
		},
		ends: content.ErrNotFound,
	},
	"Update moving under a new parent once its type stopped nesting is refused": {
		stocked: siteWith(func(stocked *shelf) { stocked.nesting["page"] = false }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, aboutUnderServices, shelvedAt, nil, 0))
		},
		log: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: aboutID, ParentID: idOf(servicesID), Type: "page", Slug: "about"}),
			begun, deferredAddresses, on("GetContent", aboutID), sent("LockTypeForMove", keyed{"page"}), rolledBack,
		},
		ends: content.ErrNotHierarchical,
	},
	"Update under the parent it files under once its type stopped nesting is refused": {
		stocked: siteWith(func(stocked *shelf) { stocked.nesting["page"] = false }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledTeam, shelvedAt, nil, 0))
		},
		log: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: teamID, ParentID: idOf(aboutID), Type: "page", Slug: "team"}),
			begun, deferredAddresses, on("GetContent", teamID), sent("LockTypeNesting", keyed{"page"}), rolledBack,
		},
		ends: content.ErrNotHierarchical,
	},
	"Update onto a page of the root listing is refused before the write": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, servicesAsRootPage, shelvedAt, nil, 0))
		},
		log: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: servicesID, Type: "page", Slug: "2"}),
			begun, deferredAddresses, on("GetContent", servicesID), readDeclaredKeys, readGroups,
			on("ValuesOfContent", servicesID), readFields, rolledBack,
		},
		ends: content.ErrReservedAddress,
		text: content.ReservedAddress("page/2").Error(),
	},
	"Update whose write finds no row while the item stands answers conflict": {
		prepare: func(f *fakeBackend) { f.breaksAt["UpdateContent"] = map[int]error{1: sql.ErrNoRows} },
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledHello, shelvedAt, nil, 0))
		},
		log:  append(helloUpTo(), sent("UpdateContent", editRow(retitledHello)), on("GetContent", helloID), rolledBack),
		ends: content.ErrConflict,
	},
	"Update whose write finds no row once the item is gone answers not found": {
		prepare: func(f *fakeBackend) {
			f.breaksAt["UpdateContent"] = map[int]error{1: sql.ErrNoRows}
			f.breaksAt["GetContent"] = map[int]error{2: sql.ErrNoRows}
		},
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledHello, shelvedAt, nil, 0))
		},
		log:  append(helloUpTo(), sent("UpdateContent", editRow(retitledHello)), on("GetContent", helloID), rolledBack),
		ends: content.ErrNotFound,
	},
	"Update carrying a descendant onto a page of the root listing is refused": {
		stocked: siteWith(func(stocked *shelf) {
			stocked.items = append(stocked.items, storedRow(ContentRow{
				ID: childID, ParentID: idOf(aboutID), Type: "page", Status: "draft", Slug: "2", Path: "about/2",
			}))
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, aboutAsPage, shelvedAt, nil, 0))
		},
		log: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: aboutID, Type: "page", Slug: "page"}),
			begun, deferredAddresses, on("GetContent", aboutID), readDeclaredKeys, readGroups,
			on("ValuesOfContent", aboutID), readFields, sent("UpdateContent", editRow(aboutAsPage)),
			fieldsCleared(aboutID, 21), on("RefreshRelationVisibility", aboutID), on("LockDescendants", aboutID),
			sent("MoveDescendants", moving{ID: aboutID, Path: "page", At: editedAt}), rolledBack,
		},
		ends: content.ErrReservedAddress,
		text: content.ReservedAddress("page/2").Error(),
	},
	"Update keeping a snapshot under no cap prunes nothing": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledHello, shelvedAt, &helloSnapshot, 0))
		},
		log: append(helloUpTo(), sent("UpdateContent", editRow(retitledHello)), fieldsCleared(helloID, 11),
			on("RefreshRelationVisibility", helloID), sent("CreateRevision", helloSnapshotRow), committed),
	},
	"Update of an item whose values no row holds keeps no earlier target": {
		prepare: func(f *fakeBackend) { f.breaks["ValuesOfContent"] = sql.ErrNoRows },
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledHello, shelvedAt, nil, 0))
		},
		log: append(helloUpTo(), sent("UpdateContent", editRow(retitledHello)), fieldsCleared(helloID, 11),
			on("RefreshRelationVisibility", helloID), committed),
		kept: toContent(ContentRow{
			ID: helloID, AuthorID: writerID, Type: "post", Status: "published", Slug: "hello", Title: "Hello, world",
			Content: "<p>Hello</p>", Excerpt: "Hello", Path: "posts/hello", Fields: content.Values{"rating": float64(5)},
			CreatedAt: shelvedAt, UpdatedAt: editedAt,
		}),
	},
	"Update keeps naming a target it held that is gone since, without indexing it": {
		stocked: siteWith(func(stocked *shelf) {
			stocked.items[5].Fields = content.Values{"rating": float64(4), "writer": []any{adaID.String()}}
			stocked.items = stocked.items[:6]
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, helloByAda, shelvedAt, nil, 0))
		},
		log: append(helloUpTo(), sent("TypesOfContent", identities{[]uuid.UUID{adaID}}),
			sent("UpdateContent", editRow(helloByAda)), fieldsCleared(helloID, 11),
			on("RefreshRelationVisibility", helloID), committed),
	},
	"Update indexes a stored target it names after a held target gone since": {
		stocked: siteWith(func(stocked *shelf) {
			stocked.items[5].Fields = content.Values{"rating": float64(4), "writer": []any{adaID.String()}}
			stocked.items[6] = storedRow(ContentRow{
				ID: otherPersonID, Type: "person", Status: "published", Slug: "other", Title: "Other", Path: "people/other",
			})
			stocked.fields[1].Many = true
		}),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, helloByBoth, shelvedAt, nil, 0))
		},
		log: append(helloUpTo(), sent("TypesOfContent", identities{[]uuid.UUID{adaID, otherPersonID}}),
			sent("UpdateContent", editRow(helloByBoth)), fieldsCleared(helloID, 11),
			sent("AddRelation", RelationRow{FromID: helloID, ToID: otherPersonID, FieldID: 11, Position: 1}),
			on("RefreshRelationVisibility", helloID), committed),
	},
	"Update pointing at a target of another type is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, helloWriterAs([]any{aboutID.String()}), shelvedAt, nil, 0))
		},
		log:  append(helloUpTo(), sent("TypesOfContent", identities{[]uuid.UUID{aboutID}}), rolledBack),
		ends: content.ErrTargetType,
		text: fmt.Errorf("%w: %s holds %s", content.ErrTargetType, "writer", "page").Error(),
	},
	"Update holding a target value that is not a list wraps the shape error": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, helloWriterAs(adaID.String()), shelvedAt, nil, 0))
		},
		log:  append(helloUpTo(), rolledBack),
		ends: content.ErrFieldShape,
		text: "sqlite: update content: " + content.ErrFieldShape.Error() + ": writer holds a list of targets",
	},
	"Update when a group location does not read wraps the decode error": {
		stocked: siteWith(func(stocked *shelf) { stocked.groups[0].Location = unreadableLocation }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledHello, shelvedAt, nil, 0))
		},
		log: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: helloID, Type: "post", Slug: "hello"}),
			begun, deferredAddresses, on("GetContent", helloID), readDeclaredKeys, readGroups, rolledBack,
		},
		text: "sqlite: update content: sqlite: reading a group location: ",
	},
	"Update whose target is removed during the write answers target not found": {
		stocked: siteWith(func(stocked *shelf) { stocked.gone = map[uuid.UUID]bool{adaID: true} }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, helloByAda, shelvedAt, nil, 0))
		},
		log: append(helloUpTo(), sent("TypesOfContent", identities{[]uuid.UUID{adaID}}),
			sent("UpdateContent", editRow(helloByAda)), fieldsCleared(helloID, 11),
			sent("AddRelation", RelationRow{FromID: helloID, ToID: adaID, FieldID: 11, Position: 1}), rolledBack),
		ends: content.ErrTargetNotFound,
		text: content.ErrTargetNotFound.Error(),
	},
}

// trashArms holds the branches of the trash steps the golden lists leave out.
var trashArms = map[string]arm{
	"Trash an item no row holds answers not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Trash(ctx, unknownID, editedAt))
		},
		log:  []step{begun, on("LockContent", unknownID), rolledBack},
		ends: content.ErrNotFound,
	},
	"Trash an item already in the trash is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Trash(ctx, contactID, editedAt))
		},
		log:  []step{begun, on("LockContent", contactID), rolledBack},
		ends: content.ErrInvalidTransition,
		code: "content_already_trashed",
		text: "content: the item is already in the trash",
	},
	"Trash an item holding children is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Trash(ctx, aboutID, editedAt))
		},
		log:  []step{begun, on("LockContent", aboutID), on("CountChildren", aboutID), rolledBack},
		ends: content.ErrHoldsChildren,
	},
	"Trash whose mark finds no row answers not found": {
		prepare: func(f *fakeBackend) { f.breaks["TrashContent"] = sql.ErrNoRows },
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Trash(ctx, helloID, editedAt))
		},
		log: []step{
			begun, on("LockContent", helloID), on("CountChildren", helloID),
			sent("TrashContent", trashing{ID: helloID, Suffix: suffixShape, At: editedAt}), rolledBack,
		},
		ends: content.ErrNotFound,
	},
	"Restore an item outside the trash is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Restore(ctx, aboutID, editedAt))
		},
		log:  []step{begun, on("LockContent", aboutID), rolledBack},
		ends: content.ErrInvalidTransition,
		code: "restore_not_trashed",
	},
	"Restore an item no row holds answers not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Restore(ctx, unknownID, editedAt))
		},
		log:  []step{begun, on("LockContent", unknownID), rolledBack},
		ends: content.ErrNotFound,
	},
	"Trash answers the item marked with a suffix the restore strips": {
		run: func(ctx context.Context, s *Store) (any, error) {
			trashed, err := s.Trash(ctx, helloID, editedAt)
			return trashedShape(trashed), err
		},
		log: []step{
			begun, on("LockContent", helloID), on("CountChildren", helloID),
			sent("TrashContent", trashing{ID: helloID, Suffix: suffixShape, At: editedAt}),
			on("RefreshRelationVisibility", helloID), committed,
		},
		kept: "trash posts/hello" + suffixShape + " " + editedAt.String(),
	},
}

// trashedShape returns the trashed item's status, address and stamp, its random suffix standing as its shape.
func trashedShape(c content.Content) string {
	return string(c.Status) + " " + trashSuffixPattern.ReplaceAllString(c.Path, suffixShape) + " " + c.UpdatedAt.String()
}

// revisionArms holds the branches of the revision steps the golden lists leave out.
var revisionArms = map[string]arm{
	"DeleteRevision of a revision no row holds answers revision not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteRevision(ctx, helloID, unknownID)
		},
		log: []step{
			begun, on("LockContent", helloID), sent("DeleteRevision", paired{ContentID: helloID, ID: unknownID}),
			rolledBack,
		},
		ends: content.ErrRevisionNotFound,
	},
	"DeleteRevision on a trashed item is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteRevision(ctx, contactID, revisionID)
		},
		log:  []step{begun, on("LockContent", contactID), rolledBack},
		ends: content.ErrTrashed,
	},
	"DeleteRevision on an item no row holds answers not found": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteRevision(ctx, unknownID, revisionID)
		},
		log:  []step{begun, on("LockContent", unknownID), rolledBack},
		ends: content.ErrNotFound,
	},
	"SaveAutosave on a trashed item is refused": {
		run: func(ctx context.Context, s *Store) (any, error) {
			autosave := helloAutosave
			autosave.ContentID = contactID
			return valued(s.SaveAutosave(ctx, autosave))
		},
		log:  []step{begun, on("LockContent", contactID), rolledBack},
		ends: content.ErrTrashed,
	},
}

// arms holds every branch the golden lists leave out.
var arms = joinedArms(createArms, readArms, updateArms, trashArms, revisionArms)

// joinedArms returns every arm of the sets under its name.
func joinedArms(sets ...map[string]arm) map[string]arm {
	all := map[string]arm{}
	for _, set := range sets {
		for name, a := range set {
			all[name] = a
		}
	}
	return all
}

// checkAnswer fails the test unless the error is the one the arm ends in, with its code and its text.
func checkAnswer(t *testing.T, a arm, err error) {
	t.Helper()
	if a.ends == nil && a.text == "" && err != nil {
		t.Fatalf("error = %v, want nil", err)
	}
	if a.ends != nil && !errors.Is(err, a.ends) {
		t.Errorf("error = %v, want %v", err, a.ends)
	}
	if a.code != "" {
		if code, _ := content.CodeOf(err); code != a.code {
			t.Errorf("code = %q, want %q", code, a.code)
		}
	}
	if a.text != "" && (err == nil || !strings.HasPrefix(err.Error(), a.text)) {
		t.Errorf("error = %v, want it to read %q", err, a.text)
	}
}

func TestEachArmTheGoldenListsLeaveOutSendsTodaysStatementsAndAnswersAsToday(t *testing.T) {
	t.Parallel()

	for name, a := range arms {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			stocked := site()
			if a.stocked != nil {
				stocked = a.stocked()
			}
			f := newFake(t, stocked, noFailure)
			if a.prepare != nil {
				a.prepare(f)
			}

			value, err := a.run(t.Context(), f.store(dbkit.SQLite))

			checkAnswer(t, a, err)
			checkLog(t, f.log, a.log)
			checkSettled(t, f)
			if a.kept != nil && !reflect.DeepEqual(value, a.kept) {
				t.Errorf("value = %#v, want %#v", value, a.kept)
			}
			if err != nil {
				checkZero(t, value)
			}
		})
	}
}
