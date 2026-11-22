// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"
	"github.com/gopherium/framework/dbkit/sqlite/sqlitetest"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/sqlite"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// contentFault fails one statement and calls the store method that sends it, on the item its arrangement returns.
type contentFault struct {
	name, statement, prefix string
	arrange                 func(t *testing.T, site plantedSite) content.Content
	call                    func(t *testing.T, store *sqlite.ContentStore, item content.Content) error
}

// contentFaults are the failure cases, one for each statement the content store sends.
var contentFaults = []contentFault{
	{"CreateContent from Create", db.CreateContent, "sqlite: create content: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.Create(t.Context(), item))
		}},
	{"LockTypeRouteWord from Create", db.LockTypeRouteWord, "sqlite: create content: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.Create(t.Context(), item))
		}},
	{"GetContent from ByID", db.GetContent, "sqlite: get content: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.ByID(t.Context(), item.ID))
		}},
	{"GetPublishedContentByPath from PublishedByPath", db.GetPublishedContentByPath,
		"sqlite: get published content by path: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.PublishedByPath(t.Context(), item.Path))
		}},
	{"SiblingSlugTaken from Update", db.SiblingSlugTaken, "sqlite: update content: ", storedPost, resend},
	{"LockDeclaredFieldKeys from Update", db.LockDeclaredFieldKeys, "sqlite: update content: ", storedPost, resend},
	{"ListFieldGroups from Update", db.ListFieldGroups, "sqlite: update content: ", storedPost, resend},
	{"ValuesOfContent from Update", db.ValuesOfContent, "sqlite: update content: ", storedPost, resend},
	{"ListContentFields from Update", db.ListContentFields, "sqlite: update content: ", storedPost, resend},
	{"UpdateContent from Update", db.UpdateContent, "sqlite: update content: ", storedPost, resend},
	{"RefreshRelationVisibility from Update", db.RefreshRelationVisibility, "sqlite: update content: ", storedPost,
		resend},
	{"CreateRevision from Update", db.CreateRevision, "sqlite: update content: ", storedPost, resendWithARevision},
	{"PruneRevisions from Update", db.PruneRevisions, "sqlite: update content: ", storedPost, resendWithARevision},
	{"LockTypeNesting from Update", db.LockTypeNesting, "sqlite: update content: ", childPage, resend},
	{"LockTypeForMove from Update", db.LockTypeForMove, "sqlite: update content: ", contactUnderAbout, resend},
	{"LockParent from Update", db.LockParent, "sqlite: update content: ", contactUnderAbout, resend},
	{"DescendantsToMove from Update", db.DescendantsToMove, "sqlite: update content: ", renamedParent, resend},
	{"MoveDescendants from Update", db.MoveDescendants, "sqlite: update content: ", renamedParent, resend},
	{"TypesOfContent from Update", db.TypesOfContent, "sqlite: update content: ", filedPost, resend},
	{"ClearRelationsOfField from Update", db.ClearRelationsOfField, "sqlite: update content: ", filedPost, resend},
	{"AddRelation from Update", db.AddRelation, "sqlite: update content: ", filedPost, resend},
	{"CountChildren from Children", db.CountChildren, "sqlite: count children: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.Children(t.Context(), item.ID))
		}},
	{"ContentDepth from Depth", db.ContentDepth, "sqlite: content depth: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.Depth(t.Context(), item.ID))
		}},
	{"LockContent from Trash", db.LockContent, "sqlite: trash content: ", storedPost, trashLater},
	{"RestampNewerAutosaves from Trash", db.RestampNewerAutosaves, "sqlite: trash content: ", storedPost, trashLater},
	{"TrashContent from Trash", db.TrashContent, "sqlite: trash content: ", storedPost, trashLater},
	{"RestoreContent from Restore", db.RestoreContent, "sqlite: restore content: ", trashedPost, restoreLater},
	{"RestoreContentKeepingSlug from Restore", db.RestoreContentKeepingSlug, "sqlite: restore content: ",
		trashedUnderATakenSlug, restoreLater},
	{"DeleteContent from Delete", db.DeleteContent, "sqlite: delete content: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return store.Delete(t.Context(), item.ID)
		}},
	{"DeleteTrash from EmptyTrash", db.DeleteTrash, "sqlite: empty the trash: ", unstoredPost, emptyTrash},
	{"CountTrash from EmptyTrash", db.CountTrash, "sqlite: empty the trash: ", unstoredPost, emptyTrash},
	{"CountContentByAuthor from CountsByAuthor", db.CountContentByAuthor, "sqlite: count content by author: ",
		unstoredPost, func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.CountsByAuthor(t.Context(), item.Type))
		}},
	{"CountContent from List", db.CountContent, "sqlite: count content: ", unstoredPost, listFlat},
	{"ListContent from List", db.ListContent, "sqlite: list content: ", unstoredPost, listFlat},
	{"ListNestedContent from List", db.ListNestedContent, "sqlite: list content: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			_, _, err := store.List(t.Context(), content.Filter{Type: item.Type, Hierarchy: true, Page: 1, PerPage: 10})
			return err
		}},
	{"ListRevisions from Revisions", db.ListRevisions, "sqlite: list revisions: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.Revisions(t.Context(), item.ID))
		}},
	{"GetRevision from RevisionByID", db.GetRevision, "sqlite: get revision: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.RevisionByID(t.Context(), item.ID, uuid.Must(uuid.NewV7())))
		}},
	{"DeleteRevision from DeleteRevision", db.DeleteRevision, "sqlite: delete revision: ", storedPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return store.DeleteRevision(t.Context(), item.ID, uuid.Must(uuid.NewV7()))
		}},
	{"UpsertAutosave from SaveAutosave", db.UpsertAutosave, "sqlite: save autosave: ", storedPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.SaveAutosave(t.Context(), contenttest.MustAutosave(t, item, item.AuthorID)))
		}},
	{"GetAutosave from Autosave", db.GetAutosave, "sqlite: get autosave: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.Autosave(t.Context(), item.ID, item.AuthorID))
		}},
	{"DeleteAutosave from DeleteAutosave", db.DeleteAutosave, "sqlite: delete autosave: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return store.DeleteAutosave(t.Context(), item.ID, item.AuthorID)
		}},
	{"CountRelatedContent from RelatedTo", db.CountRelatedContent, "sqlite: count related content: ", unstoredPost,
		relatedTo},
	{"ListRelatedContent from RelatedTo", db.ListRelatedContent, "sqlite: list related content: ", unstoredPost,
		relatedTo},
	{"CountPointingAt from PointingAt", db.CountPointingAt, "sqlite: count pointing at: ", unstoredPost, pointingAt},
	{"PointingAt from PointingAt", db.PointingAt, "sqlite: list pointing at: ", unstoredPost, pointingAt},
	{"SummariesOfTargets from TargetsByIDs", db.SummariesOfTargets, "sqlite: list target summaries: ", unstoredPost,
		func(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
			return ignoreValue(store.TargetsByIDs(t.Context(), []uuid.UUID{item.ID}))
		}},
}

