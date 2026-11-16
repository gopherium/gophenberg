// SPDX-License-Identifier: Apache-2.0

package sqlite

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contentstore"
	"github.com/gopherium/gophenberg/internal/sqlite/columns"
	"github.com/gopherium/gophenberg/internal/sqlite/db"
)

// contentRow returns the stored item as the content store reads it.
func contentRow(row db.CoreContent) contentstore.ContentRow {
	return contentstore.ContentRow{
		ID: row.ID, AuthorID: row.AuthorID, ParentID: row.ParentID, Type: row.Type, Status: row.Status,
		Slug: row.Slug, Title: row.Title, Content: row.Content, Excerpt: row.Excerpt, Path: row.Path,
		Fields: content.Values(row.Fields), PublishedAt: instantOf(row.PublishedAt), CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

// listedRow returns one listed item beside its author's name and its parent's title, as the content store reads it.
func listedRow(row db.ListContentRow) contentstore.ListedRow {
	return contentstore.ListedRow{
		ContentRow: contentstore.ContentRow{
			ID: row.ID, AuthorID: row.AuthorID, ParentID: row.ParentID, Type: row.Type, Status: row.Status,
			Slug: row.Slug, Title: row.Title, Excerpt: row.Excerpt, Path: row.Path, Fields: content.Values(row.Fields),
			PublishedAt: instantOf(row.PublishedAt), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
		},
		AuthorName: row.AuthorName, ParentTitle: row.ParentTitle,
	}
}

// nestedRow returns one item of a nested listing as the content store reads a listed item.
func nestedRow(row db.ListNestedContentRow) contentstore.ListedRow {
	return listedRow(db.ListContentRow(row))
}

// movesOf returns each descendant's new address as JSON keyed by id, and the new addresses under page.
func movesOf(rows []db.DescendantsToMoveRow) (string, []string) {
	moves := make(map[string]string, len(rows))
	var reported []string
	for _, row := range rows {
		moves[row.ID] = row.Path
		if row.Reported {
			reported = append(reported, row.Path)
		}
	}
	text, _ := json.Marshal(moves)
	return string(text), reported
}

// relatedRow returns one related item, read without its body, as the content store reads it.
func relatedRow(row db.ListRelatedContentRow) contentstore.ContentRow {
	return contentstore.ContentRow{
		ID: row.ID, AuthorID: row.AuthorID, ParentID: row.ParentID, Type: row.Type, Status: row.Status,
		Slug: row.Slug, Title: row.Title, Excerpt: row.Excerpt, Path: row.Path, Fields: content.Values(row.Fields),
		PublishedAt: instantOf(row.PublishedAt), CreatedAt: row.CreatedAt.Time, UpdatedAt: row.UpdatedAt.Time,
	}
}

// pointer returns one item pointing at a target as the domain names it.
func pointer(row db.PointingAtRow) content.Pointer {
	return content.Pointer(row)
}

// target returns one target summary as the domain names it.
func target(row db.SummariesOfTargetsRow) content.Target {
	return content.Target(row)
}

// typedItem returns one item's type as the content store reads it.
func typedItem(row db.TypesOfContentRow) contentstore.TypedItem {
	return contentstore.TypedItem(row)
}

// authorCount returns one author's count as the content store reads it.
func authorCount(row db.CountContentByAuthorRow) contentstore.AuthorCount {
	return contentstore.AuthorCount(row)
}

// revisionRow returns the stored revision or autosave as the content store reads it.
func revisionRow(row db.GetRevisionRow) contentstore.RevisionRow {
	return contentstore.RevisionRow{
		ID: row.ID, ContentID: row.ContentID, AuthorID: row.AuthorID, Kind: row.Kind, Title: row.Title,
		Content: row.Content, Excerpt: row.Excerpt, Fields: content.Values(row.Fields), CreatedAt: row.CreatedAt.Time,
	}
}

// listedRevision returns one listed revision, read without its body or values, as the content store reads it.
func listedRevision(row db.ListRevisionsRow) contentstore.RevisionRow {
	return contentstore.RevisionRow{
		ID: row.ID, ContentID: row.ContentID, AuthorID: row.AuthorID, Kind: row.Kind, Title: row.Title,
		Excerpt: row.Excerpt, CreatedAt: row.CreatedAt.Time,
	}
}

// createParams returns what storing a new item binds.
func createParams(c contentstore.ContentRow) db.CreateContentParams {
	return db.CreateContentParams{
		ID: c.ID, Type: c.Type, Status: c.Status, Slug: c.Slug, Title: c.Title, Content: c.Content,
		Excerpt: c.Excerpt, AuthorID: c.AuthorID, PublishedAt: stampOf(c.PublishedAt),
		CreatedAt: dbkit.Time{Time: c.CreatedAt}, UpdatedAt: dbkit.Time{Time: c.UpdatedAt}, ParentID: c.ParentID,
		Path: c.Path, Fields: columns.Values(c.Fields),
	}
}

// updateParams returns what writing an edited item binds while the stored row still carries the stamp.
func updateParams(c contentstore.ContentRow, expected time.Time) db.UpdateContentParams {
	return db.UpdateContentParams{
		Status: c.Status, Slug: c.Slug, Path: c.Path, ParentID: c.ParentID, Title: c.Title, Content: c.Content,
		Excerpt: c.Excerpt, Fields: columns.Values(c.Fields), PublishedAt: stampOf(c.PublishedAt),
		UpdatedAt: dbkit.Time{Time: c.UpdatedAt}, ID: c.ID, ExpectedUpdatedAt: dbkit.Time{Time: expected},
	}
}

// revisionParams returns what storing a revision binds.
func revisionParams(r contentstore.RevisionRow) db.CreateRevisionParams {
	return db.CreateRevisionParams{
		ID: r.ID, ContentID: r.ContentID, Kind: r.Kind, AuthorID: r.AuthorID, Title: r.Title, Content: r.Content,
		Excerpt: r.Excerpt, Fields: columns.Values(r.Fields), CreatedAt: dbkit.Time{Time: r.CreatedAt},
	}
}

// autosaveParams returns what storing an autosave binds.
func autosaveParams(r contentstore.RevisionRow) db.UpsertAutosaveParams {
	return db.UpsertAutosaveParams{
		ID: r.ID, ContentID: r.ContentID, AuthorID: r.AuthorID, Title: r.Title, Content: r.Content,
		Excerpt: r.Excerpt, Fields: columns.Values(r.Fields), CreatedAt: dbkit.Time{Time: r.CreatedAt},
	}
}

// countParams returns what the count statement binds, every list as JSON text and the search as typed.
func countParams(n contentstore.Narrowing) db.CountContentParams {
	return db.CountContentParams{
		Type: n.Type, FieldFilter: string(n.FieldFilter), Statuses: jsonArray(n.Statuses), Authors: jsonArray(n.Authors),
		ExcludedAuthors: jsonArray(n.ExcludedAuthors), Before: n.Before, After: n.After, Search: n.Search,
	}
}

// listParams returns what the flat list statement binds.
func listParams(p contentstore.Page) db.ListContentParams {
	n := countParams(p.Narrowing)
	return db.ListContentParams{
		Type: n.Type, FieldFilter: n.FieldFilter, Statuses: n.Statuses, Authors: n.Authors,
		ExcludedAuthors: n.ExcludedAuthors, Before: n.Before, After: n.After, Search: n.Search,
		OrderBy: p.OrderBy, OrderDir: p.OrderDir, RowLimit: int64(p.Limit), RowOffset: int64(p.Offset),
	}
}

// nestedParams returns what the nested list statement binds.
func nestedParams(p contentstore.Page) db.ListNestedContentParams {
	flat := listParams(p)
	return db.ListNestedContentParams{
		RowOffset: flat.RowOffset, RowLimit: flat.RowLimit, OrderBy: flat.OrderBy, OrderDir: flat.OrderDir,
		Type: flat.Type, FieldFilter: flat.FieldFilter, Statuses: flat.Statuses, Authors: flat.Authors,
		ExcludedAuthors: flat.ExcludedAuthors, Before: flat.Before, After: flat.After, Search: flat.Search,
	}
}

// authorColumn returns the author as the trash statement binds it, nil for every author.
func authorColumn(author *uuid.UUID) *string {
	if author == nil {
		return nil
	}
	text := author.String()
	return &text
}

// mapped returns each stored row copied as the store reads it.
func mapped[Stored, Read any](rows []Stored, copyRow func(Stored) Read) []Read {
	read := make([]Read, len(rows))
	for i, row := range rows {
		read[i] = copyRow(row)
	}
	return read
}

// instantOf returns the moment a nullable stamp holds, or nil.
func instantOf(stamp *dbkit.Time) *time.Time {
	if stamp == nil {
		return nil
	}
	return &stamp.Time
}

// stampOf returns the moment as a nullable stamp binds it, or nil.
func stampOf(at *time.Time) *dbkit.Time {
	if at == nil {
		return nil
	}
	return &dbkit.Time{Time: *at}
}
