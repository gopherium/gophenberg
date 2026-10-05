// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
	authkitpg "github.com/gopherium/gouncer/authkit/postgres"
	"github.com/peterldowns/pgtestdb"
	"github.com/pressly/goose/v3"

	"github.com/gopherium/gophenberg/internal/testdb"
)

// insertType stores a content type row with the given key, route word, and flags.
func insertType(db *sql.DB, key, routeWord string, isDefault bool) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO core.content_types
		(key, singular_label, plural_label, route_word, hierarchical, revisions,
		revision_cap, page_kind, is_default, active, created_at, updated_at)
		VALUES ($1, 'Thing', 'Things', $2, false, true, 100, 'single', $3, true, $4, $4)`,
		key, routeWord, isDefault, now,
	)
	return err
}

func TestTypeMigrationsRegisterTheBuiltInPostType(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	var routeWord string
	var isDefault, active bool
	err := db.QueryRow(
		`SELECT route_word, is_default, active FROM core.content_types WHERE key = 'post'`,
	).Scan(&routeWord, &isDefault, &active)

	if err != nil {
		t.Fatalf("reading the built-in type: %v, want it registered", err)
	}
	if routeWord != "" || !isDefault || !active {
		t.Errorf("post = route %q default %t active %t, want the active default at the root",
			routeWord, isDefault, active)
	}
}

func TestTypeMigrationsDescribeTheBuiltInPostType(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	var description string
	err := db.QueryRow(`SELECT description FROM core.content_types WHERE key = 'post'`).Scan(&description)

	if err != nil {
		t.Fatalf("reading the post description: %v, want nil", err)
	}
	if description != "Manage the posts on this site." {
		t.Errorf("post description = %q, want the shipped sentence", description)
	}
}

// describedVersion is the migration that gives every content type its description.
const describedVersion = 26

// undescribedDB returns a database migrated to the shape before descriptions and the provider that carries it on.
func undescribedDB(t *testing.T) (*sql.DB, *goose.Provider) {
	t.Helper()
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
	t.Cleanup(func() { _ = db.Close() })
	provider := coreProvider(t, db)
	if _, err := provider.UpTo(t.Context(), describedVersion-1); err != nil {
		t.Fatalf("migrating to the shape before descriptions: %v", err)
	}
	return db, provider
}

func TestTypeMigrationsLeaveOtherTypesUndescribed(t *testing.T) {
	t.Parallel()

	db, provider := undescribedDB(t)
	if err := insertType(db, "car", "cars", false); err != nil {
		t.Fatalf("storing a type before the migration: %v", err)
	}

	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatalf("migrating the stored type: %v, want nil", err)
	}

	var description string
	err := db.QueryRow(`SELECT description FROM core.content_types WHERE key = 'car'`).Scan(&description)
	if err != nil {
		t.Fatalf("reading the car description: %v, want nil", err)
	}
	if description != "" {
		t.Errorf("car description = %q, want it empty", description)
	}
}

func TestTypeMigrationsDescribeThePostWhateverItsLabels(t *testing.T) {
	t.Parallel()

	db, provider := undescribedDB(t)
	if _, err := db.Exec(
		`UPDATE core.content_types SET singular_label = 'Article', plural_label = 'Articles' WHERE key = 'post'`,
	); err != nil {
		t.Fatalf("renaming the post before the migration: %v", err)
	}

	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatalf("migrating the renamed post: %v, want nil", err)
	}

	var description string
	err := db.QueryRow(`SELECT description FROM core.content_types WHERE key = 'post'`).Scan(&description)
	if err != nil {
		t.Fatalf("reading the post description: %v, want nil", err)
	}
	if description != "Manage the posts on this site." {
		t.Errorf("post description = %q, want the shipped sentence whatever the labels", description)
	}
}

func TestTypeMigrationsHoldOneTypeAtTheRoot(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	err := insertType(db, "page", "", true)

	if code := pgErrorCode(err); code != uniqueViolation {
		t.Fatalf("a second default: %v with code %q, want %q", err, code, uniqueViolation)
	}
}

func TestTypeMigrationsKeepTheRootForTheDefault(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	err := insertType(db, "page", "", false)

	if code := pgErrorCode(err); code != checkViolation {
		t.Fatalf("the root without the default flag: %v with code %q, want %q", err, code, checkViolation)
	}
}

func TestTypeMigrationsLetTheDefaultCarryARouteWord(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	if _, err := db.Exec(
		`UPDATE core.content_types SET route_word = 'blog' WHERE key = 'post'`,
	); err != nil {
		t.Fatalf("moving the default off the root: %v, want the transfer's first act allowed", err)
	}

	err := insertType(db, "page", "", true)

	if code := pgErrorCode(err); code != uniqueViolation {
		t.Fatalf("a second default: %v with code %q, want %q", err, code, uniqueViolation)
	}
}

func TestTypeMigrationsEnforceOneTypePerRouteWord(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	if err := insertType(db, "page", "pages", false); err != nil {
		t.Fatalf("inserting the first type: %v, want nil", err)
	}

	err := insertType(db, "car", "pages", false)

	if code := pgErrorCode(err); code != uniqueViolation {
		t.Fatalf("reusing a route word: %v with code %q, want %q", err, code, uniqueViolation)
	}
}

func TestTypeMigrationsRejectAnUnknownPageKind(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	now := time.Now().UTC()

	_, err := db.Exec(
		`INSERT INTO core.content_types
		(key, singular_label, plural_label, route_word, hierarchical, revisions,
		revision_cap, page_kind, is_default, active, created_at, updated_at)
		VALUES ('car', 'Car', 'Cars', 'cars', false, true, 100, 'gallery', false, true, $1, $1)`,
		now,
	)

	if code := pgErrorCode(err); code != checkViolation {
		t.Fatalf("an unknown page kind: %v with code %q, want %q", err, code, checkViolation)
	}
}

func TestTypeMigrationsTieContentToARegisteredType(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	now := time.Now().UTC()

	_, err := db.Exec(
		`INSERT INTO core.content
		(id, type, status, slug, path, title, content, excerpt, author_id, published_at, created_at, updated_at)
		VALUES ($1, 'car', 'draft', 'ford-focus', 'cars/ford-focus', 'Ford Focus', '', '', $2, NULL, $3, $3)`,
		uuid.Must(uuid.NewV7()), author, now,
	)

	if code := pgErrorCode(err); code != foreignKeyViolation {
		t.Fatalf("content of an unregistered type: %v with code %q, want %q", err, code, foreignKeyViolation)
	}
}

func TestTypeMigrationsKeepATypeThatHoldsContent(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	if err := insertContent(db, author, "draft", "hello-world", nil); err != nil {
		t.Fatalf("inserting content: %v, want nil", err)
	}

	_, err := db.Exec(`DELETE FROM core.content_types WHERE key = 'post'`)

	if code := pgErrorCode(err); code != restrictViolation {
		t.Fatalf("deleting a type that holds content: %v with code %q, want %q", err, code, restrictViolation)
	}
}
