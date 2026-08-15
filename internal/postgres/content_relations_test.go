// SPDX-License-Identifier: Apache-2.0

package postgres_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherium/gophenberg/internal/content"
	"github.com/gopherium/gophenberg/internal/postgres"
)

// categoryType returns a content type items may be filed under.
func categoryType() content.Type {
	return content.Type{
		Key: "category", SingularLabel: "Category", PluralLabel: "Categories",
		RouteWord: "categories", Revisions: true, RevisionCap: 100,
		PageKind: content.PageKindSingle, Active: true,
	}
}

// relatingStore returns a store, an author, the pool, and a post type carrying a relation field.
func relatingStore(t *testing.T) (*postgres.ContentStore, uuid.UUID, *pgxpool.Pool) {
	t.Helper()
	store, author, pool := newContentStoreWithPool(t)
	types := postgres.NewTypeStore(pool)
	if _, err := types.Create(t.Context(), categoryType()); err != nil {
		t.Fatalf("registering the category type: %v, want nil", err)
	}
	built, err := content.NewField(content.Field{
		TypeKey: "post", Key: "categories", Label: "Categories",
		Kind: content.FieldKindRelation, RelatesTo: "category", Many: true,
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if _, err := types.CreateField(t.Context(), built); err != nil {
		t.Fatalf("declaring the relation: %v, want nil", err)
	}
	return store, author, pool
}

// storedCategory stores one category item and returns it.
func storedCategory(t *testing.T, store *postgres.ContentStore, title string, author uuid.UUID) content.Content {
	t.Helper()
	built, err := content.New(categoryType(), nil, title, author)
	if err != nil {
		t.Fatalf("New(%q) error = %v, want nil", title, err)
	}
	created, err := store.Create(t.Context(), built)
	if err != nil {
		t.Fatalf("Create(%q) error = %v, want nil", title, err)
	}
	return created
}

// fileUnder points the post's categories at the targets and stores it.
func fileUnder(
	t *testing.T, store *postgres.ContentStore, post content.Content, targets ...uuid.UUID,
) content.Content {
	t.Helper()
	version := post.UpdatedAt
	post.Relations = content.Relations{"categories": targets}
	post.UpdatedAt = time.Now().UTC()
	updated, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("filing the post: %v, want nil", err)
	}
	return updated
}

// visibilityOf reads the denormalized pair the term page scans.
func visibilityOf(t *testing.T, pool *pgxpool.Pool, fromID uuid.UUID) (bool, time.Time) {
	t.Helper()
	var visible bool
	var sortAt time.Time
	err := pool.QueryRow(
		t.Context(), `SELECT visible, sort_at FROM core.content_relations WHERE from_id = $1`, fromID,
	).Scan(&visible, &sortAt)
	if err != nil {
		t.Fatalf("reading the relation row: %v, want nil", err)
	}
	return visible, sortAt
}

func TestContentStoreCarriesRelations(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	guides := storedCategory(t, store, "Guides", author)
	post := mustCreate(t, store, "Hello world", author)

	updated := fileUnder(t, store, post, guides.ID, news.ID)

	held := updated.Relations["categories"]
	if len(held) != 2 || held[0] != guides.ID || held[1] != news.ID {
		t.Fatalf("Update() relations = %v, want both targets in the order given", held)
	}
	read, err := store.ByID(t.Context(), post.ID)
	if err != nil {
		t.Fatalf("ByID() error = %v, want nil", err)
	}
	stored := read.Relations["categories"]
	if len(stored) != 2 || stored[0] != guides.ID || stored[1] != news.ID {
		t.Errorf("ByID() relations = %v, want the author's order kept", stored)
	}
}

func TestContentStoreReplacesTheTargetsItHeld(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	guides := storedCategory(t, store, "Guides", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)

	updated := fileUnder(t, store, post, guides.ID)

	held := updated.Relations["categories"]
	if len(held) != 1 || held[0] != guides.ID {
		t.Errorf("Update() relations = %v, want the targets replaced", held)
	}
}

func TestContentStoreClearsTheTargetsItHeld(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)

	updated := fileUnder(t, store, post)

	if len(updated.Relations["categories"]) != 0 {
		t.Errorf("Update() relations = %v, want the field cleared", updated.Relations)
	}
}

