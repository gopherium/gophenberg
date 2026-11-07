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
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// shelvedAt is when every row of the site shelf was created and last updated.
var shelvedAt = time.Date(2025, time.March, 2, 9, 0, 0, 0, time.UTC)

// editedAt is the stamp a caller hands a write.
var editedAt = time.Date(2025, time.April, 6, 14, 30, 0, 0, time.UTC)

// staleAt is a stamp no stored row carries.
var staleAt = time.Date(2025, time.February, 20, 8, 0, 0, 0, time.UTC)

// The identities of the authors, the items and the revisions the site shelf holds, and of one it does not hold yet.
var (
	writerID   = uuid.MustParse("0196a000-0000-7000-8000-000000000001")
	editorID   = uuid.MustParse("0196a000-0000-7000-8000-000000000002")
	aboutID    = uuid.MustParse("0196a000-0000-7000-8000-000000000101")
	teamID     = uuid.MustParse("0196a000-0000-7000-8000-000000000102")
	servicesID = uuid.MustParse("0196a000-0000-7000-8000-000000000103")
	contactID  = uuid.MustParse("0196a000-0000-7000-8000-000000000104")
	pagedID    = uuid.MustParse("0196a000-0000-7000-8000-000000000105")
	helloID    = uuid.MustParse("0196a000-0000-7000-8000-000000000201")
	adaID      = uuid.MustParse("0196a000-0000-7000-8000-000000000301")
	newID      = uuid.MustParse("0196a000-0000-7000-8000-000000000401")
	revisionID = uuid.MustParse("0196a000-0000-7000-8000-000000000501")
	autosaveID = uuid.MustParse("0196a000-0000-7000-8000-000000000502")
)

// The locations the site groups are stored under.
const (
	postLocation = `[[{"source":"content_type","operator":"==","value":"post"}]]`
	pageLocation = `[[{"source":"content_type","operator":"==","value":"page"}]]`
)

// idOf returns a pointer to its own copy of the identity.
func idOf(id uuid.UUID) *uuid.UUID {
	return &id
}

// storedRow returns the row written by the writer when the shelf was stocked, its values never nil.
func storedRow(row ContentRow) ContentRow {
	row.AuthorID, row.CreatedAt, row.UpdatedAt = writerID, shelvedAt, shelvedAt
	if row.Fields == nil {
		row.Fields = content.Values{}
	}
	return row
}

