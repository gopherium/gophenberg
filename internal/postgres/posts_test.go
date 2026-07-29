// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/peterldowns/pgtestdb"

	"github.com/gopherium/gophenberg/internal/post"
	"github.com/gopherium/gophenberg/internal/postgres"
	"github.com/gopherium/gophenberg/internal/testdb"
)

var trashedSlug = regexp.MustCompile(`^hello-world-trashed-[a-z0-9]{8}$`)

// newPostStore returns a store over a migrated database and the id of a stored author.
func newPostStore(t *testing.T) (*postgres.PostStore, uuid.UUID) {
	t.Helper()
	store, author, _ := newPostStoreWithPool(t)
	return store, author
}

// newPostStoreWithPool returns a store, the id of a stored author, and the pool behind them.
func newPostStoreWithPool(t *testing.T) (*postgres.PostStore, uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping database test in short mode")
	}
	cfg := pgtestdb.Custom(t, testdb.Config(), testdb.Migrator())
	pool, err := pgxpool.New(t.Context(), cfg.URL())
	if err != nil {
		t.Fatalf("connecting pool: %v", err)
	}
	t.Cleanup(pool.Close)
	author := uuid.Must(uuid.NewV7())
	_, err = pool.Exec(t.Context(),
		`INSERT INTO auth.users (id, email, name, password_hash, disabled, created_at)
		VALUES ($1, $2, 'Maria Perez', 'hash', false, $3)`,
		author, author.String()+"@example.com", time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("inserting author: %v", err)
	}
	return postgres.NewPostStore(pool), author, pool
}

// mustPost returns a draft post with the given title.
func mustPost(t *testing.T, title string, author uuid.UUID) post.Post {
	t.Helper()
	p, err := post.New(post.TypePost, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	return p
}

// mustCreate stores a draft post with the given title.
func mustCreate(t *testing.T, store *postgres.PostStore, title string, author uuid.UUID) post.Post {
	t.Helper()
	created, err := store.Create(t.Context(), mustPost(t, title, author))
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return created
}

func TestPostStoreCreateAndReadBack(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	p := mustPost(t, "Hello World", author)

	created, err := store.Create(t.Context(), p)

	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}
	if created.Slug != "hello-world" {
		t.Errorf("Slug = %q, want %q", created.Slug, "hello-world")
	}
	if created.Status != post.StatusDraft {
		t.Errorf("Status = %q, want %q", created.Status, post.StatusDraft)
	}

	got, err := store.ByID(t.Context(), p.ID)

	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	if got.ID != p.ID || got.Title != "Hello World" || got.AuthorID != author {
		t.Errorf("ByID() = %+v, want the created post", got)
	}
	if got.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want nil on a draft", got.PublishedAt)
	}
	if got.CreatedAt.Location() != time.UTC || got.UpdatedAt.Location() != time.UTC {
		t.Errorf("timestamps carry location %v, want UTC", got.CreatedAt.Location())
	}
}

func TestPostStoreCreateSuffixesTakenSlugs(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	mustCreate(t, store, "Hello World", author)

	second := mustCreate(t, store, "Hello World", author)
	third := mustCreate(t, store, "Hello World", author)

	if second.Slug != "hello-world-2" {
		t.Errorf("second Slug = %q, want %q", second.Slug, "hello-world-2")
	}
	if third.Slug != "hello-world-3" {
		t.Errorf("third Slug = %q, want %q", third.Slug, "hello-world-3")
	}
}

func TestPostStoreCreateSuffixesUnderConcurrency(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	posts := []post.Post{mustPost(t, "Race", author), mustPost(t, "Race", author)}
	slugs := make([]string, len(posts))
	errs := make([]error, len(posts))

	var wg sync.WaitGroup
	for i, p := range posts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			created, err := store.Create(t.Context(), p)
			slugs[i], errs[i] = created.Slug, err
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Create(%d) error = %v, want nil", i, err)
		}
	}
	if slugs[0] == slugs[1] {
		t.Errorf("both posts stored slug %q, want distinct slugs", slugs[0])
	}
}

func TestPostStoreCreateRejectsAnUnknownAuthor(t *testing.T) {
	t.Parallel()

	store, _ := newPostStore(t)
	orphan := mustPost(t, "Orphan", uuid.Must(uuid.NewV7()))

	_, err := store.Create(t.Context(), orphan)

	if err == nil {
		t.Fatal("Create() error = nil, want a foreign key failure")
	}
}

func TestPostStoreByIDReportsMissingPosts(t *testing.T) {
	t.Parallel()

	store, _ := newPostStore(t)

	_, err := store.ByID(t.Context(), uuid.Must(uuid.NewV7()))

	if !errors.Is(err, post.ErrNotFound) {
		t.Errorf("ByID() error = %v, want %v", err, post.ErrNotFound)
	}
}

