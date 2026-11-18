// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"encoding/json"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// typeRow returns the stored type as the type store reads it.
func typeRow(row db.CoreContentType) typestore.TypeRow {
	return typestore.TypeRow{
		Key: row.Key, SingularLabel: row.SingularLabel, PluralLabel: row.PluralLabel, Description: row.Description,
		RouteWord: row.RouteWord, PageKind: row.PageKind, Origin: orEmpty(row.Origin), Hierarchical: row.Hierarchical,
		Revisions: row.Revisions, IsDefault: row.IsDefault, Active: row.Active, RevisionCap: int(row.RevisionCap),
		CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
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

// createTypeParams returns what storing a new type binds.
func createTypeParams(t typestore.TypeRow) db.CreateContentTypeParams {
	return db.CreateContentTypeParams{
		Key: t.Key, SingularLabel: t.SingularLabel, PluralLabel: t.PluralLabel, RouteWord: t.RouteWord,
		Hierarchical: t.Hierarchical, Revisions: t.Revisions, RevisionCap: int64(t.RevisionCap), PageKind: t.PageKind,
		IsDefault: t.IsDefault, Active: t.Active, Origin: originColumn(t.Origin), Description: t.Description,
		CreatedAt: dbkit.Time{Time: t.CreatedAt}, UpdatedAt: dbkit.Time{Time: t.UpdatedAt},
	}
}

// updateTypeParams returns what writing a type over the one under its key binds.
func updateTypeParams(t typestore.TypeRow) db.UpdateContentTypeParams {
	return db.UpdateContentTypeParams{
		SingularLabel: t.SingularLabel, PluralLabel: t.PluralLabel, RouteWord: t.RouteWord, Hierarchical: t.Hierarchical,
		Revisions: t.Revisions, RevisionCap: int64(t.RevisionCap), PageKind: t.PageKind, IsDefault: t.IsDefault,
		Active: t.Active, Description: t.Description, UpdatedAt: dbkit.Time{Time: t.UpdatedAt}, Key: t.Key,
	}
}

// createFieldParams returns what storing a new top field of a group binds.
func createFieldParams(f typestore.FieldRow) db.CreateContentFieldParams {
	return db.CreateContentFieldParams{
		GroupID: int64(f.GroupID), Key: f.Key, Label: f.Label, Kind: f.Kind, RelatesTo: originColumn(f.RelatesTo),
		Many: f.Many, Required: f.Required, CreatedAt: dbkit.Time{Time: f.CreatedAt},
		UpdatedAt: dbkit.Time{Time: f.UpdatedAt}, Settings: string(f.Settings), Origin: originColumn(f.Origin),
	}
}

// createSubFieldParams returns what storing a new sub field under its parent binds.
func createSubFieldParams(f typestore.FieldRow) db.CreateSubContentFieldParams {
	return db.CreateSubContentFieldParams{
		GroupID: int64(f.GroupID), ParentFieldID: parentColumn(f.ParentID), Key: f.Key, Label: f.Label, Kind: f.Kind,
		RelatesTo: originColumn(f.RelatesTo), Many: f.Many, Required: f.Required,
		CreatedAt: dbkit.Time{Time: f.CreatedAt}, UpdatedAt: dbkit.Time{Time: f.UpdatedAt},
		Settings: string(f.Settings), Depth: int64(f.Depth), Origin: originColumn(f.Origin),
	}
}

// depthOf returns a field the recount walk reached and the depth it stands at.
func depthOf(row db.FieldsUnderRow) (int64, int64) {
	return row.ID, row.Depth
}

// landingOf returns a stray field and the group it stands under.
func landingOf(row db.StrayFieldsOfGroupRow) (int64, int64) {
	return row.ID, row.GroupID
}

// idMap returns each row's identity and value as the JSON object text a map UPDATE reads with json_each.
func idMap[Row any](rows []Row, entry func(Row) (int64, int64)) string {
	held := make(map[int64]int64, len(rows))
	for _, row := range rows {
		id, value := entry(row)
		held[id] = value
	}
	text, _ := json.Marshal(held)
	return string(text)
}

// orEmpty returns the text a nullable column holds, empty for null.
func orEmpty(text *string) string {
	if text == nil {
		return ""
	}
	return *text
}

// originColumn returns the text as the nullable column holds it, null for empty.
func originColumn(text string) *string {
	if text == "" {
		return nil
	}
	return &text
}

// parentOf returns the parent a nullable column holds, 0 for a group's top.
func parentOf(parent *int64) int {
	if parent == nil {
		return 0
	}
	return int(*parent)
}

// parentColumn returns the parent as the nullable column holds it, null for a group's top.
func parentColumn(parentID int) *int64 {
	if parentID == 0 {
		return nil
	}
	held := int64(parentID)
	return &held
}
