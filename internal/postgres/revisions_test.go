// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// overflowCap returns a revision cap too large for the row limit of a query.
func overflowCap(t *testing.T) int {
	t.Helper()
	if strconv.IntSize == 32 {
		t.Skip("skipping the oversized cap on 32-bit platforms")
	}
	oversized := int64(math.MaxInt32) + 1
	return int(oversized)
}

// mustSnapshot returns a revision of the post credited to author.
func mustSnapshot(t *testing.T, p content.Content, author uuid.UUID) *content.Revision {
	t.Helper()
	revision, err := content.NewRevision(p, content.RevisionKindRevision, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	return &revision
}

// editTitle returns the post with a new title and a later timestamp.
func editTitle(p content.Content, title string) content.Content {
	edited := p
	edited.Title = title
	edited.UpdatedAt = p.UpdatedAt.Add(time.Second)
	return edited
}

func TestContentStoreUpdateStoresTheSnapshot(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "First Title", author)

	_, err := store.Update(
		t.Context(), editTitle(created, "Second Title"), created.UpdatedAt, mustSnapshot(t, created, author), 0,
	)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	revisions, err := store.Revisions(t.Context(), created.ID)
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

func TestContentStoreUpdateWithoutASnapshotStoresNoRevision(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "Only Title", author)

	if _, err := store.Update(t.Context(), editTitle(created, "Edited"), created.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	revisions, err := store.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 0 {
		t.Errorf("revisions = %d, want none without a snapshot", len(revisions))
	}
}

func TestContentStoreUpdatePrunesBeyondTheCap(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	current := mustCreate(t, store, "Title 0", author)

	for i := 1; i <= 5; i++ {
		edited := editTitle(current, "Title "+string(rune('0'+i)))
		updated, err := store.Update(t.Context(), edited, current.UpdatedAt, mustSnapshot(t, current, author), 2)
		if err != nil {
			t.Fatalf("Update(%d) error = %v, want nil", i, err)
		}
		current = updated
	}

	revisions, err := store.Revisions(t.Context(), current.ID)
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

func TestContentStoreUpdateKeepsHistoryWhenSnapshotsConflict(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "Version One", author)
	_, err := store.Update(
		t.Context(), editTitle(created, "Writer A"), created.UpdatedAt, mustSnapshot(t, created, author), 0,
	)
	if err != nil {
		t.Fatalf("first Update() error = %v, want nil", err)
	}

	_, err = store.Update(
		t.Context(), editTitle(created, "Writer B"), created.UpdatedAt, mustSnapshot(t, created, author), 0,
	)

	if !errors.Is(err, content.ErrConflict) {
		t.Fatalf("Update() with a stale token error = %v, want %v", err, content.ErrConflict)
	}
	revisions, err := store.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Errorf("revisions = %d, want only the applied write snapshotted", len(revisions))
	}
}

