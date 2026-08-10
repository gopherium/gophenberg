// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
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
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	cfg := pgtestdb.Custom(t, testdb.Config(), testdb.Migrator())
	pool, err := pgxpool.New(t.Context(), cfg.URL())
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	t.Cleanup(pool.Close)
	author := uuid.Must(uuid.NewV7())
	_, err = pool.Exec(t.Context(),
		`INSERT INTO auth.users (id, email, name, password_hash, disabled, created_at)
		VALUES ($1, $2, 'Maria Perez', 'hash', false, $3)`,
		author, author.String()+"@example.com", time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("inserting author: %v", err)
	}
	return postgres.NewMediaStore(pool), author, pool
}

// mustImage returns an image item measured and carrying two renditions.
func mustImage(t *testing.T, file, title string, author uuid.UUID) media.Media {
	t.Helper()
	m, err := media.New(file, title, "image/jpeg", author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", file, err)
	}
	m.Width = 3000
	m.Height = 2000
	m.Filesize = 1_500_000
	m.Sizes = media.RenditionMap{
		"thumbnail": {File: title + "-150x150.jpg", Width: 150, Height: 150, MimeType: "image/jpeg", Filesize: 9_000},
		"large":     {File: title + "-1024x683.jpg", Width: 1024, Height: 683, MimeType: "image/jpeg", Filesize: 210_000},
	}
	return m
}

// mustCreateMedia stores the given media item.
func mustCreateMedia(t *testing.T, store *postgres.MediaStore, m media.Media) media.Media {
	t.Helper()
	created, err := store.Create(t.Context(), m)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", m.File, err)
	}
	return created
}

func TestMediaStoreCreateAndReadBack(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	m := mustImage(t, "2026/08/harbor.jpg", "harbor", author)

	created := mustCreateMedia(t, store, m)

	if created.ID <= 0 {
		t.Errorf("ID = %d, want a positive assigned identifier", created.ID)
	}

	got, err := store.ByID(t.Context(), created.ID)

	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if got.Type != media.TypeImage || got.File != m.File || got.Title != m.Title {
		t.Errorf("ByID() = %+v, want the created item", got)
	}
	if got.MimeType != m.MimeType || got.Width != m.Width || got.Height != m.Height || got.Filesize != m.Filesize {
		t.Errorf("ByID() measurements = %+v, want the created ones", got)
	}
	if !maps.Equal(got.Sizes, m.Sizes) {
		t.Errorf("Sizes = %v, want the stored renditions", got.Sizes)
	}
	if got.AuthorID != author {
		t.Errorf("AuthorID = %v, want the uploading author", got.AuthorID)
	}
	if got.CreatedAt.Location() != time.UTC || got.UpdatedAt.Location() != time.UTC {
		t.Errorf("timestamps carry location %v, want UTC", got.CreatedAt.Location())
	}
}

func TestMediaStoreAssignsRisingIdentifiers(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)

	first := mustCreateMedia(t, store, mustImage(t, "2026/08/first.jpg", "first", author))
	second := mustCreateMedia(t, store, mustImage(t, "2026/08/second.jpg", "second", author))

	if second.ID <= first.ID {
		t.Errorf("identifiers = %d then %d, want them rising", first.ID, second.ID)
	}
}

func TestMediaStoreByIDReportsAMissingItem(t *testing.T) {
	t.Parallel()

	store, _ := newMediaStore(t)

	if _, err := store.ByID(t.Context(), 12345); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("ByID() on a missing id error = %v, want ErrNotFound", err)
	}
}

