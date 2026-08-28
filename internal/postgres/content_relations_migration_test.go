// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

// insertRelation stores one relation row pointing from one item at another.
func insertRelation(db *sql.DB, fromID uuid.UUID, fieldID int, toID uuid.UUID) error {
	_, err := db.Exec(
		`INSERT INTO core.content_relations (from_id, field_id, to_id, position, sort_at, visible)
		VALUES ($1, $2, $3, 1, $4, true)`,
		fromID, fieldID, toID, time.Now().UTC(),
	)
	return err
}

// relatableContent stores two posts and a relation field, returning both ids and the field id.
func relatableContent(t *testing.T, db *sql.DB) (uuid.UUID, uuid.UUID, int) {
	t.Helper()
	author := insertAuthor(t, db)
	if err := insertContent(db, author, "draft", "from-post", nil); err != nil {
		t.Fatalf("inserting the pointing item: %v, want nil", err)
	}
	if err := insertContent(db, author, "draft", "to-post", nil); err != nil {
		t.Fatalf("inserting the target: %v, want nil", err)
	}
	target := "post"
	if err := insertField(db, "post", "related", "relation", &target, true); err != nil {
		t.Fatalf("inserting the field: %v, want nil", err)
	}
	var fromID, toID uuid.UUID
	var fieldID int
	if err := db.QueryRow(`SELECT id FROM core.content WHERE slug = 'from-post'`).Scan(&fromID); err != nil {
		t.Fatalf("reading the pointing item: %v, want nil", err)
	}
	if err := db.QueryRow(`SELECT id FROM core.content WHERE slug = 'to-post'`).Scan(&toID); err != nil {
		t.Fatalf("reading the target: %v, want nil", err)
	}
	if err := db.QueryRow(
		`SELECT id FROM core.content_fields WHERE key = 'related' ORDER BY id LIMIT 1`,
	).Scan(&fieldID); err != nil {
		t.Fatalf("reading the field: %v, want nil", err)
	}
	return fromID, toID, fieldID
}

func TestRelationMigrationsHoldOneRowPerTarget(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	fromID, toID, fieldID := relatableContent(t, db)
	if err := insertRelation(db, fromID, fieldID, toID); err != nil {
		t.Fatalf("inserting the first relation: %v, want nil", err)
	}

	err := insertRelation(db, fromID, fieldID, toID)

	if code := pgErrorCode(err); code != uniqueViolation {
		t.Fatalf("the same target twice: %v with code %q, want %q", err, code, uniqueViolation)
	}
}

func TestRelationMigrationsRefuseAnUnstoredTarget(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	fromID, _, fieldID := relatableContent(t, db)

	err := insertRelation(db, fromID, fieldID, uuid.Must(uuid.NewV7()))

	if code := pgErrorCode(err); code != foreignKeyViolation {
		t.Fatalf("an unstored target: %v with code %q, want %q", err, code, foreignKeyViolation)
	}
}

func TestRelationMigrationsFollowADeletedTarget(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	fromID, toID, fieldID := relatableContent(t, db)
	if err := insertRelation(db, fromID, fieldID, toID); err != nil {
		t.Fatalf("inserting the relation: %v, want nil", err)
	}

	if _, err := db.Exec(`DELETE FROM core.content WHERE id = $1`, toID); err != nil {
		t.Fatalf("deleting the target: %v, want the cascade to carry its relations", err)
	}

	var held int
	if err := db.QueryRow(`SELECT count(*) FROM core.content_relations`).Scan(&held); err != nil {
		t.Fatalf("counting the relations: %v, want nil", err)
	}
	if held != 0 {
		t.Errorf("the deleted target keeps %d relations, want them gone with it", held)
	}
}

func TestRelationMigrationsFollowADeletedField(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	fromID, toID, fieldID := relatableContent(t, db)
	if err := insertRelation(db, fromID, fieldID, toID); err != nil {
		t.Fatalf("inserting the relation: %v, want nil", err)
	}

	if _, err := db.Exec(`DELETE FROM core.content_fields WHERE id = $1`, fieldID); err != nil {
		t.Fatalf("deleting the field: %v, want the cascade to carry its relations", err)
	}

	var held int
	if err := db.QueryRow(`SELECT count(*) FROM core.content_relations`).Scan(&held); err != nil {
		t.Fatalf("counting the relations: %v, want nil", err)
	}
	if held != 0 {
		t.Errorf("the deleted field keeps %d relations, want them gone with it", held)
	}
}

func TestRelationMigrationsServeTheTermPageFromAnIndex(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	fromID, toID, fieldID := relatableContent(t, db)
	if err := insertRelation(db, fromID, fieldID, toID); err != nil {
		t.Fatalf("inserting the relation: %v, want nil", err)
	}

	if _, err := db.Exec(`SET enable_seqscan = off`); err != nil {
		t.Fatalf("asking the planner for an index: %v, want nil", err)
	}
	rows, err := db.Query(`EXPLAIN (FORMAT TEXT) `+termPageQuery, toID, 20, 0)
	if err != nil {
		t.Fatalf("planning the term page scan: %v, want nil", err)
	}
	defer func() { _ = rows.Close() }()

	plan := ""
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatalf("reading the plan: %v, want nil", err)
		}
		plan += line + "\n"
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the plan: %v, want nil", err)
	}
	if !strings.Contains(plan, "content_relations_target_idx") {
		t.Errorf("the term page plan is %q, want the target index to find the pointers", plan)
	}
	if strings.Contains(plan, "Seq Scan on content c") {
		t.Errorf("the term page plan is %q, want the page limited before the rows are read", plan)
	}
}

// termPageQuery is the listing a term page runs, as internal/postgres/queries.sql holds it.
const termPageQuery = `SELECT c.id, c.type, c.status, c.slug, c.title, c.excerpt,
    c.author_id, c.published_at, c.created_at, c.updated_at, c.parent_id, c.path, c.fields
FROM (
    SELECT DISTINCT r.sort_at, r.from_id
    FROM core.content_relations r
    JOIN core.content pointing ON pointing.id = r.from_id
    JOIN core.content_types pointer ON pointer.key = pointing.type
    WHERE r.to_id = $1 AND r.visible AND pointer.active
    ORDER BY r.sort_at DESC, r.from_id
    LIMIT $2 OFFSET $3
) held
JOIN core.content c ON c.id = held.from_id
ORDER BY held.sort_at DESC, held.from_id`
