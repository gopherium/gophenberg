// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contentstore"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// contentQueries sends the content store's statements through sqlc, beside the type store's shared ones.
type contentQueries struct {
	typeQueries
}

// marked returns the value a statement answered beside its failure, marked with its dbkit class.
func marked[T any](value T, err error) (T, error) {
	return value, dbsqlite.Classify(err)
}

// CreateContent stores a new item and returns it as stored.
func (q contentQueries) CreateContent(ctx context.Context, c contentstore.ContentRow) (contentstore.ContentRow, error) {
	row, err := q.queries.CreateContent(ctx, createParams(c))
	return contentRow(row), dbsqlite.Classify(err)
}

// GetContent returns the stored item the identity names.
func (q contentQueries) GetContent(ctx context.Context, id uuid.UUID) (contentstore.ContentRow, error) {
	row, err := q.queries.GetContent(ctx, id)
	return contentRow(row), dbsqlite.Classify(err)
}

// GetPublishedContentByPath returns the published item at the address.
func (q contentQueries) GetPublishedContentByPath(ctx context.Context, path string) (contentstore.ContentRow, error) {
	row, err := q.queries.GetPublishedContentByPath(ctx, path)
	return contentRow(row), dbsqlite.Classify(err)
}

// UpdateContent stores the edited item while its stamp holds and returns it as stored.
func (q contentQueries) UpdateContent(
	ctx context.Context, c contentstore.ContentRow, expected time.Time,
) (contentstore.ContentRow, error) {
	row, err := q.queries.UpdateContent(ctx, updateParams(c, expected))
	return contentRow(row), dbsqlite.Classify(err)
}

// SiblingSlugTaken reports whether another item of the type under the same parent carries the slug.
func (q contentQueries) SiblingSlugTaken(ctx context.Context, s contentstore.SiblingSlug) (bool, error) {
	return marked(q.queries.SiblingSlugTaken(ctx, db.SiblingSlugTakenParams{
		Type: s.Type, ParentID: s.ParentID, Slug: s.Slug, ID: s.ID,
	}))
}

// CountChildren returns how many items file directly under the item.
func (q contentQueries) CountChildren(ctx context.Context, id uuid.UUID) (int64, error) {
	return marked(q.queries.CountChildren(ctx, &id))
}

// ContentDepth returns how many levels of content nest below the item.
func (q contentQueries) ContentDepth(ctx context.Context, id uuid.UUID) (int64, error) {
	return marked(q.queries.ContentDepth(ctx, id))
}

// ValuesOfContent returns the values the stored item holds.
func (q contentQueries) ValuesOfContent(ctx context.Context, id uuid.UUID) (content.Values, error) {
	values, err := q.queries.ValuesOfContent(ctx, id)
	return content.Values(values), dbsqlite.Classify(err)
}

// TrashContent marks the item trashed under the suffix and returns it as stored.
func (q contentQueries) TrashContent(
	ctx context.Context, id uuid.UUID, suffix string, at time.Time,
) (contentstore.ContentRow, error) {
	row, err := q.queries.TrashContent(ctx, db.TrashContentParams{Suffix: suffix, UpdatedAt: dbkit.Time{Time: at}, ID: id})
	return contentRow(row), dbsqlite.Classify(err)
}

// RestoreContent returns the item to draft under the address its trash suffix hid, and returns it as stored.
func (q contentQueries) RestoreContent(
	ctx context.Context, id uuid.UUID, at time.Time,
) (contentstore.ContentRow, error) {
	row, err := q.queries.RestoreContent(ctx, db.RestoreContentParams{UpdatedAt: dbkit.Time{Time: at}, ID: id})
	return contentRow(row), dbsqlite.Classify(err)
}

