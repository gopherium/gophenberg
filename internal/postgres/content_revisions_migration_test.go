// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
)

// insertRevision stores a revision row directly.
func insertRevision(
	t *testing.T, pool *pgxpool.Pool, contentID, author uuid.UUID, kind content.RevisionKind,
) error {
	t.Helper()
	_, err := pool.Exec(t.Context(),
		`INSERT INTO core.content_revisions (id, content_id, kind, author_id, title, content, excerpt, created_at)
		VALUES ($1, $2, $3, $4, 'Title', 'Content', 'Excerpt', $5)`,
		uuid.Must(uuid.NewV7()), contentID, string(kind), author, time.Now().UTC(),
	)
	return err
}

func TestRevisionMigrationsStoreBothKinds(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	stored := mustCreate(t, store, "Revised", author)

	for _, kind := range []content.RevisionKind{content.RevisionKindRevision, content.RevisionKindAutosave} {
		if err := insertRevision(t, pool, stored.ID, author, kind); err != nil {
			t.Errorf("inserting a %q revision: %v, want nil", kind, err)
		}
	}
}

func TestRevisionMigrationsRejectUnknownKinds(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	stored := mustCreate(t, store, "Revised", author)

	err := insertRevision(t, pool, stored.ID, author, "sketch")

	if code := pgErrorCode(err); code != checkViolation {
		t.Errorf("inserting an unknown kind: %v, want check violation %s", err, checkViolation)
	}
}

func TestRevisionMigrationsAllowOneAutosavePerAuthor(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	stored := mustCreate(t, store, "Revised", author)
	if err := insertRevision(t, pool, stored.ID, author, content.RevisionKindAutosave); err != nil {
		t.Fatalf("inserting the first autosave: %v, want nil", err)
	}

	autosaveErr := insertRevision(t, pool, stored.ID, author, content.RevisionKindAutosave)
	revisionErr := insertRevision(t, pool, stored.ID, author, content.RevisionKindRevision)
	secondRevisionErr := insertRevision(t, pool, stored.ID, author, content.RevisionKindRevision)

	if code := pgErrorCode(autosaveErr); code != uniqueViolation {
		t.Errorf("second autosave: %v, want unique violation %s", autosaveErr, uniqueViolation)
	}
	if revisionErr != nil || secondRevisionErr != nil {
		t.Errorf("revisions: %v and %v, want both stored", revisionErr, secondRevisionErr)
	}
}

func TestRevisionMigrationsIndexRevisionsForListingAndAuthorship(t *testing.T) {
	t.Parallel()

	db := newTestDB(t)

	names := []string{
		"content_revisions_content_id_created_at_idx",
		"content_revisions_author_id_idx",
		"content_revisions_one_autosave_per_author_idx",
	}
	for _, name := range names {
		var count int
		err := db.QueryRow(
			`SELECT count(*) FROM pg_indexes
			WHERE schemaname = 'core' AND tablename = 'content_revisions' AND indexname = $1`, name,
		).Scan(&count)
		if err != nil {
			t.Fatalf("querying pg_indexes: %v", err)
		}
		if count != 1 {
			t.Errorf("index %s count = %d, want 1", name, count)
		}
	}
}

func TestRevisionMigrationsRejectOrphans(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	stored := mustCreate(t, store, "Revised", author)

	orphanContent := insertRevision(t, pool, uuid.Must(uuid.NewV7()), author, content.RevisionKindRevision)
	orphanAuthor := insertRevision(t, pool, stored.ID, uuid.Must(uuid.NewV7()), content.RevisionKindRevision)

	if code := pgErrorCode(orphanContent); code != foreignKeyViolation {
		t.Errorf("revision without content: %v, want foreign key violation", orphanContent)
	}
	if code := pgErrorCode(orphanAuthor); code != foreignKeyViolation {
		t.Errorf("revision without an author: %v, want foreign key violation", orphanAuthor)
	}
}

func TestRevisionMigrationsCascadeWithTheContent(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	stored := mustCreate(t, store, "Doomed", author)
	if err := insertRevision(t, pool, stored.ID, author, content.RevisionKindRevision); err != nil {
		t.Fatalf("inserting a revision: %v, want nil", err)
	}

	if err := store.Delete(t.Context(), stored.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	var remaining int
	err := pool.QueryRow(t.Context(),
		"SELECT count(*) FROM core.content_revisions WHERE content_id = $1", stored.ID,
	).Scan(&remaining)
	if err != nil {
		t.Fatalf("counting revisions: %v", err)
	}
	if remaining != 0 {
		t.Errorf("revisions remaining = %d, want them removed with the content", remaining)
	}
}
