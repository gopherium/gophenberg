// SPDX-License-Identifier: Apache-2.0

package sqlite_test

import (
	"cmp"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gopherium/framework/dbkit"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/content/contenttest"
	"github.com/gopherium/gophenberg/internal/content/typestore"
	"github.com/gopherium/gophenberg/internal/sqlite"
)

// plantTypeRow stores a content type that is not the default, as the type store stores one.
const plantTypeRow = `INSERT INTO core_content_types (
    key, singular_label, plural_label, route_word, hierarchical, revisions, revision_cap, page_kind,
    is_default, active, created_at, updated_at, description
)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, 0, ?9, ?10, ?10, ?11)`

// plantGroupRow stores an active field group at the end of the order and answers its id.
const plantGroupRow = `INSERT INTO core_field_groups (key, title, location, position, active, created_at, updated_at)
VALUES (?1, ?2, ?3, (SELECT count(*) FROM core_field_groups), 1, ?4, ?4)
RETURNING id`

// plantFieldRow stores a field at the top of its group, after the fields the group already holds, and answers its id.
const plantFieldRow = `INSERT INTO core_content_fields (
    key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings, depth
)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?7, (SELECT count(*) FROM core_content_fields WHERE group_id = ?8), ?8, '{}', 0)
RETURNING id`

// plantSubFieldRow stores a field one level inside a container field of its group, after its siblings.
const plantSubFieldRow = `INSERT INTO core_content_fields (
    key, label, kind, relates_to, many, required, created_at, updated_at, position, group_id, settings,
    parent_field_id, depth
)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?7, (SELECT count(*) + 1 FROM core_content_fields WHERE parent_field_id = ?8),
    (SELECT group_id FROM core_content_fields WHERE id = ?8), '{}', ?8, 1)
RETURNING id`

// plantItemRow stores a draft item of a type at an address, under a parent or at the top, holding raw field values.
const plantItemRow = `INSERT INTO core_content
(id, type, status, slug, path, title, author_id, created_at, updated_at, parent_id, fields)
VALUES (?1, ?2, 'draft', ?3, ?4, 'Planted', ?5, ?6, ?6, ?7, ?8)`

// plantValuesRow stores a revision of an item holding raw field values, credited to an author.
const plantValuesRow = `INSERT INTO core_content_revisions
(id, content_id, kind, author_id, title, content, excerpt, fields, created_at)
VALUES (?1, ?2, 'revision', ?3, 'Planted', '', '', ?4, ?5)`

// plantIndexRow stores one visible relation index row from an item through a field to another item.
const plantIndexRow = `INSERT INTO core_content_relations (from_id, field_id, to_id, position, sort_at, visible)
VALUES (?1, ?2, ?3, 1, ?4, 1)`

// plantedAt is the stamp every planted definition carries.
var plantedAt = time.Date(2026, time.March, 2, 9, 0, 0, 0, time.UTC)

// writtenAt is the stamp the items these tests store start from.
var writtenAt = time.Date(2026, time.March, 3, 9, 0, 0, 0, time.UTC)

// plantedSite is a content store over a migrated copy of its own, beside its handle and the author it writes as.
type plantedSite struct {
	handle *sql.DB
	store  *sqlite.ContentStore
	author uuid.UUID
}

// newPlantedSite returns a content store over a fresh migrated copy, holding the post type and one author.
func newPlantedSite(t *testing.T) plantedSite {
	t.Helper()
	return plantedOn(t, migrated(t))
}

// plantedOn returns a content store over the handle, holding one author.
func plantedOn(t *testing.T, handle *sql.DB) plantedSite {
	t.Helper()
	return plantedSite{
		handle: handle, store: sqlite.NewContentStore(handle),
		author: addAuthor(t, handle, contenttest.DefaultAuthor),
	}
}

// newPageSite returns a planted site that also holds the nesting page type.
func newPageSite(t *testing.T) plantedSite {
	t.Helper()
	site := newPlantedSite(t)
	plantType(t, site.handle, contenttest.PageType())
	return site
}

