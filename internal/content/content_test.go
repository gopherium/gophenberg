// SPDX-License-Identifier: Apache-2.0

package content_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

var errEntropy = errors.New("entropy source failed")

// postType returns the built-in post type as the registry holds it.
func postType() content.Type {
	now := time.Now().UTC()
	return content.Type{
		Key:           content.TypePost,
		SingularLabel: "Post",
		PluralLabel:   "Posts",
		Revisions:     true,
		RevisionCap:   100,
		PageKind:      content.PageKindSingle,
		Default:       true,
		Active:        true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errEntropy
}

func TestNewReportsIDGenerationFailure(t *testing.T) {
	author := uuid.Must(uuid.NewV7())
	uuid.SetRand(failingReader{})
	defer uuid.SetRand(nil)

	_, err := content.New(postType(), nil, "Hello World", author)

	if !errors.Is(err, errEntropy) {
		t.Fatalf("New() error = %v, want the entropy failure in its chain", err)
	}
}

func TestNewReturnsADraftWithGeneratedFields(t *testing.T) {
	t.Parallel()

	author := uuid.Must(uuid.NewV7())
	before := time.Now().UTC()

	p, err := content.New(postType(), nil, "Hello World", author)

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if p.ID == uuid.Nil {
		t.Error("ID is nil, want a generated identifier")
	}
	if p.Status != content.StatusDraft {
		t.Errorf("Status = %q, want %q", p.Status, content.StatusDraft)
	}
	if p.Slug != "hello-world" {
		t.Errorf("Slug = %q, want %q", p.Slug, "hello-world")
	}
	if p.Title != "Hello World" {
		t.Errorf("Title = %q, want the title preserved", p.Title)
	}
	if p.Type != "post" {
		t.Errorf("Type = %q, want %q", p.Type, "post")
	}
	if p.AuthorID != author {
		t.Errorf("AuthorID = %v, want %v", p.AuthorID, author)
	}
	if p.PublishedAt != nil {
		t.Errorf("PublishedAt = %v, want nil on a draft", p.PublishedAt)
	}
	if !p.CreatedAt.Equal(p.UpdatedAt) {
		t.Errorf("CreatedAt = %v, UpdatedAt = %v, want them equal on creation", p.CreatedAt, p.UpdatedAt)
	}
	if p.CreatedAt.Before(before) {
		t.Errorf("CreatedAt = %v, want an instant at or after %v", p.CreatedAt, before)
	}
	if p.CreatedAt.Location() != time.UTC {
		t.Errorf("CreatedAt location = %v, want UTC", p.CreatedAt.Location())
	}
}

func TestNewTitlesWithoutSlugCharactersFallBackToUntitled(t *testing.T) {
	t.Parallel()

	p, err := content.New(postType(), nil, "   ", uuid.Must(uuid.NewV7()))

	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if p.Slug != "untitled" {
		t.Errorf("Slug = %q, want %q", p.Slug, "untitled")
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	inactive := postType()
	inactive.Active = false
	tests := map[string]struct {
		contentType content.Type
		author      uuid.UUID
		want        error
	}{
		"unregistered type": {contentType: content.Type{}, author: uuid.Must(uuid.NewV7()), want: content.ErrInvalidType},
		"inactive type":     {contentType: inactive, author: uuid.Must(uuid.NewV7()), want: content.ErrTypeInactive},
		"missing author":    {contentType: postType(), author: uuid.Nil, want: content.ErrInvalidAuthor},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			_, err := content.New(tc.contentType, nil, "Hello", tc.author)

			if !errors.Is(err, tc.want) {
				t.Errorf("New() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		in   string
		want string
	}{
		"lowercases and joins words":   {in: "Hello World", want: "hello-world"},
		"collapses runs of separators": {in: "multiple   spaces", want: "multiple-spaces"},
		"trims leading and trailing":   {in: "---edged---", want: "edged"},
		"drops apostrophes":            {in: "How's it going", want: "hows-it-going"},
		"keeps digits":                 {in: "Top 10 Reasons", want: "top-10-reasons"},
		"keeps unicode letters":        {in: "Cafe Munchen", want: "cafe-munchen"},
		"lowercases unicode":           {in: "MARIA PEREZ", want: "maria-perez"},
		"replaces punctuation":         {in: "Hello, World! Again?", want: "hello-world-again"},
		"empty falls back":             {in: "", want: "untitled"},
		"separators only fall back":    {in: "!!! ???", want: "untitled"},
	}

	for testName, tc := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			if got := content.Slugify(tc.in); got != tc.want {
				t.Errorf("Slugify(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSlugifyTruncatesLongTitlesWithoutTrailingSeparator(t *testing.T) {
	t.Parallel()

	got := content.Slugify(strings.Repeat("a", 199) + " b")

	if want := strings.Repeat("a", 199); got != want {
		t.Errorf("Slugify() = %q with length %d, want %d characters and no trailing hyphen", got, len(got), len(want))
	}
}