// site returns a shelf of pages, a post and a person, the groups serving them and the revisions the post keeps.
func site() shelf {
	return shelf{
		items: []ContentRow{
			storedRow(ContentRow{ID: aboutID, Type: "page", Status: "published", Slug: "about", Title: "About", Path: "about"}),
			storedRow(ContentRow{
				ID: teamID, ParentID: idOf(aboutID), Type: "page", Status: "draft", Slug: "team", Title: "Team",
				Path: "about/team",
			}),
			storedRow(ContentRow{
				ID: servicesID, Type: "page", Status: "published", Slug: "services", Title: "Services", Path: "services",
			}),
			storedRow(ContentRow{
				ID: contactID, Type: "page", Status: "trash", Slug: "contact-trashed-k3v9x2qa", Title: "Contact",
				Path: "contact-trashed-k3v9x2qa",
			}),
			storedRow(ContentRow{
				ID: pagedID, Type: "page", Status: "trash", Slug: "2-trashed-p4q8r1zz", Title: "2",
				Path: "page/2-trashed-p4q8r1zz",
			}),
			storedRow(ContentRow{
				ID: helloID, Type: "post", Status: "published", Slug: "hello", Title: "Hello", Content: "<p>Hello</p>",
				Excerpt: "Hello", Path: "posts/hello", Fields: content.Values{"rating": float64(4)},
			}),
			storedRow(ContentRow{ID: adaID, Type: "person", Status: "published", Slug: "ada", Title: "Ada", Path: "people/ada"}),
		},
		revisions: []RevisionRow{
			{
				ID: revisionID, ContentID: helloID, AuthorID: writerID, Kind: "revision", Title: "Hello",
				Content: "<p>Hi</p>", Excerpt: "Hi", Fields: content.Values{"rating": float64(3)}, CreatedAt: shelvedAt,
			},
			{
				ID: autosaveID, ContentID: helloID, AuthorID: editorID, Kind: "autosave", Title: "Hello again",
				Content: "<p>Hello again</p>", Excerpt: "Again", Fields: content.Values{}, CreatedAt: shelvedAt,
			},
		},
		groups: []typestore.GroupRow{
			{ID: 1, Position: 1, Key: "post-details", Title: "Post details", Location: []byte(postLocation), Active: true},
			{ID: 2, Position: 2, Key: "page-details", Title: "Page details", Location: []byte(pageLocation), Active: true},
			{ID: 3, Position: 3, Key: "archive", Title: "Archive", Location: []byte(postLocation)},
		},
		fields: []typestore.FieldRow{
			{ID: 10, GroupID: 1, Key: "rating", Label: "Rating", Kind: "number"},
			{ID: 11, GroupID: 1, Key: "writer", Label: "Writer", Kind: "relation", RelatesTo: "person"},
			{ID: 20, GroupID: 2, Key: "subtitle", Label: "Subtitle", Kind: "text"},
			{ID: 21, GroupID: 2, Key: "lead", Label: "Lead", Kind: "relation", RelatesTo: "person"},
			{ID: 30, GroupID: 3, Key: "summary", Label: "Summary", Kind: "text"},
		},
		nesting: map[string]bool{"page": true, "post": false, "person": false},
		listed: []ListedRow{
			{ContentRow: storedRow(ContentRow{ID: helloID, Type: "post", Status: "published", Path: "posts/hello"}),
				AuthorName: "Writer"},
			{ContentRow: storedRow(ContentRow{ID: teamID, ParentID: idOf(aboutID), Type: "page", Path: "about/team"}),
				AuthorName: "Writer", ParentTitle: "About"},
		},
		related:  []ContentRow{storedRow(ContentRow{ID: helloID, Type: "post", Status: "published", Path: "posts/hello"})},
		pointers: []content.Pointer{{ID: helloID, Type: "post", Title: "Hello", Path: "posts/hello"}},
		targets:  []content.Target{{ID: adaID, Title: "Ada", Path: "people/ada"}},
		byAuthor: []AuthorCount{{AuthorID: writerID, Total: 2}},
		depth:    2,
	}
}

// siteWith returns the site shelf as the change leaves it.
func siteWith(change func(stocked *shelf)) func() shelf {
	return func() shelf {
		stocked := site()
		change(&stocked)
		return stocked
	}
}

// The entries a call logs with nothing bound.
var (
	begun                 = step{name: beginEvent}
	committed             = step{name: commitEvent}
	rolledBack            = step{name: rollbackEvent}
	savepointOpened       = step{name: savepointEvent}
	released              = step{name: releaseEvent}
	rolledBackToSavepoint = step{name: rollbackToEvent}
	readDeclaredKeys      = step{name: "LockDeclaredFieldKeys"}
	readGroups            = step{name: "ListFieldGroups"}
	readFields            = step{name: "ListContentFields"}
	deferredAddresses     = step{name: "DeferAddressCheck"}
)

// sent returns the entry of a statement binding the columns.
func sent(name string, args any) step {
	return step{name: name, args: args}
}

// on returns the entry of a statement binding the one identity.
func on(name string, id uuid.UUID) step {
	return step{name: name, args: identified{id}}
}

// fieldsCleared returns the entry clearing the index rows of the relation field the item holds.
func fieldsCleared(id uuid.UUID, fieldID int) step {
	return sent("ClearRelationsOfField", clearing{From: id, FieldID: fieldID})
}

// launch returns a new post the site does not hold yet, carrying the values.
func launch(fields content.Values) content.Content {
	return content.Content{
		ID: newID, Type: "post", Status: content.StatusDraft, Slug: "launch", Title: "Launch", Path: "posts/launch",
		Content: "<p>Launch</p>", Excerpt: "Launch", AuthorID: writerID, Fields: fields,
		CreatedAt: editedAt, UpdatedAt: editedAt,
	}
}

// launchRow returns what storing the new post under the slug binds.
func launchRow(slug string, fields content.Values) ContentRow {
	return ContentRow{
		ID: newID, AuthorID: writerID, Type: "post", Status: "draft", Slug: slug, Title: "Launch",
		Content: "<p>Launch</p>", Excerpt: "Launch", Path: "posts/" + slug, Fields: fields,
		CreatedAt: editedAt, UpdatedAt: editedAt,
	}
}

