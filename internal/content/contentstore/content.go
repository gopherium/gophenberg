// SPDX-License-Identifier: Apache-2.0

package contentstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/typestore"
)

// slugAttempts bounds the suffixes tried when a slug is taken.
const slugAttempts = 20

// trashSuffixAlphabet holds the characters a trashed slug suffix draws from.
const trashSuffixAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// trashSuffixLength is the number of random characters in a trashed slug suffix.
const trashSuffixLength = 8

// Create stores a new content item, suffixing its slug until its address is free.
func (s *Store) Create(ctx context.Context, c content.Content) (content.Content, error) {
	prefix := content.AddressPrefix(c.Path, c.Slug)
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		created, err := s.create(ctx, c, prefix, numberedSlug(c.Slug, attempt))
		if slugRetries[s.broken(err)] {
			continue
		}
		return created, err
	}
	created, err := s.create(ctx, c, prefix, identifiedSlug(c.Slug, c.ID))
	if s.broken(err) == typestore.ContentPathUnique {
		return content.Content{}, content.ErrSlugTaken
	}
	return created, err
}

// create stores the content item under slug, addressed beneath prefix.
func (s *Store) create(
	ctx context.Context, c content.Content, prefix, slug string,
) (content.Content, error) {
	params := ContentRow{
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
	if err := content.CheckPaginationPaths([]string{params.Path}); err != nil {
		return content.Content{}, err
	}
	row, err := s.backend.Pool.CreateContent(ctx, params)
	if err != nil {
		return content.Content{}, fmt.Errorf("%s: create content: %w", s.backend.Engine, err)
	}
	return toContent(row), nil
}

// createHeld stores the item once its type still nests and every value it carries names a declared field.
func (s *Store) createHeld(
	ctx context.Context, c content.Content, params ContentRow,
) (content.Content, error) {
	var row ContentRow
	err := s.backend.InTx(ctx, func(queries Tx) error {
		prefix, err := fileable(ctx, queries, c, content.AddressPrefix(params.Path, params.Slug))
		if err != nil {
			return err
		}
		params.Path = content.AddressUnder(prefix, params.Slug)
		if err := content.CheckPaginationPaths([]string{params.Path}); err != nil {
			return err
		}
		matching, err := s.declaredValues(ctx, queries, c)
		if err != nil {
			return err
		}
		created, err := queries.CreateContent(ctx, params)
		if err != nil {
			return err
		}
		row = created
		resolved, err := s.resolveTargets(ctx, queries, matching, toContent(created), nil)
		if err != nil {
			return err
		}
		return writeRelations(ctx, queries, toContent(created), resolved)
	})
	if err != nil {
		return content.Content{}, s.writeFailure(err)
	}
	return toContent(row), nil
}

// PublishedByPath returns the published item at the address, or [content.ErrNotFound].
func (s *Store) PublishedByPath(ctx context.Context, path string) (content.Content, error) {
	row, err := s.backend.Pool.GetPublishedContentByPath(ctx, path)
	if errors.Is(err, sql.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("%s: get published content by path: %w", s.backend.Engine, err)
	}
	return toContent(row), nil
}

// Children returns how many items nest directly under the item.
func (s *Store) Children(ctx context.Context, id uuid.UUID) (int, error) {
	held, err := s.backend.Pool.CountChildren(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("%s: count children: %w", s.backend.Engine, err)
	}
	return int(held), nil
}

// ByID returns the content item with the given id, or [content.ErrNotFound].
func (s *Store) ByID(ctx context.Context, id uuid.UUID) (content.Content, error) {
	return s.byID(ctx, s.backend.Pool, id)
}

// byID returns the content item with the given id through queries, or [content.ErrNotFound].
func (s *Store) byID(ctx context.Context, queries Queries, id uuid.UUID) (content.Content, error) {
	row, err := queries.GetContent(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("%s: get content: %w", s.backend.Engine, err)
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
func toContent(row ContentRow) content.Content {
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
func (s *Store) List(ctx context.Context, f content.Filter) ([]content.ListedItem, int, error) {
	narrowed := narrowing(f)
	total, err := s.backend.Pool.CountContent(ctx, narrowed)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: count content: %w", s.backend.Engine, err)
	}
	rows, err := s.listRows(ctx, f, narrowed)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: list content: %w", s.backend.Engine, err)
	}
	items := make([]content.ListedItem, len(rows))
	for i, row := range rows {
		items[i] = listedContent(row)
	}
	return items, int(total), nil
}

// listRows returns the page the filter names, each child under its parent when the filter nests them.
func (s *Store) listRows(
	ctx context.Context, f content.Filter, narrowed Narrowing,
) ([]ListedRow, error) {
	page := Page{
		Narrowing: narrowed,
		OrderBy:   string(f.OrderBy),
		OrderDir:  string(f.Order),
		Limit:     f.PerPage,
		Offset:    content.PageOffset(f.Page, f.PerPage),
	}
	if !f.Hierarchy {
		return s.backend.Pool.ListContent(ctx, page)
	}
	return s.backend.Pool.ListNestedContent(ctx, page)
}

// narrowing returns the arguments the shared list and count queries narrow the filter by.
func narrowing(f content.Filter) Narrowing {
	statuses := make([]string, 0, len(f.Statuses))
	for _, status := range f.Statuses {
		statuses = append(statuses, string(status))
	}
	return Narrowing{
		Type:            f.Type,
		FieldFilter:     termsJSON(f.Fields),
		Statuses:        statuses,
		Authors:         append([]uuid.UUID{}, f.Authors...),
		ExcludedAuthors: append([]uuid.UUID{}, f.ExcludeAuthors...),
		Before:          f.Before,
		After:           f.After,
		Search:          f.Search,
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
func listedContent(row ListedRow) content.ListedItem {
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
func (s *Store) Update(
	ctx context.Context, c content.Content, expectedUpdatedAt time.Time, snapshot *content.Revision,
	revisionCap int,
) (content.Content, error) {
	slug, err := s.freeSiblingSlug(ctx, c)
	if err != nil {
		return content.Content{}, fmt.Errorf("%s: update content: %w", s.backend.Engine, err)
	}
	updated, err := s.update(ctx, c, slug, expectedUpdatedAt, snapshot, revisionCap)
	if s.broken(err) == typestore.ContentPathUnique {
		return content.Content{}, content.ErrSlugTaken
	}
	return updated, err
}

// freeSiblingSlug returns the slug the item may carry beside its siblings.
func (s *Store) freeSiblingSlug(ctx context.Context, c content.Content) (string, error) {
	for attempt := 1; attempt <= slugAttempts; attempt++ {
		slug := numberedSlug(c.Slug, attempt)
		taken, err := s.backend.Pool.SiblingSlugTaken(ctx, SiblingSlug{
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
func (s *Store) update(
	ctx context.Context, c content.Content, slug string, expectedUpdatedAt time.Time,
	snapshot *content.Revision, revisionCap int,
) (content.Content, error) {
	var updated content.Content
	err := s.backend.InTx(ctx, func(queries Tx) error {
		if err := queries.DeferAddressCheck(ctx); err != nil {
			return err
		}
		prefix, was, err := s.movable(ctx, queries, c, expectedUpdatedAt, content.AddressPrefix(c.Path, c.Slug))
		if err != nil {
			return err
		}
		path := content.AddressUnder(prefix, slug)
		resolved, err := s.resolvedTargets(ctx, queries, c)
		if err != nil {
			return err
		}
		edited := ContentRow{
			ID:          c.ID,
			ParentID:    c.ParentID,
			Path:        path,
			Status:      string(c.Status),
			Slug:        slug,
			Title:       c.Title,
			Content:     c.Content,
			Excerpt:     c.Excerpt,
			Fields:      storedValues(c.Fields),
			PublishedAt: c.PublishedAt,
			UpdatedAt:   c.UpdatedAt,
		}
		row, err := s.writeContent(ctx, queries, was, edited, expectedUpdatedAt)
		if err != nil {
			return err
		}
		updated = toContent(row)
		if err := writeRelations(ctx, queries, c, resolved); err != nil {
			return err
		}
		if err := moveDescendants(ctx, queries, was, edited); err != nil {
			return err
		}
		if snapshot == nil {
			return nil
		}
		return s.snapshotRevision(ctx, queries, *snapshot, revisionCap)
	})
	if err != nil {
		return content.Content{}, s.writeFailure(err)
	}
	return updated, nil
}

// writeContent applies the edited fields, telling a stale write apart from a missing item.
func (s *Store) writeContent(
	ctx context.Context, queries Queries, was string, p ContentRow, expected time.Time,
) (ContentRow, error) {
	if err := checkPaginationEdit(was, p.Path); err != nil {
		return ContentRow{}, err
	}
	row, err := queries.UpdateContent(ctx, p, expected)
	if !errors.Is(err, sql.ErrNoRows) {
		return row, err
	}
	if _, err := s.byID(ctx, queries, p.ID); err != nil {
		return ContentRow{}, err
	}
	return ContentRow{}, content.ErrConflict
}

// resolvedTargets cleans the item's values of a target that is gone and returns what the index may hold.
func (s *Store) resolvedTargets(
	ctx context.Context, queries Queries, c content.Content,
) ([]content.FieldTargets, error) {
	matching, err := s.declaredValues(ctx, queries, c)
	if err != nil {
		return nil, err
	}
	before, err := valuesBefore(ctx, queries, c.ID)
	if err != nil {
		return nil, err
	}
	return s.resolveTargets(ctx, queries, matching, c, before)
}

// valuesBefore returns the values the item holds until this write replaces them.
func valuesBefore(ctx context.Context, queries Queries, id uuid.UUID) (content.Values, error) {
	held, err := queries.ValuesOfContent(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return held, nil
}

// declaredValues returns the groups matching the item's type once every held value is declared.
func (s *Store) declaredValues(ctx context.Context, queries Queries, c content.Content) ([]int, error) {
	if err := valuesDeclared(ctx, queries, c); err != nil {
		return nil, err
	}
	return s.matchingGroupIDs(ctx, queries, c.Type)
}

// nestable returns the reason the item's type no longer takes it under a parent, holding the type as it stands.
func nestable(ctx context.Context, queries Queries, c content.Content) error {
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
func fileable(ctx context.Context, queries Queries, c content.Content, prefix string) (string, error) {
	if err := nestable(ctx, queries, c); err != nil {
		return "", err
	}
	if c.ParentID == nil {
		return prefix, nil
	}
	return parentHolds(ctx, queries, *c.ParentID, c.ID)
}

// movable returns the prefix an edit files the item under and its stored address, holding the type and any new parent.
func (s *Store) movable(
	ctx context.Context, queries Queries, c content.Content, expected time.Time, prefix string,
) (string, string, error) {
	stored, err := s.byID(ctx, queries, c.ID)
	if err != nil {
		return "", "", err
	}
	if !stored.UpdatedAt.Equal(expected) {
		return "", "", content.ErrConflict
	}
	if c.ParentID == nil {
		return prefix, stored.Path, nil
	}
	if stored.ParentID != nil && *stored.ParentID == *c.ParentID {
		return prefix, stored.Path, nestable(ctx, queries, c)
	}
	moved, err := movedUnder(ctx, queries, c)
	return moved, stored.Path, err
}

// movedUnder returns the address of the new parent, holding the type against every other move.
func movedUnder(ctx context.Context, queries Queries, c content.Content) (string, error) {
	nests, err := queries.LockTypeForMove(ctx, c.Type)
	if err != nil {
		return "", err
	}
	if !nests {
		return "", content.ErrNotHierarchical
	}
	return parentHolds(ctx, queries, *c.ParentID, c.ID)
}

// parentHolds locks the parent and every item above it, returning its address unless gone, trashed or under the item.
func parentHolds(ctx context.Context, queries Queries, id, child uuid.UUID) (string, error) {
	if err := queries.LockAncestors(ctx, id); err != nil {
		return "", err
	}
	parent, err := queries.LockParent(ctx, id, child)
	if errors.Is(err, sql.ErrNoRows) {
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
func valuesDeclared(ctx context.Context, queries Queries, c content.Content) error {
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

// domainWriteErrors lists the domain errors a failed write reports unchanged.
var domainWriteErrors = []error{
	content.ErrNotFound, content.ErrConflict, content.ErrUnknownField, content.ErrNotHierarchical,
	content.ErrTargetNotFound, content.ErrTargetType, content.ErrParentType, content.ErrParentTrashed,
	content.ErrCycle, content.ErrReservedAddress,
}

// writeFailure returns the error the write carries, and wraps anything else.
func (s *Store) writeFailure(err error) error {
	broken := s.broken(err)
	if broken == typestore.ContentPathUnique {
		return err
	}
	for _, domain := range domainWriteErrors {
		if errors.Is(err, domain) {
			return err
		}
	}
	if meant := writeErrors[broken]; meant != nil {
		return meant
	}
	return fmt.Errorf("%s: update content: %w", s.backend.Engine, err)
}

// Depth returns how many levels of content nest below the item.
func (s *Store) Depth(ctx context.Context, id uuid.UUID) (int, error) {
	levels, err := s.backend.Pool.ContentDepth(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("%s: content depth: %w", s.backend.Engine, err)
	}
	return int(levels), nil
}

// Trash marks the content item trashed, frees its address for reuse and carries a newer autosave along.
func (s *Store) Trash(ctx context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	var trashed content.Content
	err := s.backend.InTx(ctx, func(queries Tx) error {
		if err := leavable(ctx, queries, id); err != nil {
			return err
		}
		if err := queries.RestampNewerAutosaves(ctx, id, updatedAt); err != nil {
			return err
		}
		row, err := queries.TrashContent(ctx, id, trashSuffix(), updatedAt)
		if err != nil {
			return err
		}
		trashed = toContent(row)
		return queries.RefreshRelationVisibility(ctx, id)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return content.Content{}, content.ErrNotFound
		}
		if errors.Is(err, content.ErrNotFound) ||
			errors.Is(err, content.ErrHoldsChildren) ||
			errors.Is(err, content.ErrInvalidTransition) {
			return content.Content{}, err
		}
		return content.Content{}, fmt.Errorf("%s: trash content: %w", s.backend.Engine, err)
	}
	return trashed, nil
}

// leavable returns the reason the locked item may not go to the trash, if there is one.
func leavable(ctx context.Context, queries Queries, id uuid.UUID) error {
	row, err := queries.LockContent(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return content.ErrNotFound
	}
	if err != nil {
		return err
	}
	if content.Status(row.Status) == content.StatusTrash {
		return content.Refuse(content.ErrInvalidTransition, "content_already_trashed",
			"content: the item is already in the trash", nil)
	}
	held, err := queries.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if held > 0 {
		return content.ErrHoldsChildren
	}
	return nil
}

// Restore returns a trashed item to draft under its original slug when free, carrying a newer autosave along.
func (s *Store) Restore(ctx context.Context, id uuid.UUID, updatedAt time.Time) (content.Content, error) {
	var restored content.Content
	err := s.backend.InTx(ctx, func(queries Tx) error {
		if err := restorable(ctx, queries, id); err != nil {
			return err
		}
		if err := queries.RestampNewerAutosaves(ctx, id, updatedAt); err != nil {
			return err
		}
		row, err := s.restoredRow(ctx, queries, id, updatedAt)
		if err != nil {
			return err
		}
		restored = toContent(row)
		return queries.RefreshRelationVisibility(ctx, id)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return content.Content{}, content.ErrNotFound
	}
	if errors.Is(err, content.ErrInvalidTransition) {
		return content.Content{}, err
	}
	if err != nil {
		return content.Content{}, fmt.Errorf("%s: restore content: %w", s.backend.Engine, err)
	}
	return restored, nil
}

// restorable locks the item, reporting why it cannot leave the trash.
func restorable(ctx context.Context, queries Queries, id uuid.UUID) error {
	row, err := queries.LockContent(ctx, id)
	if err != nil {
		return err
	}
	held := toContent(row)
	return held.Restore()
}

// restoredRow returns the item to draft under its original slug, or the trashed one when that address is unavailable.
func (s *Store) restoredRow(
	ctx context.Context, queries Tx, id uuid.UUID, updatedAt time.Time,
) (ContentRow, error) {
	var row ContentRow
	err := queries.Savepoint(ctx, func(attempt Queries) error {
		var taken error
		row, taken = attempt.RestoreContent(ctx, id, updatedAt)
		if taken != nil {
			return taken
		}
		return content.CheckPaginationPaths([]string{row.Path})
	})
	switch {
	case s.broken(err) == typestore.ContentPathUnique, errors.Is(err, content.ErrReservedAddress):
		return queries.RestoreContentKeepingSlug(ctx, id, updatedAt)
	default:
		return row, err
	}
}

// CountsByAuthor returns how many published and private items of the type each author wrote.
func (s *Store) CountsByAuthor(ctx context.Context, contentType string) (map[uuid.UUID]int, error) {
	rows, err := s.backend.Pool.CountContentByAuthor(ctx, contentType)
	if err != nil {
		return nil, fmt.Errorf("%s: count content by author: %w", s.backend.Engine, err)
	}
	counts := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		counts[row.AuthorID] = int(row.Total)
	}
	return counts, nil
}

// Delete removes the content item, or reports [content.ErrNotFound].
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := s.backend.Pool.DeleteContent(ctx, id)
	if err != nil {
		return fmt.Errorf("%s: delete content: %w", s.backend.Engine, err)
	}
	if rows == 0 {
		return content.ErrNotFound
	}
	return nil
}

// EmptyTrash deletes the type's trash, only the author's when one is named, and counts what it deleted and left.
func (s *Store) EmptyTrash(ctx context.Context, contentType string, author *uuid.UUID) (int, int, error) {
	var deleted, kept int64
	err := s.backend.InTx(ctx, func(queries Tx) error {
		var err error
		deleted, err = queries.DeleteTrash(ctx, contentType, author)
		if err != nil {
			return err
		}
		kept, err = queries.CountTrash(ctx, contentType)
		return err
	})
	if err != nil {
		return 0, 0, fmt.Errorf("%s: empty the trash: %w", s.backend.Engine, err)
	}
	return int(deleted), int(kept), nil
}

// utcOrNil returns the instant in UTC, or nil when it is unset.
func utcOrNil(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	utc := t.UTC()
	return &utc
}

// numberedSlug returns slug for the first attempt, and slug with the attempt number appended after that.
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
