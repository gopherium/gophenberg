// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// typeQueries sends the type store's statements through sqlc, on the pool or on one transaction.
type typeQueries struct {
	conn    db.DBTX
	queries *db.Queries
}

// inTx returns the transactor that runs each write inside one pgx transaction on the pool.
func inTx(pool *pgxpool.Pool, queries *db.Queries) typestore.Transactor {
	return func(ctx context.Context, write func(typestore.Queries) error) error {
		return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
			return write(typeQueries{conn: tx, queries: queries.WithTx(tx)})
		})
	}
}

// ListContentTypes returns every stored type row in registration order.
func (q typeQueries) ListContentTypes(ctx context.Context) ([]typestore.TypeRow, error) {
	rows, err := q.queries.ListContentTypes(ctx)
	return mapped(rows, typeRow), err
}

// GetContentType returns the stored row of the type under the key.
func (q typeQueries) GetContentType(ctx context.Context, key string) (typestore.TypeRow, error) {
	row, err := q.queries.GetContentType(ctx, key)
	return typeRow(row), err
}

// CreateContentType stores a new type row and returns it as stored.
func (q typeQueries) CreateContentType(ctx context.Context, t typestore.TypeRow) (typestore.TypeRow, error) {
	row, err := q.queries.CreateContentType(ctx, db.CreateContentTypeParams{
		Origin:        originColumn(t.Origin),
		Key:           t.Key,
		SingularLabel: t.SingularLabel,
		PluralLabel:   t.PluralLabel,
		Description:   t.Description,
		RouteWord:     t.RouteWord,
		Hierarchical:  t.Hierarchical,
		Revisions:     t.Revisions,
		RevisionCap:   int32(t.RevisionCap),
		PageKind:      t.PageKind,
		IsDefault:     t.IsDefault,
		Active:        t.Active,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
	})
	return typeRow(row), err
}

// LockContentType returns the stored row of the type under the key, locked until the transaction ends.
func (q typeQueries) LockContentType(ctx context.Context, key string) (typestore.TypeRow, error) {
	row, err := q.queries.LockContentType(ctx, key)
	return typeRow(row), err
}

// LockDefaultContentType returns the stored row of the type holding the root, locked until the transaction ends.
func (q typeQueries) LockDefaultContentType(ctx context.Context) (typestore.TypeRow, error) {
	row, err := q.queries.LockDefaultContentType(ctx)
	return typeRow(row), err
}

// UpdateContentType stores the type row over the one under its key and returns it as stored.
func (q typeQueries) UpdateContentType(ctx context.Context, t typestore.TypeRow) (typestore.TypeRow, error) {
	row, err := q.queries.UpdateContentType(ctx, db.UpdateContentTypeParams{
		Key:           t.Key,
		SingularLabel: t.SingularLabel,
		PluralLabel:   t.PluralLabel,
		Description:   t.Description,
		RouteWord:     t.RouteWord,
		Hierarchical:  t.Hierarchical,
		Revisions:     t.Revisions,
		RevisionCap:   int32(t.RevisionCap),
		PageKind:      t.PageKind,
		IsDefault:     t.IsDefault,
		Active:        t.Active,
		UpdatedAt:     t.UpdatedAt,
	})
	return typeRow(row), err
}

// DeferAddressCheck postpones the unique address check to the end of the transaction.
func (q typeQueries) DeferAddressCheck(ctx context.Context) error {
	_, err := q.conn.Exec(ctx, deferAddressCheck)
	return err
}

// RetypeContentPaths moves every address of the type off the route word it answered under.
func (q typeQueries) RetypeContentPaths(ctx context.Context, t typestore.TypeRow, was string) error {
	paths, err := q.queries.RetypeContentPaths(ctx, db.RetypeContentPathsParams{
		RouteWord: t.RouteWord,
		Was:       was,
		UpdatedAt: t.UpdatedAt,
		Key:       t.Key,
	})
	if err != nil {
		return err
	}
	return content.CheckPaginationPaths(paths)
}

