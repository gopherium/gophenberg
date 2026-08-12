// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/testdb"
)

const (
	uniqueViolation     = "23505"
	checkViolation      = "23514"
	foreignKeyViolation = "23503"
	restrictViolation   = "23001"
)

// newTestDB returns a database migrated with the auth and core schemas.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	return pgtestdb.New(t, testdb.Config(), testdb.Migrator())
}

// insertAuthor stores a user the content table can reference.
func insertAuthor(t *testing.T, db *sql.DB) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	_, err := db.Exec(
		`INSERT INTO auth.users (id, email, name, password_hash, disabled, created_at)
		VALUES ($1, $2, $3, $4, false, $5)`,
		id, id.String()+"@example.com", "Maria Perez", "hash", time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("inserting author: %v", err)
	}
	return id
}

// insertContent stores a content row with the given status, slug, and publication time.
func insertContent(db *sql.DB, author uuid.UUID, status, slug string, publishedAt *time.Time) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO core.content
		(id, type, status, slug, title, content, excerpt, author_id, published_at, created_at, updated_at)
		VALUES ($1, 'post', $2, $3, 'Title', '', '', $4, $5, $6, $6)`,
		uuid.Must(uuid.NewV7()), status, slug, author, publishedAt, now,
	)
	return err
}

// pgErrorCode returns the SQLSTATE code carried by err.
func pgErrorCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}

func TestMigrationsStoreDraftContent(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)

	if err := insertContent(db, author, "draft", "hello-world", nil); err != nil {
		t.Fatalf("inserting draft: %v, want nil", err)
	}
}

func TestMigrationsRejectAnUnknownStatus(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)

	err := insertContent(db, author, "publsh", "typo-status", nil)

	if code := pgErrorCode(err); code != checkViolation {
		t.Fatalf("inserting an unknown status: %v with code %q, want %q", err, code, checkViolation)
	}
}

func TestMigrationsRequireAPublicationDateWhenPublished(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)

	err := insertContent(db, author, "published", "published-without-date", nil)

	if code := pgErrorCode(err); code != checkViolation {
		t.Fatalf("publishing without a date: %v with code %q, want %q", err, code, checkViolation)
	}
}

func TestMigrationsKeepThePublicationDateOnUnpublishedContent(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	published := time.Now().UTC()

	if err := insertContent(db, author, "draft", "unpublished-keeps-date", &published); err != nil {
		t.Fatalf("unpublishing a dated post: %v, want nil", err)
	}
}

func TestMigrationsEnforceSlugUniquenessWithinAType(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	if err := insertContent(db, author, "draft", "shared-slug", nil); err != nil {
		t.Fatalf("inserting the first post: %v, want nil", err)
	}

	err := insertContent(db, author, "draft", "shared-slug", nil)

	if code := pgErrorCode(err); code != uniqueViolation {
		t.Fatalf("reusing a slug: %v with code %q, want %q", err, code, uniqueViolation)
	}
}

func TestMigrationsRejectContentWithoutAnAuthor(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	err := insertContent(db, uuid.Must(uuid.NewV7()), "draft", "orphan-content", nil)

	if code := pgErrorCode(err); code != foreignKeyViolation {
		t.Fatalf("inserting orphan content: %v with code %q, want %q", err, code, foreignKeyViolation)
	}
}

func TestMigrationsIndexContentForListingAndAuthorship(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	for _, name := range []string{"content_type_status_published_at_idx", "content_author_id_idx"} {
		var count int
		err := db.QueryRow(
			`SELECT count(*) FROM pg_indexes
			WHERE schemaname = 'core' AND tablename = 'content' AND indexname = $1`, name,
		).Scan(&count)
		if err != nil {
			t.Fatalf("querying pg_indexes: %v", err)
		}
		if count != 1 {
			t.Errorf("index %s count = %d, want 1", name, count)
		}
	}
}

func TestMigrationsRenameKeepsTheSlugConstraint(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	var count int
	err := db.QueryRow(
		`SELECT count(*) FROM pg_constraint
		WHERE conname = 'content_type_slug_unique' AND conrelid = 'core.content'::regclass`,
	).Scan(&count)
	if err != nil {
		t.Fatalf("querying pg_constraint: %v", err)
	}
	if count != 1 {
		t.Errorf("constraint content_type_slug_unique count = %d, want 1", count)
	}
}
