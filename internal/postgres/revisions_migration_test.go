// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/post"
)

// insertRevision stores a revision row directly.
func insertRevision(
	t *testing.T, pool *pgxpool.Pool, postID, author uuid.UUID, kind post.RevisionKind,
) error {
	t.Helper()
	_, err := pool.Exec(t.Context(),
		`INSERT INTO core.post_revisions (id, post_id, kind, author_id, title, content, excerpt, created_at)
		VALUES ($1, $2, $3, $4, 'Title', 'Content', 'Excerpt', $5)`,
		uuid.Must(uuid.NewV7()), postID, string(kind), author, time.Now().UTC(),
	)
	return err
}

func TestRevisionMigrationsStoreBothKinds(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
	stored := mustCreate(t, store, "Revised", author)

	for _, kind := range []post.RevisionKind{post.RevisionKindRevision, post.RevisionKindAutosave} {
		if err := insertRevision(t, pool, stored.ID, author, kind); err != nil {
			t.Errorf("inserting a %q revision: %v, want nil", kind, err)
		}
	}
}

func TestRevisionMigrationsRejectUnknownKinds(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
	stored := mustCreate(t, store, "Revised", author)

	err := insertRevision(t, pool, stored.ID, author, "sketch")

	if code := pgErrorCode(err); code != checkViolation {
		t.Errorf("inserting an unknown kind: %v, want check violation %s", err, checkViolation)
	}
}

func TestRevisionMigrationsAllowOneAutosavePerAuthor(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
	stored := mustCreate(t, store, "Revised", author)
	if err := insertRevision(t, pool, stored.ID, author, post.RevisionKindAutosave); err != nil {
		t.Fatalf("inserting the first autosave: %v, want nil", err)
	}

	autosaveErr := insertRevision(t, pool, stored.ID, author, post.RevisionKindAutosave)
	revisionErr := insertRevision(t, pool, stored.ID, author, post.RevisionKindRevision)
	secondRevisionErr := insertRevision(t, pool, stored.ID, author, post.RevisionKindRevision)

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

	for _, name := range []string{"post_revisions_post_id_created_at_idx", "post_revisions_author_id_idx"} {
		var count int
		err := db.QueryRow(
			`SELECT count(*) FROM pg_indexes
			WHERE schemaname = 'core' AND tablename = 'post_revisions' AND indexname = $1`, name,
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

	store, author, pool := newPostStoreWithPool(t)
	stored := mustCreate(t, store, "Revised", author)

	orphanPost := insertRevision(t, pool, uuid.Must(uuid.NewV7()), author, post.RevisionKindRevision)
	orphanAuthor := insertRevision(t, pool, stored.ID, uuid.Must(uuid.NewV7()), post.RevisionKindRevision)

	if code := pgErrorCode(orphanPost); code != foreignKeyViolation {
		t.Errorf("revision without a post: %v, want foreign key violation", orphanPost)
	}
	if code := pgErrorCode(orphanAuthor); code != foreignKeyViolation {
		t.Errorf("revision without an author: %v, want foreign key violation", orphanAuthor)
	}
}

func TestRevisionMigrationsCascadeWithThePost(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
	stored := mustCreate(t, store, "Doomed", author)
	if err := insertRevision(t, pool, stored.ID, author, post.RevisionKindRevision); err != nil {
		t.Fatalf("inserting a revision: %v, want nil", err)
	}

	if err := store.Delete(t.Context(), stored.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	var remaining int
	err := pool.QueryRow(t.Context(),
		"SELECT count(*) FROM core.post_revisions WHERE post_id = $1", stored.ID,
	).Scan(&remaining)
	if err != nil {
		t.Fatalf("counting revisions: %v", err)
	}
	if remaining != 0 {
		t.Errorf("revisions remaining = %d, want them removed with the post", remaining)
	}
}
