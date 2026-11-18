// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gopherium/framework/dbkit"
	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// typeQueries sends the type store's statements through sqlc, on the handle or on one transaction.
type typeQueries struct {
	queries *db.Queries
}

// ListContentTypes returns every stored type row in registration order.
func (q typeQueries) ListContentTypes(ctx context.Context) ([]typestore.TypeRow, error) {
	rows, err := q.queries.ListContentTypes(ctx)
	return mapped(rows, typeRow), dbsqlite.Classify(err)
}

// GetContentType returns the stored row of the type under the key.
func (q typeQueries) GetContentType(ctx context.Context, key string) (typestore.TypeRow, error) {
	row, err := q.queries.GetContentType(ctx, key)
	return typeRow(row), dbsqlite.Classify(err)
}

// CreateContentType stores a new type row and returns it as stored, or [errTypeKeyHeld] when its key is stored.
func (q typeQueries) CreateContentType(ctx context.Context, t typestore.TypeRow) (typestore.TypeRow, error) {
	row, err := q.queries.CreateContentType(ctx, createTypeParams(t))
	if errors.Is(err, sql.ErrNoRows) {
		return typestore.TypeRow{}, errTypeKeyHeld
	}
	return typeRow(row), dbsqlite.Classify(err)
}

// LockContentType returns the stored row of the type under the key, held by the one writer.
func (q typeQueries) LockContentType(ctx context.Context, key string) (typestore.TypeRow, error) {
	row, err := q.queries.LockContentType(ctx, key)
	return typeRow(row), dbsqlite.Classify(err)
}

// LockDefaultContentType returns the stored row of the type holding the root, held by the one writer.
func (q typeQueries) LockDefaultContentType(ctx context.Context) (typestore.TypeRow, error) {
	row, err := q.queries.LockDefaultContentType(ctx)
	return typeRow(row), dbsqlite.Classify(err)
}

// UpdateContentType stores the type row over the one under its key and returns it as stored.
func (q typeQueries) UpdateContentType(ctx context.Context, t typestore.TypeRow) (typestore.TypeRow, error) {
	row, err := q.queries.UpdateContentType(ctx, updateTypeParams(t))
	return typeRow(row), dbsqlite.Classify(err)
}

// DeferAddressCheck sends nothing, leaving the address index checked row by row.
func (typeQueries) DeferAddressCheck(context.Context) error {
	return nil
}

// RetypeContentPaths marks every address of the type, then moves it off its old route word and returns the new ones.
func (q typeQueries) RetypeContentPaths(ctx context.Context, t typestore.TypeRow, was string) ([]string, error) {
	if err := q.queries.PrefixContentPaths(ctx, t.Key); err != nil {
		return nil, dbsqlite.Classify(err)
	}
	return marked(q.queries.RetypeContentPaths(ctx, db.RetypeContentPathsParams{
		RouteWord: t.RouteWord, Was: was, UpdatedAt: dbkit.Time{Time: t.UpdatedAt}, Key: t.Key,
	}))
}

// CountNestedContent returns how many items of the type sit inside another.
func (q typeQueries) CountNestedContent(ctx context.Context, key string) (int64, error) {
	return marked(q.queries.CountNestedContent(ctx, key))
}

// DeleteContentType removes the type under the key and returns how many rows it removed.
func (q typeQueries) DeleteContentType(ctx context.Context, key string) (int64, error) {
	return marked(q.queries.DeleteContentType(ctx, key))
}

// AdoptContentType clears the plugin origin from the type under the key and returns how many rows it changed.
func (q typeQueries) AdoptContentType(ctx context.Context, key string, at time.Time) (int64, error) {
	return marked(q.queries.AdoptContentType(ctx, db.AdoptContentTypeParams{UpdatedAt: dbkit.Time{Time: at}, Key: key}))
}

// TypeKeys returns the key of every stored type in registration order.
func (q typeQueries) TypeKeys(ctx context.Context) ([]string, error) {
	return marked(q.queries.TypeKeys(ctx))
}

