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
	{"TrashLeavesNoParkedWords", trashLeavesNoParkedWords},
	{"SaveAutosaveRefusesAnItemInTheTrash", saveAutosaveRefusesAnItemInTheTrash},
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

// trashLeavesNoParkedWords sweeps every author's autosave into the trash and keeps the revisions.
func trashLeavesNoParkedWords(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Parked", author)
	other := s.AddAuthor(t, "Another Writer")
	parkers := []uuid.UUID{author, other}
	for _, parker := range parkers {
		if _, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, created, parker)); err != nil {
			t.Fatalf("SaveAutosave() error = %v, want nil", err)
		}
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

	for _, parker := range parkers {
		if _, err := s.Content.Autosave(t.Context(), created.ID, parker); !errors.Is(err, content.ErrRevisionNotFound) {
			t.Errorf("Autosave() error = %v, want the parked words gone with the trash", err)
		}
	}
	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 || revisions[0].Kind != content.RevisionKindRevision {
		t.Errorf("revisions = %+v, want only the snapshot kept", revisions)
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