// RestoreContentKeepingSlug returns the item to draft under its trashed address, and returns it as stored.
func (q contentQueries) RestoreContentKeepingSlug(
	ctx context.Context, id uuid.UUID, at time.Time,
) (contentstore.ContentRow, error) {
	row, err := q.queries.RestoreContentKeepingSlug(ctx, db.RestoreContentKeepingSlugParams{
		UpdatedAt: dbkit.Time{Time: at}, ID: id,
	})
	return contentRow(row), dbsqlite.Classify(err)
}

// DeleteContent removes the item and returns how many rows it removed.
func (q contentQueries) DeleteContent(ctx context.Context, id uuid.UUID) (int64, error) {
	return marked(q.queries.DeleteContent(ctx, id))
}

// DeleteTrash removes the type's trashed items, only the author's when one is named, and returns how many.
func (q contentQueries) DeleteTrash(ctx context.Context, typeKey string, author *uuid.UUID) (int64, error) {
	return marked(q.queries.DeleteTrash(ctx, db.DeleteTrashParams{Type: typeKey, AuthorID: authorColumn(author)}))
}

// CountTrash returns how many trashed items of the type are stored.
func (q contentQueries) CountTrash(ctx context.Context, typeKey string) (int64, error) {
	return marked(q.queries.CountTrash(ctx, typeKey))
}

// CountContentByAuthor returns how many published and private items of the type each author wrote.
func (q contentQueries) CountContentByAuthor(ctx context.Context, typeKey string) ([]contentstore.AuthorCount, error) {
	rows, err := q.queries.CountContentByAuthor(ctx, typeKey)
	return mapped(rows, authorCount), dbsqlite.Classify(err)
}

// CountContent returns how many items the narrowing keeps.
func (q contentQueries) CountContent(ctx context.Context, n contentstore.Narrowing) (int64, error) {
	return marked(q.queries.CountContent(ctx, countParams(n)))
}

// ListContent returns the page of the narrowed listing in the order asked.
func (q contentQueries) ListContent(ctx context.Context, p contentstore.Page) ([]contentstore.ListedRow, error) {
	rows, err := q.queries.ListContent(ctx, listParams(p))
	return mapped(rows, listedRow), dbsqlite.Classify(err)
}

// ListNestedContent returns the page of the narrowed listing with each child under its parent.
func (q contentQueries) ListNestedContent(ctx context.Context, p contentstore.Page) ([]contentstore.ListedRow, error) {
	rows, err := q.queries.ListNestedContent(ctx, nestedParams(p))
	return mapped(rows, nestedRow), dbsqlite.Classify(err)
}

// LockContent returns the stored item, held by the one writer.
func (q contentQueries) LockContent(ctx context.Context, id uuid.UUID) (contentstore.ContentRow, error) {
	row, err := q.queries.LockContent(ctx, id)
	return contentRow(row), dbsqlite.Classify(err)
}

// LockTypeNesting returns whether the type nests, held as it stands by the one writer.
func (q contentQueries) LockTypeNesting(ctx context.Context, typeKey string) (bool, error) {
	return marked(q.queries.LockTypeNesting(ctx, typeKey))
}

// LockTypeForMove returns whether the type nests, held against every other move by the one writer.
func (q contentQueries) LockTypeForMove(ctx context.Context, typeKey string) (bool, error) {
	return marked(q.queries.LockTypeForMove(ctx, typeKey))
}

// LockTypeRouteWord returns the route word the type answers under, held as it stands by the one writer.
func (q contentQueries) LockTypeRouteWord(ctx context.Context, typeKey string) (string, error) {
	return marked(q.queries.LockTypeRouteWord(ctx, typeKey))
}

// LockAncestors sends nothing, leaving the item and every item above it held by the one writer.
func (contentQueries) LockAncestors(context.Context, uuid.UUID) error {
	return nil
}

// LockParent returns the parent's state and address, held by the one writer, and whether the child stands above it.
func (q contentQueries) LockParent(ctx context.Context, id, child uuid.UUID) (contentstore.ParentRow, error) {
	row, err := q.queries.LockParent(ctx, db.LockParentParams{ChildID: child.String(), ID: id})
	return contentstore.ParentRow(row), dbsqlite.Classify(err)
}