// history returns a new page filed under the about page, leading to the person.
func history() content.Content {
	return content.Content{
		ID: newID, Type: "page", ParentID: idOf(aboutID), Status: content.StatusDraft, Slug: "history",
		Title: "History", Path: "about/history", AuthorID: writerID,
		Fields:    content.Values{"lead": []any{adaID.String()}},
		CreatedAt: editedAt, UpdatedAt: editedAt,
	}
}

// historyRow is what storing the new page under the about page binds.
var historyRow = ContentRow{
	ID: newID, AuthorID: writerID, ParentID: idOf(aboutID), Type: "page", Status: "draft", Slug: "history",
	Title: "History", Path: "about/history", Fields: content.Values{"lead": []any{adaID.String()}},
	CreatedAt: editedAt, UpdatedAt: editedAt,
}

// edited returns the stored item as the caller hands it to an edit, stamped anew and changed as the edit asks.
func edited(id uuid.UUID, change func(c *content.Content)) content.Content {
	row, _ := site().itemOf(id)
	c := toContent(row)
	c.UpdatedAt = editedAt
	change(&c)
	return c
}

// editRow returns what writing the edited item binds while the stored row still carries the shelf stamp.
func editRow(c content.Content) editing {
	return editing{Row: ContentRow{
		ID: c.ID, ParentID: c.ParentID, Status: string(c.Status), Slug: c.Slug, Title: c.Title, Content: c.Content,
		Excerpt: c.Excerpt, Path: c.Path, Fields: storedValues(c.Fields), PublishedAt: c.PublishedAt,
		UpdatedAt: c.UpdatedAt,
	}, Expected: shelvedAt}
}

// retitledHello is the post retitled and rated anew.
var retitledHello = edited(helloID, func(c *content.Content) {
	c.Title, c.Fields = "Hello, world", content.Values{"rating": float64(5)}
})

// aboutUnderServices is the about page filed under the services page.
var aboutUnderServices = edited(aboutID, func(c *content.Content) {
	c.ParentID, c.Path = idOf(servicesID), "services/about"
})

// retitledTeam is the team page retitled under the parent it already files under.
var retitledTeam = edited(teamID, func(c *content.Content) { c.Title = "Our team" })

// helloByAda is the post naming the person as its writer.
var helloByAda = edited(helloID, func(c *content.Content) {
	c.Fields = content.Values{"rating": float64(4), "writer": []any{adaID.String()}}
})

// servicesAsAbout is the services page renamed to the slug the about page holds.
var servicesAsAbout = edited(servicesID, func(c *content.Content) { c.Slug, c.Path = "about", "about" })

// helloSnapshot is the revision an edit of the post keeps.
var helloSnapshot = content.Revision{
	ID: revisionID, ContentID: helloID, Kind: content.RevisionKindRevision, AuthorID: editorID, Title: "Hello",
	Content: "<p>Hello</p>", Excerpt: "Hello", Fields: content.Values{"rating": float64(4)}, CreatedAt: editedAt,
}

// helloSnapshotRow is what storing the revision of the post binds.
var helloSnapshotRow = RevisionRow{
	ID: revisionID, ContentID: helloID, AuthorID: editorID, Kind: "revision", Title: "Hello",
	Content: "<p>Hello</p>", Excerpt: "Hello", Fields: content.Values{"rating": float64(4)}, CreatedAt: editedAt,
}

// helloAutosave is the editor's autosave of the post.
var helloAutosave = content.Revision{
	ID: autosaveID, ContentID: helloID, Kind: content.RevisionKindAutosave, AuthorID: editorID,
	Title: "Hello there", Content: "<p>Hello there</p>", Excerpt: "There", CreatedAt: editedAt,
}

// valued returns the answer of a method with its value boxed.
func valued[T any](value T, err error) (any, error) {
	return value, err
}

// counted is a list a method answers beside a count.
type counted[T any] struct {
	Items T
	Total int
}

// withTotal returns the answer of a method answering a list and a count, the two boxed together.
func withTotal[T any](items T, total int, err error) (any, error) {
	return counted[T]{Items: items, Total: total}, err
}

