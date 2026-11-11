// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contentstore"
	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ content.Store = (*ContentStore)(nil)

// ContentStore persists content items in the core schema.
type ContentStore struct {
	*contentstore.Store
}

// NewContentStore returns a [ContentStore] backed by pool.
func NewContentStore(pool *pgxpool.Pool) *ContentStore {
	queries := db.New(pool)
	return &ContentStore{Store: contentstore.New(contentstore.Backend{
		Engine:    dbkit.Postgres,
		Pool:      contentQueries{typeQueries{conn: pool, queries: queries}},
		InTx:      contentInTx(pool, queries),
		Rules:     contentRulesBroken,
		ReadError: readError,
	})}
}

// contentRulesBroken lists the stored rule each Postgres failure of a content write stands for, in match order.
var contentRulesBroken = []typestore.Rule{
	{Name: slugConstraint, Broken: typestore.ContentPathUnique},
	{Code: foreignKeyViolationCode, Name: relationFromConstraint, Broken: typestore.RelationTargetExists},
	{Code: foreignKeyViolationCode, Name: relationFieldConstraint, Broken: typestore.RelationTargetExists},
	{Code: foreignKeyViolationCode, Name: relationTargetConstraint, Broken: typestore.RelationTargetExists},
	{Code: deadlockCode, Context: slugConstraint, Broken: typestore.AddressCheckDeadlocked},
}

// contentQueries sends the content store's statements through sqlc, beside the type store's shared ones.
type contentQueries struct {
	typeQueries
}

// contentTx sends the statements of one content write and opens savepoints on it.
type contentTx struct {
	contentQueries
	tx pgx.Tx
}

// contentInTx returns the transactor that runs each content write inside one pgx transaction on the pool.
func contentInTx(pool *pgxpool.Pool, queries *db.Queries) contentstore.Transactor {
	return func(ctx context.Context, write func(contentstore.Tx) error) error {
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			return write(contentTx{contentQueries: boundTo(tx, queries), tx: tx})
		})
	}
}

// boundTo returns the content statements sent on one transaction or savepoint.
func boundTo(tx pgx.Tx, queries *db.Queries) contentQueries {
	return contentQueries{typeQueries{conn: tx, queries: queries.WithTx(tx)}}
}

// Savepoint runs attempt on a savepoint of the transaction, rolling back to it when attempt fails.
func (q contentTx) Savepoint(ctx context.Context, attempt func(contentstore.Queries) error) error {
	return pgx.BeginFunc(ctx, q.tx, func(savepoint pgx.Tx) error {
		return attempt(boundTo(savepoint, q.queries))
	})
}

// CreateContent stores a new item and returns it as stored.
func (q contentQueries) CreateContent(ctx context.Context, c contentstore.ContentRow) (contentstore.ContentRow, error) {
	row, err := q.queries.CreateContent(ctx, createParams(c))
	return contentRow(row), err
}

// GetContent returns the stored item the identity names.
func (q contentQueries) GetContent(ctx context.Context, id uuid.UUID) (contentstore.ContentRow, error) {
	row, err := q.queries.GetContent(ctx, id)
	return contentRow(row), err
}

// GetPublishedContentByPath returns the published item at the address.
func (q contentQueries) GetPublishedContentByPath(ctx context.Context, path string) (contentstore.ContentRow, error) {
	row, err := q.queries.GetPublishedContentByPath(ctx, path)
	return contentRow(row), err
}

// UpdateContent stores the edited item while its stamp holds and returns it as stored.
func (q contentQueries) UpdateContent(
	ctx context.Context, c contentstore.ContentRow, expected time.Time,
) (contentstore.ContentRow, error) {
	row, err := q.queries.UpdateContent(ctx, updateParams(c, expected))
	return contentRow(row), err
}

