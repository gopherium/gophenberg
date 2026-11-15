// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// plantRevisionRow stores a revision row of a kind over an item, credited to an author and saved at a moment.
const plantRevisionRow = `INSERT INTO core_content_revisions
(id, content_id, kind, author_id, title, content, excerpt, fields, created_at)
VALUES (?1, ?2, ?3, ?4, 'Parked', '', '', '{}', ?5)`

// parkedAt stores a revision row of the kind over the item, credited to the author and saved at the moment.
func (s plantedSite) parkedAt(t *testing.T, item, author uuid.UUID, kind string, at time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	mustExec(t, s.handle, plantRevisionRow, id, item, kind, author, dbkit.Time{Time: at})
	return id
}

// savedAtOf returns when the revision row was saved.
func (s plantedSite) savedAtOf(t *testing.T, id uuid.UUID) time.Time {
	t.Helper()
	var at dbkit.Time
	if err := s.handle.QueryRowContext(t.Context(),
		"SELECT created_at FROM core_content_revisions WHERE id = ?", id).Scan(&at); err != nil {
		t.Fatalf("reading the revision row %s: %v", id, err)
	}
	return at.Time
}

// restored restores the item a second after it went to the trash and returns it as stored.
func (s plantedSite) restored(t *testing.T, trashed content.Content) content.Content {
	t.Helper()
	back, err := s.store.Restore(t.Context(), trashed.ID, trashed.UpdatedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("Restore(%q) error = %v, want nil", trashed.Title, err)
	}
	return back
}

// trashed sends the item to the trash a second after it was stored and returns it as stored.
func (s plantedSite) trashed(t *testing.T, item content.Content) content.Content {
	t.Helper()
	gone, err := s.store.Trash(t.Context(), item.ID, item.UpdatedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("Trash(%q) error = %v, want nil", item.Title, err)
	}
	return gone
}

func TestContentRestoreStripsOnlyTheSuffixTheTrashAdded(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name, title, slug string
		trash             func(t *testing.T, site plantedSite, item content.Content) content.Content
	}{
		{"a slug that never carried a suffix", "Notes", "notes",
			func(t *testing.T, site plantedSite, item content.Content) content.Content {
				mustExec(t, site.handle, "UPDATE core_content SET status = 'trash' WHERE id = ?", item.ID)
				item.Status = content.StatusTrash
				return item
			}},
		{"a slug holding the marker inside its own name", "Notes trashed abcdefgh", "notes-trashed-abcdefgh",
			func(t *testing.T, site plantedSite, item content.Content) content.Content {
				return site.trashed(t, item)
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			site := newPlantedSite(t)
			item := site.item(t, contenttest.PostType(), nil, tc.title, writtenAt)

			back := site.restored(t, tc.trash(t, site, item))

			if back.Status != content.StatusDraft || back.Slug != tc.slug || back.Path != tc.slug {
				t.Errorf("Restore() = %s %s at %s, want a draft %s at %s", back.Status, back.Slug, back.Path, tc.slug,
					tc.slug)
			}
		})
	}
}

// trashedUnderPage trashes a child of a nesting post, renames the parent to page and returns the child as stored.
func trashedUnderPage(t *testing.T, site plantedSite) content.Content {
	t.Helper()
	root := plantNestingPosts(t, site.handle)
	chapter := site.item(t, root, nil, "Chapter", writtenAt)
	gone := site.trashed(t, site.item(t, root, &chapter, "2", writtenAt))
	renamed, err := chapter.Rename(content.PageWord)
	if err != nil {
		t.Fatalf("Rename() error = %v, want nil", err)
	}
	renamed.UpdatedAt = writtenAt.Add(time.Second)
	site.edit(t, renamed, chapter.UpdatedAt)
	held, err := site.store.ByID(t.Context(), gone.ID)
	if err != nil {
		t.Fatalf("ByID() of the trashed child error = %v, want nil", err)
	}
	return held
}

func TestContentRestoreOntoAReservedAddressKeepsTheTrashedSlug(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	gone := trashedUnderPage(t, site)

	back := site.restored(t, gone)

	if back.Status != content.StatusDraft || back.Slug != gone.Slug || back.Path != "page/"+gone.Slug {
		t.Errorf("Restore() = %s %s at %s, want a draft keeping %s at page/%s", back.Status, back.Slug, back.Path,
			gone.Slug, gone.Slug)
	}
	if stored, err := site.store.ByID(t.Context(), gone.ID); err != nil || stored.Path != back.Path {
		t.Errorf("ByID() = %s, %v, want the trashed slug kept at %s", stored.Path, err, back.Path)
	}
}

func TestRestampNewerAutosavesStampsOnlyTheAutosavesSavedSinceTheItemLastChanged(t *testing.T) {
	t.Parallel()

	site := newPlantedSite(t)
	created := site.item(t, contenttest.PostType(), nil, "Restamped", writtenAt)
	other := site.item(t, contenttest.PostType(), nil, "Untouched", writtenAt)
	since := created.UpdatedAt
	stamp := since.Add(2 * time.Minute)
	rows := []struct {
		name string
		id   uuid.UUID
		want time.Time
	}{
		{"the newer autosave", site.parkedAt(t, created.ID, site.author, "autosave", since.Add(time.Minute)), stamp},
		{"the autosave saved with the item",
			site.parkedAt(t, created.ID, addAuthor(t, site.handle, "Equal Writer"), "autosave", since), stamp},
		{"the older autosave", site.parkedAt(t, created.ID, addAuthor(t, site.handle, "Older Writer"), "autosave",
			since.Add(-time.Hour)), since.Add(-time.Hour)},
		{"the newer revision", site.parkedAt(t, created.ID, site.author, "revision", since.Add(time.Minute)),
			since.Add(time.Minute)},
		{"the newer autosave of another item",
			site.parkedAt(t, other.ID, site.author, "autosave", other.UpdatedAt.Add(time.Minute)),
			other.UpdatedAt.Add(time.Minute)},
	}

	err := db.New(site.handle).RestampNewerAutosaves(t.Context(), db.RestampNewerAutosavesParams{
		SavedAt: dbkit.Time{Time: stamp}, ContentID: created.ID,
	})

	if err != nil {
		t.Fatalf("RestampNewerAutosaves() error = %v, want nil", err)
	}
	for _, row := range rows {
		if got := site.savedAtOf(t, row.id); !got.Equal(row.want) {
			t.Errorf("%s is saved at %s, want %s", row.name, got, row.want)
		}
	}
}
