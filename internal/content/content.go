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
	"golang.org/x/text/unicode/norm"
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
	ParentID    *uuid.UUID
	Path        string
	Slug        string
	Title       string
	Content     string
	Excerpt     string
	Status      Status
	AuthorID    uuid.UUID
	Fields      Values
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListedItem is a content item as a listing carries it, beside its author's name and its parent's title.
type ListedItem struct {
	Content
	AuthorName string
	// ParentTitle is the title of the item's parent, empty at the top level or under an untitled parent.
	ParentTitle string
}

// New returns a draft [Content] of the given type under parent, slugged after its title.
func New(t Type, parent *Content, title string, authorID uuid.UUID) (Content, error) {
	if t.Key == "" {
		return Content{}, ErrInvalidType
	}
	if !t.Active {
		return Content{}, ErrTypeInactive
	}
	if authorID == uuid.Nil {
		return Content{}, ErrInvalidAuthor
	}
	slug := Slugify(title)
	path, parentID, err := placeUnder(t, parent, slug, 0)
	if err != nil {
		return Content{}, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Content{}, fmt.Errorf("content: generate id: %w", err)
	}
	now := time.Now().UTC()
	return Content{
		ID:        id,
		Type:      t.Key,
		ParentID:  parentID,
		Path:      path,
		Slug:      slug,
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
	separated, afterKept := false, false
	for _, r := range FoldAccents(strings.ToLower(title)) {
		switch slugRuneKindOf(r, afterKept) {
		case slugRuneWord:
			if separated && b.Len() > 0 {
				b.WriteRune('-')
			}
			separated, afterKept = false, true
			b.WriteRune(r)
		case slugRuneMark:
			b.WriteRune(r)
		case slugRuneSeparator:
			separated, afterKept = true, false
		}
	}
	slug := truncateSlug(norm.NFC.String(b.String()))
	if slug == "" {
		return untitledSlug
	}
	return slug
}

// slugRuneKind is the role of one title character in a slug: skipped, a word character, a kept mark or a separator.
type slugRuneKind int

const (
	slugRuneSkip slugRuneKind = iota
	slugRuneWord
	slugRuneMark
	slugRuneSeparator
)

// slugRuneKindOf returns the [slugRuneKind] of r, given whether the previous character was kept.
func slugRuneKindOf(r rune, afterKept bool) slugRuneKind {
	switch {
	case isApostrophe(r) || isSeparateAccent(r):
		return slugRuneSkip
	case unicode.IsLetter(r) || unicode.IsDigit(r):
		return slugRuneWord
	case afterKept && unicode.Is(unicode.M, r):
		return slugRuneMark
	default:
		return slugRuneSeparator
	}
}

// isSeparateAccent reports whether r is a combining grave, acute, macron or caron, or the grapheme joiner.
func isSeparateAccent(r rune) bool {
	switch r {
	case 0x0300, 0x0301, 0x0304, 0x030C, 0x034F:
		return true
	}
	return false
}

// truncateSlug returns slug within [slugMaxLength] characters, with no trailing hyphen and no mark cut from its letter.
func truncateSlug(slug string) string {
	runes := []rune(slug)
	if len(runes) > slugMaxLength {
		end := slugMaxLength
		for unicode.Is(unicode.M, runes[end]) {
			end--
		}
		runes = runes[:end]
	}
	return strings.TrimRight(string(runes), "-")
}
