// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// autosaveCases are the cases over parking, reading and dropping each author's autosave.
var autosaveCases = []Case{
	{"SaveAutosaveStoresTheBuffer", saveAutosaveStoresTheBuffer},
	{"SaveAutosaveReplacesTheAuthorsAutosave", saveAutosaveReplacesTheAuthorsAutosave},
	{"SaveAutosaveKeepsOnePerAuthor", saveAutosaveKeepsOnePerAuthor},
	{"AutosaveReturnsTheAuthorsBuffer", autosaveReturnsTheAuthorsBuffer},
	{"AutosaveReportsMissingBuffers", autosaveReportsMissingBuffers},
	{"SaveAutosaveReportsAVanishedPost", saveAutosaveReportsAVanishedPost},
	{"DeleteAutosaveRemovesOnlyTheAuthorsBuffer", deleteAutosaveRemovesOnlyTheAuthorsBuffer},
	{"DeleteAutosaveToleratesAMissingBuffer", deleteAutosaveToleratesAMissingBuffer},
	{"TrashKeepsTheAutosaves", trashKeepsTheAutosaves},
	{"RestoreKeepsTheAutosaves", restoreKeepsTheAutosaves},
	{"DeleteTakesTheAutosavesAlong", deleteTakesTheAutosavesAlong},
	{"TrashStampsANewerAutosaveWithTheTrash", trashStampsANewerAutosaveWithTheTrash},
	{"RestoreStampsTheCarriedAutosaveWithTheRestore", restoreStampsTheCarriedAutosaveWithTheRestore},
	{"TrashAndRestoreLeaveAnOlderAutosaveAsItWas", trashAndRestoreLeaveAnOlderAutosaveAsItWas},
	{"ASecondTrashAndRestoreCarryTheAutosaveAgain", aSecondTrashAndRestoreCarryTheAutosaveAgain},
	{"SaveAutosaveRefusesAnItemInTheTrash", saveAutosaveRefusesAnItemInTheTrash},
}

// parkTitle parks a buffer holding the title as the author's autosave of the item.
func parkTitle(t *testing.T, s Stores, item content.Content, author uuid.UUID, title string) {
	t.Helper()
	buffer := item
	buffer.Title = title
	if _, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, buffer, author)); err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
}

// heldAutosave returns the author's autosave of the item, failing when the store holds none.
func heldAutosave(t *testing.T, s Stores, item content.Content, author uuid.UUID) content.Revision {
	t.Helper()
	kept, err := s.Content.Autosave(t.Context(), item.ID, author)
	if err != nil {
		t.Fatalf("Autosave() error = %v, want the parked words kept", err)
	}
	return kept
}

// saveAutosaveStoresTheBuffer parks the buffered title and body as an autosave credited to the author.
func saveAutosaveStoresTheBuffer(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Autosaved", author)
	buffer := created
	buffer.Title = "Buffered Title"
	buffer.Content = "<!-- wp:paragraph --><p>Buffered</p><!-- /wp:paragraph -->"

	saved, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, buffer, author))

	if err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
	if saved.Title != "Buffered Title" || saved.Content != buffer.Content {
		t.Errorf("saved = %+v, want the buffered content", saved)
	}
	if saved.Kind != content.RevisionKindAutosave || saved.AuthorID != author {
		t.Errorf("saved = %+v, want an autosave credited to the author", saved)
	}
}

// saveAutosaveReplacesTheAuthorsAutosave overwrites the author's parked buffer in place under its row id.
func saveAutosaveReplacesTheAuthorsAutosave(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Autosaved", author)
	first := created
	first.Title = "First Buffer"
	parked, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, first, author))
	if err != nil {
		t.Fatalf("first SaveAutosave() error = %v, want nil", err)
	}
	second := created
	second.Title = "Second Buffer"

	saved, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, second, author))

	if err != nil {
		t.Fatalf("second SaveAutosave() error = %v, want nil", err)
	}
	if saved.Title != "Second Buffer" {
		t.Errorf("Title = %q, want the newer buffer", saved.Title)
	}
	if saved.ID != parked.ID {
		t.Errorf("ID = %s, want the replaced buffer to keep row id %s", saved.ID, parked.ID)
	}
	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Errorf("revisions = %d, want one autosave per author", len(revisions))
	}
}

// saveAutosaveKeepsOnePerAuthor keeps a separate autosave for each author of one post.
func saveAutosaveKeepsOnePerAuthor(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	other := s.AddAuthor(t, "Another Writer")
	created := MustCreate(t, s.Content, "Shared", author)

	if _, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, created, author)); err != nil {
		t.Fatalf("SaveAutosave(author) error = %v, want nil", err)
	}
	if _, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, created, other)); err != nil {
		t.Fatalf("SaveAutosave(other) error = %v, want nil", err)
	}

	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 2 {
		t.Errorf("revisions = %d, want one autosave for each author", len(revisions))
	}
}

