// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"context"

	dbsqlite "github.com/gopherium/framework/dbkit/sqlite"

	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// typeQueries sends the type store's statements through sqlc, on the handle or on one transaction.
type typeQueries struct {
	queries *db.Queries
}

// DeferAddressCheck sends nothing, leaving the address index checked row by row.
func (typeQueries) DeferAddressCheck(context.Context) error {
	return nil
}

// ListFieldGroups returns every stored group row in position order.
func (q typeQueries) ListFieldGroups(ctx context.Context) ([]typestore.GroupRow, error) {
	rows, err := q.queries.ListFieldGroups(ctx)
	return mapped(rows, groupRow), dbsqlite.Classify(err)
}

// ListContentFields returns every stored field row by group and position.
func (q typeQueries) ListContentFields(ctx context.Context) ([]typestore.FieldRow, error) {
	rows, err := q.queries.ListContentFields(ctx)
	return mapped(rows, fieldRow), dbsqlite.Classify(err)
}

// groupRow returns the stored group as the type store reads it.
func groupRow(row db.CoreFieldGroup) typestore.GroupRow {
	return typestore.GroupRow{
		ID: int(row.ID), Position: int(row.Position), Key: row.Key, Title: row.Title, Origin: orEmpty(row.Origin),
		Location: []byte(row.Location), Active: row.Active, CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}

// fieldRow returns the stored field as the type store reads it.
func fieldRow(row db.CoreContentField) typestore.FieldRow {
	return typestore.FieldRow{
		ID: int(row.ID), GroupID: int(row.GroupID), ParentID: parentOf(row.ParentFieldID), Depth: int(row.Depth),
		Key: row.Key, Label: row.Label, Kind: row.Kind, RelatesTo: orEmpty(row.RelatesTo), Origin: orEmpty(row.Origin),
		Many: row.Many, Required: row.Required, Settings: []byte(row.Settings), CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

// orEmpty returns the text a nullable column holds, empty for null.
func orEmpty(text *string) string {
	if text == nil {
		return ""
	}
	return *text
}

// parentOf returns the parent a nullable column holds, 0 for a group's top.
func parentOf(parent *int64) int {
	if parent == nil {
		return 0
	}
	return int(*parent)
}
