// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"
)

// insertMedia stores a media row with the given kind.
func insertMedia(db *sql.DB, author uuid.UUID, mediaType, file string) error {
	now := time.Now().UTC()
	_, err := db.Exec(
		`INSERT INTO core.media
		(media_type, file, title, alt_text, caption, description, mime_type,
		width, height, filesize, sizes, author_id, created_at, updated_at)
		VALUES ($1, $2, 'Title', '', '', '', 'image/jpeg', 0, 0, 0, '{}', $3, $4, $4)`,
		mediaType, file, author, now,
	)
	return err
}

func TestMigrationsStoreAnImage(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)

	if err := insertMedia(db, author, "image", "2026/08/harbor.jpg"); err != nil {
		t.Fatalf("inserting an image: %v, want nil", err)
	}
}

func TestMigrationsRejectAnUnknownMediaType(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)

	err := insertMedia(db, author, "imag", "2026/08/harbor.jpg")

	if code := pgErrorCode(err); code != checkViolation {
		t.Fatalf("inserting an unknown media type: %v with code %q, want %q", err, code, checkViolation)
	}
}

func TestMigrationsRejectMediaWithoutAnAuthor(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	err := insertMedia(db, uuid.Must(uuid.NewV7()), "image", "2026/08/harbor.jpg")

	if code := pgErrorCode(err); code != foreignKeyViolation {
		t.Fatalf("inserting orphan media: %v with code %q, want %q", err, code, foreignKeyViolation)
	}
}

func TestMigrationsAssignRisingMediaIdentifiers(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)
	author := insertAuthor(t, db)
	for _, file := range []string{"2026/08/first.jpg", "2026/08/second.jpg"} {
		if err := insertMedia(db, author, "image", file); err != nil {
			t.Fatalf("inserting %s: %v, want nil", file, err)
		}
	}

	var first, second int64
	err := db.QueryRow(
		`SELECT min(id), max(id) FROM core.media`,
	).Scan(&first, &second)

	if err != nil {
		t.Fatalf("reading identifiers: %v", err)
	}
	if first <= 0 || second <= first {
		t.Errorf("identifiers = %d and %d, want them positive and rising", first, second)
	}
}

func TestMigrationsIndexMediaForListingAndAuthorship(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	for _, name := range []string{"media_created_at_idx", "media_media_type_idx", "media_author_id_idx"} {
		var count int
		err := db.QueryRow(
			`SELECT count(*) FROM pg_indexes
			WHERE schemaname = 'core' AND tablename = 'media' AND indexname = $1`, name,
		).Scan(&count)
		if err != nil {
			t.Fatalf("querying pg_indexes: %v", err)
		}
		if count != 1 {
			t.Errorf("index %s count = %d, want 1", name, count)
		}
	}
}
