// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
)

// overflowCap is a revision cap too large for the row limit of a query.
const overflowCap = math.MaxInt32 + 1

// mustSnapshot returns a revision of the post credited to author.
func mustSnapshot(t *testing.T, p post.Post, author uuid.UUID) *post.Revision {
	t.Helper()
	revision, err := post.NewRevision(p, post.RevisionKindRevision, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	return &revision
}

// editTitle returns the post with a new title and a later timestamp.
func editTitle(p post.Post, title string) post.Post {
	edited := p
	edited.Title = title
	edited.UpdatedAt = p.UpdatedAt.Add(time.Second)
	return edited
}

func TestPostStoreUpdateStoresTheSnapshot(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "First Title", author)

	_, err := store.Update(t.Context(), editTitle(created, "Second Title"), mustSnapshot(t, created, author), 0)

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
	if revisions[0].Kind != post.RevisionKindRevision || revisions[0].AuthorID != author {
		t.Errorf("revision = %+v, want a revision credited to the editor", revisions[0])
	}
	if revisions[0].Content != "" {
		t.Errorf("revision content = %q, want listings to omit it", revisions[0].Content)
	}
}

func TestPostStoreUpdateWithoutASnapshotStoresNoRevision(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Only Title", author)

	if _, err := store.Update(t.Context(), editTitle(created, "Edited"), nil, 0); err != nil {
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

func TestPostStoreUpdatePrunesBeyondTheCap(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	current := mustCreate(t, store, "Title 0", author)

	for i := 1; i <= 5; i++ {
		edited := editTitle(current, "Title "+string(rune('0'+i)))
		updated, err := store.Update(t.Context(), edited, mustSnapshot(t, current, author), 2)
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

func TestPostStoreUpdatePruneSparesAutosaves(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
	current := mustCreate(t, store, "Title 0", author)
	if err := insertRevision(t, pool, current.ID, author, post.RevisionKindAutosave); err != nil {
		t.Fatalf("inserting the autosave: %v, want nil", err)
	}

	for i := 1; i <= 3; i++ {
		edited := editTitle(current, "Title "+string(rune('0'+i)))
		updated, err := store.Update(t.Context(), edited, mustSnapshot(t, current, author), 1)
		if err != nil {
			t.Fatalf("Update(%d) error = %v, want nil", i, err)
		}
		current = updated
	}

	revisions, err := store.Revisions(t.Context(), current.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	kinds := map[post.RevisionKind]int{}
	for _, revision := range revisions {
		kinds[revision.Kind]++
	}
	if kinds[post.RevisionKindAutosave] != 1 {
		t.Errorf("autosaves = %d, want pruning to spare the autosave", kinds[post.RevisionKindAutosave])
	}
	if kinds[post.RevisionKindRevision] != 1 {
		t.Errorf("revisions = %d, want the cap of 1 spent on revisions only", kinds[post.RevisionKindRevision])
	}
}

func TestPostStoreUpdateKeepsEveryRevisionWithoutACap(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	current := mustCreate(t, store, "Title 0", author)

	for i := 1; i <= 3; i++ {
		edited := editTitle(current, "Title "+string(rune('0'+i)))
		updated, err := store.Update(t.Context(), edited, mustSnapshot(t, current, author), 0)
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

func TestPostStoreRevisionByIDReturnsTheContent(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "With Body", author)
	withBody := created
	withBody.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	if _, err := store.Update(t.Context(), withBody, nil, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	stored, err := store.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if _, err := store.Update(t.Context(), editTitle(stored, "Edited"), mustSnapshot(t, stored, author), 0); err != nil {
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

func TestPostStoreRevisionsScopeToTheirPost(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	owner := mustCreate(t, store, "Owner Post", author)
	other := mustCreate(t, store, "Other Post", author)
	snapshot := mustSnapshot(t, owner, author)
	if _, err := store.Update(t.Context(), editTitle(owner, "Edited"), snapshot, 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}

	_, byIDErr := store.RevisionByID(t.Context(), other.ID, snapshot.ID)
	deleteErr := store.DeleteRevision(t.Context(), other.ID, snapshot.ID)

	if !errors.Is(byIDErr, post.ErrRevisionNotFound) {
		t.Errorf("RevisionByID() through the wrong post error = %v, want %v", byIDErr, post.ErrRevisionNotFound)
	}
	if !errors.Is(deleteErr, post.ErrRevisionNotFound) {
		t.Errorf("DeleteRevision() through the wrong post error = %v, want %v", deleteErr, post.ErrRevisionNotFound)
	}
	if _, err := store.RevisionByID(t.Context(), owner.ID, snapshot.ID); err != nil {
		t.Errorf("RevisionByID() through the owner error = %v, want the revision kept", err)
	}
}

func TestPostStoreRevisionByIDReportsMissingRevisions(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Revised", author)

	_, err := store.RevisionByID(t.Context(), created.ID, uuid.Must(uuid.NewV7()))

	if !errors.Is(err, post.ErrRevisionNotFound) {
		t.Errorf("RevisionByID() error = %v, want %v", err, post.ErrRevisionNotFound)
	}
}

func TestPostStoreDeleteRevision(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Revised", author)
	if _, err := store.Update(t.Context(), editTitle(created, "Edited"), mustSnapshot(t, created, author), 0); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
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

func TestPostStoreDeleteRevisionReportsMissingRevisions(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Revised", author)

	err := store.DeleteRevision(t.Context(), created.ID, uuid.Must(uuid.NewV7()))

	if !errors.Is(err, post.ErrRevisionNotFound) {
		t.Errorf("DeleteRevision() error = %v, want %v", err, post.ErrRevisionNotFound)
	}
}

func TestPostStoreRevisionsReportDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
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
	if _, err := store.Update(t.Context(), created, snapshot, 2); err == nil {
		t.Error("Update() with a snapshot on a closed pool error = nil, want a failure")
	}
}

func TestPostStoreUpdateReportsADuplicateSnapshot(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Revised", author)
	snapshot := mustSnapshot(t, created, author)
	if _, err := store.Update(t.Context(), editTitle(created, "Second"), snapshot, 0); err != nil {
		t.Fatalf("first Update() error = %v, want nil", err)
	}

	_, err := store.Update(t.Context(), editTitle(created, "Third"), snapshot, 0)

	if err == nil {
		t.Error("Update() reusing a revision id error = nil, want a failure")
	}
}

func TestPostStoreUpdateReportsAnUnusableCap(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Revised", author)

	_, err := store.Update(t.Context(), editTitle(created, "Second"), mustSnapshot(t, created, author), overflowCap)

	if err == nil {
		t.Error("Update() with a cap beyond the row limit error = nil, want a failure")
	}
}