// LockDescendants sends nothing, leaving the descendants held by the one writer.
func (contentQueries) LockDescendants(context.Context, uuid.UUID) error {
	return nil
}

// MoveDescendants carries the item's descendants under the address and returns their new addresses under page.
func (q contentQueries) MoveDescendants(
	ctx context.Context, id uuid.UUID, path string, at time.Time,
) ([]string, error) {
	rows, err := q.queries.DescendantsToMove(ctx, db.DescendantsToMoveParams{Path: path, ID: id})
	if err != nil || len(rows) == 0 {
		return nil, dbsqlite.Classify(err)
	}
	moves, reported := movesOf(rows)
	err = q.queries.MoveDescendants(ctx, db.MoveDescendantsParams{Moves: moves, UpdatedAt: dbkit.Time{Time: at}})
	return reported, dbsqlite.Classify(err)
}

// LockDeclaredFieldKeys returns the key of every stored field, held by the one writer.
func (q contentQueries) LockDeclaredFieldKeys(ctx context.Context) ([]string, error) {
	return marked(q.queries.LockDeclaredFieldKeys(ctx))
}

// CreateRevision stores the revision.
func (q contentQueries) CreateRevision(ctx context.Context, r contentstore.RevisionRow) error {
	return dbsqlite.Classify(q.queries.CreateRevision(ctx, revisionParams(r)))
}

// ListRevisions returns the item's revisions newest first, without their content or values.
func (q contentQueries) ListRevisions(ctx context.Context, contentID uuid.UUID) ([]contentstore.RevisionRow, error) {
	rows, err := q.queries.ListRevisions(ctx, contentID)
	return mapped(rows, listedRevision), dbsqlite.Classify(err)
}

// GetRevision returns the item's revision the identity names.
func (q contentQueries) GetRevision(ctx context.Context, contentID, id uuid.UUID) (contentstore.RevisionRow, error) {
	row, err := q.queries.GetRevision(ctx, db.GetRevisionParams{ContentID: contentID, ID: id})
	return revisionRow(row), dbsqlite.Classify(err)
}

// DeleteRevision removes the item's revision and returns how many rows it removed.
func (q contentQueries) DeleteRevision(ctx context.Context, contentID, id uuid.UUID) (int64, error) {
	return marked(q.queries.DeleteRevision(ctx, db.DeleteRevisionParams{ContentID: contentID, ID: id}))
}

// PruneRevisions removes the item's revisions past the newest it keeps.
func (q contentQueries) PruneRevisions(ctx context.Context, contentID uuid.UUID, keep int) error {
	return dbsqlite.Classify(q.queries.PruneRevisions(ctx, db.PruneRevisionsParams{
		ContentID: contentID, Keep: int64(keep),
	}))
}

// UpsertAutosave stores the author's autosave, replacing any earlier one, and returns it as stored.
func (q contentQueries) UpsertAutosave(
	ctx context.Context, r contentstore.RevisionRow,
) (contentstore.RevisionRow, error) {
	row, err := q.queries.UpsertAutosave(ctx, autosaveParams(r))
	return revisionRow(db.GetRevisionRow(row)), dbsqlite.Classify(err)
}

// GetAutosave returns the author's autosave of the item.
func (q contentQueries) GetAutosave(
	ctx context.Context, contentID, authorID uuid.UUID,
) (contentstore.RevisionRow, error) {
	row, err := q.queries.GetAutosave(ctx, db.GetAutosaveParams{ContentID: contentID, AuthorID: authorID})
	return revisionRow(db.GetRevisionRow(row)), dbsqlite.Classify(err)
}

