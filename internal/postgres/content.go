// SPDX-License-Identifier: Apache-2.0

package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres/db"
)

var _ content.Store = (*ContentStore)(nil)

// slugAttempts bounds the suffixes tried when a slug is taken.
const slugAttempts = 20

// deferAddressCheck holds the address check until a move has settled.
const deferAddressCheck = "SET CONSTRAINTS core.content_path_unique DEFERRED"

// slugConstraint names the unique constraint over a content address.
const slugConstraint = "content_path_unique"

// trashSuffixAlphabet holds the characters a trashed slug suffix draws from.
const trashSuffixAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// trashSuffixLength is the number of random characters in a trashed slug suffix.
const trashSuffixLength = 8

// ContentStore persists content items in the core schema.
type ContentStore struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

// NewContentStore returns a [ContentStore] backed by pool.
func NewContentStore(pool *pgxpool.Pool) *ContentStore {
	return &ContentStore{pool: pool, queries: db.New(pool)}
}

// Create stores a new content item, suffixing its slug until its address is free.
func (s *ContentStore) Create(ctx context.Context, c content.Content) (content.Content, error) {
	prefix := content.AddressPrefix(c.Path, c.Slug)
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		created, err := s.create(ctx, c, prefix, numberedSlug(c.Slug, attempt))
		if isSlugTaken(err) {
			continue
		}
		return created, err
	}
	created, err := s.create(ctx, c, prefix, identifiedSlug(c.Slug, c.ID))
	if isSlugTaken(err) {
		return content.Content{}, content.ErrSlugTaken
	}
	return created, err
}

// create stores the content item under slug, addressed beneath prefix.
func (s *ContentStore) create(
	ctx context.Context, c content.Content, prefix, slug string,
) (content.Content, error) {
	params := db.CreateContentParams{
		ID:          c.ID,
		Type:        c.Type,
		ParentID:    c.ParentID,
		Path:        content.AddressUnder(prefix, slug),
		Status:      string(c.Status),
		Slug:        slug,
		Title:       c.Title,
		Content:     c.Content,
		Excerpt:     c.Excerpt,
		AuthorID:    c.AuthorID,
		PublishedAt: c.PublishedAt,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
		Fields:      storedValues(c.Fields),
	}
	if len(c.Fields) > 0 || c.ParentID != nil {
		return s.createHeld(ctx, c, params)
	}
	row, err := s.queries.CreateContent(ctx, params)
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: create content: %w", err)
	}
	return toContent(row), nil
}

// createHeld stores the item once its type still nests and every value it carries names a declared field.
func (s *ContentStore) createHeld(
	ctx context.Context, c content.Content, params db.CreateContentParams,
) (content.Content, error) {
	var row db.CoreContent
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		prefix, err := fileable(ctx, queries, c, content.AddressPrefix(params.Path, params.Slug))
		if err != nil {
			return err
		}
		params.Path = content.AddressUnder(prefix, params.Slug)
		matching, err := declaredValues(ctx, queries, c)
		if err != nil {
			return err
		}
		created, err := queries.CreateContent(ctx, params)
		if err != nil {
			return err
		}
		row = created
		resolved, err := resolveTargets(ctx, queries, matching, toContent(created), nil)
		if err != nil {
			return err
		}
		return writeRelations(ctx, queries, toContent(created), resolved)
	})
	if err != nil {
		return content.Content{}, writeFailure(err)
	}
	return toContent(row), nil
}

// PublishedByPath returns the published item at the address, or [content.ErrNotFound].
func (s *ContentStore) PublishedByPath(ctx context.Context, path string) (content.Content, error) {
	row, err := s.queries.GetPublishedContentByPath(ctx, path)
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: get published content by path: %w", err)
	}
	return toContent(row), nil
}

// Children returns how many items nest directly under the item.
func (s *ContentStore) Children(ctx context.Context, id uuid.UUID) (int, error) {
	held, err := s.queries.CountChildren(ctx, &id)
	if err != nil {
		return 0, fmt.Errorf("postgres: count children: %w", err)
	}
	return int(held), nil
}

