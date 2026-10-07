// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// revisionCases are the cases over storing, pruning, reading and deleting revisions.
var revisionCases = []Case{
	{"UpdateStoresTheSnapshot", updateStoresTheSnapshot},
	{"UpdateWithoutASnapshotStoresNoRevision", updateWithoutASnapshotStoresNoRevision},
	{"UpdatePrunesBeyondTheCap", updatePrunesBeyondTheCap},
	{"UpdateKeepsHistoryWhenSnapshotsConflict", updateKeepsHistoryWhenSnapshotsConflict},
	{"UpdatePruneSparesAutosaves", updatePruneSparesAutosaves},
	{"UpdateKeepsEveryRevisionWithoutACap", updateKeepsEveryRevisionWithoutACap},
	{"RevisionByIDReturnsTheContent", revisionByIDReturnsTheContent},
	{"RevisionsScopeToTheirPost", revisionsScopeToTheirPost},
	{"RevisionByIDReportsMissingRevisions", revisionByIDReportsMissingRevisions},
	{"DeleteRevision", deleteRevision},
	{"DeleteRevisionRefusesAnItemInTheTrash", deleteRevisionRefusesAnItemInTheTrash},
	{"DeleteRevisionReportsMissingRevisions", deleteRevisionReportsMissingRevisions},
	{"UpdateReportsADuplicateSnapshot", updateReportsADuplicateSnapshot},
}

// updateStoresTheSnapshot keeps the state before an edit as a revision credited to the editor.
func updateStoresTheSnapshot(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "First Title", author)

	_, err := s.Content.Update(
		t.Context(), EditTitle(created, "Second Title"), created.UpdatedAt, MustSnapshot(t, created, author), 0,
	)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("revisions = %d, want 1", len(revisions))
	}
	if revisions[0].Title != "First Title" {
		t.Errorf("revision title = %q, want the state before the edit", revisions[0].Title)
	}
	if revisions[0].Kind != content.RevisionKindRevision || revisions[0].AuthorID != author {
		t.Errorf("revision = %+v, want a revision credited to the editor", revisions[0])
	}
	if revisions[0].Content != "" {
		t.Errorf("revision content = %q, want listings to omit it", revisions[0].Content)
	}
}

// updateWithoutASnapshotStoresNoRevision stores no revision for an edit that carries no snapshot.
func updateWithoutASnapshotStoresNoRevision(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Only Title", author)

	if _, err := s.Content.Update(t.Context(), EditTitle(created, "Edited"), created.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 0 {
		t.Errorf("revisions = %d, want none without a snapshot", len(revisions))
	}
}

// updatePrunesBeyondTheCap keeps only the newest revisions up to the cap.
func updatePrunesBeyondTheCap(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	current := MustCreate(t, s.Content, "Title 0", author)

	for i := 1; i <= 5; i++ {
		edited := EditTitle(current, "Title "+string(rune('0'+i)))
		updated, err := s.Content.Update(t.Context(), edited, current.UpdatedAt, MustSnapshot(t, current, author), 2)
		if err != nil {
			t.Fatalf("Update(%d) error = %v, want nil", i, err)
		}
		current = updated
	}

	revisions, err := s.Content.Revisions(t.Context(), current.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revisions = %d, want the cap of 2", len(revisions))
	}
	if revisions[0].Title != "Title 4" || revisions[1].Title != "Title 3" {
		t.Errorf("kept %q and %q, want the two newest", revisions[0].Title, revisions[1].Title)
	}
}

// updateKeepsHistoryWhenSnapshotsConflict stores no snapshot for an edit refused as stale.
func updateKeepsHistoryWhenSnapshotsConflict(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Version One", author)
	_, err := s.Content.Update(
		t.Context(), EditTitle(created, "Writer A"), created.UpdatedAt, MustSnapshot(t, created, author), 0,
	)
	if err != nil {
		t.Fatalf("first Update() error = %v, want nil", err)
	}

	_, err = s.Content.Update(
		t.Context(), EditTitle(created, "Writer B"), created.UpdatedAt, MustSnapshot(t, created, author), 0,
	)

	if !errors.Is(err, content.ErrConflict) {
		t.Fatalf("Update() with a stale token error = %v, want %v", err, content.ErrConflict)
	}
	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Errorf("revisions = %d, want only the applied write snapshotted", len(revisions))
	}
}