// sequence is one method call on a shelf with the entries today's steps log for it.
type sequence struct {
	stocked func() shelf
	run     func(ctx context.Context, s *Store) (any, error)
	golden  []step
	kept    any
}

// runOn runs the call under the SQLite engine on a fresh fake failing the entry at the position.
func (sq sequence) runOn(t *testing.T, failAt int) (*fakeBackend, any, error) {
	stocked := site()
	if sq.stocked != nil {
		stocked = sq.stocked()
	}
	f := newFake(t, stocked, failAt)
	value, err := sq.run(t.Context(), f.store(dbkit.SQLite))
	return f, value, err
}

// itemSequences holds the calls of the item reads and creates with the entries today's steps log for each.
var itemSequences = map[string]sequence{
	"Create a post holding no values": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, launch(nil)))
		},
		golden: []step{sent("CreateContent", launchRow("launch", content.Values{}))},
		kept:   toContent(launchRow("launch", content.Values{})),
	},
	"Create a post whose address another item holds": {
		stocked: siteWith(func(stocked *shelf) { stocked.taken = map[string]bool{"posts/launch": true} }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, launch(nil)))
		},
		golden: []step{
			sent("CreateContent", launchRow("launch", content.Values{})),
			sent("CreateContent", launchRow("launch-2", content.Values{})),
		},
		kept: toContent(launchRow("launch-2", content.Values{})),
	},
	"Create a post holding a value": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, launch(content.Values{"rating": float64(5)})))
		},
		golden: []step{
			begun, readDeclaredKeys, readGroups,
			sent("CreateContent", launchRow("launch", content.Values{"rating": float64(5)})),
			readFields, fieldsCleared(newID, 11), on("RefreshRelationVisibility", newID), committed,
		},
	},
	"Create a page under a parent pointing at one target": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Create(ctx, history()))
		},
		golden: []step{
			begun, sent("LockTypeNesting", keyed{"page"}), on("LockAncestors", aboutID),
			sent("LockParent", parenting{ID: aboutID, Child: newID}), readDeclaredKeys, readGroups,
			sent("CreateContent", historyRow), readFields,
			sent("TypesOfContent", identities{[]uuid.UUID{adaID}}), fieldsCleared(newID, 21),
			sent("AddRelation", RelationRow{FromID: newID, ToID: adaID, FieldID: 21, Position: 1}),
			on("RefreshRelationVisibility", newID), committed,
		},
		kept: toContent(historyRow),
	},
	"ByID": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.ByID(ctx, helloID))
		},
		golden: []step{on("GetContent", helloID)},
	},
	"PublishedByPath": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.PublishedByPath(ctx, "posts/hello"))
		},
		golden: []step{sent("GetPublishedContentByPath", pathed{"posts/hello"})},
	},
	"Children": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Children(ctx, aboutID))
		},
		golden: []step{on("CountChildren", aboutID)},
		kept:   1,
	},
	"Depth": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Depth(ctx, aboutID))
		},
		golden: []step{on("ContentDepth", aboutID)},
		kept:   2,
	},
	"Delete": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.Delete(ctx, helloID)
		},
		golden: []step{on("DeleteContent", helloID)},
	},
	"CountsByAuthor": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.CountsByAuthor(ctx, "post"))
		},
		golden: []step{sent("CountContentByAuthor", keyed{"post"})},
		kept:   map[uuid.UUID]int{writerID: 2},
	},
}

// postFilter is a filter narrowing the posts by every term a listing takes.
var postFilter = content.Filter{
	Type: "post", Statuses: []content.Status{content.StatusPublished, content.StatusDraft},
	Authors: []uuid.UUID{writerID}, ExcludeAuthors: []uuid.UUID{editorID}, Before: &editedAt, After: &staleAt,
	Search: "50%_off", OrderBy: content.OrderByTitle, Order: content.OrderAsc, Page: 3, PerPage: 20,
	Fields: map[string]any{"rating": float64(4)},
}

// postNarrowing is what the count and list statements narrow the post filter by.
var postNarrowing = Narrowing{
	Type: "post", Search: "50%_off", FieldFilter: []byte(`{"rating":4}`), Statuses: []string{"published", "draft"},
	Authors: []uuid.UUID{writerID}, ExcludedAuthors: []uuid.UUID{editorID}, Before: &editedAt, After: &staleAt,
}