// ByID returns the content item with the given id, or [content.ErrNotFound].
func (s *ContentStore) ByID(ctx context.Context, id uuid.UUID) (content.Content, error) {
	return byID(ctx, s.queries, id)
}

// byID returns the content item with the given id through queries, or [content.ErrNotFound].
func byID(ctx context.Context, queries *db.Queries, id uuid.UUID) (content.Content, error) {
	row, err := queries.GetContent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: get content: %w", err)
	}
	return toContent(row), nil
}

// storedValues returns the values a write holds, never nil so the column stays an object.
func storedValues(v content.Values) content.Values {
	if v == nil {
		return content.Values{}
	}
	return v
}

// toContent maps a stored row to a domain content item with UTC timestamps.
func toContent(row db.CoreContent) content.Content {
	return content.Content{
		ID:          row.ID,
		Type:        row.Type,
		ParentID:    row.ParentID,
		Path:        row.Path,
		Status:      content.Status(row.Status),
		Slug:        row.Slug,
		Title:       row.Title,
		Content:     row.Content,
		Excerpt:     row.Excerpt,
		AuthorID:    row.AuthorID,
		Fields:      row.Fields,
		PublishedAt: utcOrNil(row.PublishedAt),
		CreatedAt:   row.CreatedAt.UTC(),
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
}

// List returns the items matching the filter without content, with author names and parent titles, and their total.
func (s *ContentStore) List(ctx context.Context, f content.Filter) ([]content.ListedItem, int, error) {
	narrowed := narrowing(f)
	total, err := s.queries.CountContent(ctx, narrowed)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: count content: %w", err)
	}
	rows, err := s.listRows(ctx, f, narrowed)
	if err != nil {
		return nil, 0, fmt.Errorf("postgres: list content: %w", err)
	}
	items := make([]content.ListedItem, len(rows))
	for i, row := range rows {
		items[i] = listedContent(row)
	}
	return items, int(total), nil
}

// listRows returns the page the filter names, each child under its parent when the filter nests them.
func (s *ContentStore) listRows(
	ctx context.Context, f content.Filter, narrowed db.CountContentParams,
) ([]db.ListContentRow, error) {
	page := db.ListContentParams{
		Type:            narrowed.Type,
		FieldFilter:     narrowed.FieldFilter,
		Statuses:        narrowed.Statuses,
		Authors:         narrowed.Authors,
		ExcludedAuthors: narrowed.ExcludedAuthors,
		Before:          narrowed.Before,
		After:           narrowed.After,
		Search:          narrowed.Search,
		OrderBy:         string(f.OrderBy),
		OrderDir:        string(f.Order),
		RowLimit:        int32(f.PerPage),
		RowOffset:       pageOffset(f.Page, f.PerPage),
	}
	if !f.Hierarchy {
		return s.queries.ListContent(ctx, page)
	}
	nested, err := s.queries.ListNestedContent(ctx, db.ListNestedContentParams{
		RowOffset: page.RowOffset, RowLimit: page.RowLimit, OrderBy: page.OrderBy, OrderDir: page.OrderDir,
		Type: page.Type, FieldFilter: page.FieldFilter, Statuses: page.Statuses, Authors: page.Authors,
		ExcludedAuthors: page.ExcludedAuthors, Before: page.Before, After: page.After, Search: page.Search,
	})
	if err != nil {
		return nil, err
	}
	rows := make([]db.ListContentRow, len(nested))
	for i, row := range nested {
		rows[i] = db.ListContentRow(row)
	}
	return rows, nil
}

// narrowing returns the arguments the shared list and count queries narrow the filter by.
func narrowing(f content.Filter) db.CountContentParams {
	statuses := make([]string, 0, len(f.Statuses))
	for _, status := range f.Statuses {
		statuses = append(statuses, string(status))
	}
	return db.CountContentParams{
		Type:            f.Type,
		FieldFilter:     termsJSON(f.Fields),
		Statuses:        statuses,
		Authors:         append([]uuid.UUID{}, f.Authors...),
		ExcludedAuthors: append([]uuid.UUID{}, f.ExcludeAuthors...),
		Before:          f.Before,
		After:           f.After,
		Search:          escapeLike(f.Search),
	}
}