// autosaveReturnsTheAuthorsBuffer reads back the author's own buffer and none for another author.
func autosaveReturnsTheAuthorsBuffer(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	other := s.AddAuthor(t, "Another Writer")
	created := MustCreate(t, s.Content, "Shared", author)
	ownBuffer := created
	ownBuffer.Title = "Own Buffer"
	if _, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, ownBuffer, author)); err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}

	stored, err := s.Content.Autosave(t.Context(), created.ID, author)

	if err != nil {
		t.Fatalf("Autosave() error = %v, want nil", err)
	}
	if stored.Title != "Own Buffer" {
		t.Errorf("Title = %q, want the author's own buffer", stored.Title)
	}
	if _, err := s.Content.Autosave(t.Context(), created.ID, other); !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("Autosave(other) error = %v, want %v", err, content.ErrRevisionNotFound)
	}
}

// autosaveReportsMissingBuffers answers revision not found for an author who parked nothing.
func autosaveReportsMissingBuffers(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Unsaved", author)

	_, err := s.Content.Autosave(t.Context(), created.ID, author)

	if !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("Autosave() error = %v, want %v", err, content.ErrRevisionNotFound)
	}
}

// saveAutosaveReportsAVanishedPost answers not found for an autosave of a post deleted for good.
func saveAutosaveReportsAVanishedPost(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Vanishing", author)
	autosave := MustAutosave(t, created, author)
	if err := s.Content.Delete(t.Context(), created.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if _, err := s.Content.SaveAutosave(t.Context(), autosave); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("SaveAutosave() error = %v, want %v", err, content.ErrNotFound)
	}
}

// deleteAutosaveRemovesOnlyTheAuthorsBuffer drops the author's buffer and keeps the other author's.
func deleteAutosaveRemovesOnlyTheAuthorsBuffer(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	other := s.AddAuthor(t, "Another Writer")
	created := MustCreate(t, s.Content, "Shared", author)
	if _, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, created, author)); err != nil {
		t.Fatalf("SaveAutosave(author) error = %v, want nil", err)
	}
	if _, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, created, other)); err != nil {
		t.Fatalf("SaveAutosave(other) error = %v, want nil", err)
	}

	if err := s.Content.DeleteAutosave(t.Context(), created.ID, author); err != nil {
		t.Fatalf("DeleteAutosave() error = %v, want nil", err)
	}

	if _, err := s.Content.Autosave(t.Context(), created.ID, author); !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("Autosave(author) error = %v, want %v", err, content.ErrRevisionNotFound)
	}
	if _, err := s.Content.Autosave(t.Context(), created.ID, other); err != nil {
		t.Errorf("Autosave(other) error = %v, want the other author's buffer kept", err)
	}
}

// deleteAutosaveToleratesAMissingBuffer drops nothing without an error when the author parked nothing.
func deleteAutosaveToleratesAMissingBuffer(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Unsaved", author)

	if err := s.Content.DeleteAutosave(t.Context(), created.ID, author); err != nil {
		t.Errorf("DeleteAutosave() error = %v, want nil", err)
	}
}