// postPage is the page of the post filter the list statements read.
var postPage = Page{Narrowing: postNarrowing, OrderBy: "title", OrderDir: "asc", Limit: 20, Offset: 40}

// nestedFilter returns the post filter listing each child under its parent.
func nestedFilter() content.Filter {
	f := postFilter
	f.Hierarchy = true
	return f
}

// listSequences holds the calls of the listings with the entries today's steps log for each.
var listSequences = map[string]sequence{
	"List flat": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return withTotal(s.List(ctx, postFilter))
		},
		golden: []step{sent("CountContent", postNarrowing), sent("ListContent", postPage)},
	},
	"List nested": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return withTotal(s.List(ctx, nestedFilter()))
		},
		golden: []step{sent("CountContent", postNarrowing), sent("ListNestedContent", postPage)},
	},
	"RelatedTo": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return withTotal(s.RelatedTo(ctx, adaID, 2, 10))
		},
		golden: []step{
			on("CountRelatedContent", adaID), sent("ListRelatedContent", paging{Target: adaID, Limit: 10, Offset: 10}),
		},
	},
	"PointingAt": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return withTotal(s.PointingAt(ctx, adaID, 11, 3, 5))
		},
		golden: []step{
			sent("CountPointingAt", paging{Target: adaID, FieldID: 11}),
			sent("PointingAt", paging{Target: adaID, FieldID: 11, Limit: 5, Offset: 10}),
		},
		kept: counted[[]content.Pointer]{
			Items: []content.Pointer{{ID: helloID, Type: "post", Title: "Hello", Path: "posts/hello"}}, Total: 1,
		},
	},
	"TargetsByIDs": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.TargetsByIDs(ctx, []uuid.UUID{adaID, newID}))
		},
		golden: []step{sent("SummariesOfTargets", identities{[]uuid.UUID{adaID, newID}})},
		kept:   []content.Target{{ID: adaID, Title: "Ada", Path: "people/ada"}},
	},
}