// CountNestedContent returns how many items of the type sit inside another.
func (q typeQueries) CountNestedContent(ctx context.Context, key string) (int64, error) {
	return q.queries.CountNestedContent(ctx, key)
}

// DeleteContentType removes the type under the key and returns how many rows it removed.
func (q typeQueries) DeleteContentType(ctx context.Context, key string) (int64, error) {
	return q.queries.DeleteContentType(ctx, key)
}

// AdoptContentType clears the plugin origin from the type under the key and returns how many rows it changed.
func (q typeQueries) AdoptContentType(ctx context.Context, key string, at time.Time) (int64, error) {
	return q.queries.AdoptContentType(ctx, db.AdoptContentTypeParams{UpdatedAt: at, Key: key})
}

// TypeKeys returns the key of every stored type in registration order.
func (q typeQueries) TypeKeys(ctx context.Context) ([]string, error) {
	return q.queries.TypeKeys(ctx)
}

// LockFieldGroups holds the field group lock until the transaction ends.
func (q typeQueries) LockFieldGroups(ctx context.Context) error {
	return q.queries.LockFieldGroups(ctx)
}

// ListFieldGroups returns every stored group row in position order.
func (q typeQueries) ListFieldGroups(ctx context.Context) ([]typestore.GroupRow, error) {
	rows, err := q.queries.ListFieldGroups(ctx)
	return mapped(rows, groupRow), err
}

// FieldGroupKeys returns the key of every stored group.
func (q typeQueries) FieldGroupKeys(ctx context.Context) ([]string, error) {
	return q.queries.FieldGroupKeys(ctx)
}

// CreateFieldGroup stores a new group row at the end of the order and returns it as stored.
func (q typeQueries) CreateFieldGroup(ctx context.Context, g typestore.GroupRow) (typestore.GroupRow, error) {
	row, err := q.queries.CreateFieldGroup(ctx, db.CreateFieldGroupParams{
		Key: g.Key, Title: g.Title, Location: g.Location,
		Origin: originColumn(g.Origin), CreatedAt: g.CreatedAt, UpdatedAt: g.UpdatedAt,
	})
	return groupRow(row), err
}

// UpdateFieldGroup stores the title, location and active flag of the group row and returns it as stored.
func (q typeQueries) UpdateFieldGroup(ctx context.Context, g typestore.GroupRow) (typestore.GroupRow, error) {
	row, err := q.queries.UpdateFieldGroup(ctx, db.UpdateFieldGroupParams{
		Title: g.Title, Location: g.Location, Active: g.Active,
		UpdatedAt: g.UpdatedAt, ID: int32(g.ID),
	})
	return groupRow(row), err
}

// ReorderFieldGroups stores the order the identities name on the groups.
func (q typeQueries) ReorderFieldGroups(ctx context.Context, ids []int) error {
	return q.queries.ReorderFieldGroups(ctx, int32s(ids))
}

// CarryStrayFieldsOfGroup moves the group's fields standing under another group's fields into that group.
func (q typeQueries) CarryStrayFieldsOfGroup(ctx context.Context, groupID int) error {
	return q.queries.CarryStrayFieldsOfGroup(ctx, int32(groupID))
}

// DeleteFieldGroup removes the group the identity names and returns how many rows it removed.
func (q typeQueries) DeleteFieldGroup(ctx context.Context, id int) (int64, error) {
	return q.queries.DeleteFieldGroup(ctx, int32(id))
}

// AdoptFieldGroup clears the plugin origin from the group under the key and returns how many rows it changed.
func (q typeQueries) AdoptFieldGroup(ctx context.Context, key string, at time.Time) (int64, error) {
	return q.queries.AdoptFieldGroup(ctx, db.AdoptFieldGroupParams{UpdatedAt: at, Key: key})
}

// AdoptFieldsInGroup clears the plugin origin from every field of the group under the key.
func (q typeQueries) AdoptFieldsInGroup(ctx context.Context, key string, at time.Time) error {
	return q.queries.AdoptFieldsInGroup(ctx, db.AdoptFieldsInGroupParams{UpdatedAt: at, Key: key})
}