func TestContentStoreUpdatePruneSparesAutosaves(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	current := mustCreate(t, store, "Title 0", author)
	if err := insertRevision(t, pool, current.ID, author, content.RevisionKindAutosave); err != nil {
		t.Fatalf("inserting the autosave: %v, want nil", err)
	}

	for i := 1; i <= 3; i++ {
		edited := editTitle(current, "Title "+string(rune('0'+i)))
		updated, err := store.Update(t.Context(), edited, current.UpdatedAt, mustSnapshot(t, current, author), 1)
		if err != nil {
			t.Fatalf("Update(%d) error = %v, want nil", i, err)
		}
		current = updated
	}

	revisions, err := store.Revisions(t.Context(), current.ID)
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

func TestContentStoreUpdateKeepsEveryRevisionWithoutACap(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	current := mustCreate(t, store, "Title 0", author)

	for i := 1; i <= 3; i++ {
		edited := editTitle(current, "Title "+string(rune('0'+i)))
		updated, err := store.Update(t.Context(), edited, current.UpdatedAt, mustSnapshot(t, current, author), 0)
		if err != nil {
			t.Fatalf("Update(%d) error = %v, want nil", i, err)
		}
		current = updated
	}

	revisions, err := store.Revisions(t.Context(), current.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 3 {
		t.Errorf("revisions = %d, want all three kept", len(revisions))
	}
}

func TestContentStoreRevisionByIDReturnsTheContent(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "With Body", author)
	withBody := created
	withBody.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	if _, err := store.Update(t.Context(), withBody, created.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	stored, err := store.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	_, err = store.Update(
		t.Context(), editTitle(stored, "Edited"), stored.UpdatedAt, mustSnapshot(t, stored, author), 0,
	)
	if err != nil {
		t.Fatalf("second Update() error = %v, want nil", err)
	}
	revisions, err := store.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}

	revision, err := store.RevisionByID(t.Context(), created.ID, revisions[0].ID)

	if err != nil {
		t.Fatalf("RevisionByID() error = %v, want nil", err)
	}
	if revision.Content != withBody.Content {
		t.Errorf("Content = %q, want the snapshotted body", revision.Content)
	}
}

func TestContentStoreRevisionsScopeToTheirPost(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	owner := mustCreate(t, store, "Owner Post", author)
	other := mustCreate(t, store, "Other Post", author)
	snapshot := mustSnapshot(t, owner, author)
	if _, err := store.Update(t.Context(), editTitle(owner, "Edited"), owner.UpdatedAt, snapshot, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	_, byIDErr := store.RevisionByID(t.Context(), other.ID, snapshot.ID)
	deleteErr := store.DeleteRevision(t.Context(), other.ID, snapshot.ID)

	if !errors.Is(byIDErr, content.ErrRevisionNotFound) {
		t.Errorf("RevisionByID() through the wrong post error = %v, want %v", byIDErr, content.ErrRevisionNotFound)
	}
	if !errors.Is(deleteErr, content.ErrRevisionNotFound) {
		t.Errorf("DeleteRevision() through the wrong post error = %v, want %v", deleteErr, content.ErrRevisionNotFound)
	}
	if _, err := store.RevisionByID(t.Context(), owner.ID, snapshot.ID); err != nil {
		t.Errorf("RevisionByID() through the owner error = %v, want the revision kept", err)
	}
}

func TestContentStoreRevisionByIDReportsMissingRevisions(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "Revised", author)

	_, err := store.RevisionByID(t.Context(), created.ID, uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("RevisionByID() error = %v, want %v", err, content.ErrRevisionNotFound)
	}
}

func TestContentStoreDeleteRevision(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "Revised", author)
	_, updateErr := store.Update(
		t.Context(), editTitle(created, "Edited"), created.UpdatedAt, mustSnapshot(t, created, author), 0,
	)
	if updateErr != nil {
		t.Fatalf("Update() error = %v, want nil", updateErr)
	}
	revisions, err := store.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}

	if err := store.DeleteRevision(t.Context(), created.ID, revisions[0].ID); err != nil {
		t.Fatalf("DeleteRevision() error = %v, want nil", err)
	}

	remaining, err := store.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(remaining) != 0 {
		t.Errorf("revisions = %d, want none after deletion", len(remaining))
	}
}

func TestContentStoreDeleteRevisionReportsMissingRevisions(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "Revised", author)

	err := store.DeleteRevision(t.Context(), created.ID, uuid.Must(uuid.NewV7()))

	if !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("DeleteRevision() error = %v, want %v", err, content.ErrRevisionNotFound)
	}
}

func TestContentStoreRevisionsReportDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Revised", author)
	snapshot := mustSnapshot(t, created, author)
	pool.Close()

	if _, err := store.Revisions(t.Context(), created.ID); err == nil {
		t.Error("Revisions() on a closed pool error = nil, want a failure")
	}
	if _, err := store.RevisionByID(t.Context(), created.ID, snapshot.ID); err == nil {
		t.Error("RevisionByID() on a closed pool error = nil, want a failure")
	}
	if err := store.DeleteRevision(t.Context(), created.ID, snapshot.ID); err == nil {
		t.Error("DeleteRevision() on a closed pool error = nil, want a failure")
	}
	if _, err := store.Update(t.Context(), created, created.UpdatedAt, snapshot, 2); err == nil {
		t.Error("Update() with a snapshot on a closed pool error = nil, want a failure")
	}
}

func TestContentStoreUpdateReportsADuplicateSnapshot(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "Revised", author)
	snapshot := mustSnapshot(t, created, author)
	second, err := store.Update(t.Context(), editTitle(created, "Second"), created.UpdatedAt, snapshot, 0)
	if err != nil {
		t.Fatalf("first Update() error = %v, want nil", err)
	}

	_, err = store.Update(t.Context(), editTitle(second, "Third"), second.UpdatedAt, snapshot, 0)

	if err == nil {
		t.Error("Update() reusing a revision id error = nil, want a failure")
	}
}

func TestContentStoreUpdateReportsAnUnusableCap(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "Revised", author)

	_, err := store.Update(
		t.Context(), editTitle(created, "Second"), created.UpdatedAt, mustSnapshot(t, created, author), overflowCap(t),
	)

	if err == nil {
		t.Error("Update() with a cap beyond the row limit error = nil, want a failure")
	}
}
