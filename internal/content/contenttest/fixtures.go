// SPDX-License-Identifier: Apache-2.0

package contenttest

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/gopherium/gophenberg/internal/content"
)

// DefaultAuthor is the name the suite gives the account most cases write as.
const DefaultAuthor = "Maria Perez"

// PostType returns the built-in post type as every store registers it.
func PostType() content.Type {
	return content.Type{
		Key: content.TypePost, SingularLabel: "Post", PluralLabel: "Posts",
		Revisions: true, RevisionCap: 100, PageKind: content.PageKindSingle,
		Default: true, Active: true, Description: "Manage the posts on this site.",
	}
}

// PageType returns a hierarchical page type answering under pages.
func PageType() content.Type {
	page := PostType()
	page.Key, page.SingularLabel, page.PluralLabel, page.Description = "page", "Page", "Pages", ""
	page.RouteWord, page.Hierarchical, page.Default = "pages", true, false
	return page
}

// FlatPage returns the page type told to stop nesting.
func FlatPage() content.Type {
	flat := PageType()
	flat.Hierarchical, flat.UpdatedAt = false, time.Now().UTC()
	return flat
}

// RegisterPageType stores the hierarchical page type so pages can nest.
func RegisterPageType(t *testing.T, types content.TypeStore) {
	t.Helper()
	if _, err := types.Create(t.Context(), PageType()); err != nil {
		t.Fatalf("registering the page type: %v", err)
	}
}