// trashKeepsTheAutosaves keeps every author's autosave and the revisions while the item waits in the trash.
func trashKeepsTheAutosaves(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Parked", author)
	other := s.AddAuthor(t, "Another Writer")
	parked := map[uuid.UUID]string{author: "Parked by the author", other: "Parked by another writer"}
	for parker, title := range parked {
		parkTitle(t, s, created, parker, title)
	}
	snapshot, err := content.NewRevision(created, content.RevisionKindRevision, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	edited := created
	edited.Title = "Edited"
	edited.UpdatedAt = time.Now().UTC()
	if _, err := s.Content.Update(t.Context(), edited, created.UpdatedAt, &snapshot, 100); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	if _, err := s.Content.Trash(t.Context(), created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	for parker, title := range parked {
		if kept := heldAutosave(t, s, created, parker).Title; kept != title {
			t.Errorf("Autosave() title = %q, want %q kept through the trash", kept, title)
		}
	}
	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 3 {
		t.Errorf("revisions = %+v, want the snapshot and both autosaves kept", revisions)
	}
}

// restoreKeepsTheAutosaves brings the item back from the trash with the autosave parked before it left.
func restoreKeepsTheAutosaves(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Parked", author)
	parkTitle(t, s, created, author, "Parked Buffer")
	Trash(t, s.Content, created)

	if _, err := s.Content.Restore(t.Context(), created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}

	if kept := heldAutosave(t, s, created, author).Title; kept != "Parked Buffer" {
		t.Errorf("Autosave() title = %q, want the words parked before the trash", kept)
	}
}

// deleteTakesTheAutosavesAlong removes the autosaves of an item deleted for good from the trash.
func deleteTakesTheAutosavesAlong(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Parked", author)
	parkTitle(t, s, created, author, "Parked Buffer")
	Trash(t, s.Content, created)

	if err := s.Content.Delete(t.Context(), created.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if _, err := s.Content.Autosave(t.Context(), created.ID, author); !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("Autosave() error = %v, want the autosave gone with its item", err)
	}
}

// parkAt parks the item's words as the author's autosave saved at the moment.
func parkAt(t *testing.T, s Stores, item content.Content, author uuid.UUID, at time.Time) {
	t.Helper()
	autosave := MustAutosave(t, item, author)
	autosave.CreatedAt = at
	if _, err := s.Content.SaveAutosave(t.Context(), autosave); err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
}

// trashedAt moves the item to the trash at the moment and returns it as the store answers.
func trashedAt(t *testing.T, s Stores, item content.Content, at time.Time) content.Content {
	t.Helper()
	trashed, err := s.Content.Trash(t.Context(), item.ID, at)
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	return trashed
}

// restoredAt brings the item back from the trash at the moment and returns it as the store answers.
func restoredAt(t *testing.T, s Stores, item content.Content, at time.Time) content.Content {
	t.Helper()
	restored, err := s.Content.Restore(t.Context(), item.ID, at)
	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	return restored
}

// trashStampsANewerAutosaveWithTheTrash stamps an autosave saved since the item last changed with the trash.
func trashStampsANewerAutosaveWithTheTrash(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Parked", author)
	parkAt(t, s, created, author, created.UpdatedAt.Add(time.Minute))

	trashed := trashedAt(t, s, created, created.UpdatedAt.Add(2*time.Minute))

	if saved := heldAutosave(t, s, created, author).CreatedAt; !saved.Equal(trashed.UpdatedAt) {
		t.Errorf("Autosave() saved at %s, want the trash's %s", saved, trashed.UpdatedAt)
	}
}

// restoreStampsTheCarriedAutosaveWithTheRestore stamps the autosave the trash carried with the restore.
func restoreStampsTheCarriedAutosaveWithTheRestore(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Parked", author)
	parkAt(t, s, created, author, created.UpdatedAt.Add(time.Minute))
	trashedAt(t, s, created, created.UpdatedAt.Add(2*time.Minute))

	restored := restoredAt(t, s, created, created.UpdatedAt.Add(3*time.Minute))

	if saved := heldAutosave(t, s, created, author).CreatedAt; !saved.Equal(restored.UpdatedAt) {
		t.Errorf("Autosave() saved at %s, want the restore's %s", saved, restored.UpdatedAt)
	}
}

// trashAndRestoreLeaveAnOlderAutosaveAsItWas carries the newer autosave and keeps the stamp of an older one.
func trashAndRestoreLeaveAnOlderAutosaveAsItWas(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	other := s.AddAuthor(t, "Another Writer")
	created := MustCreate(t, s.Content, "Parked", author)
	parkAt(t, s, created, author, created.UpdatedAt.Add(time.Minute))
	parkAt(t, s, created, other, created.UpdatedAt.Add(-time.Hour))
	older := heldAutosave(t, s, created, other).CreatedAt

	trashedAt(t, s, created, created.UpdatedAt.Add(2*time.Minute))
	restored := restoredAt(t, s, created, created.UpdatedAt.Add(3*time.Minute))

	if saved := heldAutosave(t, s, created, other).CreatedAt; !saved.Equal(older) {
		t.Errorf("older Autosave() saved at %s, want %s kept", saved, older)
	}
	if saved := heldAutosave(t, s, created, author).CreatedAt; !saved.Equal(restored.UpdatedAt) {
		t.Errorf("newer Autosave() saved at %s, want the restore's %s", saved, restored.UpdatedAt)
	}
}

// aSecondTrashAndRestoreCarryTheAutosaveAgain carries an autosave stamped by the first restore through the next.
func aSecondTrashAndRestoreCarryTheAutosaveAgain(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Parked", author)
	parkAt(t, s, created, author, created.UpdatedAt.Add(time.Minute))
	trashedAt(t, s, created, created.UpdatedAt.Add(2*time.Minute))
	restoredAt(t, s, created, created.UpdatedAt.Add(3*time.Minute))

	trashedAt(t, s, created, created.UpdatedAt.Add(4*time.Minute))
	restored := restoredAt(t, s, created, created.UpdatedAt.Add(5*time.Minute))

	if saved := heldAutosave(t, s, created, author).CreatedAt; !saved.Equal(restored.UpdatedAt) {
		t.Errorf("Autosave() saved at %s, want the second restore's %s", saved, restored.UpdatedAt)
	}
}

// saveAutosaveRefusesAnItemInTheTrash refuses to park words on a trashed post and parks nothing.
func saveAutosaveRefusesAnItemInTheTrash(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Trashed", author)
	if _, err := s.Content.Trash(t.Context(), created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	_, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, created, author))

	if !errors.Is(err, content.ErrTrashed) {
		t.Errorf("SaveAutosave() error = %v, want %v", err, content.ErrTrashed)
	}
	if _, err := s.Content.Autosave(t.Context(), created.ID, author); !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("Autosave() error = %v, want nothing parked on a post in the trash", err)
	}
}
