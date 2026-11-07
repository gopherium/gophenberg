// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"math"
	"time"

	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contentstore"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

// contentRow returns the stored item as the content store reads it.
func contentRow(row db.CoreContent) contentstore.ContentRow {
	return contentstore.ContentRow{
		ID: row.ID, AuthorID: row.AuthorID, ParentID: row.ParentID, Type: row.Type, Status: row.Status,
		Slug: row.Slug, Title: row.Title, Content: row.Content, Excerpt: row.Excerpt, Path: row.Path,
		Fields: row.Fields, PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// relatedRow returns one related item, read without its body, as the content store reads it.
func relatedRow(row db.ListRelatedContentRow) contentstore.ContentRow {
	return contentstore.ContentRow{
		ID: row.ID, AuthorID: row.AuthorID, ParentID: row.ParentID, Type: row.Type, Status: row.Status,
		Slug: row.Slug, Title: row.Title, Excerpt: row.Excerpt, Path: row.Path, Fields: row.Fields,
		PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// listedRow returns one listed item beside its author's name and its parent's title, as the content store reads it.
func listedRow(row db.ListContentRow) contentstore.ListedRow {
	return contentstore.ListedRow{
		ContentRow: contentstore.ContentRow{
			ID: row.ID, AuthorID: row.AuthorID, ParentID: row.ParentID, Type: row.Type, Status: row.Status,
			Slug: row.Slug, Title: row.Title, Excerpt: row.Excerpt, Path: row.Path, Fields: row.Fields,
			PublishedAt: row.PublishedAt, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		},
		AuthorName: row.AuthorName, ParentTitle: row.ParentTitle,
	}
}

// nestedRow returns one item of a nested listing as the content store reads a listed item.
func nestedRow(row db.ListNestedContentRow) contentstore.ListedRow {
	return listedRow(db.ListContentRow(row))
}

// revisionRow returns the stored revision or autosave as the content store reads it.
func revisionRow(row db.GetRevisionRow) contentstore.RevisionRow {
	return contentstore.RevisionRow{
		ID: row.ID, ContentID: row.ContentID, AuthorID: row.AuthorID, Kind: row.Kind, Title: row.Title,
		Content: row.Content, Excerpt: row.Excerpt, Fields: row.Fields, CreatedAt: row.CreatedAt,
	}
}

// listedRevision returns one listed revision, read without its body or values, as the content store reads it.
func listedRevision(row db.ListRevisionsRow) contentstore.RevisionRow {
	return contentstore.RevisionRow{
		ID: row.ID, ContentID: row.ContentID, AuthorID: row.AuthorID, Kind: row.Kind, Title: row.Title,
		Excerpt: row.Excerpt, CreatedAt: row.CreatedAt,
	}
}

// authorCount returns one author's count as the content store reads it.
func authorCount(row db.CountContentByAuthorRow) contentstore.AuthorCount {
	return contentstore.AuthorCount(row)
}

// typedItem returns one item's type as the content store reads it.
func typedItem(row db.TypesOfContentRow) contentstore.TypedItem {
	return contentstore.TypedItem(row)
}

// pointer returns one item pointing at a target as the domain names it.
func pointer(row db.PointingAtRow) content.Pointer {
	return content.Pointer(row)
}

// target returns one target summary as the domain names it.
func target(row db.SummariesOfTargetsRow) content.Target {
	return content.Target(row)
}

// createParams returns what storing a new item binds.
func createParams(c contentstore.ContentRow) db.CreateContentParams {
	return db.CreateContentParams{
		ID: c.ID, Type: c.Type, Status: c.Status, Slug: c.Slug, Title: c.Title, Content: c.Content,
		Excerpt: c.Excerpt, AuthorID: c.AuthorID, PublishedAt: c.PublishedAt, CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt, ParentID: c.ParentID, Path: c.Path, Fields: c.Fields,
	}
}

// updateParams returns what writing an edited item binds while the stored row still carries the stamp.
func updateParams(c contentstore.ContentRow, expected time.Time) db.UpdateContentParams {
	return db.UpdateContentParams{
		Status: c.Status, Slug: c.Slug, Path: c.Path, ParentID: c.ParentID, Title: c.Title, Content: c.Content,
		Excerpt: c.Excerpt, Fields: c.Fields, PublishedAt: c.PublishedAt, UpdatedAt: c.UpdatedAt, ID: c.ID,
		ExpectedUpdatedAt: expected,
	}
}

// revisionParams returns what storing a revision binds.
func revisionParams(r contentstore.RevisionRow) db.CreateRevisionParams {
	return db.CreateRevisionParams{
		ID: r.ID, ContentID: r.ContentID, Kind: r.Kind, AuthorID: r.AuthorID, Title: r.Title, Content: r.Content,
		Excerpt: r.Excerpt, Fields: r.Fields, CreatedAt: r.CreatedAt,
	}
}

// autosaveParams returns what storing an autosave binds.
func autosaveParams(r contentstore.RevisionRow) db.UpsertAutosaveParams {
	return db.UpsertAutosaveParams{
		ID: r.ID, ContentID: r.ContentID, AuthorID: r.AuthorID, Title: r.Title, Content: r.Content,
		Excerpt: r.Excerpt, Fields: r.Fields, CreatedAt: r.CreatedAt,
	}
}

// countParams returns what the count statement binds, the search escaped for ILIKE.
func countParams(n contentstore.Narrowing) db.CountContentParams {
	return db.CountContentParams{
		Type: n.Type, FieldFilter: n.FieldFilter, Statuses: n.Statuses, Authors: n.Authors,
		ExcludedAuthors: n.ExcludedAuthors, Before: n.Before, After: n.After, Search: dbkit.EscapeLike(n.Search),
	}
}

// listParams returns what the flat list statement binds, the search escaped for ILIKE.
func listParams(p contentstore.Page) db.ListContentParams {
	n := countParams(p.Narrowing)
	return db.ListContentParams{
		Type: n.Type, FieldFilter: n.FieldFilter, Statuses: n.Statuses, Authors: n.Authors,
		ExcludedAuthors: n.ExcludedAuthors, Before: n.Before, After: n.After, Search: n.Search,
		OrderBy: p.OrderBy, OrderDir: p.OrderDir, RowLimit: rowLimit(p.Limit), RowOffset: int32(p.Offset),
	}
}

// rowLimit returns the page size as the limit a query binds, bounded to the largest a PostgreSQL integer holds.
func rowLimit(size int) int32 {
	if size > math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(size)
}

// nestedParams returns what the nested list statement binds, the search escaped for ILIKE.
func nestedParams(p contentstore.Page) db.ListNestedContentParams {
	flat := listParams(p)
	return db.ListNestedContentParams{
		RowOffset: flat.RowOffset, RowLimit: flat.RowLimit, OrderBy: flat.OrderBy, OrderDir: flat.OrderDir,
		Type: flat.Type, FieldFilter: flat.FieldFilter, Statuses: flat.Statuses, Authors: flat.Authors,
		ExcludedAuthors: flat.ExcludedAuthors, Before: flat.Before, After: flat.After, Search: flat.Search,
	}
}
