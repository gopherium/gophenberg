// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/post"
)

// mustAutosave returns an autosave of the post credited to author.
func mustAutosave(t *testing.T, p post.Post, author uuid.UUID) post.Revision {
	t.Helper()
	autosave, err := post.NewRevision(p, post.RevisionKindAutosave, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	return autosave
}

// addAuthor stores a second user and returns its id.
func addAuthor(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	author := uuid.Must(uuid.NewV7())
	_, err := pool.Exec(t.Context(),
		`INSERT INTO auth.users (id, email, name, password_hash, disabled, created_at)
		VALUES ($1, $2, $3, 'hash', false, $4)`,
		author, author.String()+"@example.com", name, time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("inserting author: %v", err)
	}
	return author
}

func TestPostStoreSaveAutosaveStoresTheBuffer(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Autosaved", author)
	buffer := created
	buffer.Title = "Buffered Title"
	buffer.Content = "<!-- wp:paragraph --><p>Buffered</p><!-- /wp:paragraph -->"

	saved, err := store.SaveAutosave(t.Context(), mustAutosave(t, buffer, author))

	if err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}
	if saved.Title != "Buffered Title" || saved.Content != buffer.Content {
		t.Errorf("saved = %+v, want the buffered content", saved)
	}
	if saved.Kind != post.RevisionKindAutosave || saved.AuthorID != author {
		t.Errorf("saved = %+v, want an autosave credited to the author", saved)
	}
}

func TestPostStoreSaveAutosaveReplacesTheAuthorsAutosave(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Autosaved", author)
	first := created
	first.Title = "First Buffer"
	parked, err := store.SaveAutosave(t.Context(), mustAutosave(t, first, author))
	if err != nil {
		t.Fatalf("first SaveAutosave() error = %v, want nil", err)
	}
	second := created
	second.Title = "Second Buffer"

	saved, err := store.SaveAutosave(t.Context(), mustAutosave(t, second, author))

	if err != nil {
		t.Fatalf("second SaveAutosave() error = %v, want nil", err)
	}
	if saved.Title != "Second Buffer" {
		t.Errorf("Title = %q, want the newer buffer", saved.Title)
	}
	if saved.ID != parked.ID {
		t.Errorf("ID = %s, want the replaced buffer to keep row id %s", saved.ID, parked.ID)
	}
	revisions, err := store.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 1 {
		t.Errorf("revisions = %d, want one autosave per author", len(revisions))
	}
}

func TestPostStoreSaveAutosaveKeepsOnePerAuthor(t *testing.T) {
	t.Parallel()

	store, ada, pool := newPostStoreWithPool(t)
	grace := addAuthor(t, pool, "Grace Hopper")
	created := mustCreate(t, store, "Shared", ada)

	if _, err := store.SaveAutosave(t.Context(), mustAutosave(t, created, ada)); err != nil {
		t.Fatalf("SaveAutosave(ada) error = %v, want nil", err)
	}
	if _, err := store.SaveAutosave(t.Context(), mustAutosave(t, created, grace)); err != nil {
		t.Fatalf("SaveAutosave(grace) error = %v, want nil", err)
	}

	revisions, err := store.Revisions(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("Revisions() error = %v, want nil", err)
	}
	if len(revisions) != 2 {
		t.Errorf("revisions = %d, want one autosave for each author", len(revisions))
	}
}

func TestPostStoreAutosaveReturnsTheAuthorsBuffer(t *testing.T) {
	t.Parallel()

	store, ada, pool := newPostStoreWithPool(t)
	grace := addAuthor(t, pool, "Grace Hopper")
	created := mustCreate(t, store, "Shared", ada)
	adaBuffer := created
	adaBuffer.Title = "Ada Buffer"
	if _, err := store.SaveAutosave(t.Context(), mustAutosave(t, adaBuffer, ada)); err != nil {
		t.Fatalf("SaveAutosave() error = %v, want nil", err)
	}

	stored, err := store.Autosave(t.Context(), created.ID, ada)

	if err != nil {
		t.Fatalf("Autosave() error = %v, want nil", err)
	}
	if stored.Title != "Ada Buffer" {
		t.Errorf("Title = %q, want the author's own buffer", stored.Title)
	}
	if _, err := store.Autosave(t.Context(), created.ID, grace); !errors.Is(err, post.ErrRevisionNotFound) {
		t.Errorf("Autosave(grace) error = %v, want %v", err, post.ErrRevisionNotFound)
	}
}

func TestPostStoreAutosaveReportsMissingBuffers(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Unsaved", author)

	_, err := store.Autosave(t.Context(), created.ID, author)

	if !errors.Is(err, post.ErrRevisionNotFound) {
		t.Errorf("Autosave() error = %v, want %v", err, post.ErrRevisionNotFound)
	}
}

func TestPostStoreSaveAutosaveReportsAVanishedPost(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Vanishing", author)
	autosave := mustAutosave(t, created, author)
	if err := store.Delete(t.Context(), created.ID); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if _, err := store.SaveAutosave(t.Context(), autosave); !errors.Is(err, post.ErrNotFound) {
		t.Errorf("SaveAutosave() error = %v, want %v", err, post.ErrNotFound)
	}
}

func TestPostStoreDeleteAutosaveRemovesOnlyTheAuthorsBuffer(t *testing.T) {
	t.Parallel()

	store, ada, pool := newPostStoreWithPool(t)
	grace := addAuthor(t, pool, "Grace Hopper")
	created := mustCreate(t, store, "Shared", ada)
	if _, err := store.SaveAutosave(t.Context(), mustAutosave(t, created, ada)); err != nil {
		t.Fatalf("SaveAutosave(ada) error = %v, want nil", err)
	}
	if _, err := store.SaveAutosave(t.Context(), mustAutosave(t, created, grace)); err != nil {
		t.Fatalf("SaveAutosave(grace) error = %v, want nil", err)
	}

	if err := store.DeleteAutosave(t.Context(), created.ID, ada); err != nil {
		t.Fatalf("DeleteAutosave() error = %v, want nil", err)
	}

	if _, err := store.Autosave(t.Context(), created.ID, ada); !errors.Is(err, post.ErrRevisionNotFound) {
		t.Errorf("Autosave(ada) error = %v, want %v", err, post.ErrRevisionNotFound)
	}
	if _, err := store.Autosave(t.Context(), created.ID, grace); err != nil {
		t.Errorf("Autosave(grace) error = %v, want the other author's buffer kept", err)
	}
}

func TestPostStoreDeleteAutosaveToleratesAMissingBuffer(t *testing.T) {
	t.Parallel()

	store, author := newPostStore(t)
	created := mustCreate(t, store, "Unsaved", author)

	if err := store.DeleteAutosave(t.Context(), created.ID, author); err != nil {
		t.Errorf("DeleteAutosave() error = %v, want nil", err)
	}
}

func TestPostStoreAutosavesReportDatabaseFailures(t *testing.T) {
	t.Parallel()

	store, author, pool := newPostStoreWithPool(t)
	created := mustCreate(t, store, "Doomed", author)
	autosave := mustAutosave(t, created, author)
	pool.Close()

	if _, err := store.SaveAutosave(t.Context(), autosave); err == nil {
		t.Error("SaveAutosave() on a closed pool error = nil, want a failure")
	}
	if _, err := store.Autosave(t.Context(), created.ID, author); err == nil {
		t.Error("Autosave() on a closed pool error = nil, want a failure")
	}
	if err := store.DeleteAutosave(t.Context(), created.ID, author); err == nil {
		t.Error("DeleteAutosave() on a closed pool error = nil, want a failure")
	}
}