// LockFieldGroups sends nothing, leaving every group held by the one writer.
func (typeQueries) LockFieldGroups(context.Context) error {
	return nil
}

// ListFieldGroups returns every stored group row in position order.
func (q typeQueries) ListFieldGroups(ctx context.Context) ([]typestore.GroupRow, error) {
	rows, err := q.queries.ListFieldGroups(ctx)
	return mapped(rows, groupRow), dbsqlite.Classify(err)
}

// FieldGroupKeys returns the key of every stored group.
func (q typeQueries) FieldGroupKeys(ctx context.Context) ([]string, error) {
	return marked(q.queries.FieldGroupKeys(ctx))
}

// CreateFieldGroup stores a new group row at the end of the order and returns it as stored.
func (q typeQueries) CreateFieldGroup(ctx context.Context, g typestore.GroupRow) (typestore.GroupRow, error) {
	row, err := q.queries.CreateFieldGroup(ctx, db.CreateFieldGroupParams{
		Key: g.Key, Title: g.Title, Location: string(g.Location), Origin: originColumn(g.Origin),
		CreatedAt: dbkit.Time{Time: g.CreatedAt}, UpdatedAt: dbkit.Time{Time: g.UpdatedAt},
	})
	return groupRow(row), dbsqlite.Classify(err)
}

// UpdateFieldGroup stores the title, location and active flag of the group row while its stamp holds.
func (q typeQueries) UpdateFieldGroup(
	ctx context.Context, g typestore.GroupRow, expected time.Time,
) (typestore.GroupRow, error) {
	row, err := q.queries.UpdateFieldGroup(ctx, db.UpdateFieldGroupParams{
		Title: g.Title, Location: string(g.Location), Active: g.Active, UpdatedAt: dbkit.Time{Time: g.UpdatedAt},
		ID: int64(g.ID), ExpectedUpdatedAt: dbkit.Time{Time: expected},
	})
	return groupRow(row), dbsqlite.Classify(err)
}

// ReorderFieldGroups stores the order the identities name on the groups.
func (q typeQueries) ReorderFieldGroups(ctx context.Context, ids []int) error {
	return dbsqlite.Classify(q.queries.ReorderFieldGroups(ctx, jsonArray(ids)))
}

// CarryStrayFieldsOfGroup moves the group's fields standing under another group's fields into that group.
func (q typeQueries) CarryStrayFieldsOfGroup(ctx context.Context, groupID int) error {
	rows, err := q.queries.StrayFieldsOfGroup(ctx, int64(groupID))
	if err != nil {
		return dbsqlite.Classify(err)
	}
	return dbsqlite.Classify(q.queries.CarryStrayFieldsOfGroup(ctx, idMap(rows, landingOf)))
}

// DeleteFieldGroup removes the group the identity names and returns how many rows it removed.
func (q typeQueries) DeleteFieldGroup(ctx context.Context, id int) (int64, error) {
	return marked(q.queries.DeleteFieldGroup(ctx, int64(id)))
}

// AdoptFieldGroup clears the plugin origin from the group under the key and returns how many rows it changed.
func (q typeQueries) AdoptFieldGroup(ctx context.Context, key string, at time.Time) (int64, error) {
	return marked(q.queries.AdoptFieldGroup(ctx, db.AdoptFieldGroupParams{UpdatedAt: dbkit.Time{Time: at}, Key: key}))
}

// AdoptFieldsInGroup clears the plugin origin from every field of the group under the key.
func (q typeQueries) AdoptFieldsInGroup(ctx context.Context, key string, at time.Time) error {
	return dbsqlite.Classify(q.queries.AdoptFieldsInGroup(ctx, db.AdoptFieldsInGroupParams{
		UpdatedAt: dbkit.Time{Time: at}, Key: key,
	}))
}