// updateSequences holds the calls of the item edit with the entries today's steps log for each.
var updateSequences = map[string]sequence{
	"Update keeping its address and a snapshot": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledHello, shelvedAt, &helloSnapshot, 10))
		},
		golden: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: helloID, Type: "post", Slug: "hello"}),
			begun, deferredAddresses, on("GetContent", helloID), readDeclaredKeys, readGroups,
			on("ValuesOfContent", helloID), readFields, sent("UpdateContent", editRow(retitledHello)),
			fieldsCleared(helloID, 11), on("RefreshRelationVisibility", helloID),
			sent("CreateRevision", helloSnapshotRow), sent("PruneRevisions", pruning{ContentID: helloID, Keep: 10}),
			committed,
		},
	},
	"Update moving under a new parent with a snapshot": {
		run: func(ctx context.Context, s *Store) (any, error) {
			snapshot := helloSnapshot
			snapshot.ContentID = aboutID
			return valued(s.Update(ctx, aboutUnderServices, shelvedAt, &snapshot, 3))
		},
		golden: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: aboutID, ParentID: idOf(servicesID), Type: "page", Slug: "about"}),
			begun, deferredAddresses, on("GetContent", aboutID), sent("LockTypeForMove", keyed{"page"}),
			on("LockAncestors", servicesID), sent("LockParent", parenting{ID: servicesID, Child: aboutID}),
			readDeclaredKeys, readGroups, on("ValuesOfContent", aboutID), readFields,
			sent("UpdateContent", editRow(aboutUnderServices)), fieldsCleared(aboutID, 21),
			on("RefreshRelationVisibility", aboutID), on("LockDescendants", aboutID),
			sent("MoveDescendants", moving{ID: aboutID, Path: "services/about", At: editedAt}),
			sent("CreateRevision", RevisionRow{
				ID: revisionID, ContentID: aboutID, AuthorID: editorID, Kind: "revision", Title: "Hello",
				Content: "<p>Hello</p>", Excerpt: "Hello", Fields: content.Values{"rating": float64(4)}, CreatedAt: editedAt,
			}),
			sent("PruneRevisions", pruning{ContentID: aboutID, Keep: 3}), committed,
		},
	},
	"Update under the parent it already files under": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, retitledTeam, shelvedAt, nil, 10))
		},
		golden: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: teamID, ParentID: idOf(aboutID), Type: "page", Slug: "team"}),
			begun, deferredAddresses, on("GetContent", teamID), sent("LockTypeNesting", keyed{"page"}),
			readDeclaredKeys, readGroups, on("ValuesOfContent", teamID), readFields,
			sent("UpdateContent", editRow(retitledTeam)), fieldsCleared(teamID, 21),
			on("RefreshRelationVisibility", teamID), committed,
		},
	},
	"Update pointing at one target": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, helloByAda, shelvedAt, nil, 0))
		},
		golden: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: helloID, Type: "post", Slug: "hello"}),
			begun, deferredAddresses, on("GetContent", helloID), readDeclaredKeys, readGroups,
			on("ValuesOfContent", helloID), readFields, sent("TypesOfContent", identities{[]uuid.UUID{adaID}}),
			sent("UpdateContent", editRow(helloByAda)), fieldsCleared(helloID, 11),
			sent("AddRelation", RelationRow{FromID: helloID, ToID: adaID, FieldID: 11, Position: 1}),
			on("RefreshRelationVisibility", helloID), committed,
		},
	},
	"Update onto a slug a sibling holds": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Update(ctx, servicesAsAbout, shelvedAt, nil, 0))
		},
		golden: []step{
			sent("SiblingSlugTaken", SiblingSlug{ID: servicesID, Type: "page", Slug: "about"}),
			sent("SiblingSlugTaken", SiblingSlug{ID: servicesID, Type: "page", Slug: "about-2"}),
			begun, deferredAddresses, on("GetContent", servicesID), readDeclaredKeys, readGroups,
			on("ValuesOfContent", servicesID), readFields,
			sent("UpdateContent", editing{Row: ContentRow{
				ID: servicesID, Status: "published", Slug: "about-2", Title: "Services", Path: "about-2",
				Fields: content.Values{}, UpdatedAt: editedAt,
			}, Expected: shelvedAt}),
			fieldsCleared(servicesID, 21), on("RefreshRelationVisibility", servicesID),
			on("LockDescendants", servicesID),
			sent("MoveDescendants", moving{ID: servicesID, Path: "about-2", At: editedAt}), committed,
		},
	},
}