// SiblingSlugTaken reports whether another item of the type under the same parent carries the slug.
func (q contentQueries) SiblingSlugTaken(ctx context.Context, s contentstore.SiblingSlug) (bool, error) {
	return q.queries.SiblingSlugTaken(ctx, db.SiblingSlugTakenParams{
		Type: s.Type, ParentID: s.ParentID, Slug: s.Slug, ID: s.ID,
	})
}

// CountChildren returns how many items file directly under the item.
func (q contentQueries) CountChildren(ctx context.Context, id uuid.UUID) (int64, error) {
	return q.queries.CountChildren(ctx, &id)
}

// ContentDepth returns how many levels of content nest below the item.
func (q contentQueries) ContentDepth(ctx context.Context, id uuid.UUID) (int64, error) {
	levels, err := q.queries.ContentDepth(ctx, id)
	return int64(levels), err
}

// ValuesOfContent returns the values the stored item holds.
func (q contentQueries) ValuesOfContent(ctx context.Context, id uuid.UUID) (content.Values, error) {
	return q.queries.ValuesOfContent(ctx, id)
}

// TrashContent marks the item trashed under the suffix and returns it as stored.
func (q contentQueries) TrashContent(
	ctx context.Context, id uuid.UUID, suffix string, at time.Time,
) (contentstore.ContentRow, error) {
	row, err := q.queries.TrashContent(ctx, db.TrashContentParams{ID: id, Suffix: suffix, UpdatedAt: at})
	return contentRow(row), err
}

// RestoreContent returns the item to draft under the address its trash suffix hid, and returns it as stored.
func (q contentQueries) RestoreContent(
	ctx context.Context, id uuid.UUID, at time.Time,
) (contentstore.ContentRow, error) {
	row, err := q.queries.RestoreContent(ctx, db.RestoreContentParams{ID: id, UpdatedAt: at})
	return contentRow(row), err
}

// RestoreContentKeepingSlug returns the item to draft under its trashed address, and returns it as stored.
func (q contentQueries) RestoreContentKeepingSlug(
	ctx context.Context, id uuid.UUID, at time.Time,
) (contentstore.ContentRow, error) {
	row, err := q.queries.RestoreContentKeepingSlug(ctx, db.RestoreContentKeepingSlugParams{ID: id, UpdatedAt: at})
	return contentRow(row), err
}

// DeleteContent removes the item and returns how many rows it removed.
func (q contentQueries) DeleteContent(ctx context.Context, id uuid.UUID) (int64, error) {
	return q.queries.DeleteContent(ctx, id)
}

// DeleteTrash removes the type's trashed items, only the author's when one is named, and returns how many.
func (q contentQueries) DeleteTrash(ctx context.Context, typeKey string, author *uuid.UUID) (int64, error) {
	return q.queries.DeleteTrash(ctx, db.DeleteTrashParams{Type: typeKey, AuthorID: author})
}

// CountTrash returns how many trashed items of the type are stored.
func (q contentQueries) CountTrash(ctx context.Context, typeKey string) (int64, error) {
	return q.queries.CountTrash(ctx, typeKey)
}

// CountContentByAuthor returns how many published and private items of the type each author wrote.
func (q contentQueries) CountContentByAuthor(ctx context.Context, typeKey string) ([]contentstore.AuthorCount, error) {
	rows, err := q.queries.CountContentByAuthor(ctx, typeKey)
	return mapped(rows, authorCount), err
}

// CountContent returns how many items the narrowing keeps.
func (q contentQueries) CountContent(ctx context.Context, n contentstore.Narrowing) (int64, error) {
	return q.queries.CountContent(ctx, countParams(n))
}

// ListContent returns the page of the narrowed listing in the order asked.
func (q contentQueries) ListContent(ctx context.Context, p contentstore.Page) ([]contentstore.ListedRow, error) {
	rows, err := q.queries.ListContent(ctx, listParams(p))
	return mapped(rows, listedRow), err
}