// termsJSON returns the field terms as the jsonb containment object, an empty one matching every row.
func termsJSON(terms map[string]any) []byte {
	if len(terms) == 0 {
		return []byte("{}")
	}
	raw, _ := json.Marshal(terms)
	return raw
}

// listedContent returns one listed row as the item it stands for, beside its author's name and its parent's title.
func listedContent(row db.ListContentRow) content.ListedItem {
	return content.ListedItem{
		Content: content.Content{
			ID:          row.ID,
			Type:        row.Type,
			ParentID:    row.ParentID,
			Path:        row.Path,
			Status:      content.Status(row.Status),
			Slug:        row.Slug,
			Title:       row.Title,
			Excerpt:     row.Excerpt,
			AuthorID:    row.AuthorID,
			PublishedAt: utcOrNil(row.PublishedAt),
			CreatedAt:   row.CreatedAt.UTC(),
			UpdatedAt:   row.UpdatedAt.UTC(),
			Fields:      row.Fields,
		},
		AuthorName:  row.AuthorName,
		ParentTitle: row.ParentTitle,
	}
}

// Update stores the item's editable fields and any snapshot, settling its address among its siblings.
func (s *ContentStore) Update(
	ctx context.Context, c content.Content, expectedUpdatedAt time.Time, snapshot *content.Revision,
	revisionCap int,
) (content.Content, error) {
	slug, err := s.freeSiblingSlug(ctx, c)
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: update content: %w", err)
	}
	updated, err := s.update(ctx, c, slug, expectedUpdatedAt, snapshot, revisionCap)
	if isSlugTaken(err) {
		return content.Content{}, content.ErrSlugTaken
	}
	return updated, err
}

// freeSiblingSlug returns the slug the item may carry beside its siblings.
func (s *ContentStore) freeSiblingSlug(ctx context.Context, c content.Content) (string, error) {
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		slug := numberedSlug(c.Slug, attempt)
		taken, err := s.queries.SiblingSlugTaken(ctx, db.SiblingSlugTakenParams{
			Type:     c.Type,
			ParentID: c.ParentID,
			Slug:     slug,
			ID:       c.ID,
		})
		if err != nil {
			return "", fmt.Errorf("read sibling slugs: %w", err)
		}
		if !taken {
			return slug, nil
		}
	}
	return "", content.ErrSlugTaken
}

// update writes the content item, its descendants and any snapshot under slug.
func (s *ContentStore) update(
	ctx context.Context, c content.Content, slug string, expectedUpdatedAt time.Time,
	snapshot *content.Revision, revisionCap int,
) (content.Content, error) {
	var updated content.Content
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if _, err := tx.Exec(ctx, deferAddressCheck); err != nil {
			return err
		}
		prefix, err := movable(ctx, queries, c, expectedUpdatedAt, content.AddressPrefix(c.Path, c.Slug))
		if err != nil {
			return err
		}
		path := content.AddressUnder(prefix, slug)
		resolved, err := resolvedTargets(ctx, queries, c)
		if err != nil {
			return err
		}
		row, err := writeContent(ctx, queries, db.UpdateContentParams{
			ID:                c.ID,
			ParentID:          c.ParentID,
			Path:              path,
			Status:            string(c.Status),
			Slug:              slug,
			Title:             c.Title,
			Content:           c.Content,
			Excerpt:           c.Excerpt,
			Fields:            storedValues(c.Fields),
			PublishedAt:       c.PublishedAt,
			UpdatedAt:         c.UpdatedAt,
			ExpectedUpdatedAt: expectedUpdatedAt,
		})
		if err != nil {
			return err
		}
		updated = toContent(row)
		if err := writeRelations(ctx, queries, c, resolved); err != nil {
			return err
		}
		if err := queries.MoveDescendants(ctx, db.MoveDescendantsParams{
			ID:        c.ID,
			Path:      path,
			UpdatedAt: c.UpdatedAt,
		}); err != nil {
			return err
		}
		if snapshot == nil {
			return nil
		}
		return snapshotRevision(ctx, queries, *snapshot, revisionCap)
	})
	if err != nil {
		return content.Content{}, writeFailure(err)
	}
	return updated, nil
}