// ListContentFields returns every stored field row by group and position.
func (q typeQueries) ListContentFields(ctx context.Context) ([]typestore.FieldRow, error) {
	rows, err := q.queries.ListContentFields(ctx)
	return mapped(rows, fieldRow), err
}

// FieldByID returns the stored row of the field the identity names.
func (q typeQueries) FieldByID(ctx context.Context, id int) (typestore.FieldRow, error) {
	row, err := q.queries.FieldByID(ctx, int32(id))
	return fieldRow(row), err
}

// CreateContentField stores a new field row at the end of its group's top and returns it as stored.
func (q typeQueries) CreateContentField(ctx context.Context, f typestore.FieldRow) (typestore.FieldRow, error) {
	row, err := q.queries.CreateContentField(ctx, db.CreateContentFieldParams{
		GroupID:   int32(f.GroupID),
		Origin:    originColumn(f.Origin),
		Key:       f.Key,
		Label:     f.Label,
		Kind:      f.Kind,
		RelatesTo: originColumn(f.RelatesTo),
		Many:      f.Many,
		Required:  f.Required,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
		Settings:  f.Settings,
	})
	return fieldRow(row), err
}

// CreateSubContentField stores a new field row at the end of its parent and returns it as stored.
func (q typeQueries) CreateSubContentField(ctx context.Context, f typestore.FieldRow) (typestore.FieldRow, error) {
	row, err := q.queries.CreateSubContentField(ctx, db.CreateSubContentFieldParams{
		GroupID:       int32(f.GroupID),
		Origin:        originColumn(f.Origin),
		ParentFieldID: pgtype.Int4{Int32: int32(f.ParentID), Valid: true},
		Key:           f.Key,
		Label:         f.Label,
		Kind:          f.Kind,
		RelatesTo:     originColumn(f.RelatesTo),
		Many:          f.Many,
		Required:      f.Required,
		CreatedAt:     f.CreatedAt,
		UpdatedAt:     f.UpdatedAt,
		Settings:      f.Settings,
		Depth:         int32(f.Depth),
	})
	return fieldRow(row), err
}

// UpdateContentField stores the label, required flag and settings of a group's top field while its stamp holds.
func (q typeQueries) UpdateContentField(
	ctx context.Context, f typestore.FieldRow, expected time.Time,
) (typestore.FieldRow, error) {
	row, err := q.queries.UpdateContentField(ctx, db.UpdateContentFieldParams{
		Label:             f.Label,
		Required:          f.Required,
		Settings:          f.Settings,
		UpdatedAt:         f.UpdatedAt,
		ExpectedUpdatedAt: expected,
		GroupID:           int32(f.GroupID),
		Key:               f.Key,
	})
	return fieldRow(row), err
}

// UpdateSubContentField stores the label, required flag and settings of a sub field while its stamp holds.
func (q typeQueries) UpdateSubContentField(
	ctx context.Context, f typestore.FieldRow, expected time.Time,
) (typestore.FieldRow, error) {
	row, err := q.queries.UpdateSubContentField(ctx, db.UpdateSubContentFieldParams{
		Label:             f.Label,
		Required:          f.Required,
		Settings:          f.Settings,
		UpdatedAt:         f.UpdatedAt,
		ExpectedUpdatedAt: expected,
		ID:                int32(f.ID),
	})
	return fieldRow(row), err
}

// ReorderContentFields stores the order the keys name on the group's top fields and returns how many it moved.
func (q typeQueries) ReorderContentFields(ctx context.Context, groupID int, keys []string) (int64, error) {
	return q.queries.ReorderContentFields(ctx, db.ReorderContentFieldsParams{
		Keys:    keys,
		GroupID: int32(groupID),
	})
}

// ReorderSubContentFields stores the order the keys name on the container's sub fields and returns how many it moved.
func (q typeQueries) ReorderSubContentFields(ctx context.Context, parentID int, keys []string) (int64, error) {
	return q.queries.ReorderSubContentFields(ctx, db.ReorderSubContentFieldsParams{
		ParentFieldID: pgtype.Int4{Int32: int32(parentID), Valid: true},
		Keys:          keys,
	})
}