// ListContentFields returns every stored field row by group and position.
func (q typeQueries) ListContentFields(ctx context.Context) ([]typestore.FieldRow, error) {
	rows, err := q.queries.ListContentFields(ctx)
	return mapped(rows, fieldRow), dbsqlite.Classify(err)
}

// FieldByID returns the stored row of the field the identity names.
func (q typeQueries) FieldByID(ctx context.Context, id int) (typestore.FieldRow, error) {
	row, err := q.queries.FieldByID(ctx, int64(id))
	return fieldRow(row), dbsqlite.Classify(err)
}

// CreateContentField stores a new field row at the end of its group's top and returns it as stored.
func (q typeQueries) CreateContentField(ctx context.Context, f typestore.FieldRow) (typestore.FieldRow, error) {
	row, err := q.queries.CreateContentField(ctx, createFieldParams(f))
	return fieldRow(row), dbsqlite.Classify(err)
}

// CreateSubContentField stores a new field row at the end of its parent and returns it as stored.
func (q typeQueries) CreateSubContentField(ctx context.Context, f typestore.FieldRow) (typestore.FieldRow, error) {
	row, err := q.queries.CreateSubContentField(ctx, createSubFieldParams(f))
	return fieldRow(row), dbsqlite.Classify(err)
}

// UpdateContentField stores the label, required flag and settings of a group's top field while its stamp holds.
func (q typeQueries) UpdateContentField(
	ctx context.Context, f typestore.FieldRow, expected time.Time,
) (typestore.FieldRow, error) {
	row, err := q.queries.UpdateContentField(ctx, db.UpdateContentFieldParams{
		Label: f.Label, Required: f.Required, Settings: string(f.Settings), UpdatedAt: dbkit.Time{Time: f.UpdatedAt},
		GroupID: int64(f.GroupID), Key: f.Key, ExpectedUpdatedAt: dbkit.Time{Time: expected},
	})
	return fieldRow(row), dbsqlite.Classify(err)
}

// UpdateSubContentField stores the label, required flag and settings of a sub field while its stamp holds.
func (q typeQueries) UpdateSubContentField(
	ctx context.Context, f typestore.FieldRow, expected time.Time,
) (typestore.FieldRow, error) {
	row, err := q.queries.UpdateSubContentField(ctx, db.UpdateSubContentFieldParams{
		Label: f.Label, Required: f.Required, Settings: string(f.Settings), UpdatedAt: dbkit.Time{Time: f.UpdatedAt},
		ID: int64(f.ID), ExpectedUpdatedAt: dbkit.Time{Time: expected},
	})
	return fieldRow(row), dbsqlite.Classify(err)
}

// ReorderContentFields stores the order the keys name on the group's top fields and returns how many it moved.
func (q typeQueries) ReorderContentFields(ctx context.Context, groupID int, keys []string) (int64, error) {
	return marked(q.queries.ReorderContentFields(ctx, db.ReorderContentFieldsParams{
		Keys: jsonArray(keys), GroupID: int64(groupID),
	}))
}

// ReorderSubContentFields stores the order the keys name on the container's sub fields and returns how many it moved.
func (q typeQueries) ReorderSubContentFields(ctx context.Context, parentID int, keys []string) (int64, error) {
	return marked(q.queries.ReorderSubContentFields(ctx, db.ReorderSubContentFieldsParams{
		Keys: jsonArray(keys), ParentFieldID: parentColumn(parentID),
	}))
}

// ReparentContentField stands the field last under the group and parent and returns it as stored.
func (q typeQueries) ReparentContentField(
	ctx context.Context, id, toGroup, toParent int, at time.Time,
) (typestore.FieldRow, error) {
	row, err := q.queries.ReparentContentField(ctx, db.ReparentContentFieldParams{
		ToGroup: int64(toGroup), ToParent: parentColumn(toParent), UpdatedAt: dbkit.Time{Time: at}, ID: int64(id),
	})
	return fieldRow(row), dbsqlite.Classify(err)
}

