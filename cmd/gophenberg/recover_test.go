// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	authkitpg "github.com/gopherium/gouncer/authkit/postgres"

	"github.com/gopherium/gophenberg/internal/media"
	"github.com/gopherium/gophenberg/internal/mediahost"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/seed"
)

// leftDocument is the small PDF the uploads a stop cut short carry.
const leftDocument = "%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%EOF\n"

// seededPool returns a pool over a database holding the demo data, and the id of the demo admin.
func seededPool(t *testing.T, databaseURL string) (*pgxpool.Pool, uuid.UUID) {
	t.Helper()
	env := map[string]string{"GOPHENBERG_DATABASE_URL": databaseURL}
	if err := seedDemoData(t.Context(), testGetenv(env), io.Discard); err != nil {
		t.Fatalf("seeding the demo data: %v", err)
	}
	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	t.Cleanup(pool.Close)
	admin, err := authkitpg.NewUserStore(pool).UserByEmail(t.Context(), seed.AdminEmail)
	if err != nil {
		t.Fatalf("reading the demo admin: %v", err)
	}
	return pool, admin.ID
}

// leftUpload stores the files of an upload that a stop cut short before it finished.
func leftUpload(t *testing.T, dir, name string, author uuid.UUID) media.Media {
	t.Helper()
	item, err := mediahost.New(mediahost.Config{Dir: dir}).Ingest(t.Context(), name, []byte(leftDocument), author)
	if err != nil {
		t.Fatalf("Ingest(%q) error = %v, want nil", name, err)
	}
	return item
}

// onDisk reports whether the media folder still holds the file.
func onDisk(t *testing.T, dir, file string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, filepath.FromSlash(file)))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("reading %q: %v", file, err)
	}
	return err == nil
}

// startAndStop runs the server over the media folder until it listens, stops it, and returns what it logged.
func startAndStop(t *testing.T, databaseURL, dir string) string {
	t.Helper()
	env := map[string]string{
		"GOPHENBERG_DATABASE_URL": databaseURL,
		"GOPHENBERG_ADDR":         "localhost:0",
		"GOPHENBERG_MEDIA_DIR":    dir,
	}
	logs := &lockedLog{}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	if err := run(ctx, testGetenv(env), io.MultiWriter(logs, cancelOnListen{cancel: cancel}), noPlugins); err != nil {
		t.Fatalf("run() error = %v, want a clean shutdown", err)
	}
	return logs.String()
}

func TestRunDeletesOnlyTheUploadsAStopLeftUnsaved(t *testing.T) {
	t.Parallel()

	databaseURL := emptyDatabaseURL(t)
	pool, admin := seededPool(t, databaseURL)
	dir := t.TempDir()
	unsaved := leftUpload(t, dir, "unsaved.pdf", admin)
	saved := leftUpload(t, dir, "saved.pdf", admin)
	if _, err := postgres.NewMediaStore(pool).Create(t.Context(), saved); err != nil {
		t.Fatalf("saving %q: %v", saved.File, err)
	}

	logged := startAndStop(t, databaseURL, dir)

	if onDisk(t, dir, unsaved.File) {
		t.Errorf("%s is still stored, want the file of an upload never saved deleted at start", unsaved.File)
	}
	if !onDisk(t, dir, saved.File) {
		t.Errorf("%s is gone, want the file of a saved item kept", saved.File)
	}
	if !strings.Contains(logged, `msg="unsaved upload deleted" file=`+unsaved.File) {
		t.Errorf("log = %q, want the deleted file named", logged)
	}
}

func TestRunStartsWhenItCannotDeleteAnUnsavedUpload(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("running as root, which deletes from a folder whatever its mode says")
	}
	databaseURL := emptyDatabaseURL(t)
	_, admin := seededPool(t, databaseURL)
	dir := t.TempDir()
	unsaved := leftUpload(t, dir, "unsaved.pdf", admin)
	month := filepath.Dir(filepath.Join(dir, filepath.FromSlash(unsaved.File)))
	if err := os.Chmod(month, 0o555); err != nil {
		t.Fatalf("locking %q: %v", month, err)
	}
	t.Cleanup(func() { _ = os.Chmod(month, 0o755) })

	logged := startAndStop(t, databaseURL, dir)

	if !strings.Contains(logged, `msg="unsaved uploads kept for the next start"`) {
		t.Errorf("log = %q, want the file it could not delete reported", logged)
	}
	if !onDisk(t, dir, unsaved.File) {
		t.Errorf("%s is gone, want a file the server could not delete still there", unsaved.File)
	}
}