// writeContent applies the edited fields, telling a stale write apart from a missing item.
func writeContent(ctx context.Context, queries *db.Queries, p db.UpdateContentParams) (db.CoreContent, error) {
	row, err := queries.UpdateContent(ctx, p)
	if !errors.Is(err, pgx.ErrNoRows) {
		return row, err
	}
	if _, err := byID(ctx, queries, p.ID); err != nil {
		return db.CoreContent{}, err
	}
	return db.CoreContent{}, content.ErrConflict
}

// resolvedTargets cleans the item's values of a target that is gone and returns what the index may hold.
func resolvedTargets(
	ctx context.Context, queries *db.Queries, c content.Content,
) ([]content.FieldTargets, error) {
	matching, err := declaredValues(ctx, queries, c)
	if err != nil {
		return nil, err
	}
	before, err := valuesBefore(ctx, queries, c.ID)
	if err != nil {
		return nil, err
	}
	return resolveTargets(ctx, queries, matching, c, before)
}

// valuesBefore returns the values the item holds until this write replaces them.
func valuesBefore(ctx context.Context, queries *db.Queries, id uuid.UUID) (content.Values, error) {
	held, err := queries.ValuesOfContent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return held, nil
}

// declaredValues returns the groups matching the item's type once every held value is declared.
func declaredValues(ctx context.Context, queries *db.Queries, c content.Content) ([]int32, error) {
	if err := valuesDeclared(ctx, queries, c); err != nil {
		return nil, err
	}
	return matchingGroupIDs(ctx, queries, c.Type)
}

// nestable returns the reason the item's type no longer takes it under a parent, holding the type as it stands.
func nestable(ctx context.Context, queries *db.Queries, c content.Content) error {
	if c.ParentID == nil {
		return nil
	}
	nests, err := queries.LockTypeNesting(ctx, c.Type)
	if err != nil {
		return err
	}
	if !nests {
		return content.ErrNotHierarchical
	}
	return nil
}

// fileable returns the address prefix a new item takes, holding the type and its parent as they stand.
func fileable(ctx context.Context, queries *db.Queries, c content.Content, prefix string) (string, error) {
	if err := nestable(ctx, queries, c); err != nil {
		return "", err
	}
	if c.ParentID == nil {
		return prefix, nil
	}
	return parentHolds(ctx, queries, *c.ParentID, c.ID)
}

// movable returns the address prefix an edit files the item under, holding the type and any new parent.
func movable(
	ctx context.Context, queries *db.Queries, c content.Content, expected time.Time, prefix string,
) (string, error) {
	if c.ParentID == nil {
		return prefix, nil
	}
	stored, err := byID(ctx, queries, c.ID)
	if err != nil {
		return "", err
	}
	if !stored.UpdatedAt.Equal(expected) {
		return "", content.ErrConflict
	}
	if stored.ParentID != nil && *stored.ParentID == *c.ParentID {
		return prefix, nestable(ctx, queries, c)
	}
	return movedUnder(ctx, queries, c)
}

// movedUnder returns the address of the new parent, holding the type against every other move.
func movedUnder(ctx context.Context, queries *db.Queries, c content.Content) (string, error) {
	nests, err := queries.LockTypeForMove(ctx, c.Type)
	if err != nil {
		return "", err
	}
	if !nests {
		return "", content.ErrNotHierarchical
	}
	return parentHolds(ctx, queries, *c.ParentID, c.ID)
}

