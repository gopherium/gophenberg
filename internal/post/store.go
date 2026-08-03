// SPDX-License-Identifier: Apache-2.0

package post

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RevisionKind distinguishes an update snapshot from a per-author autosave.
type RevisionKind string

// The kinds of revision a post carries.
const (
	RevisionKindRevision RevisionKind = "revision"
	RevisionKindAutosave RevisionKind = "autosave"
)

// ErrRevisionNotFound reports that no revision exists for the requested ID.
var ErrRevisionNotFound = errors.New("post: revision not found")

// ErrConflict reports that the post changed after the update was prepared.
var ErrConflict = errors.New("post: conflicting update")

// Revision is a snapshot of a post's editable content.
type Revision struct {
	ID        uuid.UUID
	PostID    uuid.UUID
	Kind      RevisionKind
	AuthorID  uuid.UUID
	Title     string
	Content   string
	Excerpt   string
	CreatedAt time.Time
}

// NewRevision returns a snapshot of the post's editable content, credited to the given author.
func NewRevision(p Post, kind RevisionKind, authorID uuid.UUID) (Revision, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Revision{}, fmt.Errorf("post: generate revision id: %w", err)
	}
	return Revision{
		ID:        id,
		PostID:    p.ID,
		Kind:      kind,
		AuthorID:  authorID,
		Title:     p.Title,
		Content:   p.Content,
		Excerpt:   p.Excerpt,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// ErrInvalidOrderBy reports that a sort column is not one the CMS sorts by.
var ErrInvalidOrderBy = errors.New("post: invalid orderby")

// ErrInvalidOrder reports that a sort direction is not one the CMS sorts in.
var ErrInvalidOrder = errors.New("post: invalid order")

// OrderBy names a column a listing can be sorted by.
type OrderBy string

// The columns a listing can be sorted by.
const (
	OrderByDate  OrderBy = "date"
	OrderByTitle OrderBy = "title"
)

// Order names a sort direction.
type Order string

// The directions a listing can be sorted in.
const (
	OrderAsc  Order = "asc"
	OrderDesc Order = "desc"
)

// ParseOrderBy returns the column named by raw, or [ErrInvalidOrderBy].
func ParseOrderBy(raw string) (OrderBy, error) {
	switch OrderBy(raw) {
	case OrderByDate, OrderByTitle:
		return OrderBy(raw), nil
	default:
		return "", ErrInvalidOrderBy
	}
}

// ParseOrder returns the direction named by raw, or [ErrInvalidOrder].
func ParseOrder(raw string) (Order, error) {
	switch Order(raw) {
	case OrderAsc, OrderDesc:
		return Order(raw), nil
	default:
		return "", ErrInvalidOrder
	}
}

// Filter narrows a post listing.
type Filter struct {
	Type    string
	Status  Status
	Search  string
	OrderBy OrderBy
	Order   Order
	Page    int
	PerPage int
}

// Store persists posts and their revisions.
type Store interface {
	Create(ctx context.Context, p Post) (Post, error)
	ByID(ctx context.Context, id uuid.UUID) (Post, error)
	PublishedBySlug(ctx context.Context, postType, slug string) (Post, error)
	List(ctx context.Context, f Filter) ([]Post, int, error)
	Update(ctx context.Context, p Post, expectedUpdatedAt time.Time, snapshot *Revision, revisionCap int) (Post, error)
	Trash(ctx context.Context, id uuid.UUID, updatedAt time.Time) (Post, error)
	Restore(ctx context.Context, id uuid.UUID, updatedAt time.Time) (Post, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Counts(ctx context.Context, postType string) (map[Status]int, error)
	Revisions(ctx context.Context, postID uuid.UUID) ([]Revision, error)
	RevisionByID(ctx context.Context, postID, revisionID uuid.UUID) (Revision, error)
	DeleteRevision(ctx context.Context, postID, revisionID uuid.UUID) error
	SaveAutosave(ctx context.Context, autosave Revision) (Revision, error)
	Autosave(ctx context.Context, postID, authorID uuid.UUID) (Revision, error)
	DeleteAutosave(ctx context.Context, postID, authorID uuid.UUID) error
}