// newFaultSite returns a planted site holding the page type, whose connections answer with the faults it returns.
func newFaultSite(t *testing.T) (plantedSite, *sqlitetest.Faults) {
	t.Helper()
	handle, faults := faultyHandle(t)
	plantType(t, handle, contenttest.PageType())
	return plantedOn(t, handle), faults
}

// busyFailure returns the failure the driver hands back for a write behind another writer.
func busyFailure(t *testing.T) error {
	t.Helper()
	handle := impatientHandle(t)
	holdWrites(t, handle)
	_, err := handle.ExecContext(t.Context(), "UPDATE core_content_types SET active = active")
	if !errors.Is(dbsqlite.Classify(err), dbkit.ErrBusy) {
		t.Fatalf("a write behind another writer error = %v, want the driver's busy failure", err)
	}
	return err
}

// unstoredPost returns a draft post the file never held.
func unstoredPost(t *testing.T, site plantedSite) content.Content {
	t.Helper()
	return contenttest.MustPost(t, "Harbor Notes", site.author)
}

// storedPost stores a draft post and returns it as stored.
func storedPost(t *testing.T, site plantedSite) content.Content {
	t.Helper()
	return site.item(t, contenttest.PostType(), nil, "Harbor Notes", writtenAt)
}

// trashedPost stores a draft post, sends it to the trash and returns it as stored.
func trashedPost(t *testing.T, site plantedSite) content.Content {
	t.Helper()
	return site.trashed(t, storedPost(t, site))
}

// trashedUnderATakenSlug trashes a post, stores another under its slug and returns the trashed one.
func trashedUnderATakenSlug(t *testing.T, site plantedSite) content.Content {
	t.Helper()
	gone := trashedPost(t, site)
	storedPost(t, site)
	return gone
}

// childPage stores About with Team below it and returns Team as stored.
func childPage(t *testing.T, site plantedSite) content.Content {
	t.Helper()
	about := site.page(t, nil, "About", writtenAt)
	return site.page(t, &about, "Team", writtenAt)
}

// renamedParent stores About with Team below it and returns About renamed to about-us.
func renamedParent(t *testing.T, site plantedSite) content.Content {
	t.Helper()
	about := site.page(t, nil, "About", writtenAt)
	site.page(t, &about, "Team", writtenAt)
	renamed, err := about.Rename("about-us")
	if err != nil {
		t.Fatalf("Rename() error = %v, want nil", err)
	}
	return renamed
}

// contactUnderAbout stores About and Contact and returns Contact filed under About.
func contactUnderAbout(t *testing.T, site plantedSite) content.Content {
	t.Helper()
	about := site.page(t, nil, "About", writtenAt)
	contact := site.page(t, nil, "Contact", writtenAt)
	contact.ParentID = &about.ID
	return contact.Place(about.Path, contact.Slug)
}

// filedPost plants a categories field on posts and returns a stored post holding a stored category in it.
func filedPost(t *testing.T, site plantedSite) content.Content {
	t.Helper()
	plantType(t, site.handle, contenttest.CategoryType())
	plantField(t, site.handle, plantGroup(t, site.handle, content.TypePost), relationTo(content.TypePost, "categories"))
	news := site.item(t, contenttest.CategoryType(), nil, "News", writtenAt)
	post := storedPost(t, site)
	post.Fields = content.Values{"categories": contenttest.NamedTargets([]uuid.UUID{news.ID})}
	return post
}