func TestPostStoreUpdateChangesEditableFields(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Draft Title", author)
	edited := created
	edited.Title = "Published Title"
	edited.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	edited.Excerpt = "Summary"
	published := time.Now().UTC().Truncate(time.Microsecond)
	edited.Status = post.StatusPublished
	edited.PublishedAt = &published
	edited.UpdatedAt = published

	updated, err := store.Update(t.Context(), edited, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Title != "Published Title" || updated.Excerpt != "Summary" {
		t.Errorf("Update() = %+v, want the edited title and excerpt", updated)
	}
	if updated.Status != post.StatusPublished {
		t.Errorf("Status = %q, want %q", updated.Status, post.StatusPublished)
	}
	if updated.PublishedAt == nil || !updated.PublishedAt.Equal(published) {
		t.Errorf("PublishedAt = %v, want %v", updated.PublishedAt, published)
	}
}

func TestPostStoreUpdateSuffixesTakenSlugs(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	mustCreate(t, store, "Taken Slug", author)
	other := mustCreate(t, store, "Other Slug", author)
	edited := other
	edited.Slug = "taken-slug"

	updated, err := store.Update(t.Context(), edited, nil, 0)

	if err != nil {
		t.Fatalf("Update() error = %v, want nil", err)
	}
	if updated.Slug != "taken-slug-2" {
		t.Errorf("Slug = %q, want %q", updated.Slug, "taken-slug-2")
	}
}

func TestPostStoreUpdateReportsMissingPosts(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	missing := mustPost(t, "Missing", author)

	_, err := store.Update(t.Context(), missing, nil, 0)

	if !errors.Is(err, post.ErrNotFound) {
		t.Errorf("Update() error = %v, want %v", err, post.ErrNotFound)
	}
}

func TestPostStoreUpdateWithASnapshotReportsMissingPosts(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	missing := mustPost(t, "Missing", author)

	_, err := store.Update(t.Context(), missing, mustSnapshot(t, missing, author), 0)

	if !errors.Is(err, post.ErrNotFound) {
		t.Errorf("Update() error = %v, want %v", err, post.ErrNotFound)
	}
	revisions, revErr := store.Revisions(t.Context(), missing.ID)
	if revErr != nil {
		t.Fatalf("Revisions() error = %v, want nil", revErr)
	}
	if len(revisions) != 0 {
		t.Errorf("revisions = %d, want none stored for the missing post", len(revisions))
	}
}

func TestPostStoreUpdateWrapsDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
	created := mustCreate(t, store, "Wrapped", author)
	pool.Close()

	_, err := store.Update(t.Context(), created, nil, 0)

	if err == nil {
		t.Fatal("Update() on a closed pool error = nil, want a failure")
	}
	if !strings.Contains(err.Error(), "postgres: update post") {
		t.Errorf("Update() error = %q, want the update post prefix", err)
	}
}

func TestPostStoreTrashRenamesTheSlug(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Hello World", author)

	trashed, err := store.Trash(t.Context(), created.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if trashed.Status != post.StatusTrash {
		t.Errorf("Status = %q, want %q", trashed.Status, post.StatusTrash)
	}
	if !trashedSlug.MatchString(trashed.Slug) {
		t.Errorf("Slug = %q, want it to match %v", trashed.Slug, trashedSlug)
	}
}

func TestPostStoreTrashReportsMissingPosts(t *testing.T) {
	t.Parallel()

	store, _ := newPostStore(t)

	_, err := store.Trash(t.Context(), uuid.Must(uuid.NewV7()), time.Now().UTC())

	if !errors.Is(err, post.ErrNotFound) {
		t.Errorf("Trash() error = %v, want %v", err, post.ErrNotFound)
	}
}

func TestPostStoreRestoreRecoversTheOriginalSlug(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Hello World", author)
	if _, err := store.Trash(t.Context(), created.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	restored, err := store.Restore(t.Context(), created.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if restored.Status != post.StatusDraft {
		t.Errorf("Status = %q, want %q", restored.Status, post.StatusDraft)
	}
	if restored.Slug != "hello-world" {
		t.Errorf("Slug = %q, want the original %q", restored.Slug, "hello-world")
	}
}

func TestPostStoreRestoreKeepsTheRenamedSlugWhenTheOriginalIsTaken(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Hello World", author)
	trashed, err := store.Trash(t.Context(), created.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	replacement := mustPost(t, "Hello World", author)
	if _, err := store.Create(t.Context(), replacement); err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	restored, err := store.Restore(t.Context(), created.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if restored.Slug != trashed.Slug {
		t.Errorf("Slug = %q, want the renamed %q kept", restored.Slug, trashed.Slug)
	}
}

func TestPostStoreRestoreReportsMissingPosts(t *testing.T) {
	t.Parallel()

	store, _ := newPostStore(t)

	_, err := store.Restore(t.Context(), uuid.Must(uuid.NewV7()), time.Now().UTC())

	if !errors.Is(err, post.ErrNotFound) {
		t.Errorf("Restore() error = %v, want %v", err, post.ErrNotFound)
	}
}

func TestPostStoreDeleteRemovesThePost(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Doomed", author)

	if err := store.Delete(t.Context(), created.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if _, err := store.ByID(t.Context(), created.ID); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("ByID() after delete error = %v, want %v", err, post.ErrNotFound)
	}
}

func TestPostStoreDeleteReportsMissingPosts(t *testing.T) {
	t.Parallel()

	store, _ := newPostStore(t)

	err := store.Delete(t.Context(), uuid.Must(uuid.NewV7()))

	if !errors.Is(err, post.ErrNotFound) {
		t.Errorf("Delete() error = %v, want %v", err, post.ErrNotFound)
	}
}
