// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

func TestNewRevisionSnapshotsTheEditableContent(t *testing.T) {
	t.Parallel()

	author := uuid.Must(uuid.NewV7())
	editor := uuid.Must(uuid.NewV7())
	p, err := content.New(postType(), "Snapshot Me", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	p.Content = "<!-- wp:paragraph --><p>Body</p><!-- /wp:paragraph -->"
	p.Excerpt = "Summary"
	before := time.Now().UTC()

	revision, err := content.NewRevision(p, content.RevisionKindRevision, editor)

	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	if revision.ID == uuid.Nil || revision.ID == p.ID {
		t.Errorf("ID = %v, want an identifier of its own", revision.ID)
	}
	if revision.ContentID != p.ID {
		t.Errorf("ContentID = %v, want %v", revision.ContentID, p.ID)
	}
	if revision.AuthorID != editor {
		t.Errorf("AuthorID = %v, want the editor %v rather than the post's author", revision.AuthorID, editor)
	}
	if revision.Kind != content.RevisionKindRevision {
		t.Errorf("Kind = %q, want %q", revision.Kind, content.RevisionKindRevision)
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
	p, err := content.New(postType(), "Autosaved", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}

	revision, err := content.NewRevision(p, content.RevisionKindAutosave, author)

	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	if revision.Kind != content.RevisionKindAutosave {
		t.Errorf("Kind = %q, want %q", revision.Kind, content.RevisionKindAutosave)
	}
}

func TestNewRevisionReportsIDGenerationFailure(t *testing.T) {
	author := uuid.Must(uuid.NewV7())
	p, err := content.New(postType(), "Doomed", author)
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	uuid.SetRand(failingReader{})
	defer uuid.SetRand(nil)

	_, err = content.NewRevision(p, content.RevisionKindRevision, author)

	if !errors.Is(err, errEntropy) {
		t.Fatalf("NewRevision() error = %v, want the entropy failure in its chain", err)
	}
}