// trashSequences holds the calls of the trash and its restore with the entries today's steps log for each.
var trashSequences = map[string]sequence{
	"Trash": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Trash(ctx, helloID, editedAt))
		},
		golden: []step{
			begun, on("LockContent", helloID), on("CountChildren", helloID),
			sent("TrashContent", trashing{ID: helloID, Suffix: suffixShape, At: editedAt}),
			on("DeleteAutosavesOfContent", helloID), on("RefreshRelationVisibility", helloID), committed,
		},
	},
	"Restore under its original address": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Restore(ctx, contactID, editedAt))
		},
		golden: []step{
			begun, on("LockContent", contactID), savepointOpened,
			sent("RestoreContent", stamped{ID: contactID, At: editedAt}), released,
			on("RefreshRelationVisibility", contactID), committed,
		},
		kept: toContent(storedRowAt(ContentRow{
			ID: contactID, Type: "page", Status: "draft", Slug: "contact", Title: "Contact", Path: "contact",
		}, editedAt)),
	},
	"Restore onto an address another item took": {
		stocked: siteWith(func(stocked *shelf) { stocked.taken = map[string]bool{"contact": true} }),
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Restore(ctx, contactID, editedAt))
		},
		golden: []step{
			begun, on("LockContent", contactID), savepointOpened,
			sent("RestoreContent", stamped{ID: contactID, At: editedAt}), rolledBackToSavepoint,
			sent("RestoreContentKeepingSlug", stamped{ID: contactID, At: editedAt}),
			on("RefreshRelationVisibility", contactID), committed,
		},
		kept: toContent(storedRowAt(ContentRow{
			ID: contactID, Type: "page", Status: "draft", Slug: "contact-trashed-k3v9x2qa", Title: "Contact",
			Path: "contact-trashed-k3v9x2qa",
		}, editedAt)),
	},
	"Restore onto a page of the root listing": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Restore(ctx, pagedID, editedAt))
		},
		golden: []step{
			begun, on("LockContent", pagedID), savepointOpened,
			sent("RestoreContent", stamped{ID: pagedID, At: editedAt}), rolledBackToSavepoint,
			sent("RestoreContentKeepingSlug", stamped{ID: pagedID, At: editedAt}),
			on("RefreshRelationVisibility", pagedID), committed,
		},
		kept: toContent(storedRowAt(ContentRow{
			ID: pagedID, Type: "page", Status: "draft", Slug: "2-trashed-p4q8r1zz", Title: "2",
			Path: "page/2-trashed-p4q8r1zz",
		}, editedAt)),
	},
	"EmptyTrash of every author": {
		run: func(ctx context.Context, s *Store) (any, error) {
			deleted, kept, err := s.EmptyTrash(ctx, "page", nil)
			return counted[int]{Items: deleted, Total: kept}, err
		},
		golden: []step{begun, sent("DeleteTrash", sweeping{Type: "page"}), sent("CountTrash", keyed{"page"}), committed},
		kept:   counted[int]{Items: 2},
	},
	"EmptyTrash of one author": {
		stocked: siteWith(func(stocked *shelf) { stocked.items[3].AuthorID = editorID }),
		run: func(ctx context.Context, s *Store) (any, error) {
			deleted, kept, err := s.EmptyTrash(ctx, "page", idOf(writerID))
			return counted[int]{Items: deleted, Total: kept}, err
		},
		golden: []step{
			begun, sent("DeleteTrash", sweeping{Type: "page", Author: idOf(writerID)}), sent("CountTrash", keyed{"page"}),
			committed,
		},
		kept: counted[int]{Items: 1, Total: 1},
	},
}

// storedRowAt returns the row as the shelf stocked it, last written at the stamp.
func storedRowAt(row ContentRow, at time.Time) ContentRow {
	row = storedRow(row)
	row.UpdatedAt = at
	return row
}

// revisionSequences holds the calls of the revisions and autosaves with the entries today's steps log for each.
var revisionSequences = map[string]sequence{
	"Revisions": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Revisions(ctx, helloID))
		},
		golden: []step{on("ListRevisions", helloID)},
	},
	"RevisionByID": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.RevisionByID(ctx, helloID, revisionID))
		},
		golden: []step{sent("GetRevision", paired{ContentID: helloID, ID: revisionID})},
	},
	"DeleteRevision": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteRevision(ctx, helloID, revisionID)
		},
		golden: []step{
			begun, on("LockContent", helloID), sent("DeleteRevision", paired{ContentID: helloID, ID: revisionID}),
			committed,
		},
	},
	"SaveAutosave": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.SaveAutosave(ctx, helloAutosave))
		},
		golden: []step{
			begun, on("LockContent", helloID),
			sent("UpsertAutosave", RevisionRow{
				ID: autosaveID, ContentID: helloID, AuthorID: editorID, Title: "Hello there",
				Content: "<p>Hello there</p>", Excerpt: "There", Fields: content.Values{}, CreatedAt: editedAt,
			}),
			committed,
		},
		kept: content.Revision{
			ID: autosaveID, ContentID: helloID, Kind: content.RevisionKindAutosave, AuthorID: editorID,
			Title: "Hello there", Content: "<p>Hello there</p>", Excerpt: "There", Fields: content.Values{},
			CreatedAt: editedAt,
		},
	},
	"Autosave": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return valued(s.Autosave(ctx, helloID, editorID))
		},
		golden: []step{sent("GetAutosave", paired{ContentID: helloID, ID: editorID})},
	},
	"DeleteAutosave": {
		run: func(ctx context.Context, s *Store) (any, error) {
			return nil, s.DeleteAutosave(ctx, helloID, editorID)
		},
		golden: []step{sent("DeleteAutosave", paired{ContentID: helloID, ID: editorID})},
	},
}

// sequences holds every call with the entries today's steps log for it.
var sequences = joined(itemSequences, listSequences, updateSequences, trashSequences, revisionSequences)

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

