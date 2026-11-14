// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/media/mediatest"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// mustImage and mustCreateMedia are the shared fixtures under the names these tests use.
var (
	mustImage       = mediatest.MustImage
	mustCreateMedia = mediatest.MustCreate
)

// newMediaStore returns a store over a migrated database and the id of a stored author.
func newMediaStore(t *testing.T) (*postgres.MediaStore, uuid.UUID) {
	t.Helper()
	store, author, _ := newMediaStoreWithPool(t)
	return store, author
}

// newMediaStoreWithPool returns a store, the id of a stored author, and the pool behind them.
func newMediaStoreWithPool(t *testing.T) (*postgres.MediaStore, uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	pool := migratedPool(t)
	return postgres.NewMediaStore(pool), addAuthor(t, pool, mediatest.DefaultAuthor), pool
}

func TestMediaStoreSurfacesCorruptSizes(t *testing.T) {
	t.Parallel()

	store, author, pool := newMediaStoreWithPool(t)
	created := mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))
	if _, err := pool.Exec(t.Context(), `UPDATE core.media SET sizes = '[]' WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("corrupting the sizes: %v", err)
	}

	if _, err := store.ByID(t.Context(), created.ID); err == nil {
		t.Error("ByID() over corrupt sizes error = nil, want a failure")
	}
}

func TestMediaStoreListReportsAFailingListing(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))

	if _, _, err := store.List(t.Context(), media.Filter{Page: 1, PerPage: -1}); err == nil {
		t.Error("List() with a negative page size error = nil, want a failure")
	}
}

func TestMediaStoreReportsDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, author, pool := newMediaStoreWithPool(t)
	created := mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))
	pool.Close()

	if _, err := store.Create(t.Context(), mustImage(t, "2026/08/cliff.jpg", "cliff", author)); err == nil {
		t.Error("Create() on a closed pool error = nil, want a failure")
	}
	if _, err := store.ByID(t.Context(), created.ID); err == nil {
		t.Error("ByID() on a closed pool error = nil, want a failure")
	}
	if _, _, err := store.List(t.Context(), media.Filter{Page: 1, PerPage: 10}); err == nil {
		t.Error("List() on a closed pool error = nil, want a failure")
	}
	if _, err := store.Update(t.Context(), created, created.UpdatedAt); err == nil {
		t.Error("Update() on a closed pool error = nil, want a failure")
	}
	if _, err := store.Delete(t.Context(), created.ID); err == nil {
		t.Error("Delete() on a closed pool error = nil, want a failure")
	}
	if _, err := store.ByIDs(t.Context(), []int64{created.ID}); err == nil {
		t.Error("ByIDs() on a closed pool error = nil, want a failure")
	}
	if _, err := store.Saved(t.Context(), []string{created.File}); err == nil {
		t.Error("Saved() on a closed pool error = nil, want a failure")
	}
}

func TestMediaUpdateBehindAHeldItemAnswersDbkitsBusyClass(t *testing.T) {
	t.Parallel()

	store, author, pool := newMediaStoreWithPool(t)
	created := mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))
	holdInRival(t, pool.Config().ConnString(), "SELECT id FROM core.media FOR UPDATE")
	timed := postgres.NewMediaStore(lockTimedPool(t, pool.Config().ConnString()))
	edited := created
	edited.Title = "Edited"
	edited.UpdatedAt = time.Now().UTC()

	_, err := timed.Update(t.Context(), edited, created.UpdatedAt)

	if !errors.Is(err, dbkit.ErrBusy) || !strings.HasPrefix(fmt.Sprint(err), "postgres: update media: ") {
		t.Errorf("Update() behind a held item error = %v, want dbkit.ErrBusy under the update", err)
	}
}