func TestContentStoreRefusesATargetOfTheWrongType(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	other := mustCreate(t, store, "Second post", author)
	post := mustCreate(t, store, "Hello world", author)
	post.Relations = content.Relations{"categories": {other.ID}}
	post.UpdatedAt = time.Now().UTC()

	_, err := store.Update(t.Context(), post, post.CreatedAt, nil, 0)

	if !errors.Is(err, content.ErrTargetType) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrTargetType)
	}
}

func TestContentStoreRefusesATargetNothingHolds(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	post := mustCreate(t, store, "Hello world", author)
	post.Relations = content.Relations{"categories": {uuid.Must(uuid.NewV7())}}
	post.UpdatedAt = time.Now().UTC()

	_, err := store.Update(t.Context(), post, post.CreatedAt, nil, 0)

	if !errors.Is(err, content.ErrTargetNotFound) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrTargetNotFound)
	}
}

func TestContentStoreHidesTheRelationsOfADraft(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)

	visible, sortAt := visibilityOf(t, pool, post.ID)

	if visible {
		t.Errorf("the relation of a draft is visible, want it hidden until the item is published")
	}
	if !sortAt.Equal(post.CreatedAt) {
		t.Errorf("sort_at = %v, want the unpublished item's created stamp %v", sortAt, post.CreatedAt)
	}
}

func TestContentStoreShowsTheRelationsOfAPublishedItem(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)
	version := post.UpdatedAt
	if err := post.Transition(content.StatusPublished); err != nil {
		t.Fatalf("Transition() error = %v, want nil", err)
	}

	published, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("publishing: %v, want nil", err)
	}

	visible, sortAt := visibilityOf(t, pool, post.ID)
	if !visible {
		t.Errorf("the relation of a published item is hidden, want it on the term page")
	}
	if !sortAt.Equal(*published.PublishedAt) {
		t.Errorf("sort_at = %v, want the published stamp %v", sortAt, *published.PublishedAt)
	}
}

func TestContentStoreHidesTheRelationsOfATrashedItem(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)
	version := post.UpdatedAt
	if err := post.Transition(content.StatusPublished); err != nil {
		t.Fatalf("Transition() error = %v, want nil", err)
	}
	if _, err := store.Update(t.Context(), post, version, nil, 0); err != nil {
		t.Fatalf("publishing: %v, want nil", err)
	}

	if _, err := store.Trash(t.Context(), post.ID, time.Now().UTC()); err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	if visible, _ := visibilityOf(t, pool, post.ID); visible {
		t.Errorf("the relation of a trashed item is visible, want it off the term page")
	}
}

func TestContentStoreHidesTheRelationsOfARestoredDraft(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)
	trashed, err := store.Trash(t.Context(), post.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if _, err := pool.Exec(
		t.Context(), `UPDATE core.content_relations SET visible = true WHERE from_id = $1`, post.ID,
	); err != nil {
		t.Fatalf("planting a visible relation: %v, want nil", err)
	}
	if visible, _ := visibilityOf(t, pool, post.ID); !visible {
		t.Fatal("the planted relation is hidden, want the test to start from the state the restore must clear")
	}

	if _, err := store.Restore(t.Context(), post.ID, trashed.UpdatedAt); err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}

	if visible, _ := visibilityOf(t, pool, post.ID); visible {
		t.Errorf("the relation of a restored draft is visible, want the restore to have hidden it")
	}
}

func TestContentStoreAnswersTrashWithItsRelations(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)

	trashed, err := store.Trash(t.Context(), post.ID, time.Now().UTC())

	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}
	if held := trashed.Relations["categories"]; len(held) != 1 || held[0] != news.ID {
		t.Errorf("Trash() relations = %v, want the targets the item still holds", trashed.Relations)
	}
}

func TestContentStoreAnswersRestoreWithItsRelations(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)
	trashed, err := store.Trash(t.Context(), post.ID, time.Now().UTC())
	if err != nil {
		t.Fatalf("Trash() error = %v, want nil", err)
	}

	restored, err := store.Restore(t.Context(), post.ID, trashed.UpdatedAt)

	if err != nil {
		t.Fatalf("Restore() error = %v, want nil", err)
	}
	if held := restored.Relations["categories"]; len(held) != 1 || held[0] != news.ID {
		t.Errorf("Restore() relations = %v, want the targets the item still holds", restored.Relations)
	}
}

