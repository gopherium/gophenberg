// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"testing"
	"time"
)

// insertField stores a field definition row with the given shape.
func insertField(db *sql.DB, typeKey, key, kind string, relatesTo *string, many bool) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO core.content_fields
		(type_key, key, label, kind, relates_to, many, required, created_at, updated_at)
		VALUES ($1, $2, 'A Field', $3, $4, $5, false, $6, $6)`,
		typeKey, key, kind, relatesTo, many, now,
	)
	return err
}

func TestFieldMigrationsHoldOneKeyPerType(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	if err := insertField(db, "post", "color", "text", nil, false); err != nil {
		t.Fatalf("inserting the first field: %v, want nil", err)
	}

	err := insertField(db, "post", "color", "number", nil, false)

	if code := pgErrorCode(err); code != uniqueViolation {
		t.Fatalf("a taken field key: %v with code %q, want %q", err, code, uniqueViolation)
	}
}

func TestFieldMigrationsRejectAnUnknownKind(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	err := insertField(db, "post", "taste", "flavor", nil, false)

	if code := pgErrorCode(err); code != checkViolation {
		t.Fatalf("an unknown kind: %v with code %q, want %q", err, code, checkViolation)
	}
}

func TestFieldMigrationsTieARelationToATarget(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	err := insertField(db, "post", "engine", "relation", nil, false)

	if code := pgErrorCode(err); code != checkViolation {
		t.Fatalf("a relation without a target: %v with code %q, want %q", err, code, checkViolation)
	}
}

func TestFieldMigrationsKeepTargetsOffScalars(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	target := "post"

	err := insertField(db, "post", "color", "text", &target, false)

	if code := pgErrorCode(err); code != checkViolation {
		t.Fatalf("a scalar with a target: %v with code %q, want %q", err, code, checkViolation)
	}
}

func TestFieldMigrationsKeepManyForRelations(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	err := insertField(db, "post", "color", "text", nil, true)

	if code := pgErrorCode(err); code != checkViolation {
		t.Fatalf("many on a scalar: %v with code %q, want %q", err, code, checkViolation)
	}
}

func TestFieldMigrationsFollowTheirType(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	if err := insertType(db, "car", "cars", false); err != nil {
		t.Fatalf("inserting the type: %v, want nil", err)
	}
	if err := insertField(db, "car", "color", "text", nil, false); err != nil {
		t.Fatalf("inserting the field: %v, want nil", err)
	}

	if _, err := db.Exec(`DELETE FROM core.content_types WHERE key = 'car'`); err != nil {
		t.Fatalf("deleting the empty type: %v, want the cascade to carry its fields", err)
	}

	var remaining int
	if err := db.QueryRow(
		`SELECT count(*) FROM core.content_fields WHERE type_key = 'car'`,
	).Scan(&remaining); err != nil {
		t.Fatalf("counting the fields: %v, want nil", err)
	}
	if remaining != 0 {
		t.Errorf("the deleted type keeps %d fields, want them gone with it", remaining)
	}
}

func TestFieldMigrationsGiveContentAFieldsColumn(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	if err := insertContent(db, author, "draft", "hello-world", nil); err != nil {
		t.Fatalf("inserting content: %v, want nil", err)
	}

	var stored string
	err := db.QueryRow(`SELECT fields FROM core.content WHERE slug = 'hello-world'`).Scan(&stored)

	if err != nil {
		t.Fatalf("reading the fields column: %v, want it present", err)
	}
	if stored != "{}" {
		t.Errorf("a fresh row holds %q, want the empty object", stored)
	}
}

func TestFieldMigrationsGiveRevisionsAFieldsColumn(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	var stored string
	err := db.QueryRow(
		`SELECT column_default FROM information_schema.columns
		WHERE table_schema = 'core' AND table_name = 'content_revisions' AND column_name = 'fields'`,
	).Scan(&stored)

	if err != nil {
		t.Fatalf("reading the revisions fields column: %v, want it present", err)
	}
	if stored != "'{}'::jsonb" {
		t.Errorf("the revisions fields column defaults to %q, want the empty object", stored)
	}
}
