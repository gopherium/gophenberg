// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"

	"github.com/gopherium/gophenberg/internal/content"
)

// trashCases are the cases over emptying the trash of one type.
var trashCases = []Case{
	{"EmptiesTheTrashOfOneTypeAlone", emptiesTheTrashOfOneTypeAlone},
	{"EmptiesOnlyTheTrashTheAuthorWrote", emptiesOnlyTheTrashTheAuthorWrote},
	{"EmptiesAnEmptyTrash", emptiesAnEmptyTrash},
	{"EmptyTrashTakesTheRevisionsAlong", emptyTrashTakesTheRevisionsAlong},
}

// emptiesTheTrashOfOneTypeAlone deletes the trashed items of the named type and leaves every other item.
func emptiesTheTrashOfOneTypeAlone(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	RegisterPageType(t, s.Types)
	page := MustNest(t, s.Content, nil, "Old Page", author)
	Trash(t, s.Content, page)
	post := MustCreate(t, s.Content, "Old Post", author)
	Trash(t, s.Content, post)
	draft := MustNest(t, s.Content, nil, "Kept Page", author)

	deleted, kept, err := s.Content.EmptyTrash(t.Context(), "page", nil)

	if err != nil {
		t.Fatalf("EmptyTrash() error = %v, want nil", err)
	}
	if deleted != 1 || kept != 0 {
		t.Errorf("EmptyTrash() = %d deleted and %d kept, want 1 and 0", deleted, kept)
	}
	if StillStored(t, s.Content, page) || !StillStored(t, s.Content, post) || !StillStored(t, s.Content, draft) {
		t.Error("the store lost the wrong items, want only the trashed page gone")
	}
}

// emptiesOnlyTheTrashTheAuthorWrote deletes the author's trashed items and counts the other author's as kept.
func emptiesOnlyTheTrashTheAuthorWrote(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	other := s.AddAuthor(t, "Another Writer")
	mine := MustCreate(t, s.Content, "Mine", author)
	Trash(t, s.Content, mine)
	theirs := MustCreate(t, s.Content, "Theirs", other)
	Trash(t, s.Content, theirs)

	deleted, kept, err := s.Content.EmptyTrash(t.Context(), content.TypePost, &author)

	if err != nil {
		t.Fatalf("EmptyTrash() error = %v, want nil", err)
	}
	if deleted != 1 || kept != 1 {
		t.Errorf("EmptyTrash() = %d deleted and %d kept, want 1 and 1", deleted, kept)
	}
	if StillStored(t, s.Content, mine) || !StillStored(t, s.Content, theirs) {
		t.Error("the store lost the wrong items, want only the author's trashed post gone")
	}
}

// emptiesAnEmptyTrash deletes and keeps nothing when no item sits in the trash.
func emptiesAnEmptyTrash(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	MustCreate(t, s.Content, "Draft", author)

	deleted, kept, err := s.Content.EmptyTrash(t.Context(), content.TypePost, nil)

	if err != nil || deleted != 0 || kept != 0 {
		t.Errorf("EmptyTrash() = %d, %d, %v, want nothing deleted, nothing kept and no error", deleted, kept, err)
	}
}

// emptyTrashTakesTheRevisionsAlong deletes the revisions of each item it deletes.
func emptyTrashTakesTheRevisionsAlong(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "With History", author)
	edited := created
	edited.Title = "With History, Edited"
	revision, err := content.NewRevision(created, content.RevisionKindRevision, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	if _, err := s.Content.Update(t.Context(), edited, created.UpdatedAt, &revision, 10); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	Trash(t, s.Content, created)

	if _, _, err := s.Content.EmptyTrash(t.Context(), content.TypePost, nil); err != nil {
		t.Fatalf("EmptyTrash() error = %v, want nil", err)
	}

	if _, err := s.Content.RevisionByID(
		t.Context(), created.ID, revision.ID,
	); !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("RevisionByID() error = %v, want the revision gone with its item", err)
	}
}