func TestContentStoreRefusesATargetDeletedMidWrite(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	news := storedCategory(t, store, "News", author)
	post := mustCreate(t, store, "Hello world", author)
	post.Relations = content.Relations{"categories": {news.ID}}
	post.UpdatedAt = time.Now().UTC()
	if _, err := pool.Exec(t.Context(), `DELETE FROM core.content WHERE id = $1`, news.ID); err != nil {
		t.Fatalf("removing the target: %v, want nil", err)
	}

	_, err := store.Update(t.Context(), post, post.CreatedAt, nil, 0)

	if !errors.Is(err, content.ErrTargetNotFound) {
		t.Fatalf("Update() error = %v, want %v", err, content.ErrTargetNotFound)
	}
}

// publishItem takes stored content public and returns it.
func publishItem(t *testing.T, store *postgres.ContentStore, c content.Content) content.Content {
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

func TestContentStoreListsWhatPointsAtATerm(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	first := publishItem(t, store, fileUnder(t, store, mustCreate(t, store, "First", author), news.ID))
	second := publishItem(t, store, fileUnder(t, store, mustCreate(t, store, "Second", author), news.ID))

	items, total, err := store.RelatedTo(t.Context(), news.ID, 1, 20)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("RelatedTo() = %d items of %d, want both pointers", len(items), total)
	}
	if items[0].ID != second.ID || items[1].ID != first.ID {
		t.Errorf("RelatedTo() = %q then %q, want the newest first", items[0].Title, items[1].Title)
	}
}

func TestContentStoreLeavesADraftOffATerm(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	fileUnder(t, store, mustCreate(t, store, "Not yet", author), news.ID)

	items, total, err := store.RelatedTo(t.Context(), news.ID, 1, 20)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("RelatedTo() = %d items of %d, want a draft left off the term page", len(items), total)
	}
}

func TestContentStoreListsAnItemFiledTwiceOnce(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	types := postgres.NewTypeStore(pool)
	series, err := content.NewField(content.Field{
		TypeKey: "post", Key: "series", Label: "Series",
		Kind: content.FieldKindRelation, RelatesTo: "category",
	})
	if err != nil {
		t.Fatalf("NewField() error = %v, want nil", err)
	}
	if _, err := types.CreateField(t.Context(), series); err != nil {
		t.Fatalf("declaring the second relation: %v, want nil", err)
	}
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	post := fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID)
	version := post.UpdatedAt
	post.Relations = content.Relations{"categories": {news.ID}, "series": {news.ID}}
	post.UpdatedAt = time.Now().UTC()
	filed, err := store.Update(t.Context(), post, version, nil, 0)
	if err != nil {
		t.Fatalf("filing through both fields: %v, want nil", err)
	}
	publishItem(t, store, filed)

	items, total, err := store.RelatedTo(t.Context(), news.ID, 1, 20)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 1 || len(items) != 1 {
		t.Errorf("RelatedTo() = %d items of %d, want an item filed twice listed once", len(items), total)
	}
}

func TestContentStoreLeavesAnInactiveTypeOffATerm(t *testing.T) {
	t.Parallel()

	store, author, pool := relatingStore(t)
	types := postgres.NewTypeStore(pool)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	publishItem(t, store, fileUnder(t, store, mustCreate(t, store, "Hello world", author), news.ID))
	closed := postType()
	closed.Active, closed.Default, closed.RouteWord = false, false, "posts"
	closed.UpdatedAt = time.Now().UTC()
	if _, err := types.Update(t.Context(), closed); err != nil {
		t.Fatalf("closing the post type: %v, want nil", err)
	}

	items, total, err := store.RelatedTo(t.Context(), news.ID, 1, 20)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 0 || len(items) != 0 {
		t.Errorf("RelatedTo() = %d items of %d, want a closed type off the term page", len(items), total)
	}
}

func TestContentStorePagesATerm(t *testing.T) {
	t.Parallel()

	store, author, _ := relatingStore(t)
	news := publishItem(t, store, storedCategory(t, store, "News", author))
	oldest := publishItem(t, store, fileUnder(t, store, mustCreate(t, store, "Oldest", author), news.ID))
	publishItem(t, store, fileUnder(t, store, mustCreate(t, store, "Newest", author), news.ID))

	items, total, err := store.RelatedTo(t.Context(), news.ID, 2, 1)

	if err != nil {
		t.Fatalf("RelatedTo() error = %v, want nil", err)
	}
	if total != 2 || len(items) != 1 || items[0].ID != oldest.ID {
		t.Errorf("RelatedTo() page 2 = %d items of %d, want the older pointer alone", len(items), total)
	}
}