// ReparentContentField stands the field last under the group and parent and returns it as stored.
func (q typeQueries) ReparentContentField(
	ctx context.Context, id, toGroup, toParent int, at time.Time,
) (typestore.FieldRow, error) {
	row, err := q.queries.ReparentContentField(ctx, db.ReparentContentFieldParams{
		ID: int32(id), ToGroup: int32(toGroup), ToParent: parentColumn(toParent),
		UpdatedAt: at,
	})
	return fieldRow(row), err
}

// RecountContentFieldDepth stores the group and the depth on the field and every field below it.
func (q typeQueries) RecountContentFieldDepth(ctx context.Context, id, toGroup, depth int) error {
	return q.queries.RecountContentFieldDepth(ctx, db.RecountContentFieldDepthParams{
		ID: int32(id), Depth: int32(depth), ToGroup: int32(toGroup),
	})
}

// DeleteContentField removes the group's top field under the key and returns how many rows it removed.
func (q typeQueries) DeleteContentField(ctx context.Context, groupID int, key string) (int64, error) {
	return q.queries.DeleteContentField(ctx, db.DeleteContentFieldParams{
		GroupID: int32(groupID), Key: key,
	})
}

// DeleteFieldByID removes the field the identity names and returns how many rows it removed.
func (q typeQueries) DeleteFieldByID(ctx context.Context, id int) (int64, error) {
	return q.queries.DeleteFieldByID(ctx, int32(id))
}

// ClearContentFieldValues removes the value under the key from every item of the types.
func (q typeQueries) ClearContentFieldValues(ctx context.Context, key string, types []string) error {
	return q.queries.ClearContentFieldValues(ctx, db.ClearContentFieldValuesParams{
		Key: key, Types: types,
	})
}

// ClearRevisionFieldValues removes the value under the key from every revision of the types.
func (q typeQueries) ClearRevisionFieldValues(ctx context.Context, key string, types []string) error {
	return q.queries.ClearRevisionFieldValues(ctx, db.ClearRevisionFieldValuesParams{
		Key: key, Types: types,
	})
}

// StripContentFieldPath removes whatever stands at the path from every item of the swept types.
func (q typeQueries) StripContentFieldPath(ctx context.Context, s typestore.Strip) error {
	return q.queries.StripContentFieldPath(ctx, db.StripContentFieldPathParams{
		Path: s.Path, Types: s.Types, Key: s.Key,
	})
}

// StripRevisionFieldPath removes whatever stands at the path from every revision of the swept types.
func (q typeQueries) StripRevisionFieldPath(ctx context.Context, s typestore.Strip) error {
	return q.queries.StripRevisionFieldPath(ctx, db.StripRevisionFieldPathParams{
		Path: s.Path, Types: s.Types, Key: s.Key,
	})
}

// StripContentLayout removes the rows a layout held at the path from every item of the swept types.
func (q typeQueries) StripContentLayout(ctx context.Context, s typestore.Strip) error {
	return q.queries.StripContentLayout(ctx, db.StripContentLayoutParams{
		Path: s.Path, Types: s.Types, Key: s.Key,
	})
}

// StripRevisionLayout removes the rows a layout held at the path from every revision of the swept types.
func (q typeQueries) StripRevisionLayout(ctx context.Context, s typestore.Strip) error {
	return q.queries.StripRevisionLayout(ctx, db.StripRevisionLayoutParams{
		Path: s.Path, Types: s.Types, Key: s.Key,
	})
}

// DeleteRelationsOfFields removes the index rows the fields hold for the items of the types.
func (q typeQueries) DeleteRelationsOfFields(ctx context.Context, types []string, fields []int) error {
	return q.queries.DeleteRelationsOfFields(ctx, db.DeleteRelationsOfFieldsParams{
		Types: types, Fields: int32s(fields),
	})
}
