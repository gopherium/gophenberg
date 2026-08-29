// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"testing"
	"time"
)

// ensureGroup returns the id of the group naming the type, storing one when none does.
func ensureGroup(db *sql.DB, typeKey string) (int64, error) {
	location := `[[{"source": "content_type", "operator": "==", "value": "` + typeKey + `"}]]`
	var id int64
	err := db.QueryRow(`SELECT id FROM core.field_groups WHERE location = $1`, location).Scan(&id)
	if err == nil {
		return id, nil
	}
	err = db.QueryRow(
		`INSERT INTO core.field_groups (title, location, created_at, updated_at)
		VALUES ($1, $2, now(), now()) RETURNING id`,
		typeKey+" fields", location,
	).Scan(&id)
	return id, err
}

// insertField stores a field definition row with the given shape under the type's group.
func insertField(db *sql.DB, typeKey, key, kind string, relatesTo *string, many bool) error {
	groupID, err := ensureGroup(db, typeKey)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = db.Exec(
		`INSERT INTO core.content_fields
		(group_id, key, label, kind, relates_to, many, required, created_at, updated_at)
		VALUES ($1, $2, 'A Field', $3, $4, $5, false, $6, $6)`,
		groupID, key, kind, relatesTo, many, now,
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

func TestFieldMigrationsLeaveKindValidityToTheRegistry(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	err := insertField(db, "post", "taste", "flavor", nil, false)

	if err != nil {
		t.Fatalf("an unlisted kind: %v, want the registry to be the only gate", err)
	}
}

func TestFieldMigrationsAcceptAManyMedia(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	err := insertField(db, "post", "gallery", "media", nil, true)

	if err != nil {
		t.Fatalf("many on a media: %v, want the widened constraint to accept it", err)
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

func TestFieldMigrationsKeepAGroupHoldingFields(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	if err := insertField(db, "car", "color", "text", nil, false); err != nil {
		t.Fatalf("inserting the field: %v, want nil", err)
	}
	groupID, err := ensureGroup(db, "car")
	if err != nil {
		t.Fatalf("finding the group: %v, want nil", err)
	}

	_, err = db.Exec(`DELETE FROM core.field_groups WHERE id = $1`, groupID)

	if code := pgErrorCode(err); code != restrictViolation {
		t.Fatalf("deleting a holding group: %v with code %q, want %q", err, code, restrictViolation)
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