func TestMediaStoreSurfacesACorruptRenditionMap(t *testing.T) {
	t.Parallel()

	store, author, pool := newMediaStoreWithPool(t)
	created := mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))
	if _, err := pool.Exec(t.Context(), `UPDATE core.media SET sizes = '[]' WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("corrupting the rendition map: %v", err)
	}

	if _, err := store.ByID(t.Context(), created.ID); err == nil {
		t.Error("ByID() over a corrupt rendition map error = nil, want a failure")
	}
}

func TestMediaStoreListsNewestFirst(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	mustCreateMedia(t, store, mustImage(t, "2026/08/older.jpg", "older", author))
	newest := mustCreateMedia(t, store, mustImage(t, "2026/08/newest.jpg", "newest", author))

	items, total, err := store.List(t.Context(), media.Filter{Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("List() = %d items with total %d, want both stored items", len(items), total)
	}
	if items[0].ID != newest.ID {
		t.Errorf("first listed = %q, want the newest upload", items[0].File)
	}
}

func TestMediaStoreListFiltersByType(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))
	manual, err := media.New("2026/08/manual.pdf", "manual", "application/pdf", author)
	if err != nil {
		t.Fatalf("New(manual) error = %v, want nil", err)
	}
	mustCreateMedia(t, store, manual)

	images, imageTotal, err := store.List(t.Context(), media.Filter{Type: media.TypeImage, Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List(images) error = %v, want nil", err)
	}
	if imageTotal != 1 || len(images) != 1 || images[0].Type != media.TypeImage {
		t.Errorf("List(images) = %d items with total %d, want only the image", len(images), imageTotal)
	}

	everything, allTotal, err := store.List(t.Context(), media.Filter{Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List(all) error = %v, want nil", err)
	}
	if allTotal != 2 || len(everything) != 2 {
		t.Errorf("List(all) = %d items with total %d, want every kind", len(everything), allTotal)
	}
}

func TestMediaStoreListSearchesTitleAndFile(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	mustCreateMedia(t, store, mustImage(t, "2026/08/dsc-0042.jpg", "Harbor at dawn", author))
	mustCreateMedia(t, store, mustImage(t, "2026/08/cliff.jpg", "Cliff", author))

	byTitle, _, err := store.List(t.Context(), media.Filter{Search: "harbor", Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List(harbor) error = %v, want nil", err)
	}
	if len(byTitle) != 1 || byTitle[0].Title != "Harbor at dawn" {
		t.Errorf("List(harbor) = %d items, want the title match alone", len(byTitle))
	}

	byFile, _, err := store.List(t.Context(), media.Filter{Search: "dsc-0042", Page: 1, PerPage: 10})
	if err != nil {
		t.Fatalf("List(dsc-0042) error = %v, want nil", err)
	}
	if len(byFile) != 1 || byFile[0].File != "2026/08/dsc-0042.jpg" {
		t.Errorf("List(dsc-0042) = %d items, want the file match alone", len(byFile))
	}
}

func TestMediaStoreListTreatsPatternCharactersAsText(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))

	items, total, err := store.List(t.Context(), media.Filter{Search: "h%r", Page: 1, PerPage: 10})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("List(h%%r) = %d items with total %d, want the wildcard read as text", len(items), total)
	}
}

func TestMediaStoreListPaginates(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	for _, file := range []string{"2026/08/one.jpg", "2026/08/two.jpg", "2026/08/three.jpg"} {
		mustCreateMedia(t, store, mustImage(t, file, file, author))
	}

	secondPage, total, err := store.List(t.Context(), media.Filter{Page: 2, PerPage: 2})

	if err != nil {
		t.Fatalf("List() error = %v, want nil", err)
	}
	if total != 3 {
		t.Errorf("total = %d, want every item counted across pages", total)
	}
	if len(secondPage) != 1 {
		t.Errorf("second page holds %d items, want the remainder", len(secondPage))
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

func TestMediaStoreUpdateEditsDescriptions(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	created := mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))
	edited := created
	edited.Title = "Harbor at dawn"
	edited.AltText = "Fishing boats moored at sunrise"
	edited.Caption = "The harbor before the market opens"
	edited.Description = "Taken from the eastern pier"
	edited.UpdatedAt = created.UpdatedAt.Add(time.Second)

	updated, err := store.Update(t.Context(), edited, created.UpdatedAt)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Title != edited.Title || updated.AltText != edited.AltText {
		t.Errorf("Update() = %+v, want the edited descriptions", updated)
	}
	if updated.Caption != edited.Caption || updated.Description != edited.Description {
		t.Errorf("Update() = %+v, want the edited descriptions", updated)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want it moved past %v", updated.UpdatedAt, created.UpdatedAt)
	}
	if updated.File != created.File || !maps.Equal(updated.Sizes, created.Sizes) {
		t.Errorf("Update() = %+v, want the stored file untouched", updated)
	}
}

func TestMediaStoreUpdateRefusesAStaleEdit(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	created := mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))
	edited := created
	edited.Title = "Harbor at dawn"
	edited.UpdatedAt = created.UpdatedAt.Add(time.Second)
	if _, err := store.Update(t.Context(), edited, created.UpdatedAt); err != nil {
		t.Fatalf("first Update() error = %v, want nil", err)
	}

	staleEdit := created
	staleEdit.Title = "Harbor at noon"
	staleEdit.UpdatedAt = created.UpdatedAt.Add(2 * time.Second)

	_, err := store.Update(t.Context(), staleEdit, created.UpdatedAt)

	if !errors.Is(err, media.ErrConflict) {
		t.Errorf("stale Update() error = %v, want ErrConflict", err)
	}

	got, err := store.ByID(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if got.Title != "Harbor at dawn" {
		t.Errorf("Title = %q, want the first edit untouched", got.Title)
	}
}

func TestMediaStoreUpdateReportsAMissingItem(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	ghost := mustImage(t, "2026/08/ghost.jpg", "ghost", author)
	ghost.ID = 12345

	if _, err := store.Update(t.Context(), ghost, ghost.UpdatedAt); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("Update() on a missing id error = %v, want ErrNotFound", err)
	}
}

func TestMediaStoreDeleteReturnsTheDeletedItem(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	created := mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))

	deleted, err := store.Delete(t.Context(), created.ID)

	if err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	if deleted.File != created.File || !maps.Equal(deleted.Sizes, created.Sizes) {
		t.Errorf("Delete() = %+v, want the files the caller must remove", deleted)
	}

	if _, err := store.ByID(t.Context(), created.ID); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("ByID() after delete error = %v, want ErrNotFound", err)
	}
}

func TestMediaStoreDeleteReportsAMissingItem(t *testing.T) {
	t.Parallel()

	store, author := newMediaStore(t)
	created := mustCreateMedia(t, store, mustImage(t, "2026/08/harbor.jpg", "harbor", author))
	if _, err := store.Delete(t.Context(), created.ID); err != nil {
		t.Fatalf("first Delete() error = %v, want nil", err)
	}

	if _, err := store.Delete(t.Context(), created.ID); !errors.Is(err, media.ErrNotFound) {
		t.Errorf("second Delete() error = %v, want ErrNotFound", err)
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
}
