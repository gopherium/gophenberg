// SPDX-License-Identifier: Apache-2.0

package content

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RevisionKind distinguishes an update snapshot from a per-author autosave.
type RevisionKind string

// The kinds of revision a content item carries.
const (
	RevisionKindRevision RevisionKind = "revision"
	RevisionKindAutosave RevisionKind = "autosave"
)

// ErrRevisionNotFound reports that no revision exists for the requested ID.
var ErrRevisionNotFound = errors.New("content: revision not found")

// ErrConflict reports that the content item changed after the update was prepared.
var ErrConflict = errors.New("content: conflicting update")

// Revision is a snapshot of a content item's editable content.
type Revision struct {
	ID        uuid.UUID
	ContentID uuid.UUID
	Kind      RevisionKind
	AuthorID  uuid.UUID
	Title     string
	Content   string
	Excerpt   string
	CreatedAt time.Time
}

// NewRevision returns a snapshot of the item's editable content, credited to the given author.
func NewRevision(c Content, kind RevisionKind, authorID uuid.UUID) (Revision, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return Revision{}, fmt.Errorf("content: generate revision id: %w", err)
	}
	return Revision{
		ID:        id,
		ContentID: c.ID,
		Kind:      kind,
		AuthorID:  authorID,
		Title:     c.Title,
		Content:   c.Content,
		Excerpt:   c.Excerpt,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// ErrInvalidOrderBy reports that a sort column is not one the CMS sorts by.
var ErrInvalidOrderBy = errors.New("content: invalid orderby")

// ErrInvalidOrder reports that a sort direction is not one the CMS sorts in.
var ErrInvalidOrder = errors.New("content: invalid order")

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

// Filter narrows a content listing.
type Filter struct {
	Type    string
	Status  Status
	Search  string
	OrderBy OrderBy
	Order   Order
	Page    int
	PerPage int
}

// Store persists content items and their revisions.
type Store interface {
	Create(ctx context.Context, c Content) (Content, error)
	ByID(ctx context.Context, id uuid.UUID) (Content, error)
	PublishedBySlug(ctx context.Context, contentType, slug string) (Content, error)
	PublishedByPath(ctx context.Context, path string) (Content, error)
	Children(ctx context.Context, id uuid.UUID) (int, error)
	List(ctx context.Context, f Filter) ([]Content, int, error)
	Update(
		ctx context.Context, c Content, expectedUpdatedAt time.Time, snapshot *Revision, revisionCap int,
	) (Content, error)
	Trash(ctx context.Context, id uuid.UUID, updatedAt time.Time) (Content, error)
	Restore(ctx context.Context, id uuid.UUID, updatedAt time.Time) (Content, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Counts(ctx context.Context, contentType string) (map[Status]int, error)
	Revisions(ctx context.Context, contentID uuid.UUID) ([]Revision, error)
	RevisionByID(ctx context.Context, contentID, revisionID uuid.UUID) (Revision, error)
	DeleteRevision(ctx context.Context, contentID, revisionID uuid.UUID) error
	SaveAutosave(ctx context.Context, autosave Revision) (Revision, error)
	Autosave(ctx context.Context, contentID, authorID uuid.UUID) (Revision, error)
	DeleteAutosave(ctx context.Context, contentID, authorID uuid.UUID) error
}