// RecountContentFieldDepth stores the group and the depth on the field and every field below it.
func (q typeQueries) RecountContentFieldDepth(ctx context.Context, id, toGroup, depth int) error {
	rows, err := q.queries.FieldsUnder(ctx, db.FieldsUnderParams{Depth: int64(depth), ID: int64(id)})
	if err != nil {
		return dbsqlite.Classify(err)
	}
	return dbsqlite.Classify(q.queries.RecountContentFieldDepth(ctx, db.RecountContentFieldDepthParams{
		Depths: idMap(rows, depthOf), ToGroup: int64(toGroup),
	}))
}

// DeleteContentField removes the group's top field under the key and returns how many rows it removed.
func (q typeQueries) DeleteContentField(ctx context.Context, groupID int, key string) (int64, error) {
	return marked(q.queries.DeleteContentField(ctx, db.DeleteContentFieldParams{GroupID: int64(groupID), Key: key}))
}

// DeleteFieldByID removes the field the identity names and returns how many rows it removed.
func (q typeQueries) DeleteFieldByID(ctx context.Context, id int) (int64, error) {
	return marked(q.queries.DeleteFieldByID(ctx, int64(id)))
}

// ClearContentFieldValues removes the value under the key from every item of the types.
func (q typeQueries) ClearContentFieldValues(ctx context.Context, key string, types []string) error {
	return dbsqlite.Classify(q.queries.ClearContentFieldValues(ctx, db.ClearContentFieldValuesParams{
		Key: key, Types: jsonArray(types),
	}))
}

// ClearRevisionFieldValues removes the value under the key from every revision of the types.
func (q typeQueries) ClearRevisionFieldValues(ctx context.Context, key string, types []string) error {
	return dbsqlite.Classify(q.queries.ClearRevisionFieldValues(ctx, db.ClearRevisionFieldValuesParams{
		Key: key, Types: jsonArray(types),
	}))
}

// StripContentFieldPath removes whatever stands at the path from every item of the swept types.
func (q typeQueries) StripContentFieldPath(ctx context.Context, s typestore.Strip) error {
	return dbsqlite.Classify(q.queries.StripContentFieldPath(ctx, db.StripContentFieldPathParams{
		Path: jsonArray(s.Path), Types: jsonArray(s.Types), Key: s.Key,
	}))
}

// StripRevisionFieldPath removes whatever stands at the path from every revision of the swept types.
func (q typeQueries) StripRevisionFieldPath(ctx context.Context, s typestore.Strip) error {
	return dbsqlite.Classify(q.queries.StripRevisionFieldPath(ctx, db.StripRevisionFieldPathParams{
		Path: jsonArray(s.Path), Types: jsonArray(s.Types), Key: s.Key,
	}))
}

// StripContentLayout removes the rows a layout held at the path from every item of the swept types.
func (q typeQueries) StripContentLayout(ctx context.Context, s typestore.Strip) error {
	return dbsqlite.Classify(q.queries.StripContentLayout(ctx, db.StripContentLayoutParams{
		Path: jsonArray(s.Path), Types: jsonArray(s.Types), Key: s.Key,
	}))
}

// StripRevisionLayout removes the rows a layout held at the path from every revision of the swept types.
func (q typeQueries) StripRevisionLayout(ctx context.Context, s typestore.Strip) error {
	return dbsqlite.Classify(q.queries.StripRevisionLayout(ctx, db.StripRevisionLayoutParams{
		Path: jsonArray(s.Path), Types: jsonArray(s.Types), Key: s.Key,
	}))
}

// DeleteRelationsOfFields removes the index rows the fields hold for the items of the types.
func (q typeQueries) DeleteRelationsOfFields(ctx context.Context, types []string, fields []int) error {
	return dbsqlite.Classify(q.queries.DeleteRelationsOfFields(ctx, db.DeleteRelationsOfFieldsParams{
		Fields: jsonArray(fields), Types: jsonArray(types),
	}))
}