// updatePruneSparesAutosaves spends the revision cap on revisions alone and keeps the autosave.
func updatePruneSparesAutosaves(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	current := MustCreate(t, s.Content, "Title 0", author)
	if _, err := s.Content.SaveAutosave(t.Context(), MustAutosave(t, current, author)); err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}

	for i := 1; i <= 3; i++ {
		edited := EditTitle(current, "Title "+string(rune('0'+i)))
		updated, err := s.Content.Update(t.Context(), edited, current.UpdatedAt, MustSnapshot(t, current, author), 1)
		if err != nil {
			t.Fatalf("Update(%d) error = %v, want nil", i, err)
		}
		current = updated
	}

	revisions, err := s.Content.Revisions(t.Context(), current.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	kinds := map[content.RevisionKind]int{}
	for _, revision := range revisions {
		kinds[revision.Kind]++
	}
	if kinds[content.RevisionKindAutosave] != 1 {
		t.Errorf("autosaves = %d, want pruning to spare the autosave", kinds[content.RevisionKindAutosave])
	}
	if kinds[content.RevisionKindRevision] != 1 {
		t.Errorf("revisions = %d, want the cap of 1 spent on revisions only", kinds[content.RevisionKindRevision])
	}
}

// updateKeepsEveryRevisionWithoutACap keeps every revision when the cap is zero.
func updateKeepsEveryRevisionWithoutACap(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	current := MustCreate(t, s.Content, "Title 0", author)

	for i := 1; i <= 3; i++ {
		edited := EditTitle(current, "Title "+string(rune('0'+i)))
		updated, err := s.Content.Update(t.Context(), edited, current.UpdatedAt, MustSnapshot(t, current, author), 0)
		if err != nil {
			t.Fatalf("Update(%d) error = %v, want nil", i, err)
		}
		current = updated
	}

	revisions, err := s.Content.Revisions(t.Context(), current.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 3 {
		t.Errorf("revisions = %d, want all three kept", len(revisions))
	}
}

// revisionByIDReturnsTheContent reads one revision back with the body it snapshotted.
func revisionByIDReturnsTheContent(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "With Body", author)
	withBody := created
	withBody.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	if _, err := s.Content.Update(t.Context(), withBody, created.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	stored, err := s.Content.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	_, err = s.Content.Update(
		t.Context(), EditTitle(stored, "Edited"), stored.UpdatedAt, MustSnapshot(t, stored, author), 0,
	)
	if err != nil {
		t.Fatalf("second Update() error = %v, want nil", err)
	}
	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("revisions = %d, want the one snapshot", len(revisions))
	}

	revision, err := s.Content.RevisionByID(t.Context(), created.ID, revisions[0].ID)

	if err != nil {
		t.Fatalf("RevisionByID() error = %v, want nil", err)
	}
	if revision.Content != withBody.Content {
		t.Errorf("Content = %q, want the snapshotted body", revision.Content)
	}
}

