// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"io/fs"
	"testing"
	"time"

	"github.com/google/uuid"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/peterldowns/pgtestdb"
	"github.com/pressly/goose/v3"

	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

// insertNested stores a content row at the given path under an optional parent.
func insertNested(db *sql.DB, author uuid.UUID, slug, path string, parent *uuid.UUID) (uuid.UUID, error) {
	id := uuid.Must(uuid.NewV7())
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO core.content
		(id, type, status, slug, path, parent_id, title, content, excerpt, author_id,
		published_at, created_at, updated_at)
		VALUES ($1, 'post', 'draft', $2, $3, $4, 'Title', '', '', $5, NULL, $6, $6)`,
		id, slug, path, parent, author, now,
	)
	return id, err
}

// nestingVersion is the migration that gives content its parent and its address.
const nestingVersion = 8

func TestPathMigrationsBackfillExistingContent(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}

	cfg := pgtestdb.Custom(t, testdb.Config(), pgtestdb.NoopMigrator{})
	if err := authkitpg.Migrate(t.Context(), cfg.URL()); err != nil {
		t.Fatalf("auth Migrate() error = %v, want nil", err)
	}
	db, err := sql.Open("pgx", cfg.URL())
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer func() { _ = db.Close() }()
	provider := coreProvider(t, db)
	if _, err := provider.UpTo(t.Context(), nestingVersion-1); err != nil {
		t.Fatalf("migrating to the shape before nesting: %v", err)
	}
	author := insertAuthor(t, db)
	now := time.Now().UTC()
	if _, err := db.Exec(
		`INSERT INTO core.content
		(id, type, status, slug, title, content, excerpt, author_id, published_at, created_at, updated_at)
		VALUES ($1, 'post', 'draft', 'hello-world', 'Hello World', '', '', $2, NULL, $3, $3)`,
		uuid.Must(uuid.NewV7()), author, now,
	); err != nil {
		t.Fatalf("storing content before the migration: %v", err)
	}

	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatalf("migrating the stored content: %v, want the backfill to carry it", err)
	}

	var path string
	var parent *uuid.UUID
	err = db.QueryRow(`SELECT path, parent_id FROM core.content WHERE slug = 'hello-world'`).
		Scan(&path, &parent)
	if err != nil {
		t.Fatalf("reading the stored path: %v, want nil", err)
	}
	if path != "hello-world" || parent != nil {
		t.Errorf("path = %q under %v, want the slug at the root", path, parent)
	}
}

// coreProvider returns a goose provider over the core migrations.
func coreProvider(t *testing.T, db *sql.DB) *goose.Provider {
	t.Helper()
	source, err := fs.Sub(postgres.Migrations, "migrations")
	if err != nil {
		t.Fatalf("reading the migrations: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, source)
	if err != nil {
		t.Fatalf("building the migration provider: %v", err)
	}
	return provider
}

func TestPathMigrationsHoldOneItemPerAddress(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	if _, err := insertNested(db, author, "about", "about", nil); err != nil {
		t.Fatalf("inserting the first item: %v, want nil", err)
	}

	_, err := insertNested(db, author, "about", "about", nil)

	if code := pgErrorCode(err); code != uniqueViolation {
		t.Fatalf("reusing an address: %v with code %q, want %q", err, code, uniqueViolation)
	}
}

func TestPathMigrationsLetSiblingsOfDifferentParentsShareASlug(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	about, err := insertNested(db, author, "about", "about", nil)
	if err != nil {
		t.Fatalf("inserting about: %v, want nil", err)
	}
	careers, err := insertNested(db, author, "careers", "careers", nil)
	if err != nil {
		t.Fatalf("inserting careers: %v, want nil", err)
	}

	if _, err := insertNested(db, author, "team", "about/team", &about); err != nil {
		t.Fatalf("inserting the first team: %v, want nil", err)
	}
	if _, err := insertNested(db, author, "team", "careers/team", &careers); err != nil {
		t.Errorf("inserting the second team: %v, want the same slug allowed under another parent", err)
	}
}

func TestPathMigrationsKeepAParentThatHoldsChildren(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	about, err := insertNested(db, author, "about", "about", nil)
	if err != nil {
		t.Fatalf("inserting about: %v, want nil", err)
	}
	if _, err := insertNested(db, author, "team", "about/team", &about); err != nil {
		t.Fatalf("inserting the child: %v, want nil", err)
	}

	_, err = db.Exec(`DELETE FROM core.content WHERE id = $1`, about)

	if code := pgErrorCode(err); code != restrictViolation {
		t.Fatalf("deleting a parent: %v with code %q, want %q", err, code, restrictViolation)
	}
}

func TestPathMigrationsIndexAddressesForPrefixScans(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	var count int
	err := db.QueryRow(
		`SELECT count(*) FROM pg_indexes
		WHERE schemaname = 'core' AND tablename = 'content' AND indexname = 'content_path_prefix_idx'`,
	).Scan(&count)

	if err != nil {
		t.Fatalf("querying pg_indexes: %v", err)
	}
	if count != 1 {
		t.Errorf("the prefix index count = %d, want 1", count)
	}
}

func TestPathMigrationsDeferTheAddressCheckInsideATransaction(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	first, err := insertNested(db, author, "first", "first", nil)
	if err != nil {
		t.Fatalf("inserting the first item: %v, want nil", err)
	}
	second, err := insertNested(db, author, "second", "second", nil)
	if err != nil {
		t.Fatalf("inserting the second item: %v, want nil", err)
	}

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("beginning the swap: %v", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`SET CONSTRAINTS core.content_path_unique DEFERRED`); err != nil {
		t.Fatalf("deferring the address check: %v, want it deferrable", err)
	}
	if _, err := tx.Exec(`UPDATE core.content SET path = 'second' WHERE id = $1`, first); err != nil {
		t.Fatalf("moving the first item onto the second address: %v, want the check deferred", err)
	}
	if _, err := tx.Exec(`UPDATE core.content SET path = 'first' WHERE id = $1`, second); err != nil {
		t.Fatalf("moving the second item onto the first address: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Errorf("committing the swap: %v, want the addresses settled by the end", err)
	}
}

func TestPathMigrationsRefuseAMissingParent(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	missingParent := uuid.Must(uuid.NewV7())

	_, err := insertNested(db, author, "team", "about/team", &missingParent)

	if code := pgErrorCode(err); code != foreignKeyViolation {
		t.Fatalf("nesting under a missing parent: %v with code %q, want %q", err, code, foreignKeyViolation)
	}
}