// item stores a draft of the type under parent, stamped at the moment, and returns it as stored.
func (s plantedSite) item(
	t *testing.T, kind content.Type, parent *content.Content, title string, at time.Time,
) content.Content {
	t.Helper()
	built, err := content.New(kind, parent, title, s.author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	built.CreatedAt, built.UpdatedAt = at, at
	created, err := s.store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return created
}

// post stores a draft post holding the body and the values, stamped at the start moment, and returns it as stored.
func (s plantedSite) post(t *testing.T, title, body string, values content.Values) content.Content {
	t.Helper()
	built := contenttest.MustPost(t, title, s.author)
	built.Content, built.Fields = body, values
	built.CreatedAt, built.UpdatedAt = writtenAt, writtenAt
	created, err := s.store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return created
}

// page stores a draft page under parent, stamped at the moment, and returns it as stored.
func (s plantedSite) page(t *testing.T, parent *content.Content, title string, at time.Time) content.Content {
	t.Helper()
	return s.item(t, contenttest.PageType(), parent, title, at)
}

// edit stores the edited item over the stamp it was read at and returns it as stored.
func (s plantedSite) edit(t *testing.T, edited content.Content, read time.Time) content.Content {
	t.Helper()
	updated, err := s.store.Update(t.Context(), edited, read, nil, 0)
	if err != nil {
		t.Fatalf("Update(%q) error = %v, want nil", edited.Title, err)
	}
	return updated
}

// publish stores the item published at the moment and returns it as stored.
func (s plantedSite) publish(t *testing.T, item content.Content, at time.Time) content.Content {
	t.Helper()
	edited := item
	edited.Status, edited.PublishedAt, edited.UpdatedAt = content.StatusPublished, &at, at
	return s.edit(t, edited, item.UpdatedAt)
}

// titles returns the titles the store lists under the filter and its total, on a first page of twenty unless named.
func (s plantedSite) titles(t *testing.T, f content.Filter) ([]string, int) {
	t.Helper()
	f.Page, f.PerPage = cmp.Or(f.Page, 1), cmp.Or(f.PerPage, 20)
	rows, total, err := s.store.List(t.Context(), f)
	if err != nil {
		t.Fatalf("List(%+v) error = %v, want nil", f, err)
	}
	return contenttest.TitlesOf(rows), total
}

// plantType stores the type as the type store stores one.
func plantType(t *testing.T, handle *sql.DB, kind content.Type) {
	t.Helper()
	mustExec(t, handle, plantTypeRow, kind.Key, kind.SingularLabel, kind.PluralLabel, kind.RouteWord,
		kind.Hierarchical, kind.Revisions, kind.RevisionCap, string(kind.PageKind), kind.Active,
		dbkit.Time{Time: plantedAt}, kind.Description)
}

// plantNestingPosts lets the built-in post type nest, as the type store stores the change, and returns the type.
func plantNestingPosts(t *testing.T, handle *sql.DB) content.Type {
	t.Helper()
	mustExec(t, handle, "UPDATE core_content_types SET hierarchical = 1 WHERE key = ?", content.TypePost)
	root := contenttest.PostType()
	root.Hierarchical = true
	return root
}

// locationOf returns the stored location of a group serving the type.
func locationOf(typeKey string) []byte {
	location, _ := json.Marshal(content.Rules{{{
		Source: content.ScreenContentType, Operator: content.OperatorIs, Value: typeKey,
	}}}.Normalize())
	return location
}

// plantGroup stores a group serving the type as the type store stores one, and returns its id.
func plantGroup(t *testing.T, handle *sql.DB, typeKey string) int64 {
	t.Helper()
	var id int64
	err := handle.QueryRowContext(t.Context(), plantGroupRow, typeKey+"_details", "Details",
		string(locationOf(typeKey)), dbkit.Time{Time: plantedAt}).Scan(&id)
	if err != nil {
		t.Fatalf("planting the group of %s: %v", typeKey, err)
	}
	return id
}

// plantField stores the field at the top of the group as the type store stores one, and returns its id.
func plantField(t *testing.T, handle *sql.DB, groupID int64, f content.Field) int {
	t.Helper()
	return plantFieldAs(t, handle, plantFieldRow, f, groupID)
}

// plantSubField stores the field inside the container field as the type store stores one, and returns its id.
func plantSubField(t *testing.T, handle *sql.DB, parentID int, f content.Field) int {
	t.Helper()
	return plantFieldAs(t, handle, plantSubFieldRow, f, int64(parentID))
}

// plantFieldAs stores the field through the statement at the place its last argument names, and returns its id.
func plantFieldAs(t *testing.T, handle *sql.DB, statement string, f content.Field, place int64) int {
	t.Helper()
	var relatesTo *string
	if f.RelatesTo != "" {
		relatesTo = &f.RelatesTo
	}
	var id int
	err := handle.QueryRowContext(t.Context(), statement, f.Key, f.Label, string(f.Kind), relatesTo, f.Many,
		f.Required, dbkit.Time{Time: plantedAt}, place).Scan(&id)
	if err != nil {
		t.Fatalf("planting the field %s: %v", f.Key, err)
	}
	return id
}

// typedSite is a planted site with the SQLite type store on the same file.
type typedSite struct {
	plantedSite
	types *sqlite.TypeStore
}

// newTypedSite returns a planted site over a fresh migrated copy with its type store, holding the post type alone.
func newTypedSite(t *testing.T) typedSite {
	t.Helper()
	site := newPlantedSite(t)
	return typedSite{plantedSite: site, types: sqlite.NewTypeStore(site.handle)}
}

// fieldInGroup stores the field at the top of the group through the type store and returns it as stored.
func fieldInGroup(t *testing.T, types content.TypeStore, groupID int, f content.Field) content.Field {
	t.Helper()
	stored, err := types.CreateFieldInGroup(t.Context(), groupID, f, nil)
	if err != nil {
		t.Fatalf("CreateFieldInGroup(%s) error = %v, want nil", f.Key, err)
	}
	return stored
}

// pointedThrough stores an item of the first type pointing at an item of the second through each field, in the index.
func (s plantedSite) pointedThrough(t *testing.T, from, to string, fields ...int) {
	t.Helper()
	pointing, _ := s.plantTyped(t, from, "pointing", "{}")
	pointed, _ := s.plantTyped(t, to, "pointed", "{}")
	for _, field := range fields {
		s.plantIndex(t, pointing, pointed, field)
	}
}

// plantItem stores a draft item of the type at the address, under parent or at the top for nil, holding the values.
func (s plantedSite) plantItem(t *testing.T, typeKey, address string, parent *uuid.UUID, values string) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV7())
	slug := address[strings.LastIndex(address, "/")+1:]
	mustExec(t, s.handle, plantItemRow, id, typeKey, slug, address, s.author, dbkit.Time{Time: writtenAt}, parent,
		values)
	return id
}