// ListNestedContent returns the page of the narrowed listing with each child under its parent.
func (q contentQueries) ListNestedContent(ctx context.Context, p contentstore.Page) ([]contentstore.ListedRow, error) {
	rows, err := q.queries.ListNestedContent(ctx, nestedParams(p))
	return mapped(rows, nestedRow), err
}

// LockContent returns the stored item, locked until the transaction ends.
func (q contentQueries) LockContent(ctx context.Context, id uuid.UUID) (contentstore.ContentRow, error) {
	row, err := q.queries.LockContent(ctx, id)
	return contentRow(row), err
}

// LockTypeNesting returns whether the type nests, holding it as it stands.
func (q contentQueries) LockTypeNesting(ctx context.Context, typeKey string) (bool, error) {
	return q.queries.LockTypeNesting(ctx, typeKey)
}

// LockTypeForMove returns whether the type nests, holding it against every other move.
func (q contentQueries) LockTypeForMove(ctx context.Context, typeKey string) (bool, error) {
	return q.queries.LockTypeForMove(ctx, typeKey)
}

// LockAncestors holds the item and every item above it.
func (q contentQueries) LockAncestors(ctx context.Context, id uuid.UUID) error {
	return q.queries.LockAncestors(ctx, id)
}

// LockParent returns the parent's state and address, held against a move, and whether the child stands above it.
func (q contentQueries) LockParent(ctx context.Context, id, child uuid.UUID) (contentstore.ParentRow, error) {
	row, err := q.queries.LockParent(ctx, db.LockParentParams{ID: id, ChildID: child})
	return contentstore.ParentRow(row), err
}

// LockDescendants holds every item below the item.
func (q contentQueries) LockDescendants(ctx context.Context, id uuid.UUID) error {
	return q.queries.LockDescendants(ctx, id)
}

// MoveDescendants carries the item's descendants under the address and returns their new addresses under page.
func (q contentQueries) MoveDescendants(
	ctx context.Context, id uuid.UUID, path string, at time.Time,
) ([]string, error) {
	return q.queries.MoveDescendants(ctx, db.MoveDescendantsParams{ID: id, Path: path, UpdatedAt: at})
}

// LockDeclaredFieldKeys returns the key of every stored field, held until the transaction ends.
func (q contentQueries) LockDeclaredFieldKeys(ctx context.Context) ([]string, error) {
	return q.queries.LockDeclaredFieldKeys(ctx)
}

// CreateRevision stores the revision.
func (q contentQueries) CreateRevision(ctx context.Context, r contentstore.RevisionRow) error {
	return q.queries.CreateRevision(ctx, revisionParams(r))
}

// ListRevisions returns the item's revisions newest first, without their content or values.
func (q contentQueries) ListRevisions(ctx context.Context, contentID uuid.UUID) ([]contentstore.RevisionRow, error) {
	rows, err := q.queries.ListRevisions(ctx, contentID)
	return mapped(rows, listedRevision), err
}

// GetRevision returns the item's revision the identity names.
func (q contentQueries) GetRevision(ctx context.Context, contentID, id uuid.UUID) (contentstore.RevisionRow, error) {
	row, err := q.queries.GetRevision(ctx, db.GetRevisionParams{ContentID: contentID, ID: id})
	return revisionRow(row), err
}

// DeleteRevision removes the item's revision and returns how many rows it removed.
func (q contentQueries) DeleteRevision(ctx context.Context, contentID, id uuid.UUID) (int64, error) {
	return q.queries.DeleteRevision(ctx, db.DeleteRevisionParams{ContentID: contentID, ID: id})
}

// PruneRevisions removes the item's revisions past the newest it keeps.
func (q contentQueries) PruneRevisions(ctx context.Context, contentID uuid.UUID, keep int) error {
	return q.queries.PruneRevisions(ctx, db.PruneRevisionsParams{ContentID: contentID, Keep: int32(keep)})
}

