// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"errors"
	"math"
	"regexp"
	"strings"
	"time"
)

// TypePost is the key of the built-in post type.
const TypePost = "post"

// ErrTypeNotFound reports that no content type carries the requested key.
var ErrTypeNotFound = errors.New("content: type not found")

// ErrTypeTaken reports that a content type already carries the requested key.
var ErrTypeTaken = errors.New("content: type key taken")

// ErrRouteWordTaken reports that another content type answers under the route word.
var ErrRouteWordTaken = errors.New("content: route word taken")

// ErrRouteWordReserved reports that the route word names a place the CMS keeps.
var ErrRouteWordReserved = errors.New("content: route word reserved")

// ErrRootTaken reports that another content type already lives at the root.
var ErrRootTaken = errors.New("content: another type holds the root")

// ErrDefaultRequired reports that the default content type may not be deactivated or removed.
var ErrDefaultRequired = errors.New("content: the default type must stay")

// ErrTypeInUse reports that the content type still holds content.
var ErrTypeInUse = errors.New("content: type still holds content")

// ErrTypeInactive reports that the content type is not active.
var ErrTypeInactive = errors.New("content: type is not active")

// ErrInvalidKey reports that a type key is not a lowercase word.
var ErrInvalidKey = errors.New("content: invalid type key")

// ErrInvalidRouteWord reports that a route word is not a lowercase word.
var ErrInvalidRouteWord = errors.New("content: invalid route word")

// ErrInvalidLabel reports that a type carries no label.
var ErrInvalidLabel = errors.New("content: invalid type label")

// ErrInvalidPageKind reports that a page kind is not one the CMS renders.
var ErrInvalidPageKind = errors.New("content: invalid page kind")

// ErrInvalidRevisionCap reports that a revision cap is not a number of rows.
var ErrInvalidRevisionCap = errors.New("content: invalid revision cap")

// PageKind is what the public page of one content item renders.
type PageKind string

// The page kinds a content type declares.
const (
	PageKindSingle  PageKind = "single"
	PageKindArchive PageKind = "archive"
)

// typeWord matches a key or route word: a lowercase letter, then lowercase
// letters, digits or hyphens.
var typeWord = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// ReservedRouteWords are the first address segments the CMS keeps for itself.
var ReservedRouteWords = []string{"admin", "api", "gophenberg", "_gophenberg", "media"}

// Type describes a kind of content the CMS stores.
type Type struct {
	Key           string
	SingularLabel string
	PluralLabel   string
	RouteWord     string
	Hierarchical  bool
	Revisions     bool
	RevisionCap   int
	PageKind      PageKind
	Default       bool
	Active        bool
	Fields        []Field
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// TypeStore persists the content type registry and its field definitions.
type TypeStore interface {
	List(ctx context.Context) ([]Type, error)
	ByKey(ctx context.Context, key string) (Type, error)
	Create(ctx context.Context, t Type) (Type, error)
	Update(ctx context.Context, t Type) (Type, error)
	Delete(ctx context.Context, key string) error
	CreateField(ctx context.Context, f Field) (Field, error)
	UpdateField(ctx context.Context, f Field) (Field, error)
	DeleteField(ctx context.Context, typeKey, key string) error
}

// NewType returns a content type ready to store, or the reason it is not one.
func NewType(key, singular, plural, routeWord string) (Type, error) {
	now := time.Now().UTC()
	t := Type{
		Key:           strings.TrimSpace(key),
		SingularLabel: strings.TrimSpace(singular),
		PluralLabel:   strings.TrimSpace(plural),
		RouteWord:     strings.TrimSpace(routeWord),
		Revisions:     true,
		RevisionCap:   defaultRevisionCap,
		PageKind:      PageKindSingle,
		Active:        true,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := t.Validate(); err != nil {
		return Type{}, err
	}
	return t, nil
}

// defaultRevisionCap bounds the revisions a new content type keeps.
const defaultRevisionCap = 100

// Validate reports whether the type may be stored.
func (t Type) Validate() error {
	if !typeWord.MatchString(t.Key) {
		return ErrInvalidKey
	}
	if t.SingularLabel == "" || t.PluralLabel == "" {
		return ErrInvalidLabel
	}
	if t.RevisionCap < 0 || t.RevisionCap > math.MaxInt32 {
		return ErrInvalidRevisionCap
	}
	if err := t.validatePageKind(); err != nil {
		return err
	}
	return t.validateRouteWord()
}

// validatePageKind reports whether the CMS renders the type's item page.
func (t Type) validatePageKind() error {
	switch t.PageKind {
	case PageKindSingle, PageKindArchive:
		return nil
	default:
		return ErrInvalidPageKind
	}
}

// validateRouteWord reports whether the route word may address the type.
func (t Type) validateRouteWord() error {
	for _, reserved := range ReservedRouteWords {
		if t.RouteWord == reserved {
			return ErrRouteWordReserved
		}
	}
	if t.RouteWord == "" {
		if t.Default {
			return nil
		}
		return ErrRootTaken
	}
	if !typeWord.MatchString(t.RouteWord) {
		return ErrInvalidRouteWord
	}
	return nil
}
