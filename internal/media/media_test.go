// SPDX-License-Identifier: Apache-2.0

package media_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/media"
)

// mustAuthor returns an author id for a valid media item.
func mustAuthor(t *testing.T) uuid.UUID {
	t.Helper()
	return uuid.Must(uuid.NewV7())
}

func TestNewDerivesAnImageFromItsMimeType(t *testing.T) {
	t.Parallel()

	author := mustAuthor(t)

	m, err := media.New("2026/08/harbor.jpg", "harbor", "image/jpeg", author)

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if m.Type != media.TypeImage {
		t.Errorf("Type = %q, want %q", m.Type, media.TypeImage)
	}
	if m.File != "2026/08/harbor.jpg" {
		t.Errorf("File = %q, want the given path", m.File)
	}
	if m.Title != "harbor" {
		t.Errorf("Title = %q, want the given title", m.Title)
	}
	if m.MimeType != "image/jpeg" {
		t.Errorf("MimeType = %q, want the given type", m.MimeType)
	}
	if m.AuthorID != author {
		t.Errorf("AuthorID = %v, want the given author", m.AuthorID)
	}
}

func TestNewTreatsOtherMimeTypesAsPlainFiles(t *testing.T) {
	t.Parallel()

	m, err := media.New("2026/08/manual.pdf", "manual", "application/pdf", mustAuthor(t))

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if m.Type != media.TypeFile {
		t.Errorf("Type = %q, want %q", m.Type, media.TypeFile)
	}
}

func TestNewStartsEmptyBeyondItsIdentity(t *testing.T) {
	t.Parallel()

	m, err := media.New("2026/08/harbor.jpg", "harbor", "image/jpeg", mustAuthor(t))

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if m.ID != 0 {
		t.Errorf("ID = %d, want 0 before the store assigns one", m.ID)
	}
	if m.AltText != "" || m.Caption != "" || m.Description != "" {
		t.Error("descriptions start set, want them empty")
	}
	if m.Width != 0 || m.Height != 0 || m.Filesize != 0 {
		t.Error("measurements start set, want them zero until the pipeline fills them")
	}
	if m.Sizes == nil {
		t.Error("Sizes = nil, want an empty rendition map")
	}
	if len(m.Sizes) != 0 {
		t.Errorf("Sizes holds %d renditions, want none", len(m.Sizes))
	}
}

func TestNewStampsUTCTimestamps(t *testing.T) {
	t.Parallel()

	before := time.Now().UTC()
	m, err := media.New("2026/08/harbor.jpg", "harbor", "image/jpeg", mustAuthor(t))
	after := time.Now().UTC()

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if m.CreatedAt.Location() != time.UTC || m.UpdatedAt.Location() != time.UTC {
		t.Errorf("timestamps carry location %v, want UTC", m.CreatedAt.Location())
	}
	if m.CreatedAt.Before(before) || m.CreatedAt.After(after) {
		t.Errorf("CreatedAt = %v, want between %v and %v", m.CreatedAt, before, after)
	}
	if !m.UpdatedAt.Equal(m.CreatedAt) {
		t.Errorf("UpdatedAt = %v, want it equal to CreatedAt at birth", m.UpdatedAt)
	}
}

func TestNewRefusesABlankFile(t *testing.T) {
	t.Parallel()

	blankFiles := []string{"", "   "}
	for _, blankFile := range blankFiles {
		if _, err := media.New(blankFile, "harbor", "image/jpeg", mustAuthor(t)); !errors.Is(err, media.ErrInvalidFile) {
			t.Errorf("New(%q) error = %v, want ErrInvalidFile", blankFile, err)
		}
	}
}

func TestNewRefusesABlankMimeType(t *testing.T) {
	t.Parallel()

	blankMimes := []string{"", "   "}
	for _, blankMime := range blankMimes {
		_, err := media.New("2026/08/harbor.jpg", "harbor", blankMime, mustAuthor(t))
		if !errors.Is(err, media.ErrInvalidMime) {
			t.Errorf("New() with mime %q error = %v, want ErrInvalidMime", blankMime, err)
		}
	}
}

func TestParseTypeAcceptsTheKnownKinds(t *testing.T) {
	t.Parallel()

	for raw, want := range map[string]media.Type{"image": media.TypeImage, "file": media.TypeFile} {
		got, err := media.ParseType(raw)
		if err != nil {
			t.Fatalf("ParseType(%q) error = %v, want nil", raw, err)
		}
		if got != want {
			t.Errorf("ParseType(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestParseTypeRefusesAnUnknownKind(t *testing.T) {
	t.Parallel()

	unknownKinds := []string{"audio", "", "IMAGE"}
	for _, unknownKind := range unknownKinds {
		if _, err := media.ParseType(unknownKind); !errors.Is(err, media.ErrInvalidType) {
			t.Errorf("ParseType(%q) error = %v, want ErrInvalidType", unknownKind, err)
		}
	}
}

func TestNewRefusesAMissingAuthor(t *testing.T) {
	t.Parallel()

	_, err := media.New("2026/08/harbor.jpg", "harbor", "image/jpeg", uuid.Nil)
	if !errors.Is(err, media.ErrInvalidAuthor) {
		t.Errorf("New() without an author error = %v, want ErrInvalidAuthor", err)
	}
}
