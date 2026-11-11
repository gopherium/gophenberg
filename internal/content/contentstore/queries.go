// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// Queries sends every statement the content store is built from, one call per statement.
type Queries interface {
	ItemQueries
	ListQueries
	LockQueries
	RevisionQueries
	RelationQueries
	DefinitionQueries
}

// ItemQueries sends the statements that read and write one content item.
type ItemQueries interface {
	CreateContent(ctx context.Context, c ContentRow) (ContentRow, error)
	GetContent(ctx context.Context, id uuid.UUID) (ContentRow, error)
	GetPublishedContentByPath(ctx context.Context, path string) (ContentRow, error)
	UpdateContent(ctx context.Context, c ContentRow, expected time.Time) (ContentRow, error)
	SiblingSlugTaken(ctx context.Context, s SiblingSlug) (bool, error)
	CountChildren(ctx context.Context, id uuid.UUID) (int64, error)
	ContentDepth(ctx context.Context, id uuid.UUID) (int64, error)
	ValuesOfContent(ctx context.Context, id uuid.UUID) (content.Values, error)
	TrashContent(ctx context.Context, id uuid.UUID, suffix string, at time.Time) (ContentRow, error)
	RestoreContent(ctx context.Context, id uuid.UUID, at time.Time) (ContentRow, error)
	RestoreContentKeepingSlug(ctx context.Context, id uuid.UUID, at time.Time) (ContentRow, error)
	DeleteContent(ctx context.Context, id uuid.UUID) (int64, error)
	DeleteTrash(ctx context.Context, typeKey string, author *uuid.UUID) (int64, error)
	CountTrash(ctx context.Context, typeKey string) (int64, error)
	CountContentByAuthor(ctx context.Context, typeKey string) ([]AuthorCount, error)
}

// ListQueries sends the statements that count and page a narrowed listing.
type ListQueries interface {
	CountContent(ctx context.Context, n Narrowing) (int64, error)
	ListContent(ctx context.Context, p Page) ([]ListedRow, error)
	ListNestedContent(ctx context.Context, p Page) ([]ListedRow, error)
}

// LockQueries sends the statements that hold rows, walk the tree and settle addresses.
type LockQueries interface {
	LockContent(ctx context.Context, id uuid.UUID) (ContentRow, error)
	LockTypeNesting(ctx context.Context, typeKey string) (bool, error)
	LockTypeForMove(ctx context.Context, typeKey string) (bool, error)
	LockAncestors(ctx context.Context, id uuid.UUID) error
	LockParent(ctx context.Context, id, child uuid.UUID) (ParentRow, error)
	LockDescendants(ctx context.Context, id uuid.UUID) error
	MoveDescendants(ctx context.Context, id uuid.UUID, path string, at time.Time) ([]string, error)
	LockDeclaredFieldKeys(ctx context.Context) ([]string, error)
	DeferAddressCheck(ctx context.Context) error
}

// RevisionQueries sends the statements that read and write revisions and autosaves.
type RevisionQueries interface {
	CreateRevision(ctx context.Context, r RevisionRow) error
	ListRevisions(ctx context.Context, contentID uuid.UUID) ([]RevisionRow, error)
	GetRevision(ctx context.Context, contentID, id uuid.UUID) (RevisionRow, error)
	DeleteRevision(ctx context.Context, contentID, id uuid.UUID) (int64, error)
	PruneRevisions(ctx context.Context, contentID uuid.UUID, keep int) error
	UpsertAutosave(ctx context.Context, r RevisionRow) (RevisionRow, error)
	GetAutosave(ctx context.Context, contentID, authorID uuid.UUID) (RevisionRow, error)
	DeleteAutosave(ctx context.Context, contentID, authorID uuid.UUID) error
}

// RelationQueries sends the statements that check, index and read what relation fields point at.
type RelationQueries interface {
	TypesOfContent(ctx context.Context, ids []uuid.UUID) ([]TypedItem, error)
	ClearRelationsOfField(ctx context.Context, from uuid.UUID, fieldID int) error
	AddRelation(ctx context.Context, r RelationRow) error
	RefreshRelationVisibility(ctx context.Context, id uuid.UUID) error
	CountRelatedContent(ctx context.Context, target uuid.UUID) (int64, error)
	ListRelatedContent(ctx context.Context, target uuid.UUID, limit, offset int) ([]ContentRow, error)
	CountPointingAt(ctx context.Context, target uuid.UUID, fieldID int) (int64, error)
	PointingAt(ctx context.Context, p Pointing) ([]content.Pointer, error)
	SummariesOfTargets(ctx context.Context, ids []uuid.UUID) ([]content.Target, error)
}

// DefinitionQueries sends the field definition reads a content write checks its values against.
type DefinitionQueries interface {
	ListFieldGroups(ctx context.Context) ([]typestore.GroupRow, error)
	ListContentFields(ctx context.Context) ([]typestore.FieldRow, error)
}

// ContentRow is one stored content item as every engine reads and writes it.
type ContentRow struct {
	ID, AuthorID                                      uuid.UUID
	ParentID                                          *uuid.UUID
	Type, Status, Slug, Title, Content, Excerpt, Path string
	Fields                                            content.Values
	PublishedAt                                       *time.Time
	CreatedAt, UpdatedAt                              time.Time
}

// ListedRow is one listed item beside its author's name and its parent's title.
type ListedRow struct {
	ContentRow
	AuthorName, ParentTitle string
}

// RevisionRow is one stored revision or autosave.
type RevisionRow struct {
	ID, ContentID, AuthorID       uuid.UUID
	Kind, Title, Content, Excerpt string
	Fields                        content.Values
	CreatedAt                     time.Time
}

// ParentRow is the parent an item files under, and whether the item already stands above it.
type ParentRow struct {
	Status, Path string
	HoldsChild   bool
}

// SiblingSlug names the slug an item would carry beside its siblings.
type SiblingSlug struct {
	ID         uuid.UUID
	ParentID   *uuid.UUID
	Type, Slug string
}

// Narrowing holds what the list and count statements narrow by, its field terms already encoded as JSON.
type Narrowing struct {
	Type, Search             string
	FieldFilter              []byte
	Statuses                 []string
	Authors, ExcludedAuthors []uuid.UUID
	Before, After            *time.Time
}

// Page is one page of a narrowed listing in the order asked.
type Page struct {
	Narrowing
	OrderBy, OrderDir string
	Limit, Offset     int
}

// Pointing is one page of the items pointing at a target through a field.
type Pointing struct {
	Target                 uuid.UUID
	FieldID, Limit, Offset int
}

// TypedItem is the type one item belongs to.
type TypedItem struct {
	ID   uuid.UUID
	Type string
}

// AuthorCount is how many listed items one author wrote.
type AuthorCount struct {
	AuthorID uuid.UUID
	Total    int64
}

// RelationRow is one row of the index a relation field writes.
type RelationRow struct {
	FromID, ToID      uuid.UUID
	FieldID, Position int
}
