// SPDX-License-Identifier: Apache-2.0

package post_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/post"
)

func TestNewRevisionSnapshotsTheEditableContent(t *testing.T) {
	t.Parallel()

	author := uuid.Must(uuid.NewV7())
	editor := uuid.Must(uuid.NewV7())
	p, err := post.New(post.TypePost, "Snapshot Me", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	p.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	p.Excerpt = "Summary"
	before := time.Now().UTC()

	revision, err := post.NewRevision(p, post.RevisionKindRevision, editor)

	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	if revision.ID == uuid.Nil || revision.ID == p.ID {
		t.Errorf("ID = %v, want an identifier of its own", revision.ID)
	}
	if revision.PostID != p.ID {
		t.Errorf("PostID = %v, want %v", revision.PostID, p.ID)
	}
	if revision.AuthorID != editor {
		t.Errorf("AuthorID = %v, want the editor %v rather than the post's author", revision.AuthorID, editor)
	}
	if revision.Kind != post.RevisionKindRevision {
		t.Errorf("Kind = %q, want %q", revision.Kind, post.RevisionKindRevision)
	}
	if revision.Title != p.Title || revision.Content != p.Content || revision.Excerpt != p.Excerpt {
		t.Errorf("revision = %+v, want the post's editable fields", revision)
	}
	if revision.CreatedAt.Before(before) || revision.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt = %v, want a recent UTC instant", revision.CreatedAt)
	}
}

func TestNewRevisionCarriesTheAutosaveKind(t *testing.T) {
	t.Parallel()

	author := uuid.Must(uuid.NewV7())
	p, err := post.New(post.TypePost, "Autosaved", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	revision, err := post.NewRevision(p, post.RevisionKindAutosave, author)

	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	if revision.Kind != post.RevisionKindAutosave {
		t.Errorf("Kind = %q, want %q", revision.Kind, post.RevisionKindAutosave)
	}
}

func TestNewRevisionReportsIDGenerationFailure(t *testing.T) {
	author := uuid.Must(uuid.NewV7())
	p, err := post.New(post.TypePost, "Doomed", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	uuid.SetRand(failingReader{})
	defer uuid.SetRand(nil)

	_, err = post.NewRevision(p, post.RevisionKindRevision, author)

	if !errors.Is(err, errEntropy) {
		t.Fatalf("NewRevision() error = %v, want the entropy failure in its chain", err)
	}
}
