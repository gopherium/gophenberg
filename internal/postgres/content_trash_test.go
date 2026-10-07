// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// stillStored is the shared fixture under the name these tests use.
var stillStored = contenttest.StillStored

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