// parentHolds locks the parent and returns its address, refusing one gone, in the trash, or under the item.
func parentHolds(ctx context.Context, queries *db.Queries, id, child uuid.UUID) (string, error) {
	parent, err := queries.LockParent(ctx, db.LockParentParams{ID: id, ChildID: child})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", content.ErrParentType
	}
	if err != nil {
		return "", err
	}
	if content.Status(parent.Status) == content.StatusTrash {
		return "", content.ErrParentTrashed
	}
	if parent.HoldsChild {
		return "", content.ErrCycle
	}
	return parent.Path, nil
}

// valuesDeclared refuses a value whose field no group declares at all.
func valuesDeclared(ctx context.Context, queries *db.Queries, c content.Content) error {
	keys, err := queries.LockDeclaredFieldKeys(ctx)
	if err != nil {
		return err
	}
	declared := make(map[string]bool, len(keys))
	for _, key := range keys {
		declared[key] = true
	}
	for key := range c.Fields {
		if !declared[key] {
			return fmt.Errorf("%w: %s", content.ErrUnknownField, key)
		}
	}
	return nil
}

// writeRefusals lists the domain errors a failed write reports unchanged.
var writeRefusals = []error{
	content.ErrNotFound, content.ErrConflict, content.ErrUnknownField, content.ErrNotHierarchical,
	content.ErrTargetNotFound, content.ErrTargetType, content.ErrParentType, content.ErrParentTrashed,
	content.ErrCycle,
}

// writeFailure returns the error the write carries, and wraps anything else.
func writeFailure(err error) error {
	if isSlugTaken(err) {
		return err
	}
	for _, domain := range writeRefusals {
		if errors.Is(err, domain) {
			return err
		}
	}
	if isTargetGone(err) {
		return content.ErrTargetNotFound
	}
	return fmt.Errorf("postgres: update content: %w", err)
}

// Depth returns how many levels of content nest below the item.
func (s *ContentStore) Depth(ctx context.Context, id uuid.UUID) (int, error) {
	levels, err := s.queries.ContentDepth(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("postgres: content depth: %w", err)
	}
	return int(levels), nil
}

// Trash marks the content item trashed, frees its address for reuse and drops its parked words.
func (s *ContentStore) Trash(ctx context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	var trashed content.Content
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := leavable(ctx, queries, id); err != nil {
			return err
		}
		row, err := queries.TrashContent(ctx, db.TrashContentParams{
			ID:        id,
			Suffix:    trashSuffix(),
			UpdatedAt: updatedAt,
		})
		if err != nil {
			return err
		}
		trashed = toContent(row)
		if err := queries.DeleteAutosavesOfContent(ctx, id); err != nil {
			return err
		}
		return queries.RefreshRelationVisibility(ctx, id)
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return content.Content{}, content.ErrNotFound
		}
		if errors.Is(err, content.ErrNotFound) ||
			errors.Is(err, content.ErrHoldsChildren) ||
			errors.Is(err, content.ErrInvalidTransition) {
			return content.Content{}, err
		}
		return content.Content{}, fmt.Errorf("postgres: trash content: %w", err)
	}
	return trashed, nil
}

// leavable returns the reason the locked item may not go to the trash, if there is one.
func leavable(ctx context.Context, queries *db.Queries, id uuid.UUID) error {
	row, err := queries.LockContent(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return content.ErrNotFound
	}
	if err != nil {
		return err
	}
	if content.Status(row.Status) == content.StatusTrash {
		return content.Refuse(content.ErrInvalidTransition, "content_already_trashed",
			"content: the item is already in the trash", nil)
	}
	held, err := queries.CountChildren(ctx, &id)
	if err != nil {
		return err
	}
	if held > 0 {
		return content.ErrHoldsChildren
	}
	return nil
}

// Restore returns a trashed item to draft under its original slug when free, and refuses one out of the trash.
func (s *ContentStore) Restore(ctx context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	var restored content.Content
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		if err := restorable(ctx, queries, id); err != nil {
			return err
		}
		row, err := restoredRow(ctx, queries, tx, id, updatedAt)
		if err != nil {
			return err
		}
		restored = toContent(row)
		return queries.RefreshRelationVisibility(ctx, id)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if errors.Is(err, content.ErrInvalidTransition) {
		return content.Content{}, err
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("postgres: restore content: %w", err)
	}
	return restored, nil
}

