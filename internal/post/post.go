// SPDX-License-Identifier: Apache-2.0

// Package post defines the content object at the center of the CMS.
package post

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// ErrInvalidType reports that a post type is not registered.
var ErrInvalidType = errors.New("post: invalid type")

// ErrInvalidAuthor reports that a post carries no author.
var ErrInvalidAuthor = errors.New("post: invalid author")

// ErrNotFound reports that no post exists for the requested ID.
var ErrNotFound = errors.New("post: not found")

// ErrSlugTaken reports that a post type already holds the requested slug.
var ErrSlugTaken = errors.New("post: slug taken")

// slugMaxLength bounds a generated slug in characters.
const slugMaxLength = 200

// untitledSlug is the slug of a post whose title yields no slug characters.
const untitledSlug = "untitled"

// Post is a content object holding Gutenberg-serialized block HTML.
type Post struct {
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

// New returns a draft [Post] of the given type, slugged after its title.
func New(postType, title string, authorID uuid.UUID) (Post, error) {
	trimmedType := strings.TrimSpace(postType)
	if _, ok := TypeByName(trimmedType); !ok {
		return Post{}, ErrInvalidType
	}
	if authorID == uuid.Nil {
		return Post{}, ErrInvalidAuthor
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Post{}, fmt.Errorf("post: generate id: %w", err)
	}
	now := time.Now().UTC()
	return Post{
		ID:        id,
		Type:      trimmedType,
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
