// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
)

// mustSnapshot and editTitle are the shared fixtures under the names these tests use.
var (
	mustSnapshot = contenttest.MustSnapshot
	editTitle    = contenttest.EditTitle
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

func TestRevisionDeleteQueuedBehindATrashIsRefused(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	created := mustCreate(t, store, "Contended", author)
	if _, err := store.Update(
		t.Context(), editTitle(created, "Edited"), created.UpdatedAt, mustSnapshot(t, created, author), 0,
	); err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	revisions, err := store.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	rival := rivalTransaction(t, pool)
	if _, err := rival.Exec(t.Context(),
		`SELECT id FROM core.content WHERE id = $1 FOR UPDATE`, created.ID); err != nil {
		t.Fatalf("locking the post: %v", err)
	}
	if _, err := rival.Exec(t.Context(),
		`UPDATE core.content SET status = 'trash' WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("trashing the post: %v", err)
	}
	removed := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		removed <- store.DeleteRevision(ctx, created.ID, revisions[0].ID)
	}()
	waitingOn(t, pool, "%FROM core.content p%FOR UPDATE%")

	if err := rival.Commit(t.Context()); err != nil {
		t.Fatalf("committing the trash: %v", err)
	}

	if err := <-removed; !errors.Is(err, content.ErrTrashed) {
		t.Errorf("DeleteRevision() error = %v, want the trash that landed first refusing it", err)
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