// restorable locks the item, reporting why it cannot leave the trash.
func restorable(ctx context.Context, queries *db.Queries, id uuid.UUID) error {
	row, err := queries.LockContent(ctx, id)
	if err != nil {
		return err
	}
	held := toContent(row)
	return held.Restore()
}

// restoredRow returns the item to draft under its original slug, or under the trashed one when that is taken.
func restoredRow(
	ctx context.Context, queries *db.Queries, tx pgx.Tx, id uuid.UUID, updatedAt time.Time,
) (db.CoreContent, error) {
	var row db.CoreContent
	err := pgx.BeginFunc(ctx, tx, func(attempt pgx.Tx) error {
		var taken error
		row, taken = queries.WithTx(attempt).RestoreContent(
			ctx, db.RestoreContentParams{ID: id, UpdatedAt: updatedAt},
		)
		return taken
	})
	if !isSlugTaken(err) {
		return row, err
	}
	return queries.RestoreContentKeepingSlug(ctx, db.RestoreContentKeepingSlugParams{ID: id, UpdatedAt: updatedAt})
}

// Counts returns the number of items of the type in each status.
func (s *ContentStore) Counts(ctx context.Context, contentType string) (map[content.Status]int, error) {
	rows, err := s.queries.CountContentByStatus(ctx, contentType)
	if err != nil {
		return nil, fmt.Errorf("postgres: count content by status: %w", err)
	}
	counts := make(map[content.Status]int, len(rows))
	for _, row := range rows {
		counts[content.Status(row.Status)] = int(row.Total)
	}
	return counts, nil
}

// Delete removes the content item, or reports [content.ErrNotFound].
func (s *ContentStore) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := s.queries.DeleteContent(ctx, id)
	if err != nil {
		return fmt.Errorf("postgres: delete content: %w", err)
	}
	if rows == 0 {
		return content.ErrNotFound
	}
	return nil
}

// EmptyTrash deletes the type's trash, only the author's when one is named, and counts what it deleted and left.
func (s *ContentStore) EmptyTrash(ctx context.Context, contentType string, author *uuid.UUID) (int, int, error) {
	var deleted, kept int64
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		var err error
		deleted, err = queries.DeleteTrash(ctx, db.DeleteTrashParams{Type: contentType, AuthorID: author})
		if err != nil {
			return err
		}
		kept, err = queries.CountTrash(ctx, contentType)
		return err
	})
	if err != nil {
		return 0, 0, fmt.Errorf("postgres: empty the trash: %w", err)
	}
	return int(deleted), int(kept), nil
}

// pageOffset returns the row offset of a page, bounded to what the query accepts.
func pageOffset(page, perPage int) int32 {
	if perPage > 0 && page-1 > math.MaxInt32/perPage {
		return math.MaxInt32
	}
	return int32((page - 1) * perPage)
}

// utcOrNil returns the instant in UTC, or nil when it is unset.
func utcOrNil(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

// numberedSlug returns slug for the first attempt, and slug with the attempt
// number appended after that.
func numberedSlug(slug string, attempt int) string {
	if attempt == 1 {
		return slug
	}
	return slug + "-" + strconv.Itoa(attempt)
}

// identifiedSlug returns slug carrying the id of the content item holding it.
func identifiedSlug(slug string, id uuid.UUID) string {
	return slug + "-" + strings.ReplaceAll(id.String(), "-", "")
}

// trashSuffix returns the marker appended to a trashed content item's slug.
func trashSuffix() string {
	var b strings.Builder
	b.WriteString("-trashed-")
	for range trashSuffixLength {
		b.WriteByte(trashSuffixAlphabet[rand.IntN(len(trashSuffixAlphabet))])
	}
	return b.String()
}

// escapeLike returns search with the pattern characters of LIKE escaped.
func escapeLike(search string) string {
	return strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(search)
}

// isSlugTaken reports whether err is the unique violation over a type and slug.
func isSlugTaken(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.ConstraintName == slugConstraint
}
