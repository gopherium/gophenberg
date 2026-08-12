// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
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
		(id, type, status, slug, title, content, excerpt, author_id, published_at, created_at, updated_at)
		VALUES ($1, 'car', 'draft', 'ford-focus', 'Ford Focus', '', '', $2, NULL, $3, $3)`,
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
