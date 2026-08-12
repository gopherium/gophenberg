// SPDX-License-Identifier: Apache-2.0

// Package content defines the content object at the center of the CMS.
package content

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// ErrInvalidType reports that a content type is not registered.
var ErrInvalidType = errors.New("content: invalid type")

// ErrInvalidAuthor reports that a content item carries no author.
var ErrInvalidAuthor = errors.New("content: invalid author")

// ErrNotFound reports that no content item exists for the requested ID.
var ErrNotFound = errors.New("content: not found")

// ErrSlugTaken reports that a content type already holds the requested slug.
var ErrSlugTaken = errors.New("content: slug taken")

// slugMaxLength bounds a generated slug in characters.
const slugMaxLength = 200

// untitledSlug is the slug of a content item whose title yields no slug characters.
const untitledSlug = "untitled"

// Content is a content item holding Gutenberg-serialized block HTML.
type Content struct {
	ID          uuid.UUID
	Type        string
	Slug        string
	Title       string
	Content     string
	Excerpt     string
	Status      Status
	AuthorID    uuid.UUID
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// New returns a draft [Content] of the given type, slugged after its title.
func New(t Type, title string, authorID uuid.UUID) (Content, error) {
	if t.Key == "" {
		return Content{}, ErrInvalidType
	}
	if !t.Active {
		return Content{}, ErrTypeInactive
	}
	if authorID == uuid.Nil {
		return Content{}, ErrInvalidAuthor
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Content{}, fmt.Errorf("content: generate id: %w", err)
	}
	now := time.Now().UTC()
	return Content{
		ID:        id,
		Type:      t.Key,
		Slug:      Slugify(title),
		Title:     title,
		Status:    StatusDraft,
		AuthorID:  authorID,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Slugify returns title as a URL slug, or [untitledSlug] when it holds no
// slug characters.
func Slugify(title string) string {
	var b strings.Builder
	separated := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r == '\'' || r == '’':
			continue
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if separated && b.Len() > 0 {
				b.WriteRune('-')
			}
			separated = false
			b.WriteRune(r)
		default:
			separated = true
		}
	}
	slug := truncateSlug(b.String())
	if slug == "" {
		return untitledSlug
	}
	return slug
}

// truncateSlug returns slug bounded to [slugMaxLength] characters without a trailing separator.
func truncateSlug(slug string) string {
	runes := []rune(slug)
	if len(runes) > slugMaxLength {
		runes = runes[:slugMaxLength]
	}
	return strings.TrimRight(string(runes), "-")
}