// plantTyped stores a draft item of the type at the top under the slug and one revision of it, both holding the values.
func (s plantedSite) plantTyped(t *testing.T, typeKey, slug, values string) (uuid.UUID, uuid.UUID) {
	t.Helper()
	item := s.plantItem(t, typeKey, slug, nil, values)
	revision := uuid.Must(uuid.NewV7())
	mustExec(t, s.handle, plantValuesRow, revision, item, s.author, values, dbkit.Time{Time: writtenAt})
	return item, revision
}

// plantIndex stores a visible relation index row from the item through the field to the target.
func (s plantedSite) plantIndex(t *testing.T, from, to uuid.UUID, field int) {
	t.Helper()
	mustExec(t, s.handle, plantIndexRow, from, field, to, dbkit.Time{Time: writtenAt})
}

// textOf returns the single text lookup with args answers on handle.
func textOf(t *testing.T, handle *sql.DB, lookup string, args ...any) string {
	t.Helper()
	var held string
	if err := handle.QueryRowContext(t.Context(), lookup, args...).Scan(&held); err != nil {
		t.Fatalf("%s: %v", lookup, err)
	}
	return held
}

// groupOfField returns the group the field row sits in, straight from the table.
func groupOfField(t *testing.T, handle *sql.DB, id int) int {
	t.Helper()
	return int(count(t, handle, "SELECT group_id FROM core_content_fields WHERE id = ?", id))
}

// positionOf returns the stored position of the field row the identity names.
func positionOf(t *testing.T, handle *sql.DB, id int) int {
	t.Helper()
	return int(count(t, handle, "SELECT position FROM core_content_fields WHERE id = ?", id))
}

// standsUnder fails t unless the field row stands in the group under the parent at the depth, 0 naming the top.
func standsUnder(t *testing.T, handle *sql.DB, id, group, parent, depth int) {
	t.Helper()
	var held struct{ group, parent, depth int }
	if err := handle.QueryRowContext(t.Context(),
		"SELECT group_id, coalesce(parent_field_id, 0), depth FROM core_content_fields WHERE id = ?", id,
	).Scan(&held.group, &held.parent, &held.depth); err != nil {
		t.Fatalf("reading the row of field %d: %v", id, err)
	}
	if held.group != group || held.parent != parent || held.depth != depth {
		t.Errorf("field %d stands in group %d under %d at depth %d, want group %d under %d at depth %d",
			id, held.group, held.parent, held.depth, group, parent, depth)
	}
}

func TestPlantedPostGroupDeclaresTheValuesAPostCarries(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	store := sqlite.NewContentStore(handle)
	group := plantGroup(t, handle, content.TypePost)
	plantField(t, handle, group, contenttest.PostField("subtitle", content.FieldKindText))
	post := contenttest.MustPost(t, "Harbor Notes", addAuthor(t, handle, contenttest.DefaultAuthor))
	post.Fields = content.Values{"subtitle": "Morning tide"}

	created, err := store.Create(t.Context(), post)
	if err != nil {
		t.Fatalf("Create() with a value of the planted field error = %v, want nil", err)
	}

	stored, err := store.ByID(t.Context(), created.ID)
	if err != nil || !maps.Equal(stored.Fields, post.Fields) {
		t.Errorf("ByID() = %v, %v, want the planted field's value %v", stored.Fields, err, post.Fields)
	}
}

