// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// groupRow returns the stored group as the type store reads it.
func groupRow(row db.CoreFieldGroup) typestore.GroupRow {
	return typestore.GroupRow{
		ID:        int(row.ID),
		Position:  int(row.Position),
		Key:       row.Key,
		Title:     row.Title,
		Origin:    originOf(row.Origin),
		Location:  row.Location,
		Active:    row.Active,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

// fieldRow returns the stored field as the type store reads it.
func fieldRow(row db.CoreContentField) typestore.FieldRow {
	return typestore.FieldRow{
		ID:        int(row.ID),
		GroupID:   int(row.GroupID),
		ParentID:  parentOf(row.ParentFieldID),
		Depth:     int(row.Depth),
		Key:       row.Key,
		Label:     row.Label,
		Kind:      row.Kind,
		RelatesTo: originOf(row.RelatesTo),
		Origin:    originOf(row.Origin),
		Many:      row.Many,
		Required:  row.Required,
		Settings:  row.Settings,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

// typeRow returns the stored type as the type store reads it.
func typeRow(row db.CoreContentType) typestore.TypeRow {
	return typestore.TypeRow{
		Key:           row.Key,
		SingularLabel: row.SingularLabel,
		PluralLabel:   row.PluralLabel,
		Description:   row.Description,
		RouteWord:     row.RouteWord,
		PageKind:      row.PageKind,
		Origin:        originOf(row.Origin),
		Hierarchical:  row.Hierarchical,
		Revisions:     row.Revisions,
		IsDefault:     row.IsDefault,
		Active:        row.Active,
		RevisionCap:   int(row.RevisionCap),
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

// mapped returns each stored row copied as the type store reads it.
func mapped[Stored, Read any](rows []Stored, copyRow func(Stored) Read) []Read {
	read := make([]Read, len(rows))
	for i, row := range rows {
		read[i] = copyRow(row)
	}
	return read
}

// parentOf returns the parent a nullable column holds, 0 for a group's top.
func parentOf(parent pgtype.Int4) int {
	if !parent.Valid {
		return 0
	}
	return int(parent.Int32)
}

// parentColumn returns the parent as the nullable column holds it, null for a group's top.
func parentColumn(parentID int) pgtype.Int4 {
	return pgtype.Int4{Int32: int32(parentID), Valid: parentID != 0}
}

// int32s returns the identities as the integer columns hold them.
func int32s(ids []int) []int32 {
	held := make([]int32, len(ids))
	for i, id := range ids {
		held[i] = int32(id)
	}
	return held
}