// UpsertAutosave stores the author's autosave, replacing any earlier one, and returns it as stored.
func (q contentQueries) UpsertAutosave(
	ctx context.Context, r contentstore.RevisionRow,
) (contentstore.RevisionRow, error) {
	row, err := q.queries.UpsertAutosave(ctx, autosaveParams(r))
	return revisionRow(db.GetRevisionRow(row)), err
}

// GetAutosave returns the author's autosave of the item.
func (q contentQueries) GetAutosave(
	ctx context.Context, contentID, authorID uuid.UUID,
) (contentstore.RevisionRow, error) {
	row, err := q.queries.GetAutosave(ctx, db.GetAutosaveParams{ContentID: contentID, AuthorID: authorID})
	return revisionRow(db.GetRevisionRow(row)), err
}

// DeleteAutosave removes the author's autosave of the item.
func (q contentQueries) DeleteAutosave(ctx context.Context, contentID, authorID uuid.UUID) error {
	return q.queries.DeleteAutosave(ctx, db.DeleteAutosaveParams{ContentID: contentID, AuthorID: authorID})
}

// TypesOfContent returns the type of each stored item the identities name.
func (q contentQueries) TypesOfContent(ctx context.Context, ids []uuid.UUID) ([]contentstore.TypedItem, error) {
	rows, err := q.queries.TypesOfContent(ctx, ids)
	return mapped(rows, typedItem), err
}

// ClearRelationsOfField removes the index rows the item holds for the relation field.
func (q contentQueries) ClearRelationsOfField(ctx context.Context, from uuid.UUID, fieldID int) error {
	return q.queries.ClearRelationsOfField(ctx, db.ClearRelationsOfFieldParams{FromID: from, FieldID: int32(fieldID)})
}

// AddRelation stores one index row of a relation field.
func (q contentQueries) AddRelation(ctx context.Context, r contentstore.RelationRow) error {
	return q.queries.AddRelation(ctx, db.AddRelationParams{
		FromID: r.FromID, FieldID: int32(r.FieldID), ToID: r.ToID, Position: int32(r.Position),
	})
}

// RefreshRelationVisibility refreshes what a term page reads of the index rows pointing from or at the item.
func (q contentQueries) RefreshRelationVisibility(ctx context.Context, id uuid.UUID) error {
	return q.queries.RefreshRelationVisibility(ctx, id)
}

// CountRelatedContent returns how many published items of active types point at the target.
func (q contentQueries) CountRelatedContent(ctx context.Context, target uuid.UUID) (int64, error) {
	return q.queries.CountRelatedContent(ctx, target)
}

// ListRelatedContent returns one page of the published items of active types pointing at the target.
func (q contentQueries) ListRelatedContent(
	ctx context.Context, target uuid.UUID, limit, offset int,
) ([]contentstore.ContentRow, error) {
	rows, err := q.queries.ListRelatedContent(ctx, db.ListRelatedContentParams{
		Target: target, RowLimit: rowLimit(limit), RowOffset: int32(offset),
	})
	return mapped(rows, relatedRow), err
}

// CountPointingAt returns how many published items point at the target through the field.
func (q contentQueries) CountPointingAt(ctx context.Context, target uuid.UUID, fieldID int) (int64, error) {
	return q.queries.CountPointingAt(ctx, db.CountPointingAtParams{Target: target, Field: int32(fieldID)})
}

// PointingAt returns one page of the published items pointing at the target through the field.
func (q contentQueries) PointingAt(ctx context.Context, p contentstore.Pointing) ([]content.Pointer, error) {
	rows, err := q.queries.PointingAt(ctx, db.PointingAtParams{
		Target: p.Target, Field: int32(p.FieldID), RowLimit: rowLimit(p.Limit), RowOffset: int32(p.Offset),
	})
	return mapped(rows, pointer), err
}

// SummariesOfTargets returns the published items of active types the identities name.
func (q contentQueries) SummariesOfTargets(ctx context.Context, ids []uuid.UUID) ([]content.Target, error) {
	rows, err := q.queries.SummariesOfTargets(ctx, ids)
	return mapped(rows, target), err
}