// resend stores the item over the stamp it carries.
func resend(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
	return ignoreValue(store.Update(t.Context(), item, item.UpdatedAt, nil, 0))
}

// resendWithARevision stores the item over the stamp it carries beside a revision of it, keeping one revision.
func resendWithARevision(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
	snapshot := contenttest.MustSnapshot(t, item, item.AuthorID)
	return ignoreValue(store.Update(t.Context(), item, item.UpdatedAt, snapshot, 1))
}

// trashLater sends the item to the trash a second after its stamp.
func trashLater(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
	return ignoreValue(store.Trash(t.Context(), item.ID, item.UpdatedAt.Add(time.Second)))
}

// restoreLater restores the item a second after its stamp.
func restoreLater(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
	return ignoreValue(store.Restore(t.Context(), item.ID, item.UpdatedAt.Add(time.Second)))
}

// emptyTrash empties the trash of the item's type for every author.
func emptyTrash(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
	_, _, err := store.EmptyTrash(t.Context(), item.Type, nil)
	return err
}

// listFlat lists the first page of ten items of the item's type.
func listFlat(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
	_, _, err := store.List(t.Context(), content.Filter{Type: item.Type, Page: 1, PerPage: 10})
	return err
}

// relatedTo lists the first page of ten items pointing at the item.
func relatedTo(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
	_, _, err := store.RelatedTo(t.Context(), item.ID, 1, 10)
	return err
}

// pointingAt lists the first page of ten items pointing at the item through the first field.
func pointingAt(t *testing.T, store *sqlite.ContentStore, item content.Content) error {
	_, _, err := store.PointingAt(t.Context(), item.ID, 1, 1, 10)
	return err
}

func TestContentStoreReportsAFailedStatementUnderItsCallWithItsClass(t *testing.T) {
	t.Parallel()

	failure := busyFailure(t)
	for _, tc := range contentFaults {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			site, faults := newFaultSite(t)
			item := tc.arrange(t, site)
			faults.FailStatement(tc.statement, failure)

			err := tc.call(t, site.store, item)

			wantBusy(t, tc.name, err, tc.prefix)
			if !errors.Is(err, failure) {
				t.Errorf("%s error = %v, want the failure the statement answered", tc.name, err)
			}
		})
	}
}

func TestContentRestoreWhoseSavepointStatementFailsLeavesTheItemInTheTrash(t *testing.T) {
	t.Parallel()

	reserved := fmt.Sprint(content.ReservedAddress("page/2"))
	for _, tc := range []struct {
		name, statement, attempt string
		arrange                  func(t *testing.T, site plantedSite) content.Content
	}{
		{"the mark before the attempt", "SAVEPOINT restore_attempt", "", trashedUnderPage},
		{"the release after an attempt that passed", "RELEASE restore_attempt", "", trashedPost},
		{"the rollback after an attempt onto a reserved address", "ROLLBACK TO restore_attempt", reserved,
			trashedUnderPage},
		{"the release after that rollback", "RELEASE restore_attempt", reserved, trashedUnderPage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			site, faults := newFaultSite(t)
			gone := tc.arrange(t, site)
			faults.FailStatement(tc.statement, errPlanted)

			err := restoreLater(t, site.store, gone)

			wantPlanted(t, err, "sqlite: restore content: ")
			if errors.Is(err, content.ErrReservedAddress) {
				t.Errorf("Restore() error = %v, want the failed statement alone, never the reserved address", err)
			}
			if tc.attempt != "" && !strings.Contains(fmt.Sprint(err), tc.attempt) {
				t.Errorf("Restore() error = %v, want the attempt's own failure %q in its text", err, tc.attempt)
			}
			held, err := site.store.ByID(t.Context(), gone.ID)
			if err != nil || held.Status != content.StatusTrash || held.Slug != gone.Slug || held.Path != gone.Path {
				t.Errorf("after the failed restore ByID() = %s %s at %s, %v, want it left in the trash as %s at %s",
					held.Status, held.Slug, held.Path, err, gone.Slug, gone.Path)
			}
		})
	}
}

func TestContentWriteWhoseCommitFailsStoresNothing(t *testing.T) {
	t.Parallel()

	site, faults := newFaultSite(t)
	post := storedPost(t, site)
	faults.FailNextCommit(errPlanted)

	_, err := site.store.Update(t.Context(), contenttest.EditTitle(post, "Harbor Notes Revised"), post.UpdatedAt, nil, 0)

	wantPlanted(t, err, "sqlite: update content: ")
	held, err := site.store.ByID(t.Context(), post.ID)
	if err != nil || held.Title != post.Title || !held.UpdatedAt.Equal(post.UpdatedAt) {
		t.Errorf("after the failed commit ByID() = %q at %v, %v, want %q at %v as stored before", held.Title,
			held.UpdatedAt, err, post.Title, post.UpdatedAt)
	}
}