func TestPlantedDefinitionsReadBackEveryColumnTheContentStoreCopies(t *testing.T) {
	t.Parallel()

	handle := migrated(t)
	revised := plantedAt.Add(time.Hour)
	serving, retired := plantGroup(t, handle, content.TypePost), plantGroup(t, handle, "page")
	mustExec(t, handle, "UPDATE core_field_groups SET active = 0, origin = ?, updated_at = ? WHERE id = ?", "events",
		dbkit.Time{Time: revised}, retired)
	related := plantField(t, handle, serving, content.Field{
		Key: "related", Label: "Related", Kind: content.FieldKindRelation, RelatesTo: content.TypePost, Many: true,
	})
	team := plantField(t, handle, serving, content.Field{
		Key: "team", Label: "Team", Kind: content.FieldKindRepeater, Required: true,
	})
	mustExec(t, handle, "UPDATE core_content_fields SET origin = ?, settings = ?, updated_at = ? WHERE id = ?",
		"events", `{"layout":"table"}`, dbkit.Time{Time: revised}, team)
	role := plantSubField(t, handle, team, content.Field{Key: "role", Label: "Role", Kind: content.FieldKindText})
	group := int(serving)
	wantGroups := []typestore.GroupRow{
		{ID: group, Key: "post_details", Title: "Details", Location: locationOf(content.TypePost), Active: true,
			CreatedAt: plantedAt, UpdatedAt: plantedAt},
		{ID: int(retired), Position: 1, Key: "page_details", Title: "Details", Origin: "events",
			Location: locationOf("page"), CreatedAt: plantedAt, UpdatedAt: revised},
	}
	wantFields := []typestore.FieldRow{
		{ID: related, GroupID: group, Key: "related", Label: "Related", Kind: "relation", RelatesTo: content.TypePost,
			Many: true, Settings: []byte("{}"), CreatedAt: plantedAt, UpdatedAt: plantedAt},
		{ID: team, GroupID: group, Key: "team", Label: "Team", Kind: "repeater", Origin: "events", Required: true,
			Settings: []byte(`{"layout":"table"}`), CreatedAt: plantedAt, UpdatedAt: revised},
		{ID: role, GroupID: group, ParentID: team, Depth: 1, Key: "role", Label: "Role", Kind: "text",
			Settings: []byte("{}"), CreatedAt: plantedAt, UpdatedAt: plantedAt},
	}
	reads := sqlite.FieldDefinitions(handle)

	groups, groupsErr := reads.ListFieldGroups(t.Context())
	fields, fieldsErr := reads.ListContentFields(t.Context())

	if got, want := listing(groups, groupText), listing(wantGroups, groupText); groupsErr != nil || got != want {
		t.Errorf("ListFieldGroups() error = %v, read back\n%s\nwant\n%s", groupsErr, got, want)
	}
	if got, want := listing(fields, fieldText), listing(wantFields, fieldText); fieldsErr != nil || got != want {
		t.Errorf("ListContentFields() error = %v, read back\n%s\nwant\n%s", fieldsErr, got, want)
	}
}

// listing returns each row on a line of its own, in the text the function gives it.
func listing[T any](rows []T, text func(T) string) string {
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		lines = append(lines, text(row))
	}
	return strings.Join(lines, "\n")
}

// groupText returns every column of the group row as one line, its stamps in UTC.
func groupText(g typestore.GroupRow) string {
	return fmt.Sprintf("group %d at %d key %q title %q origin %q location %s active %t created %v updated %v",
		g.ID, g.Position, g.Key, g.Title, g.Origin, g.Location, g.Active, g.CreatedAt.UTC(), g.UpdatedAt.UTC())
}

// fieldText returns every column of the field row as one line, its stamps in UTC.
func fieldText(f typestore.FieldRow) string {
	return fmt.Sprintf("field %d in %d under %d depth %d key %q label %q kind %s relates to %q origin %q many %t "+
		"required %t settings %s created %v updated %v", f.ID, f.GroupID, f.ParentID, f.Depth, f.Key, f.Label,
		f.Kind, f.RelatesTo, f.Origin, f.Many, f.Required, f.Settings, f.CreatedAt.UTC(), f.UpdatedAt.UTC())
}