// revisionsScopeToTheirPost refuses to read or delete a revision through a post it does not belong to.
func revisionsScopeToTheirPost(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	owner := MustCreate(t, s.Content, "Owner Post", author)
	other := MustCreate(t, s.Content, "Other Post", author)
	snapshot := MustSnapshot(t, owner, author)
	if _, err := s.Content.Update(t.Context(), EditTitle(owner, "Edited"), owner.UpdatedAt, snapshot, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	_, byIDErr := s.Content.RevisionByID(t.Context(), other.ID, snapshot.ID)
	deleteErr := s.Content.DeleteRevision(t.Context(), other.ID, snapshot.ID)

	if !errors.Is(byIDErr, content.ErrRevisionNotFound) {
		t.Errorf("RevisionByID() through the wrong post error = %v, want %v", byIDErr, content.ErrRevisionNotFound)
	}
	if !errors.Is(deleteErr, content.ErrRevisionNotFound) {
		t.Errorf("DeleteRevision() through the wrong post error = %v, want %v", deleteErr, content.ErrRevisionNotFound)
	}
	if _, err := s.Content.RevisionByID(t.Context(), owner.ID, snapshot.ID); err != nil {
		t.Errorf("RevisionByID() through the owner error = %v, want the revision kept", err)
	}
}

// revisionByIDReportsMissingRevisions answers revision not found for an id nothing holds.
func revisionByIDReportsMissingRevisions(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Revised", author)

	_, err := s.Content.RevisionByID(t.Context(), created.ID, uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("RevisionByID() error = %v, want %v", err, content.ErrRevisionNotFound)
	}
}

// deleteRevision removes one revision for good.
func deleteRevision(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Revised", author)
	_, updateErr := s.Content.Update(
		t.Context(), EditTitle(created, "Edited"), created.UpdatedAt, MustSnapshot(t, created, author), 0,
	)
	if updateErr != nil {
		t.Fatalf("Update() error = %v, want nil", updateErr)
	}
	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("revisions = %d, want the one snapshot", len(revisions))
	}

	if err := s.Content.DeleteRevision(t.Context(), created.ID, revisions[0].ID); err != nil {
		t.Fatalf("DeleteRevision() error = %v, want nil", err)
	}

	remaining, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(remaining) != 0 {
		t.Errorf("revisions = %d, want none after deletion", len(remaining))
	}
}

// deleteRevisionRefusesAnItemInTheTrash refuses to delete a revision of a trashed post and keeps it.
func deleteRevisionRefusesAnItemInTheTrash(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Revised", author)
	if _, err := s.Content.Update(
		t.Context(), EditTitle(created, "Edited"), created.UpdatedAt, MustSnapshot(t, created, author), 0,
	); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	revisions, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Fatalf("revisions = %d, want the one snapshot", len(revisions))
	}
	if _, err := s.Content.Trash(t.Context(), created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	err = s.Content.DeleteRevision(t.Context(), created.ID, revisions[0].ID)

	if !errors.Is(err, content.ErrTrashed) {
		t.Errorf("DeleteRevision() error = %v, want %v", err, content.ErrTrashed)
	}
	remaining, err := s.Content.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(remaining) != 1 {
		t.Errorf("revisions = %d, want the revision of a post in the trash kept", len(remaining))
	}
}

// deleteRevisionReportsMissingRevisions answers revision not found for an id nothing holds.
func deleteRevisionReportsMissingRevisions(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Revised", author)

	err := s.Content.DeleteRevision(t.Context(), created.ID, uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("DeleteRevision() error = %v, want %v", err, content.ErrRevisionNotFound)
	}
}

// updateReportsADuplicateSnapshot refuses an edit whose snapshot reuses a stored revision id.
func updateReportsADuplicateSnapshot(t *testing.T, s Stores) {
	author := s.AddAuthor(t, DefaultAuthor)
	created := MustCreate(t, s.Content, "Revised", author)
	snapshot := MustSnapshot(t, created, author)
	second, err := s.Content.Update(t.Context(), EditTitle(created, "Second"), created.UpdatedAt, snapshot, 0)
	if err != nil {
		t.Fatalf("first Update() error = %v, want nil", err)
	}

	_, err = s.Content.Update(t.Context(), EditTitle(second, "Third"), second.UpdatedAt, snapshot, 0)

	if err == nil {
		t.Error("Update() reusing a revision id error = nil, want a failure")
	}
}
