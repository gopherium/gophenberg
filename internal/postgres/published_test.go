// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// blockMarkup is the serialized block content a published fixture carries.
const blockMarkup = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"

// fixtureTitle is the title every published fixture carries.
const fixtureTitle = "Hello World"

// insertContentWithStatus stores a post of the given type, status, and slug through raw SQL.
func insertContentWithStatus(
	t *testing.T, pool *pgxpool.Pool, author uuid.UUID, postType string, status content.Status, slug string,
) {
	t.Helper()
	now := time.Now().UTC()
	_, err := pool.Exec(t.Context(),
		`INSERT INTO core.content (id, type, status, slug, title, content, excerpt, author_id,
			published_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'Stored Title', $5, '', $6, $7, $7, $7)`,
		uuid.Must(uuid.NewV7()), postType, string(status), slug, blockMarkup, author, now,
	)
	if err != nil {
		t.Fatalf("inserting a %s post: %v", status, err)
	}
}

// publishWithContent stores a published post titled [fixtureTitle] carrying [blockMarkup].
func publishWithContent(t *testing.T, store *postgres.ContentStore, author uuid.UUID) content.Content {
	t.Helper()
	published := publish(t, store, fixtureTitle, author, time.Now().UTC())
	edited := published
	edited.Content = blockMarkup
	updated, err := store.Update(t.Context(), edited, published.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("adding content to %q: %v", fixtureTitle, err)
	}
	return updated
}

func TestContentStoreFindsAPublishedPostBySlug(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	published := publishWithContent(t, store, author)

	found, err := store.PublishedBySlug(t.Context(), content.TypePost, published.Slug)

	if err != nil {
		t.Fatalf("PublishedBySlug() error = %v, want nil", err)
	}
	if found.ID != published.ID {
		t.Errorf("ID = %v, want %v", found.ID, published.ID)
	}
	if found.Content != blockMarkup {
		t.Errorf("Content = %q, want the stored block markup %q", found.Content, blockMarkup)
	}
	if found.PublishedAt == nil {
		t.Fatal("PublishedAt = nil, want the publication instant")
	}
	if found.PublishedAt.Location() != time.UTC {
		t.Errorf("PublishedAt location = %v, want %v", found.PublishedAt.Location(), time.UTC)
	}
}

func TestContentStoreHidesEveryStatusButPublished(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	statuses := []content.Status{
		content.StatusDraft,
		content.StatusPending,
		content.StatusPrivate,
		content.StatusScheduled,
		content.StatusTrash,
	}
	for _, status := range statuses {
		insertContentWithStatus(t, pool, author, content.TypePost, status, "hidden-"+string(status))
	}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			t.Parallel()

			_, err := store.PublishedBySlug(t.Context(), content.TypePost, "hidden-"+string(status))

			if !errors.Is(err, content.ErrNotFound) {
				t.Errorf("PublishedBySlug() error = %v, want %v", err, content.ErrNotFound)
			}
		})
	}
}

func TestContentStoreScopesTheLookupToThePostType(t *testing.T) {
	t.Parallel()

	store, author, pool := newContentStoreWithPool(t)
	insertContentWithStatus(t, pool, author, "page", content.StatusPublished, "about-us")

	found, err := store.PublishedBySlug(t.Context(), "page", "about-us")
	if err != nil {
		t.Fatalf("PublishedBySlug(page) error = %v, want nil", err)
	}
	if found.Type != "page" || found.Slug != "about-us" {
		t.Errorf("found = %q %q, want the page carrying the slug", found.Type, found.Slug)
	}

	_, err = store.PublishedBySlug(t.Context(), content.TypePost, "about-us")

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedBySlug(post) error = %v, want %v", err, content.ErrNotFound)
	}
}

func TestContentStoreReportsAnUnknownSlug(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	publishWithContent(t, store, author)

	_, err := store.PublishedBySlug(t.Context(), content.TypePost, "never-written")

	if !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedBySlug() error = %v, want %v", err, content.ErrNotFound)
	}
}

func TestContentStoreHidesARestoredPostUntilItIsPublishedAgain(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	published := publishWithContent(t, store, author)
	slug := published.Slug

	trashed, err := store.Trash(t.Context(), published.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if _, err := store.PublishedBySlug(t.Context(), content.TypePost, trashed.Slug); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedBySlug() at the trashed slug error = %v, want %v", err, content.ErrNotFound)
	}
	if _, err := store.PublishedBySlug(t.Context(), content.TypePost, slug); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedBySlug() at the freed slug error = %v, want %v", err, content.ErrNotFound)
	}

	restored, err := store.Restore(t.Context(), trashed.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if restored.Status != content.StatusDraft {
		t.Fatalf("Status after Restore() = %q, want %q", restored.Status, content.StatusDraft)
	}
	if _, err := store.PublishedBySlug(t.Context(), content.TypePost, slug); !errors.Is(err, content.ErrNotFound) {
		t.Errorf("PublishedBySlug() after restoring error = %v, want %v", err, content.ErrNotFound)
	}

	republished := restored
	republished.Status = content.StatusPublished
	if _, err := store.Update(t.Context(), republished, restored.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("republishing: %v", err)
	}

	found, err := store.PublishedBySlug(t.Context(), content.TypePost, slug)

	if err != nil {
		t.Fatalf("PublishedBySlug() after republishing error = %v, want nil", err)
	}
	if found.ID != published.ID {
		t.Errorf("ID = %v, want %v", found.ID, published.ID)
	}
}

func TestContentStoreServesARestoredPostAtTheSlugItKept(t *testing.T) {
	t.Parallel()

	store, author := newContentStore(t)
	first := publishWithContent(t, store, author)
	trashed, err := store.Trash(t.Context(), first.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	second := publishWithContent(t, store, author)
	if second.Slug != first.Slug {
		t.Fatalf("second Slug = %q, want the freed %q", second.Slug, first.Slug)
	}

	restored, err := store.Restore(t.Context(), trashed.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	republished := restored
	republished.Status = content.StatusPublished
	if _, err := store.Update(t.Context(), republished, restored.UpdatedAt, nil, 0); err != nil {
		t.Fatalf("republishing: %v", err)
	}

	found, err := store.PublishedBySlug(t.Context(), content.TypePost, restored.Slug)

	if err != nil {
		t.Fatalf("PublishedBySlug() at the kept slug error = %v, want nil", err)
	}
	if found.ID != first.ID {
		t.Errorf("ID = %v, want the restored post %v", found.ID, first.ID)
	}
	if !trashedSlug.MatchString(restored.Slug) {
		t.Errorf("restored Slug = %q, want the trash suffix the collision keeps", restored.Slug)
	}
}