// MustPost returns a draft post with the given title.
func MustPost(t *testing.T, title string, author uuid.UUID) content.Content {
	t.Helper()
	p, err := content.New(PostType(), nil, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	return p
}

// MustCreate stores a draft post with the given title.
func MustCreate(t *testing.T, store content.Store, title string, author uuid.UUID) content.Content {
	t.Helper()
	created, err := store.Create(t.Context(), MustPost(t, title, author))
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return created
}

// StalePage returns a page built against the type as it nested, filed under the parent.
func StalePage(t *testing.T, parent *content.Content, title string, author uuid.UUID) content.Content {
	t.Helper()
	built, err := content.New(PageType(), parent, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	return built
}

// MustNest stores a page under the parent and returns it.
func MustNest(
	t *testing.T, store content.Store, parent *content.Content, title string, author uuid.UUID,
) content.Content {
	t.Helper()
	built, err := content.New(PageType(), parent, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	stored, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return stored
}

// Publish stores a post already published at the given time.
func Publish(t *testing.T, store content.Store, title string, author uuid.UUID, at time.Time) content.Content {
	t.Helper()
	created := MustCreate(t, store, title, author)
	edited := created
	edited.Status = content.StatusPublished
	edited.PublishedAt = &at
	edited.UpdatedAt = at
	updated, err := store.Update(t.Context(), edited, created.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("publishing %q: %v", title, err)
	}
	return updated
}

// PublishItem takes stored content public and returns it.
func PublishItem(t *testing.T, store content.Store, c content.Content) content.Content {
	t.Helper()
	version := c.UpdatedAt
	if err := c.Transition(content.StatusPublished); err != nil {
		t.Fatalf("Transition() error = %v, want nil", err)
	}
	published, err := store.Update(t.Context(), c, version, nil, 0)
	if err != nil {
		t.Fatalf("publishing %q: %v, want nil", c.Title, err)
	}
	return published
}

// Trash sends a stored item to the trash.
func Trash(t *testing.T, store content.Store, item content.Content) {
	t.Helper()
	if _, err := store.Trash(t.Context(), item.ID, time.Now().UTC()); err != nil {
		t.Fatalf("trashing %q: %v", item.Title, err)
	}
}

// StillStored reports whether the store still holds the item.
func StillStored(t *testing.T, store content.Store, item content.Content) bool {
	t.Helper()
	_, err := store.ByID(t.Context(), item.ID)
	if err != nil && !errors.Is(err, content.ErrNotFound) {
		t.Fatalf("ByID(%q) error = %v, want nil or not found", item.Title, err)
	}
	return err == nil
}

// AddressOf returns the address the store holds for the item.
func AddressOf(t *testing.T, store content.Store, id uuid.UUID) string {
	t.Helper()
	stored, err := store.ByID(t.Context(), id)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	return stored.Path
}

// MustSnapshot returns a revision of the post credited to author.
func MustSnapshot(t *testing.T, p content.Content, author uuid.UUID) *content.Revision {
	t.Helper()
	revision, err := content.NewRevision(p, content.RevisionKindRevision, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	return &revision
}

// MustAutosave returns an autosave of the post credited to author.
func MustAutosave(t *testing.T, p content.Content, author uuid.UUID) content.Revision {
	t.Helper()
	autosave, err := content.NewRevision(p, content.RevisionKindAutosave, author)
	if err != nil {
		t.Fatalf("NewRevision() error = %v, want nil", err)
	}
	return autosave
}

// EditTitle returns the post with a new title and a later timestamp.
func EditTitle(p content.Content, title string) content.Content {
	edited := p
	edited.Title = title
	edited.UpdatedAt = p.UpdatedAt.Add(time.Second)
	return edited
}

// CategoryType returns a content type items may be filed under.
func CategoryType() content.Type {
	return content.Type{
		Key: "category", SingularLabel: "Category", PluralLabel: "Categories",
		RouteWord: "categories", Revisions: true, RevisionCap: 100,
		PageKind: content.PageKindSingle, Active: true,
	}
}

// RegisterCategoryType stores the category type so posts may point at categories.
func RegisterCategoryType(t *testing.T, types content.TypeStore) {
	t.Helper()
	if _, err := types.Create(t.Context(), CategoryType()); err != nil {
		t.Fatalf("registering the category type: %v, want nil", err)
	}
}

// PostField returns a field of the kind on the post type under the key.
func PostField(key string, kind content.FieldKind) content.Field {
	return content.Field{TypeKey: content.TypePost, Key: key, Label: "A Field", Kind: kind}
}

// DeclareFields stores a group on the post type declaring the fields in order, and returns the group holding them.
func DeclareFields(t *testing.T, types content.TypeStore, fields ...content.Field) content.Group {
	t.Helper()
	group, err := types.CreateGroup(t.Context(), content.Group{
		Title: "Post details",
		Location: content.Rules{{{
			Source: content.ScreenContentType, Operator: content.OperatorIs, Value: content.TypePost,
		}}},
	})
	if err != nil {
		t.Fatalf("CreateGroup() error = %v, want nil", err)
	}
	for _, f := range fields {
		built, err := content.NewField(f)
		if err != nil {
			t.Fatalf("NewField(%q) error = %v, want nil", f.Key, err)
		}
		stored, err := types.CreateFieldInGroup(t.Context(), group.ID, built, nil)
		if err != nil {
			t.Fatalf("declaring %q: %v, want nil", f.Key, err)
		}
		group.Fields = append(group.Fields, stored)
	}
	return group
}

// RelateToCategories registers the category type and declares the categories relation on the post type.
func RelateToCategories(t *testing.T, types content.TypeStore) {
	t.Helper()
	RegisterCategoryType(t, types)
	DeclareFields(t, types, content.Field{
		TypeKey: content.TypePost, Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	})
}

// NamedTargets returns the identities as a relation value stores them.
func NamedTargets(targets []uuid.UUID) []any {
	named := make([]any, len(targets))
	for i, target := range targets {
		named[i] = target.String()
	}
	return named
}

// HeldTargets returns the identities the item's categories value names.
func HeldTargets(t *testing.T, c content.Content) []uuid.UUID {
	t.Helper()
	key := "categories"
	listed, named := c.Fields[key].([]any)
	if !named {
		return nil
	}
	held := make([]uuid.UUID, 0, len(listed))
	for _, raw := range listed {
		written, ok := raw.(string)
		if !ok {
			t.Fatalf("%q holds %v, want identities", key, raw)
		}
		id, err := uuid.Parse(written)
		if err != nil {
			t.Fatalf("%q holds %q, want an identity", key, written)
		}
		held = append(held, id)
	}
	return held
}

// StoredCategory stores one category item and returns it.
func StoredCategory(t *testing.T, store content.Store, title string, author uuid.UUID) content.Content {
	t.Helper()
	built, err := content.New(CategoryType(), nil, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	created, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return created
}

// FileUnder points the post's categories at the targets and stores it.
func FileUnder(t *testing.T, store content.Store, post content.Content, targets ...uuid.UUID) content.Content {
	t.Helper()
	version := post.UpdatedAt
	post.Fields = content.Values{"categories": NamedTargets(targets)}
	post.UpdatedAt = time.Now().UTC()
	updated, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("filing the post: %v, want nil", err)
	}
	return updated
}

// RowsPointing declares a repeater on posts holding a relation that points at categories, and returns the relation.
func RowsPointing(t *testing.T, types content.TypeStore) content.Field {
	t.Helper()
	rows := DeclareFields(t, types, content.Field{
		TypeKey: content.TypePost, Key: "team", Label: "Team", Kind: content.FieldKindRepeater,
	}).Fields[0]
	inside, err := content.NewSubField(content.Field{
		Key: "filed", Label: "Filed", Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	}, content.FieldKindRepeater)
	if err != nil {
		t.Fatalf("NewSubField(filed) error = %v, want nil", err)
	}
	declared, err := types.CreateSubField(t.Context(), rows.ID, inside, content.DefaultFieldDepth)
	if err != nil {
		t.Fatalf("declaring the relation inside the rows: %v, want nil", err)
	}
	return declared
}

// RowsFiledUnder returns the rows value pointing at every target through the relation inside it.
func RowsFiledUnder(targets ...uuid.UUID) content.Values {
	rows := make([]any, len(targets))
	for i, target := range targets {
		rows[i] = map[string]any{"filed": []any{target.String()}}
	}
	return content.Values{"team": rows}
}

// PointedAtBy returns how many published items point at the target through any field.
func PointedAtBy(t *testing.T, store content.Store, target uuid.UUID) int {
	t.Helper()
	_, total, err := store.RelatedTo(t.Context(), target, 1, 20)
	if err != nil {
		t.Fatalf("reading what points at the target: %v, want nil", err)
	}
	return total
}

// TitlesOf returns the titles of the listed items in order.
func TitlesOf(posts []content.ListedItem) []string {
	titles := make([]string, len(posts))
	for i, p := range posts {
		titles[i] = p.Title
	}
	return titles
}

// Holding stores a draft post carrying the given field values.
func Holding(
	t *testing.T, store content.Store, title string, author uuid.UUID, values content.Values,
) content.Content {
	t.Helper()
	created := MustCreate(t, store, title, author)
	created.Fields = values
	updated, err := store.Update(t.Context(), created, created.UpdatedAt, nil, 0)
	if err != nil {
		t.Fatalf("storing the values of %q: %v", title, err)
	}
	return updated
}
