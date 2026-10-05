// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// stillStored reports whether the store still holds the item.
func stillStored(t *testing.T, store *postgres.ContentStore, item content.Content) bool {
	t.Helper()
	_, err := store.ByID(t.Context(), item.ID)
	if err != nil && !errors.Is(err, content.ErrNotFound) {
		t.Fatalf("ByID(%q) error = %v, want nil or not found", item.Title, err)
	}
	return err == nil
}

func TestContentStoreEmptiesTheTrashOfOneTypeAlone(t *testing.T) {
	t.Parallel()

	store, author := newNestingStore(t)
	page := mustNest(t, store, nil, "Old Page", author)
	trash(t, store, page)
	post := mustCreate(t, store, "Old Post", author)
	trash(t, store, post)
	draft := mustNest(t, store, nil, "Kept Page", author)

	deleted, kept, err := store.EmptyTrash(t.Context(), "page", nil)

	if err != nil {
		t.Fatalf("EmptyTrash() error = %v, want nil", err)
	}
	if deleted != 1 || kept != 0 {
		t.Errorf("EmptyTrash() = %d deleted and %d kept, want 1 and 0", deleted, kept)
	}
	if stillStored(t, store, page) || !stillStored(t, store, post) || !stillStored(t, store, draft) {
		t.Error("the store lost the wrong items, want only the trashed page gone")
	}
}

func TestContentStoreEmptiesOnlyTheTrashTheAuthorWrote(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	other := writer(t, pool)
	mine := mustCreate(t, store, "Mine", author)
	trash(t, store, mine)
	theirs := mustCreate(t, store, "Theirs", other)
	trash(t, store, theirs)

	deleted, kept, err := store.EmptyTrash(t.Context(), content.TypePost, &author)

	if err != nil {
		t.Fatalf("EmptyTrash() error = %v, want nil", err)
	}
	if deleted != 1 || kept != 1 {
		t.Errorf("EmptyTrash() = %d deleted and %d kept, want 1 and 1", deleted, kept)
	}
	if stillStored(t, store, mine) || !stillStored(t, store, theirs) {
		t.Error("the store lost the wrong items, want only the author's trashed post gone")
	}
}

func TestContentStoreEmptyTrashCountsNothingKeptOnceAWaitedItemLeftTheTrash(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	restored := mustCreate(t, store, "Restored Meanwhile", author)
	trash(t, store, restored)
	emptied := mustCreate(t, store, "Emptied", author)
	trash(t, store, emptied)
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content SET status = 'draft' WHERE id = $1`, restored.ID); err != nil {
		t.Fatalf("restoring the item: %v", err)
	}
	type answer struct {
		deleted, kept int
		err           error
	}
	answered := make(chan answer, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		deleted, kept, err := store.EmptyTrash(ctx, content.TypePost, nil)
		answered <- answer{deleted, kept, err}
	}()
	waitingOn(t, pool, "%DELETE FROM core.content%")

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the restore: %v", err)
	}

	got := <-answered
	if got.err != nil || got.deleted != 1 || got.kept != 0 {
		t.Errorf("EmptyTrash() = %d deleted, %d kept, %v, want 1 deleted, 0 kept and no error",
			got.deleted, got.kept, got.err)
	}
	if !stillStored(t, store, restored) || stillStored(t, store, emptied) {
		t.Error("the store lost the wrong items, want the restored item kept and the other one gone")
	}
}

func TestContentStoreEmptiesAnEmptyTrash(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	mustCreate(t, store, "Draft", author)

	deleted, kept, err := store.EmptyTrash(t.Context(), content.TypePost, nil)

	if err != nil || deleted != 0 || kept != 0 {
		t.Errorf("EmptyTrash() = %d, %d, %v, want nothing deleted, nothing kept and no error", deleted, kept, err)
	}
}

func TestContentStoreEmptyTrashTakesTheRevisionsAlong(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	created := mustCreate(t, store, "With History", author)
	edited := created
	edited.Title = "With History, Edited"
	revision, err := content.NewRevision(created, content.RevisionKindRevision, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	if _, err := store.Update(t.Context(), edited, created.UpdatedAt, &revision, 10); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	trash(t, store, created)

	if _, _, err := store.EmptyTrash(t.Context(), content.TypePost, nil); err != nil {
		t.Fatalf("EmptyTrash() error = %v, want nil", err)
	}

	if _, err := store.RevisionByID(t.Context(), created.ID, revision.ID); !errors.Is(err, content.ErrRevisionNotFound) {
		t.Errorf("RevisionByID() error = %v, want the revision gone with its item", err)
	}
}

func TestContentStoreEmptyTrashReportsDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, _, pool := newContentStoreWithPool(t)
	pool.Close()

	_, _, err := store.EmptyTrash(t.Context(), content.TypePost, &uuid.Nil)

	if err == nil {
		t.Error("EmptyTrash() on a closed pool error = nil, want a failure")
	}
}

func TestContentStoreEmptyTrashReportsADeleteTheDatabaseRefuses(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	gone := mustCreate(t, store, "Trashed", author)
	trash(t, store, gone)
	sabotage(t, pool, "ALTER TABLE core.content RENAME COLUMN status TO standing")

	_, _, err := store.EmptyTrash(t.Context(), content.TypePost, nil)

	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("EmptyTrash() error = %v, want the refused delete reported as such", err)
	}
}