// DeleteAutosave removes the author's autosave of the item.
func (q contentQueries) DeleteAutosave(ctx context.Context, contentID, authorID uuid.UUID) error {
	return dbsqlite.Classify(q.queries.DeleteAutosave(ctx, db.DeleteAutosaveParams{
		ContentID: contentID, AuthorID: authorID,
	}))
}

// RestampNewerAutosaves stamps each autosave of the item saved since the item last changed with the moment.
func (q contentQueries) RestampNewerAutosaves(ctx context.Context, contentID uuid.UUID, at time.Time) error {
	return dbsqlite.Classify(q.queries.RestampNewerAutosaves(ctx, db.RestampNewerAutosavesParams{
		SavedAt: dbkit.Time{Time: at}, ContentID: contentID,
	}))
}

// TypesOfContent returns the type of each stored item the identities name.
func (q contentQueries) TypesOfContent(ctx context.Context, ids []uuid.UUID) ([]contentstore.TypedItem, error) {
	rows, err := q.queries.TypesOfContent(ctx, jsonArray(ids))
	return mapped(rows, typedItem), dbsqlite.Classify(err)
}

// ClearRelationsOfField removes the index rows the item holds for the relation field.
func (q contentQueries) ClearRelationsOfField(ctx context.Context, from uuid.UUID, fieldID int) error {
	return dbsqlite.Classify(q.queries.ClearRelationsOfField(ctx, db.ClearRelationsOfFieldParams{
		FromID: from, FieldID: int64(fieldID),
	}))
}

// AddRelation stores one index row of a relation field.
func (q contentQueries) AddRelation(ctx context.Context, r contentstore.RelationRow) error {
	return dbsqlite.Classify(q.queries.AddRelation(ctx, db.AddRelationParams{
		FromID: r.FromID, FieldID: int64(r.FieldID), ToID: r.ToID, Position: int64(r.Position),
	}))
}

// RefreshRelationVisibility refreshes what a term page reads of the index rows pointing from or at the item.
func (q contentQueries) RefreshRelationVisibility(ctx context.Context, id uuid.UUID) error {
	return dbsqlite.Classify(q.queries.RefreshRelationVisibility(ctx, id))
}

// CountRelatedContent returns how many published items of active types point at the target.
func (q contentQueries) CountRelatedContent(ctx context.Context, target uuid.UUID) (int64, error) {
	return marked(q.queries.CountRelatedContent(ctx, target))
}

// ListRelatedContent returns one page of the published items of active types pointing at the target.
func (q contentQueries) ListRelatedContent(
	ctx context.Context, target uuid.UUID, limit, offset int,
) ([]contentstore.ContentRow, error) {
	rows, err := q.queries.ListRelatedContent(ctx, db.ListRelatedContentParams{
		Target: target, RowLimit: int64(limit), RowOffset: int64(offset),
	})
	return mapped(rows, relatedRow), dbsqlite.Classify(err)
}

// CountPointingAt returns how many published items point at the target through the field.
func (q contentQueries) CountPointingAt(ctx context.Context, target uuid.UUID, fieldID int) (int64, error) {
	return marked(q.queries.CountPointingAt(ctx, db.CountPointingAtParams{Target: target, Field: int64(fieldID)}))
}

// PointingAt returns one page of the published items pointing at the target through the field.
func (q contentQueries) PointingAt(ctx context.Context, p contentstore.Pointing) ([]content.Pointer, error) {
	rows, err := q.queries.PointingAt(ctx, db.PointingAtParams{
		Target: p.Target, Field: int64(p.FieldID), RowLimit: int64(p.Limit), RowOffset: int64(p.Offset),
	})
	return mapped(rows, pointer), dbsqlite.Classify(err)
}

// SummariesOfTargets returns the published items of active types the identities name.
func (q contentQueries) SummariesOfTargets(ctx context.Context, ids []uuid.UUID) ([]content.Target, error) {
	rows, err := q.queries.SummariesOfTargets(ctx, jsonArray(ids))
	return mapped(rows, target), dbsqlite.Classify(err)
}