// contentSentinels lists every sentinel a content store method answers with today, and the missing row.
var contentSentinels = []error{
	content.ErrNotFound, content.ErrConflict, content.ErrUnknownField, content.ErrNotHierarchical,
	content.ErrTargetNotFound, content.ErrTargetType, content.ErrParentType, content.ErrParentTrashed,
	content.ErrCycle, content.ErrReservedAddress, content.ErrSlugTaken, content.ErrHoldsChildren,
	content.ErrInvalidTransition, content.ErrTrashed, content.ErrRevisionNotFound, sql.ErrNoRows,
}

// failable reports whether the fake can fail the entry: a statement, the begin, the commit, a savepoint or its release.
func failable(entry step) bool {
	return entry.name != rollbackEvent && entry.name != rollbackToEvent
}

// stoppedAt returns the entries a call logs when the one at the position fails, as pgx ends a transaction.
func stoppedAt(golden []step, at int) []step {
	stopped := slices.Clone(golden[:at+1])
	open, saving := false, false
	for _, entry := range golden[:at] {
		switch entry.name {
		case beginEvent:
			open = true
		case commitEvent, rollbackEvent:
			open = false
		case savepointEvent:
			saving = true
		case releaseEvent, rollbackToEvent:
			saving = false
		}
	}
	failed := golden[at].name
	if !open || failed == commitEvent {
		return stopped
	}
	if saving && failed != releaseEvent {
		stopped = append(stopped, rolledBackToSavepoint)
	}
	return append(stopped, rolledBack)
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

// checkSettled fails the test unless the call left no transaction and no savepoint open.
func checkSettled(t *testing.T, f *fakeBackend) {
	t.Helper()
	if f.open {
		t.Error("a transaction was left open")
	}
	if f.saving {
		t.Error("a savepoint was left open")
	}
}

// checkInjected fails the test unless the error carries the injected failure under the engine's name alone.
func checkInjected(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, errInjected) {
		t.Fatalf("error = %v, want the injected failure carried", err)
	}
	if !strings.HasPrefix(err.Error(), dbkit.SQLite.String()+": ") || strings.Contains(err.Error(), "postgres") {
		t.Errorf("error = %q, want it wrapped under the engine's name alone", err)
	}
}

// checkNoSentinel fails the test when the error reads as any sentinel a store answers with.
func checkNoSentinel(t *testing.T, err error) {
	t.Helper()
	for _, sentinel := range contentSentinels {
		if errors.Is(err, sentinel) {
			t.Errorf("errors.Is(%v, %v) = true, want no sentinel", err, sentinel)
		}
	}
}

// checkZero fails the test unless the method answered the zero value beside the failure.
func checkZero(t *testing.T, value any) {
	t.Helper()
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

			if err != nil {
				t.Fatalf("error = %v, want nil", err)
			}
			checkLog(t, f.log, sq.golden)
			checkSettled(t, f)
			if sq.kept != nil && !reflect.DeepEqual(value, sq.kept) {
				t.Errorf("value = %+v, want %+v", value, sq.kept)
			}
		})
	}
}

func TestEachMethodStopsWhereAnEntryFailsAndEndsItsTransactionAsPgxDoes(t *testing.T) {
	t.Parallel()

	for name, sq := range sequences {
		for at, entry := range sq.golden {
			if !failable(entry) {
				continue
			}
			t.Run(fmt.Sprintf("%s when entry %d %s fails", name, at, entry.name), func(t *testing.T) {
				t.Parallel()

				f, value, err := sq.runOn(t, at)

				checkInjected(t, err)
				checkNoSentinel(t, err)
				checkLog(t, f.log, stoppedAt(sq.golden, at))
				checkZero(t, value)
				checkSettled(t, f)
			})
		}
	}
}

func TestUpdateAnswersSlugTakenWhenTheDeferredAddressCheckFailsTheCommit(t *testing.T) {
	t.Parallel()

	sq := updateSequences["Update keeping its address and a snapshot"]
	sq.stocked = siteWith(func(stocked *shelf) { stocked.commits = addressTaken })

	f, value, err := sq.runOn(t, noFailure)

	if err != content.ErrSlugTaken {
		t.Errorf("error = %v, want %v", err, content.ErrSlugTaken)
	}
	checkLog(t, f.log, sq.golden)
	checkZero(t, value)
}
